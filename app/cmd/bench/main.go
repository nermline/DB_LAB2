// Command bench compares the same operations on PostgreSQL (normalized tables
// and jsonb) and MongoDB (requirement 5).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lab2/internal/bench"
	"lab2/internal/model"
	"lab2/internal/mongodb"
	"lab2/internal/pg"
)

type enrollmentRef struct{ StudentID, EnrollmentID int64 }

type meta struct {
	Students           int64
	Groups             []string
	ClosedSemesters    []int64
	CurrentSemester    int64
	Courses            []int64
	CurrentEnrollments []enrollmentRef
}

type report struct {
	Environment  map[string]string `json:"environment"`
	Dataset      map[string]int64  `json:"dataset"`
	Verification []string          `json:"verification"`
	Storage      map[string]int64  `json:"storage_bytes"`
	Results      []bench.Result    `json:"results"`
}

func main() {
	pgDSN := flag.String("pg", env("PG_DSN", "postgres://localhost:5432/university_lab2"), "PostgreSQL DSN")
	mongoURI := flag.String("mongo", env("MONGO_URI", "mongodb://localhost:27017"), "MongoDB URI")
	mongoDB := flag.String("mongo-db", "university_lab2", "MongoDB database")
	n := flag.Int("n", 2000, "measured calls of the student record test")
	warmup := flag.Int("warmup", 200, "warmup calls before every sequential test")
	duration := flag.Duration("duration", 5*time.Second, "duration of every concurrent run")
	workersFlag := flag.String("workers", "1,8,32,64", "concurrency levels")
	seed := flag.Uint64("seed", 7, "random seed of the request sequence")
	out := flag.String("out", "../results", "results directory")
	flag.Parse()

	ctx := context.Background()
	pgs, err := pg.Open(ctx, *pgDSN, 80)
	must(err)
	defer pgs.Pool.Close()
	ms, err := mongodb.Open(ctx, *mongoURI, *mongoDB, 80)
	must(err)
	defer ms.Close(ctx)

	log.Println("waiting until no vacuum is running")
	must(pgs.WaitNoVacuum(ctx))
	m := loadMeta(ctx, pgs)
	rep := report{Environment: environment(ctx, pgs, ms), Dataset: dataset(ctx, pgs), Storage: map[string]int64{}}
	r := rand.New(rand.NewPCG(*seed, 1))
	randomStudent := func(r *rand.Rand) int64 { return 1 + r.Int64N(m.Students) }

	log.Println("verification")
	rep.Verification = verify(ctx, pgs, ms, m, r)
	for _, v := range rep.Verification {
		log.Println("  " + v)
	}

	add := func(res bench.Result, err error) {
		must(err)
		rep.Results = append(rep.Results, res)
		log.Printf("%-22s %-26s w=%-2d p50=%-9s p95=%-9s ops/s=%.0f %s", res.Test, res.Variant, res.Workers,
			ms3(res.P50), ms3(res.P95), res.Throughput, res.Note)
	}

	// B1. Student portal: read the whole record of a student.
	ids := make([]int64, *n+*warmup)
	for i := range ids {
		ids[i] = randomStudent(r)
	}
	recordVariants := []struct {
		name string
		f    func(context.Context, int64) (model.StudentRecord, error)
	}{
		{"PostgreSQL tables (JOIN)", pgs.RecordJoin},
		{"PostgreSQL tables (json_agg)", pgs.RecordJSON},
		{"PostgreSQL jsonb", pgs.RecordJSONB},
		{"MongoDB", ms.Record},
	}
	for _, v := range recordVariants {
		add(bench.Sequential("B1 student record", v.name, *warmup, *n, func(i int) error {
			_, err := v.f(ctx, ids[i])
			return err
		}))
	}

	// B2. Dean's office: students of a group with academic debts.
	groups := make([]string, 600)
	for i := range groups {
		groups[i] = m.Groups[r.IntN(len(m.Groups))]
	}
	debtorVariants := []struct {
		name string
		f    func(context.Context, string) ([]model.Debtor, error)
	}{
		{"PostgreSQL tables", pgs.GroupDebtors},
		{"MongoDB", ms.GroupDebtors},
	}
	for _, v := range debtorVariants {
		add(bench.Sequential("B2 group debtors", v.name, 100, 500, func(i int) error {
			_, err := v.f(ctx, groups[i])
			return err
		}))
	}

	// B3. Analytics: average score and pass rate of every course in a semester.
	statVariants := []struct {
		name string
		f    func(context.Context, int64) ([]model.CourseStat, error)
	}{
		{"PostgreSQL tables", pgs.CourseStats},
		{"MongoDB", ms.CourseStats},
	}
	for _, v := range statVariants {
		add(bench.Sequential("B3 course statistics", v.name, 2, 16, func(i int) error {
			_, err := v.f(ctx, m.ClosedSemesters[i%len(m.ClosedSemesters)])
			return err
		}))
	}

	// B4. Teacher adds a grade (durable write: commit / journaled write).
	refs := make([]enrollmentRef, 600)
	for i := range refs {
		refs[i] = m.CurrentEnrollments[r.IntN(len(m.CurrentEnrollments))]
	}
	grade := model.GradeRecord{Type: "bonus", Title: benchmarkTitle, Score: 1, MaxScore: 5,
		GradedOn: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), TeacherID: 1, Teacher: "benchmark"}
	pgAsync, err := pg.Open(ctx, *pgDSN, 4, "synchronous_commit", "off")
	must(err)
	defer pgAsync.Pool.Close()
	writeModes := []struct {
		test string
		pg   *pg.Store
		mg   *mongodb.Store
		note string
	}{
		{"B4a add grade (default durability)", pgs, ms, ""},
		{"B4b add grade (async commit in both)", pgAsync, ms.Unjournaled(), ""},
	}
	for _, mode := range writeModes {
		pgNote, mgNote := "synchronous_commit=on, wal_sync_method="+rep.Environment["postgresql_wal_sync_method"], "j: true (F_FULLFSYNC of the journal)"
		if mode.pg == pgAsync {
			pgNote, mgNote = "synchronous_commit=off", "w: 1, j: false"
		}
		res, err := bench.Sequential(mode.test, "PostgreSQL tables", 100, 500, func(i int) error {
			_, err := mode.pg.AddGrade(ctx, refs[i].EnrollmentID, grade)
			return err
		})
		res.Note = pgNote
		add(res, err)
		res, err = bench.Sequential(mode.test, "MongoDB", 100, 500, func(i int) error {
			g := grade
			g.GradeID = int64(-1 - i)
			return mode.mg.AddGrade(ctx, refs[i].StudentID, m.CurrentSemester, refs[i].EnrollmentID, g)
		})
		res.Note = mgNote
		add(res, err)
		must(pgs.DeleteBenchmarkGrades(ctx, benchmarkTitle))
		touched := make([]int64, len(refs))
		for i, ref := range refs {
			touched[i] = ref.StudentID
		}
		must(ms.DeleteBenchmarkGrades(ctx, touched, benchmarkTitle))
	}

	// B5. Reference data change: rename a course (normalized: 1 row, denormalized: every document).
	courses := r.Perm(len(m.Courses))[:32]
	names := map[int64]string{}
	for _, idx := range courses {
		id := m.Courses[idx]
		names[id] = courseName(ctx, pgs, id)
	}
	var modified int64
	for _, v := range []struct {
		name string
		f    func(context.Context, int64, string) (int64, error)
	}{{"PostgreSQL tables", pgs.RenameCourse}, {"MongoDB", ms.RenameCourse}} {
		modified = 0
		res, err := bench.Sequential("B5 rename course", v.name, 2, 30, func(i int) error {
			id := m.Courses[courses[i]]
			cnt, err := v.f(ctx, id, names[id]+" (renamed)")
			if i < 30 {
				modified += cnt
			}
			return err
		})
		res.Note = fmt.Sprintf("rows/documents changed per call: %d", modified/30)
		add(res, err)
	}
	for id, name := range names {
		_, err := pgs.RenameCourse(ctx, id, name)
		must(err)
		_, err = ms.RenameCourse(ctx, id, name)
		must(err)
	}

	// B6. Exam session peak: many students open their records at the same time.
	for _, w := range parseInts(*workersFlag) {
		for _, v := range recordVariants {
			rs := make([]*rand.Rand, w)
			for i := range rs {
				rs[i] = rand.New(rand.NewPCG(*seed, uint64(100+i)))
			}
			add(bench.Concurrent("B6 concurrent record", v.name, w, *duration, func(worker, _ int) error {
				_, err := v.f(ctx, randomStudent(rs[worker]))
				return err
			}))
		}
	}

	// Storage.
	tables := []string{"faculties", "departments", "specialties", "teachers", "student_groups",
		"students", "semesters", "courses", "enrollments", "grades"}
	size, err := pgs.Size(ctx, tables...)
	must(err)
	rep.Storage["postgresql_tables_total"] = size
	size, err = pgs.Size(ctx, "student_records")
	must(err)
	rep.Storage["postgresql_jsonb_total"] = size
	storage, indexes, data, err := ms.Size(ctx)
	must(err)
	rep.Storage["mongodb_storage"] = storage
	rep.Storage["mongodb_indexes"] = indexes
	rep.Storage["mongodb_total"] = storage + indexes
	rep.Storage["mongodb_uncompressed_bson"] = data

	must(os.MkdirAll(*out, 0o755))
	f, err := os.Create(filepath.Join(*out, "results.json"))
	must(err)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	must(enc.Encode(rep))
	f.Close()
	must(os.WriteFile(filepath.Join(*out, "results.md"), []byte(markdown(rep, *out)), 0o644))
	log.Printf("results written to %s", *out)
}

const benchmarkTitle = "Benchmark bonus"

func loadMeta(ctx context.Context, s *pg.Store) meta {
	var m meta
	must(s.Pool.QueryRow(ctx, `SELECT max(id) FROM students`).Scan(&m.Students))
	rows, err := s.Pool.Query(ctx, `SELECT code FROM student_groups ORDER BY id`)
	must(err)
	for rows.Next() {
		var c string
		must(rows.Scan(&c))
		m.Groups = append(m.Groups, c)
	}
	must(rows.Err())
	must(s.Pool.QueryRow(ctx, `SELECT max(id) FROM semesters`).Scan(&m.CurrentSemester))
	for id := int64(1); id < m.CurrentSemester; id++ {
		m.ClosedSemesters = append(m.ClosedSemesters, id)
	}
	rows, err = s.Pool.Query(ctx, `SELECT id FROM courses ORDER BY id`)
	must(err)
	for rows.Next() {
		var id int64
		must(rows.Scan(&id))
		m.Courses = append(m.Courses, id)
	}
	must(rows.Err())
	rows, err = s.Pool.Query(ctx, `SELECT student_id, id FROM enrollments WHERE semester_id = $1 ORDER BY id`, m.CurrentSemester)
	must(err)
	for rows.Next() {
		var e enrollmentRef
		must(rows.Scan(&e.StudentID, &e.EnrollmentID))
		m.CurrentEnrollments = append(m.CurrentEnrollments, e)
	}
	must(rows.Err())
	return m
}

func courseName(ctx context.Context, s *pg.Store, id int64) string {
	var name string
	must(s.Pool.QueryRow(ctx, `SELECT name FROM courses WHERE id = $1`, id).Scan(&name))
	return name
}

// verify checks that all variants return the same data before anything is measured.
func verify(ctx context.Context, pgs *pg.Store, ms *mongodb.Store, m meta, r *rand.Rand) []string {
	const samples = 200
	for i := 0; i < samples; i++ {
		id := 1 + r.Int64N(m.Students)
		a, err := pgs.RecordJoin(ctx, id)
		must(err)
		b, err := pgs.RecordJSON(ctx, id)
		must(err)
		c, err := pgs.RecordJSONB(ctx, id)
		must(err)
		d, err := ms.Record(ctx, id)
		must(err)
		for name, x := range map[string]model.StudentRecord{"json_agg": b, "jsonb": c, "MongoDB": d} {
			if !reflect.DeepEqual(normalize(a), normalize(x)) {
				log.Fatalf("student %d: %s differs from the normalized tables", id, name)
			}
		}
	}
	var out []string
	out = append(out, fmt.Sprintf("student record: %d random students, identical in all 4 variants", samples))

	for i := 0; i < 50; i++ {
		g := m.Groups[r.IntN(len(m.Groups))]
		a, err := pgs.GroupDebtors(ctx, g)
		must(err)
		b, err := ms.GroupDebtors(ctx, g)
		must(err)
		if !reflect.DeepEqual(a, b) && !(len(a) == 0 && len(b) == 0) {
			log.Fatalf("group %s: debtors differ", g)
		}
	}
	out = append(out, "group debtors: 50 random groups, identical in PostgreSQL and MongoDB")

	for _, sem := range m.ClosedSemesters {
		a, err := pgs.CourseStats(ctx, sem)
		must(err)
		b, err := ms.CourseStats(ctx, sem)
		must(err)
		if len(a) != len(b) {
			log.Fatalf("semester %d: course statistics differ", sem)
		}
		for i := range a {
			if a[i].Code != b[i].Code || a[i].Name != b[i].Name || a[i].Students != b[i].Students ||
				math.Abs(a[i].AvgScore-b[i].AvgScore) > 1e-9 || math.Abs(a[i].PassRate-b[i].PassRate) > 1e-9 {
				log.Fatalf("semester %d course %s: statistics differ", sem, a[i].Code)
			}
		}
	}
	out = append(out, fmt.Sprintf("course statistics: all %d closed semesters, identical in PostgreSQL and MongoDB", len(m.ClosedSemesters)))
	return out
}

func normalize(r model.StudentRecord) model.StudentRecord {
	if r.Semesters == nil {
		r.Semesters = []model.SemesterRecord{}
	}
	return r
}

func environment(ctx context.Context, pgs *pg.Store, ms *mongodb.Store) map[string]string {
	e := map[string]string{
		"go":   runtime.Version(),
		"os":   runtime.GOOS + "/" + runtime.GOARCH,
		"cpus": strconv.Itoa(runtime.NumCPU()),
		"date": time.Now().Format("2006-01-02 15:04"),
	}
	var v, sb, sc string
	var ws string
	must(pgs.Pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_setting('shared_buffers'),
		current_setting('synchronous_commit'), current_setting('wal_sync_method')`).Scan(&v, &sb, &sc, &ws))
	e["postgresql"] = v
	e["postgresql_shared_buffers"] = sb
	e["postgresql_synchronous_commit"] = sc
	e["postgresql_wal_sync_method"] = ws
	var st struct {
		Version    string `bson:"version"`
		WiredTiger struct {
			Cache struct {
				Max float64 `bson:"maximum bytes configured"`
			} `bson:"cache"`
		} `bson:"wiredTiger"`
	}
	must(ms.DB.RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&st))
	e["mongodb"] = st.Version
	e["mongodb_wiredtiger_cache"] = fmt.Sprintf("%.1f GB", st.WiredTiger.Cache.Max/1e9)
	e["mongodb_write_concern"] = "j: true (journaled)"
	return e
}

func dataset(ctx context.Context, pgs *pg.Store) map[string]int64 {
	out := map[string]int64{}
	for _, t := range []string{"faculties", "departments", "teachers", "courses", "student_groups", "students", "enrollments", "grades"} {
		var c int64
		must(pgs.Pool.QueryRow(ctx, `SELECT count(*) FROM `+t).Scan(&c))
		out[t] = c
	}
	return out
}

func markdown(rep report, dir string) string {
	var b strings.Builder
	b.WriteString("# Benchmark results\n\n")
	fmt.Fprintf(&b, "Generated %s on %s, %s CPUs. PostgreSQL %s (shared_buffers %s, synchronous_commit %s), MongoDB %s (WiredTiger cache %s, write concern %s).\n\n",
		rep.Environment["date"], rep.Environment["os"], rep.Environment["cpus"], rep.Environment["postgresql"],
		rep.Environment["postgresql_shared_buffers"], rep.Environment["postgresql_synchronous_commit"],
		rep.Environment["mongodb"], rep.Environment["mongodb_wiredtiger_cache"], rep.Environment["mongodb_write_concern"])
	b.WriteString("## Dataset\n\n| Table | Rows |\n|---|---:|\n")
	for _, t := range []string{"faculties", "departments", "teachers", "courses", "student_groups", "students", "enrollments", "grades"} {
		fmt.Fprintf(&b, "| %s | %d |\n", t, rep.Dataset[t])
	}
	b.WriteString("\n## Verification\n\n")
	for _, v := range rep.Verification {
		b.WriteString("- " + v + "\n")
	}
	if raw, err := os.ReadFile(filepath.Join(dir, "load.json")); err == nil {
		var l map[string]any
		if json.Unmarshal(raw, &l) == nil {
			b.WriteString("\n## Load time\n\n| Target | Data | Keys and indexes |\n|---|---:|---:|\n")
			fmt.Fprintf(&b, "| PostgreSQL tables (COPY) | %s | %s |\n", sec(l["PGTables"]), sec(l["PGTablesIndexes"]))
			fmt.Fprintf(&b, "| PostgreSQL jsonb (COPY) | %s | - |\n", sec(l["PGJSONB"]))
			fmt.Fprintf(&b, "| MongoDB (insertMany) | %s | %s |\n", sec(l["Mongo"]), sec(l["MongoIndexes"]))
		}
	}
	b.WriteString("\n## Storage\n\n| Storage | Size |\n|---|---:|\n")
	for _, k := range []string{"postgresql_tables_total", "postgresql_jsonb_total", "mongodb_storage", "mongodb_indexes", "mongodb_total", "mongodb_uncompressed_bson"} {
		fmt.Fprintf(&b, "| %s | %.0f MB |\n", k, float64(rep.Storage[k])/1024/1024)
	}
	test := ""
	for _, r := range rep.Results {
		if r.Test != test {
			test = r.Test
			fmt.Fprintf(&b, "\n## %s\n\n| Variant | Workers | Calls | p50, ms | p95, ms | p99, ms | mean, ms | ops/s | Note |\n|---|---:|---:|---:|---:|---:|---:|---:|---|\n", test)
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %s | %s | %s | %.0f | %s |\n", r.Variant, r.Workers, r.Ops,
			ms3(r.P50), ms3(r.P95), ms3(r.P99), ms3(r.Mean), r.Throughput, r.Note)
	}
	return b.String()
}

func sec(v any) string {
	if f, ok := v.(float64); ok {
		return fmt.Sprintf("%.1f s", f/1e9)
	}
	return "-"
}

func ms3(d time.Duration) string { return fmt.Sprintf("%.3f", float64(d)/1e6) }

func parseInts(s string) []int {
	var out []int
	for _, p := range strings.Split(s, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		must(err)
		out = append(out, v)
	}
	return out
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
