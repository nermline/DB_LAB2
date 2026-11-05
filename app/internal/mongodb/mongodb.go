// Package mongodb stores the student record as one document (requirement 3).
package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"

	"lab2/internal/model"
)

const Collection = "student_records"

type Store struct {
	Client  *mongo.Client
	DB      *mongo.Database
	Records *mongo.Collection
}

// Open connects to MongoDB. Writes wait for the on-disk journal (j: true),
// the same durability as a committed PostgreSQL transaction.
func Open(ctx context.Context, uri, database string, maxPool uint64) (*Store, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri).SetMaxPoolSize(maxPool))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	db := client.Database(database)
	coll := db.Collection(Collection, options.Collection().SetWriteConcern(writeconcern.Journaled()))
	return &Store{Client: client, DB: db, Records: coll}, nil
}

func (s *Store) Close(ctx context.Context) error { return s.Client.Disconnect(ctx) }

// Unjournaled returns the same store whose writes are acknowledged without waiting
// for the journal flush (w: 1, j: false), like synchronous_commit = off in PostgreSQL.
func (s *Store) Unjournaled() *Store {
	c := *s
	c.Records = s.DB.Collection(Collection, options.Collection().SetWriteConcern(writeconcern.W1()))
	return &c
}

func (s *Store) Drop(ctx context.Context) error { return s.DB.Drop(ctx) }

// Insert stores a batch of documents.
func (s *Store) Insert(ctx context.Context, records []model.StudentRecord) error {
	docs := make([]any, len(records))
	for i := range records {
		docs[i] = records[i]
	}
	_, err := s.Records.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
	return err
}

// Settle flushes all dirty pages to disk (fsync command), so that a checkpoint
// does not start in the middle of the benchmark.
func (s *Store) Settle(ctx context.Context) error {
	return s.Client.Database("admin").RunCommand(ctx, bson.D{{Key: "fsync", Value: 1}}).Err()
}

// CreateIndexes creates the indexes used by the benchmark queries.
func (s *Store) CreateIndexes(ctx context.Context) error {
	_, err := s.Records.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "record_book_no", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "group.code", Value: 1}}},
		{Keys: bson.D{{Key: "semesters.semester_id", Value: 1}}},
		{Keys: bson.D{{Key: "semesters.courses.course_id", Value: 1}}},
	})
	return err
}

// Record reads the whole student record: one document by primary key.
func (s *Store) Record(ctx context.Context, id int64) (model.StudentRecord, error) {
	var rec model.StudentRecord
	err := s.Records.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&rec)
	return rec, err
}

// GroupDebtors returns the students of a group that have failed courses.
func (s *Store) GroupDebtors(ctx context.Context, groupCode string) ([]model.Debtor, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "group.code", Value: groupCode}, {Key: "semesters.courses.status", Value: "failed"}}}},
		{{Key: "$project", Value: bson.D{
			{Key: "last_name", Value: 1}, {Key: "first_name", Value: 1},
			{Key: "failed", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$map", Value: bson.D{
				{Key: "input", Value: "$semesters"}, {Key: "as", Value: "s"},
				{Key: "in", Value: bson.D{{Key: "$size", Value: bson.D{{Key: "$filter", Value: bson.D{
					{Key: "input", Value: "$$s.courses"}, {Key: "as", Value: "c"},
					{Key: "cond", Value: bson.D{{Key: "$eq", Value: bson.A{"$$c.status", "failed"}}}},
				}}}}}},
			}}}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	cur, err := s.Records.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID        int64  `bson:"_id"`
		LastName  string `bson:"last_name"`
		FirstName string `bson:"first_name"`
		Failed    int    `bson:"failed"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]model.Debtor, len(rows))
	for i, r := range rows {
		out[i] = model.Debtor{StudentID: r.ID, LastName: r.LastName, FirstName: r.FirstName, Failed: r.Failed}
	}
	return out, nil
}

// CourseStats returns the average score and the pass rate of every course in a semester.
func (s *Store) CourseStats(ctx context.Context, semesterID int64) ([]model.CourseStat, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "semesters.semester_id", Value: semesterID}}}},
		{{Key: "$unwind", Value: "$semesters"}},
		{{Key: "$match", Value: bson.D{{Key: "semesters.semester_id", Value: semesterID}}}},
		{{Key: "$unwind", Value: "$semesters.courses"}},
		{{Key: "$match", Value: bson.D{{Key: "semesters.courses.status", Value: bson.D{{Key: "$in", Value: bson.A{"completed", "failed"}}}}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$semesters.courses.code"},
			{Key: "name", Value: bson.D{{Key: "$first", Value: "$semesters.courses.name"}}},
			{Key: "students", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "avg", Value: bson.D{{Key: "$avg", Value: "$semesters.courses.total_score"}}},
			{Key: "passed", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$eq", Value: bson.A{"$semesters.courses.status", "completed"}}}, 1, 0}}}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	cur, err := s.Records.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		Code     string  `bson:"_id"`
		Name     string  `bson:"name"`
		Students int     `bson:"students"`
		Avg      float64 `bson:"avg"`
		Passed   int     `bson:"passed"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]model.CourseStat, len(rows))
	for i, r := range rows {
		out[i] = model.CourseStat{Code: r.Code, Name: r.Name, Students: r.Students, AvgScore: r.Avg,
			PassRate: 100 * float64(r.Passed) / float64(r.Students)}
	}
	return out, nil
}

// AddGrade appends a grade to an embedded course of the document.
func (s *Store) AddGrade(ctx context.Context, studentID, semesterID, enrollmentID int64, g model.GradeRecord) error {
	_, err := s.Records.UpdateOne(ctx,
		bson.D{{Key: "_id", Value: studentID}},
		bson.D{{Key: "$push", Value: bson.D{{Key: "semesters.$[s].courses.$[c].grades", Value: g}}}},
		options.UpdateOne().SetArrayFilters([]any{
			bson.D{{Key: "s.semester_id", Value: semesterID}},
			bson.D{{Key: "c.enrollment_id", Value: enrollmentID}},
		}))
	return err
}

// DeleteBenchmarkGrades removes the grades added by the benchmark.
func (s *Store) DeleteBenchmarkGrades(ctx context.Context, studentIDs []int64, title string) error {
	_, err := s.Records.UpdateMany(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: studentIDs}}}},
		bson.D{{Key: "$pull", Value: bson.D{{Key: "semesters.$[].courses.$[].grades", Value: bson.D{
			{Key: "title", Value: title}}}}}})
	return err
}

// RenameCourse changes the course name in every document that contains the course
// (the price of denormalization: the name is copied into each student record).
func (s *Store) RenameCourse(ctx context.Context, courseID int64, name string) (int64, error) {
	res, err := s.Records.UpdateMany(ctx,
		bson.D{{Key: "semesters.courses.course_id", Value: courseID}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "semesters.$[].courses.$[c].name", Value: name}}}},
		options.UpdateMany().SetArrayFilters([]any{bson.D{{Key: "c.course_id", Value: courseID}}}))
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// Size returns the collection size: compressed data on disk, indexes and uncompressed BSON.
func (s *Store) Size(ctx context.Context) (storage, indexes, dataSize int64, err error) {
	cur, err := s.Records.Aggregate(ctx, mongo.Pipeline{{{Key: "$collStats", Value: bson.D{{Key: "storageStats", Value: bson.D{}}}}}})
	if err != nil {
		return 0, 0, 0, err
	}
	var rows []struct {
		Stats struct {
			StorageSize    float64 `bson:"storageSize"`
			TotalIndexSize float64 `bson:"totalIndexSize"`
			Size           float64 `bson:"size"`
		} `bson:"storageStats"`
	}
	if err := cur.All(ctx, &rows); err != nil || len(rows) == 0 {
		return 0, 0, 0, err
	}
	st := rows[0].Stats
	return int64(st.StorageSize), int64(st.TotalIndexSize), int64(st.Size), nil
}
