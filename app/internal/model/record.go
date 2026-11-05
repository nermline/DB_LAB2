// Package model describes the student record document. The same struct is
// stored in MongoDB (bson tags), in PostgreSQL jsonb (json tags) and is
// assembled from the normalized PostgreSQL tables.
package model

import "time"

type StudentRecord struct {
	ID             int64            `bson:"_id" json:"id"`
	RecordBookNo   string           `bson:"record_book_no" json:"record_book_no"`
	LastName       string           `bson:"last_name" json:"last_name"`
	FirstName      string           `bson:"first_name" json:"first_name"`
	MiddleName     string           `bson:"middle_name" json:"middle_name"`
	BirthDate      time.Time        `bson:"birth_date" json:"birth_date"`
	Email          string           `bson:"email" json:"email"`
	Phone          string           `bson:"phone" json:"phone"`
	EnrollmentDate time.Time        `bson:"enrollment_date" json:"enrollment_date"`
	Status         string           `bson:"status" json:"status"`
	Funding        string           `bson:"funding" json:"funding"`
	Group          GroupInfo        `bson:"group" json:"group"`
	Semesters      []SemesterRecord `bson:"semesters" json:"semesters"`
}

type GroupInfo struct {
	ID            int64   `bson:"id" json:"id"`
	Code          string  `bson:"code" json:"code"`
	AdmissionYear int     `bson:"admission_year" json:"admission_year"`
	Specialty     NamedID `bson:"specialty" json:"specialty"`
	Department    NamedID `bson:"department" json:"department"`
	Faculty       NamedID `bson:"faculty" json:"faculty"`
}

type NamedID struct {
	ID   int64  `bson:"id" json:"id"`
	Code string `bson:"code" json:"code"`
	Name string `bson:"name" json:"name"`
}

type SemesterRecord struct {
	SemesterID   int64          `bson:"semester_id" json:"semester_id"`
	AcademicYear string         `bson:"academic_year" json:"academic_year"`
	Term         int            `bson:"term" json:"term"`
	Courses      []CourseRecord `bson:"courses" json:"courses"`
}

type CourseRecord struct {
	EnrollmentID  int64         `bson:"enrollment_id" json:"enrollment_id"`
	CourseID      int64         `bson:"course_id" json:"course_id"`
	Code          string        `bson:"code" json:"code"`
	Name          string        `bson:"name" json:"name"`
	Credits       float64       `bson:"credits" json:"credits"`
	Control       string        `bson:"control" json:"control"`
	Status        string        `bson:"status" json:"status"`
	TotalScore    *float64      `bson:"total_score" json:"total_score"`
	ECTSGrade     *string       `bson:"ects_grade" json:"ects_grade"`
	NationalGrade *string       `bson:"national_grade" json:"national_grade"`
	Grades        []GradeRecord `bson:"grades" json:"grades"`
}

type GradeRecord struct {
	GradeID   int64     `bson:"grade_id" json:"grade_id"`
	Type      string    `bson:"type" json:"type"`
	Title     string    `bson:"title" json:"title"`
	Score     float64   `bson:"score" json:"score"`
	MaxScore  float64   `bson:"max_score" json:"max_score"`
	GradedOn  time.Time `bson:"graded_on" json:"graded_on"`
	TeacherID int64     `bson:"teacher_id" json:"teacher_id"`
	Teacher   string    `bson:"teacher" json:"teacher"`
}
