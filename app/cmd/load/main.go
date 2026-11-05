// Command load generates the dataset and loads the same data into PostgreSQL
// (normalized tables + jsonb) and MongoDB (one document per student).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"lab2/internal/gen"
	"lab2/internal/mongodb"
	"lab2/internal/pg"
)

type loadResult struct {
	Students, Enrollments, Grades int
	PGTables, PGTablesIndexes     time.Duration
	PGJSONB                       time.Duration
	Mongo, MongoIndexes           time.Duration
}

func main() {
	pgDSN := flag.String("pg", env("PG_DSN", "postgres://localhost:5432/university_lab2"), "PostgreSQL DSN")
	mongoURI := flag.String("mongo", env("MONGO_URI", "mongodb://localhost:27017"), "MongoDB URI")
	mongoDB := flag.String("mongo-db", "university_lab2", "MongoDB database")
	students := flag.Int("students", 50000, "number of students")
	seed := flag.Uint64("seed", 42, "random seed")
	batch := flag.Int("batch", 1000, "students per batch")
	sqlDir := flag.String("sql", "../sql", "directory with schema.sql and constraints_indexes.sql")
	out := flag.String("out", "../results/load.json", "where to write the load statistics")
	flag.Parse()

	ctx := context.Background()
	must(pg.CreateDatabase(ctx, *pgDSN))
	must(pg.ExecFile(ctx, *pgDSN, filepath.Join(*sqlDir, "schema.sql")))
	pgs, err := pg.Open(ctx, *pgDSN, 4)
	must(err)
	defer pgs.Pool.Close()

	ms, err := mongodb.Open(ctx, *mongoURI, *mongoDB, 4)
	must(err)
	defer ms.Close(ctx)
	must(ms.Drop(ctx))

	d := gen.New(*seed, *students)
	must(pgs.LoadReference(ctx, d))
	log.Printf("reference data: %d faculties, %d departments, %d teachers, %d courses, %d groups, %d semesters",
		len(d.Faculties), len(d.Departments), len(d.Teachers), len(d.Courses), len(d.Groups), len(d.Semesters))

	var res loadResult
	for {
		st, en, gr := d.Batch(*batch)
		if len(st) == 0 {
			break
		}
		records := d.Records(st, en, gr)
		res.Students += len(st)
		res.Enrollments += len(en)
		res.Grades += len(gr)

		res.PGTables += timed(func() error { return pgs.LoadBatch(ctx, st, en, gr) })
		res.PGJSONB += timed(func() error { return pgs.LoadDocuments(ctx, records) })
		res.Mongo += timed(func() error { return ms.Insert(ctx, records) })
		if res.Students%10000 == 0 {
			log.Printf("%d students loaded", res.Students)
		}
	}
	res.PGTablesIndexes = timed(func() error {
		return pg.ExecFile(ctx, *pgDSN, filepath.Join(*sqlDir, "constraints_indexes.sql"))
	})
	res.MongoIndexes = timed(func() error { return ms.CreateIndexes(ctx) })
	log.Println("VACUUM (ANALYZE) in PostgreSQL, fsync in MongoDB")
	must(pgs.Settle(ctx))
	must(ms.Settle(ctx))

	fmt.Printf("students %d, enrollments %d, grades %d\n", res.Students, res.Enrollments, res.Grades)
	fmt.Printf("PostgreSQL tables: load %v, keys + indexes %v\n", res.PGTables.Round(time.Millisecond), res.PGTablesIndexes.Round(time.Millisecond))
	fmt.Printf("PostgreSQL jsonb:  load %v\n", res.PGJSONB.Round(time.Millisecond))
	fmt.Printf("MongoDB:           load %v, indexes %v\n", res.Mongo.Round(time.Millisecond), res.MongoIndexes.Round(time.Millisecond))

	must(os.MkdirAll(filepath.Dir(*out), 0o755))
	f, err := os.Create(*out)
	must(err)
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	must(enc.Encode(res))
}

func timed(f func() error) time.Duration {
	start := time.Now()
	must(f())
	return time.Since(start)
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
