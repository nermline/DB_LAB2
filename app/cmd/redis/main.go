// Command redis shows Redis (requirement 6) on the real dataset: a cache of the
// student record (cache-aside with TTL) and a group ranking in a sorted set.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"

	"lab2/internal/mongodb"
	"lab2/internal/pg"
)

const ttl = 5 * time.Minute

func main() {
	pgDSN := flag.String("pg", env("PG_DSN", "postgres://localhost:5432/university_lab2"), "PostgreSQL DSN")
	mongoURI := flag.String("mongo", env("MONGO_URI", "mongodb://localhost:27017"), "MongoDB URI")
	redisAddr := flag.String("redis", env("REDIS_ADDR", "localhost:6379"), "Redis address")
	n := flag.Int("n", 1000, "number of record reads")
	flag.Parse()

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{Addr: *redisAddr})
	defer rdb.Close()
	must(rdb.Ping(ctx).Err())
	pgs, err := pg.Open(ctx, *pgDSN, 4)
	must(err)
	defer pgs.Pool.Close()
	ms, err := mongodb.Open(ctx, *mongoURI, "university_lab2", 4)
	must(err)
	defer ms.Close(ctx)

	// 1. Cache-aside: GET from Redis, on a miss read MongoDB and SET with a TTL.
	var students int64
	must(pgs.Pool.QueryRow(ctx, `SELECT max(id) FROM students`).Scan(&students))
	r := rand.New(rand.NewPCG(11, 1))
	hot := make([]int64, 200)
	for i := range hot {
		hot[i] = 1 + r.Int64N(students)
	}
	keys := make([]string, len(hot))
	for i, id := range hot {
		keys[i] = recordKey(id)
	}
	must(rdb.Del(ctx, keys...).Err())

	// The portal API answers GET /students/{id}/record with JSON. Without a cache the document is read
	// from MongoDB and encoded to JSON; the cache keeps the ready JSON response.
	var direct, miss, hit []time.Duration
	for i := 0; i < *n; i++ {
		id := hot[i%len(hot)]
		start := time.Now()
		rec, err := ms.Record(ctx, id)
		must(err)
		_, err = json.Marshal(rec)
		must(err)
		direct = append(direct, time.Since(start))
	}
	for i := 0; i < *n; i++ {
		id := hot[i%len(hot)]
		start := time.Now()
		_, wasHit, err := cachedResponse(ctx, rdb, ms, id)
		must(err)
		if wasHit {
			hit = append(hit, time.Since(start))
		} else {
			miss = append(miss, time.Since(start))
		}
	}
	ttlLeft, err := rdb.TTL(ctx, recordKey(hot[0])).Result()
	must(err)
	fmt.Println("== Cache-aside of the JSON response GET /students/{id}/record (200 hot students, MongoDB behind the cache)")
	fmt.Printf("%-44s %6s %10s %10s\n", "path", "calls", "p50, ms", "p95, ms")
	for _, row := range []struct {
		name string
		lat  []time.Duration
	}{{"MongoDB find + JSON encoding (no cache)", direct}, {"cache miss (GET + MongoDB + JSON + SET)", miss}, {"cache hit (GET, JSON returned as is)", hit}} {
		fmt.Printf("%-44s %6d %10.3f %10.3f\n", row.name, len(row.lat), pct(row.lat, 0.5), pct(row.lat, 0.95))
	}
	fmt.Printf("hit ratio %.0f%%, TTL of a cached response: %v\n\n", 100*float64(len(hit))/float64(*n), ttlLeft.Round(time.Second))

	// 2. Group ranking: ZADD scores from PostgreSQL, then read the leaderboard from Redis.
	group, semester := "SE-24-01", int64(8)
	rows, err := pgs.Pool.Query(ctx, `
		SELECT s.last_name || ' ' || s.first_name || ' #' || s.id,
		       round(sum(e.total_score * c.credits) / sum(c.credits), 2)::float8,
		       rank() OVER (ORDER BY round(sum(e.total_score * c.credits) / sum(c.credits), 2) DESC)
		FROM students s
		JOIN student_groups g ON g.id = s.group_id
		JOIN enrollments e    ON e.student_id = s.id AND e.semester_id = $2
		JOIN courses c        ON c.id = e.course_id
		WHERE g.code = $1
		GROUP BY s.id
		ORDER BY 3, 1`, group, semester)
	must(err)
	type entry struct {
		name  string
		score float64
		rank  int64
	}
	var sqlRanking []entry
	for rows.Next() {
		var e entry
		must(rows.Scan(&e.name, &e.score, &e.rank))
		sqlRanking = append(sqlRanking, e)
	}
	must(rows.Err())

	key := fmt.Sprintf("uni:rating:%s:%d", group, semester)
	must(rdb.Del(ctx, key).Err())
	members := make([]redis.Z, len(sqlRanking))
	for i, e := range sqlRanking {
		members[i] = redis.Z{Score: e.score, Member: e.name}
	}
	must(rdb.ZAdd(ctx, key, members...).Err())

	top, err := rdb.ZRevRangeWithScores(ctx, key, 0, 4).Result()
	must(err)
	fmt.Printf("== Ranking of group %s, semester %d (ZADD %d members, ZREVRANGE 0 4 WITHSCORES)\n", group, semester, len(members))
	for i, z := range top {
		fmt.Printf("%d. %-40s %.2f\n", i+1, z.Member, z.Score)
	}
	last := sqlRanking[len(sqlRanking)-1].name
	rank, err := rdb.ZRevRank(ctx, key, last).Result()
	must(err)
	candidates, err := rdb.ZCount(ctx, key, "90", "+inf").Result()
	must(err)
	fmt.Printf("ZREVRANK of %q: %d (0-based), students with rating >= 90: %d\n", last, rank, candidates)

	same := true
	for i, z := range top {
		if sqlRanking[i].score != z.Score {
			same = false
		}
	}
	fmt.Printf("Redis top-5 scores match the SQL RANK() ordering: %v\n", same)
}

// cachedResponse returns the JSON of the student record: from Redis on a hit,
// from MongoDB (then stored in Redis with a TTL) on a miss.
func cachedResponse(ctx context.Context, rdb *redis.Client, ms *mongodb.Store, id int64) ([]byte, bool, error) {
	raw, err := rdb.Get(ctx, recordKey(id)).Bytes()
	if err == nil {
		return raw, true, nil
	}
	if !errors.Is(err, redis.Nil) {
		return nil, false, err
	}
	rec, err := ms.Record(ctx, id)
	if err != nil {
		return nil, false, err
	}
	if raw, err = json.Marshal(rec); err != nil {
		return nil, false, err
	}
	return raw, false, rdb.Set(ctx, recordKey(id), raw, ttl).Err()
}

func recordKey(id int64) string { return fmt.Sprintf("uni:record:%d", id) }

func pct(lat []time.Duration, p float64) float64 {
	if len(lat) == 0 {
		return 0
	}
	s := slices.Clone(lat)
	slices.Sort(s)
	return float64(s[min(len(s)-1, int(float64(len(s))*p))]) / 1e6
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
