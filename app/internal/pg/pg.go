// Package pg works with the normalized PostgreSQL schema (requirement 4)
// and with the jsonb copy of the document.
package pg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"lab2/internal/gen"
	"lab2/internal/model"
)

var enumTypes = []string{"degree_level", "teacher_position", "student_status", "funding_type",
	"control_form", "enrollment_status", "assessment_type"}

type Store struct {
	Pool *pgxpool.Pool
}

// Open connects to the database; enum types are registered so that COPY can send them.
// params are session settings of every connection (for example synchronous_commit).
func Open(ctx context.Context, dsn string, maxConns int32, params ...string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	for i := 0; i+1 < len(params); i += 2 {
		cfg.ConnConfig.RuntimeParams[params[i]] = params[i+1]
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		var exists bool
		if err := c.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'student_status')`).Scan(&exists); err != nil || !exists {
			return err
		}
		types, err := c.LoadTypes(ctx, enumTypes)
		if err != nil {
			return err
		}
		c.TypeMap().RegisterTypes(types)
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Store{Pool: pool}, pool.Ping(ctx)
}

// CreateDatabase creates the database if it does not exist (connects to the "postgres" database).
func CreateDatabase(ctx context.Context, dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	name := cfg.Database
	cfg.Database = "postgres"
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	}
	return err
}

// ExecFile runs a SQL script on a fresh connection.
func ExecFile(ctx context.Context, dsn, path string) error {
	sql, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, string(sql))
	return err
}

func (s *Store) copy(ctx context.Context, table string, cols []string, rows [][]any) error {
	_, err := s.Pool.CopyFrom(ctx, pgx.Identifier{table}, cols, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("copy %s: %w", table, err)
	}
	return nil
}

// LoadReference loads faculties, departments, specialties, teachers, groups, semesters and courses.
func (s *Store) LoadReference(ctx context.Context, d *gen.Dataset) error {
	var rows [][]any
	for _, f := range d.Faculties {
		rows = append(rows, []any{f.ID, f.Code, f.Name})
	}
	if err := s.copy(ctx, "faculties", []string{"id", "code", "name"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, x := range d.Departments {
		rows = append(rows, []any{x.ID, x.FacultyID, x.Code, x.Name})
	}
	if err := s.copy(ctx, "departments", []string{"id", "faculty_id", "code", "name"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, x := range d.Specialties {
		rows = append(rows, []any{x.ID, x.DepartmentID, x.Code, x.Name, "bachelor"})
	}
	if err := s.copy(ctx, "specialties", []string{"id", "department_id", "code", "name", "degree_level"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, t := range d.Teachers {
		rows = append(rows, []any{t.ID, t.DepartmentID, t.LastName, t.FirstName, t.MiddleName, t.Position, nullable(t.AcademicDegree)})
	}
	if err := s.copy(ctx, "teachers", []string{"id", "department_id", "last_name", "first_name", "middle_name", "position", "academic_degree"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, g := range d.Groups {
		rows = append(rows, []any{g.ID, g.SpecialtyID, g.Code, int16(g.AdmissionYear), g.CuratorID})
	}
	if err := s.copy(ctx, "student_groups", []string{"id", "specialty_id", "code", "admission_year", "curator_teacher_id"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, x := range d.Semesters {
		rows = append(rows, []any{x.ID, x.AcademicYear, int16(x.Term), x.Start, x.End})
	}
	if err := s.copy(ctx, "semesters", []string{"id", "academic_year", "term", "start_date", "end_date"}, rows); err != nil {
		return err
	}
	rows = nil
	for _, c := range d.Courses {
		rows = append(rows, []any{c.ID, c.DepartmentID, c.Code, c.Name, c.Credits, c.Control})
	}
	return s.copy(ctx, "courses", []string{"id", "department_id", "code", "name", "credits", "control"}, rows)
}

// LoadBatch loads students, enrollments and grades of one batch (relational model).
func (s *Store) LoadBatch(ctx context.Context, students []gen.Student, enrollments []gen.Enrollment, grades []gen.Grade) error {
	rows := make([][]any, 0, len(students))
	for _, x := range students {
		rows = append(rows, []any{x.ID, x.GroupID, x.RecordBookNo, x.LastName, x.FirstName, x.MiddleName,
			x.BirthDate, x.Email, x.Phone, x.EnrollmentDate, x.Status, x.Funding})
	}
	if err := s.copy(ctx, "students", []string{"id", "group_id", "record_book_no", "last_name", "first_name",
		"middle_name", "birth_date", "email", "phone", "enrollment_date", "status", "funding"}, rows); err != nil {
		return err
	}
	rows = make([][]any, 0, len(enrollments))
	for _, e := range enrollments {
		rows = append(rows, []any{e.ID, e.StudentID, e.CourseID, e.SemesterID, e.Status, e.TotalScore, e.ECTSGrade, e.NationalGrade})
	}
	if err := s.copy(ctx, "enrollments", []string{"id", "student_id", "course_id", "semester_id", "status",
		"total_score", "ects_grade", "national_grade"}, rows); err != nil {
		return err
	}
	rows = make([][]any, 0, len(grades))
	for _, g := range grades {
		rows = append(rows, []any{g.ID, g.EnrollmentID, g.Type, g.Title, g.Score, g.MaxScore, g.GradedOn, g.TeacherID})
	}
	return s.copy(ctx, "grades", []string{"id", "enrollment_id", "assessment_type", "title", "score", "max_score",
		"graded_on", "teacher_id"}, rows)
}

// LoadDocuments stores the documents in the jsonb table.
func (s *Store) LoadDocuments(ctx context.Context, records []model.StudentRecord) error {
	rows := make([][]any, 0, len(records))
	for _, r := range records {
		doc, err := json.Marshal(r)
		if err != nil {
			return err
		}
		rows = append(rows, []any{r.ID, doc})
	}
	return s.copy(ctx, "student_records", []string{"id", "doc"}, rows)
}

// Settle runs VACUUM (ANALYZE) after the bulk load and waits until no vacuum is running,
// so that autovacuum does not start in the middle of the benchmark.
func (s *Store) Settle(ctx context.Context) error {
	if _, err := s.Pool.Exec(ctx, `VACUUM (ANALYZE)`); err != nil {
		return err
	}
	return s.WaitNoVacuum(ctx)
}

// WaitNoVacuum waits until no (auto)vacuum is running in the database.
func (s *Store) WaitNoVacuum(ctx context.Context) error {
	for {
		var running int
		if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_progress_vacuum WHERE datname = current_database()`).Scan(&running); err != nil {
			return err
		}
		if running == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Size returns the total size (data + indexes + TOAST) of the given tables.
func (s *Store) Size(ctx context.Context, tables ...string) (int64, error) {
	var size int64
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(sum(pg_total_relation_size(t::regclass)), 0)::bigint FROM unnest($1::text[]) AS t`, tables).Scan(&size)
	return size, err
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
