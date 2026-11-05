package pg

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"

	"lab2/internal/model"
)

const headerSQL = `
SELECT s.id, s.record_book_no, s.last_name, s.first_name, COALESCE(s.middle_name, ''), s.birth_date,
       COALESCE(s.email, ''), COALESCE(s.phone, ''), s.enrollment_date, s.status, s.funding,
       g.id, g.code, g.admission_year, sp.id, sp.code, sp.name, d.id, d.code, d.name, f.id, f.code, f.name
FROM students s
JOIN student_groups g ON g.id  = s.group_id
JOIN specialties sp   ON sp.id = g.specialty_id
JOIN departments d    ON d.id  = sp.department_id
JOIN faculties f      ON f.id  = d.faculty_id
WHERE s.id = $1`

const bodySQL = `
SELECT e.semester_id, sm.academic_year, sm.term,
       e.id, c.id, c.code, c.name, c.credits, c.control, e.status, e.total_score, e.ects_grade, e.national_grade,
       gr.id, gr.assessment_type, gr.title, gr.score, gr.max_score, gr.graded_on,
       t.id, t.last_name || ' ' || t.first_name || ' ' || t.middle_name
FROM enrollments e
JOIN semesters sm      ON sm.id = e.semester_id
JOIN courses c         ON c.id  = e.course_id
LEFT JOIN grades gr    ON gr.enrollment_id = e.id
LEFT JOIN teachers t   ON t.id  = gr.teacher_id
WHERE e.student_id = $1
ORDER BY e.semester_id, c.code, gr.graded_on, gr.id`

// RecordJoin reads the student record from the normalized tables (two queries in one round trip)
// and assembles the document in the application.
func (s *Store) RecordJoin(ctx context.Context, id int64) (model.StudentRecord, error) {
	var rec model.StudentRecord
	batch := &pgx.Batch{}
	batch.Queue(headerSQL, id).QueryRow(func(row pgx.Row) error {
		g := &rec.Group
		return row.Scan(&rec.ID, &rec.RecordBookNo, &rec.LastName, &rec.FirstName, &rec.MiddleName, &rec.BirthDate,
			&rec.Email, &rec.Phone, &rec.EnrollmentDate, &rec.Status, &rec.Funding,
			&g.ID, &g.Code, &g.AdmissionYear, &g.Specialty.ID, &g.Specialty.Code, &g.Specialty.Name,
			&g.Department.ID, &g.Department.Code, &g.Department.Name, &g.Faculty.ID, &g.Faculty.Code, &g.Faculty.Name)
	})
	rec.Semesters = []model.SemesterRecord{}
	batch.Queue(bodySQL, id).Query(func(rows pgx.Rows) error {
		var (
			semID, enrollmentID    int64
			academicYear           string
			term                   int
			course                 model.CourseRecord
			total                  *float64
			ects, national         *string
			gradeID, teacherID     *int64
			gType, gTitle, teacher *string
			score, maxScore        *float64
			gradedOn               *time.Time
		)
		for rows.Next() {
			if err := rows.Scan(&semID, &academicYear, &term, &enrollmentID, &course.CourseID, &course.Code, &course.Name,
				&course.Credits, &course.Control, &course.Status, &total, &ects, &national,
				&gradeID, &gType, &gTitle, &score, &maxScore, &gradedOn, &teacherID, &teacher); err != nil {
				return err
			}
			n := len(rec.Semesters)
			if n == 0 || rec.Semesters[n-1].SemesterID != semID {
				rec.Semesters = append(rec.Semesters, model.SemesterRecord{SemesterID: semID, AcademicYear: academicYear,
					Term: term, Courses: []model.CourseRecord{}})
				n++
			}
			sem := &rec.Semesters[n-1]
			m := len(sem.Courses)
			if m == 0 || sem.Courses[m-1].EnrollmentID != enrollmentID {
				c := course
				c.EnrollmentID = enrollmentID
				c.TotalScore, c.ECTSGrade, c.NationalGrade = copyPtr(total), copyPtr(ects), copyPtr(national)
				c.Grades = []model.GradeRecord{}
				sem.Courses = append(sem.Courses, c)
				m++
			}
			if gradeID != nil {
				g := model.GradeRecord{GradeID: *gradeID, Type: *gType, Title: *gTitle, Score: *score,
					MaxScore: *maxScore, GradedOn: *gradedOn}
				if teacherID != nil {
					g.TeacherID, g.Teacher = *teacherID, *teacher
				}
				sem.Courses[m-1].Grades = append(sem.Courses[m-1].Grades, g)
			}
		}
		return rows.Err()
	})
	err := s.Pool.SendBatch(ctx, batch).Close()
	return rec, err
}

const recordJSONSQL = `
SELECT json_build_object(
    'id', s.id, 'record_book_no', s.record_book_no, 'last_name', s.last_name, 'first_name', s.first_name,
    'middle_name', COALESCE(s.middle_name, ''), 'birth_date', to_char(s.birth_date, 'YYYY-MM-DD"T00:00:00Z"'),
    'email', COALESCE(s.email, ''), 'phone', COALESCE(s.phone, ''),
    'enrollment_date', to_char(s.enrollment_date, 'YYYY-MM-DD"T00:00:00Z"'), 'status', s.status, 'funding', s.funding,
    'group', json_build_object('id', g.id, 'code', g.code, 'admission_year', g.admission_year,
        'specialty',  json_build_object('id', sp.id, 'code', sp.code, 'name', sp.name),
        'department', json_build_object('id', d.id, 'code', d.code, 'name', d.name),
        'faculty',    json_build_object('id', f.id, 'code', f.code, 'name', f.name)),
    'semesters', COALESCE((
        SELECT json_agg(json_build_object('semester_id', x.semester_id, 'academic_year', x.academic_year,
                                          'term', x.term, 'courses', x.courses) ORDER BY x.semester_id)
        FROM (
            SELECT e.semester_id, sm.academic_year, sm.term,
                   json_agg(json_build_object(
                       'enrollment_id', e.id, 'course_id', c.id, 'code', c.code, 'name', c.name,
                       'credits', c.credits, 'control', c.control, 'status', e.status,
                       'total_score', e.total_score, 'ects_grade', e.ects_grade, 'national_grade', e.national_grade,
                       'grades', COALESCE((
                           SELECT json_agg(json_build_object(
                               'grade_id', gr.id, 'type', gr.assessment_type, 'title', gr.title, 'score', gr.score,
                               'max_score', gr.max_score, 'graded_on', to_char(gr.graded_on, 'YYYY-MM-DD"T00:00:00Z"'),
                               'teacher_id', t.id, 'teacher', t.last_name || ' ' || t.first_name || ' ' || t.middle_name)
                               ORDER BY gr.graded_on, gr.id)
                           FROM grades gr
                           LEFT JOIN teachers t ON t.id = gr.teacher_id
                           WHERE gr.enrollment_id = e.id), '[]'::json))
                       ORDER BY c.code) AS courses
            FROM enrollments e
            JOIN semesters sm ON sm.id = e.semester_id
            JOIN courses c    ON c.id  = e.course_id
            WHERE e.student_id = s.id
            GROUP BY e.semester_id, sm.academic_year, sm.term) x), '[]'::json))
FROM students s
JOIN student_groups g ON g.id  = s.group_id
JOIN specialties sp   ON sp.id = g.specialty_id
JOIN departments d    ON d.id  = sp.department_id
JOIN faculties f      ON f.id  = d.faculty_id
WHERE s.id = $1`

func copyPtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// RecordJSON builds the whole document inside PostgreSQL (json_agg over the normalized tables).
func (s *Store) RecordJSON(ctx context.Context, id int64) (model.StudentRecord, error) {
	var raw []byte
	var rec model.StudentRecord
	if err := s.Pool.QueryRow(ctx, recordJSONSQL, id).Scan(&raw); err != nil {
		return rec, err
	}
	return rec, json.Unmarshal(raw, &rec)
}

// RecordJSONB reads the ready document from the jsonb table.
func (s *Store) RecordJSONB(ctx context.Context, id int64) (model.StudentRecord, error) {
	var raw []byte
	var rec model.StudentRecord
	if err := s.Pool.QueryRow(ctx, `SELECT doc FROM student_records WHERE id = $1`, id).Scan(&raw); err != nil {
		return rec, err
	}
	return rec, json.Unmarshal(raw, &rec)
}

// GroupDebtors returns the students of a group that have failed courses.
func (s *Store) GroupDebtors(ctx context.Context, groupCode string) ([]model.Debtor, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT s.id, s.last_name, s.first_name, count(*)
		FROM student_groups g
		JOIN students s    ON s.group_id = g.id
		JOIN enrollments e ON e.student_id = s.id AND e.status = 'failed'
		WHERE g.code = $1
		GROUP BY s.id
		ORDER BY s.id`, groupCode)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Debtor, error) {
		var d model.Debtor
		err := row.Scan(&d.StudentID, &d.LastName, &d.FirstName, &d.Failed)
		return d, err
	})
}

// CourseStats returns the average score and the pass rate of every course in a semester.
func (s *Store) CourseStats(ctx context.Context, semesterID int64) ([]model.CourseStat, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT c.code, c.name, count(*),
		       avg(e.total_score)::float8,
		       (100.0 * count(*) FILTER (WHERE e.status = 'completed') / count(*))::float8
		FROM enrollments e
		JOIN courses c ON c.id = e.course_id
		WHERE e.semester_id = $1 AND e.status IN ('completed', 'failed')
		GROUP BY c.code, c.name
		ORDER BY c.code`, semesterID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.CourseStat, error) {
		var c model.CourseStat
		err := row.Scan(&c.Code, &c.Name, &c.Students, &c.AvgScore, &c.PassRate)
		return c, err
	})
}

// AddGrade inserts one grade (autocommit, durable like a journaled MongoDB write).
func (s *Store) AddGrade(ctx context.Context, enrollmentID int64, g model.GradeRecord) (int64, error) {
	var id int64
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO grades (enrollment_id, assessment_type, title, score, max_score, graded_on, teacher_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		enrollmentID, g.Type, g.Title, g.Score, g.MaxScore, g.GradedOn, g.TeacherID).Scan(&id)
	return id, err
}

// DeleteBenchmarkGrades removes the grades added by the benchmark.
func (s *Store) DeleteBenchmarkGrades(ctx context.Context, title string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM grades WHERE title = $1`, title)
	return err
}

// RenameCourse changes the course name: one row in the normalized model.
func (s *Store) RenameCourse(ctx context.Context, courseID int64, name string) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `UPDATE courses SET name = $2 WHERE id = $1`, courseID, name)
	return tag.RowsAffected(), err
}
