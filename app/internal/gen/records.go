package gen

import (
	"sort"

	"lab2/internal/model"
)

// Records builds one document per student from a generated batch.
// Ordering: semesters by id, courses by code, grades by date and id.
func (d *Dataset) Records(students []Student, enrollments []Enrollment, grades []Grade) []model.StudentRecord {
	gradesByEnrollment := map[int64][]Grade{}
	for _, g := range grades {
		gradesByEnrollment[g.EnrollmentID] = append(gradesByEnrollment[g.EnrollmentID], g)
	}
	enrollmentsByStudent := map[int64][]Enrollment{}
	for _, e := range enrollments {
		enrollmentsByStudent[e.StudentID] = append(enrollmentsByStudent[e.StudentID], e)
	}

	out := make([]model.StudentRecord, 0, len(students))
	for _, s := range students {
		g := d.Groups[s.GroupID-1]
		sp := d.Specialties[g.SpecialtyID-1]
		dep := d.Departments[sp.DepartmentID-1]
		fac := d.Faculties[dep.FacultyID-1]
		rec := model.StudentRecord{
			ID: s.ID, RecordBookNo: s.RecordBookNo, LastName: s.LastName, FirstName: s.FirstName,
			MiddleName: s.MiddleName, BirthDate: s.BirthDate, Email: s.Email, Phone: s.Phone,
			EnrollmentDate: s.EnrollmentDate, Status: s.Status, Funding: s.Funding,
			Group: model.GroupInfo{
				ID: g.ID, Code: g.Code, AdmissionYear: g.AdmissionYear,
				Specialty:  model.NamedID{ID: sp.ID, Code: sp.Code, Name: sp.Name},
				Department: model.NamedID{ID: dep.ID, Code: dep.Code, Name: dep.Name},
				Faculty:    model.NamedID{ID: fac.ID, Code: fac.Code, Name: fac.Name},
			},
			Semesters: []model.SemesterRecord{},
		}

		bySemester := map[int64]*model.SemesterRecord{}
		for _, e := range enrollmentsByStudent[s.ID] {
			sr, ok := bySemester[e.SemesterID]
			if !ok {
				sem := d.Semesters[e.SemesterID-1]
				sr = &model.SemesterRecord{SemesterID: sem.ID, AcademicYear: sem.AcademicYear, Term: sem.Term}
				bySemester[e.SemesterID] = sr
			}
			c := d.Courses[e.CourseID-1]
			cr := model.CourseRecord{
				EnrollmentID: e.ID, CourseID: c.ID, Code: c.Code, Name: c.Name, Credits: c.Credits,
				Control: c.Control, Status: e.Status, TotalScore: e.TotalScore, ECTSGrade: e.ECTSGrade,
				NationalGrade: e.NationalGrade, Grades: []model.GradeRecord{},
			}
			gs := gradesByEnrollment[e.ID]
			sort.Slice(gs, func(i, j int) bool {
				if !gs[i].GradedOn.Equal(gs[j].GradedOn) {
					return gs[i].GradedOn.Before(gs[j].GradedOn)
				}
				return gs[i].ID < gs[j].ID
			})
			for _, gr := range gs {
				t := d.Teachers[gr.TeacherID-1]
				cr.Grades = append(cr.Grades, model.GradeRecord{
					GradeID: gr.ID, Type: gr.Type, Title: gr.Title, Score: gr.Score, MaxScore: gr.MaxScore,
					GradedOn: gr.GradedOn, TeacherID: t.ID, Teacher: t.FullName(),
				})
			}
			sr.Courses = append(sr.Courses, cr)
		}
		for _, sr := range bySemester {
			sort.Slice(sr.Courses, func(i, j int) bool { return sr.Courses[i].Code < sr.Courses[j].Code })
			rec.Semesters = append(rec.Semesters, *sr)
		}
		sort.Slice(rec.Semesters, func(i, j int) bool { return rec.Semesters[i].SemesterID < rec.Semesters[j].SemesterID })
		out = append(out, rec)
	}
	return out
}

func (t Teacher) FullName() string { return t.LastName + " " + t.FirstName + " " + t.MiddleName }
