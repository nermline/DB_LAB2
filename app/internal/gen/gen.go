// Package gen generates a deterministic university dataset. The same seed
// always produces the same rows, so PostgreSQL and MongoDB get identical data.
package gen

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

type Faculty struct {
	ID         int64
	Code, Name string
}

type Department struct {
	ID, FacultyID int64
	Code, Name    string
	Field         string
}

type Specialty struct {
	ID, DepartmentID int64
	Code, Name       string
}

type Teacher struct {
	ID, DepartmentID                int64
	LastName, FirstName, MiddleName string
	Position, AcademicDegree        string
}

type Course struct {
	ID, DepartmentID int64
	Code, Name       string
	Credits          float64
	Control          string
	HasLab           bool
}

type Semester struct {
	ID           int64
	AcademicYear string
	Term         int
	Start, End   time.Time
}

type Group struct {
	ID, SpecialtyID, CuratorID int64
	Code                       string
	AdmissionYear              int
}

type Student struct {
	ID, GroupID                     int64
	RecordBookNo                    string
	LastName, FirstName, MiddleName string
	BirthDate, EnrollmentDate       time.Time
	Email, Phone, Status, Funding   string
}

type Enrollment struct {
	ID, StudentID, CourseID, SemesterID int64
	Status                              string
	TotalScore                          *float64
	ECTSGrade, NationalGrade            *string
}

type Grade struct {
	ID, EnrollmentID, TeacherID int64
	Type, Title                 string
	Score, MaxScore             float64
	GradedOn                    time.Time
}

// Dataset holds the reference data; students are generated in batches.
type Dataset struct {
	Seed        uint64
	Faculties   []Faculty
	Departments []Department
	Specialties []Specialty
	Teachers    []Teacher
	Courses     []Course
	Semesters   []Semester
	Groups      []Group

	curriculum      map[int64][][]int64 // specialty id -> semester number (0..7) -> course ids
	deptTeachers    map[int64][]int64
	groupStudents   []int // number of students per group (same order as Groups)
	CurrentSemester int64

	nextEnrollmentID int64
	nextGradeID      int64
	nextStudentID    int64
	groupCursor      int
	groupFilled      int
}

const (
	TeachersPerDepartment = 60
	CoursesPerDepartment  = 20
	CoursesPerSemester    = 6
	GroupSize             = 25
	FirstAdmissionYear    = 2022
	Cohorts               = 4
)

var faculties = []struct {
	code, name string
	depts      [][3]string // code, department name, field
}{
	{"FIT", "Faculty of Information Technology", [][3]string{
		{"SE", "Department of Software Engineering", "Software Engineering"},
		{"CS", "Department of Computer Science", "Computer Science"},
		{"CY", "Department of Cybersecurity", "Cybersecurity"},
		{"IS", "Department of Information Systems", "Information Systems"}}},
	{"FAM", "Faculty of Applied Mathematics", [][3]string{
		{"HM", "Department of Higher Mathematics", "Higher Mathematics"},
		{"AM", "Department of Applied Mathematics", "Applied Mathematics"},
		{"ST", "Department of Statistics", "Statistics"},
		{"MM", "Department of Mathematical Modeling", "Mathematical Modeling"}}},
	{"FEM", "Faculty of Economics and Management", [][3]string{
		{"EC", "Department of Economics", "Economics"},
		{"MG", "Department of Management", "Management"},
		{"FN", "Department of Finance", "Finance"},
		{"MK", "Department of Marketing", "Marketing"}}},
	{"FL", "Faculty of Law", [][3]string{
		{"CL", "Department of Civil Law", "Civil Law"},
		{"CR", "Department of Criminal Law", "Criminal Law"},
		{"IL", "Department of International Law", "International Law"},
		{"KL", "Department of Constitutional Law", "Constitutional Law"}}},
	{"FPH", "Faculty of Physics", [][3]string{
		{"GP", "Department of General Physics", "General Physics"},
		{"TP", "Department of Theoretical Physics", "Theoretical Physics"},
		{"OP", "Department of Optics", "Optics"},
		{"EL", "Department of Electronics", "Electronics"}}},
	{"FCH", "Faculty of Chemistry", [][3]string{
		{"OC", "Department of Organic Chemistry", "Organic Chemistry"},
		{"IC", "Department of Inorganic Chemistry", "Inorganic Chemistry"},
		{"AC", "Department of Analytical Chemistry", "Analytical Chemistry"},
		{"CT", "Department of Chemical Technology", "Chemical Technology"}}},
	{"FB", "Faculty of Biology", [][3]string{
		{"BC", "Department of Biochemistry", "Biochemistry"},
		{"GN", "Department of Genetics", "Genetics"},
		{"EG", "Department of Ecology", "Ecology"},
		{"MB", "Department of Microbiology", "Microbiology"}}},
	{"FPL", "Faculty of Philology", [][3]string{
		{"UP", "Department of Ukrainian Philology", "Ukrainian Philology"},
		{"EP", "Department of English Philology", "English Philology"},
		{"LN", "Department of Linguistics", "Linguistics"},
		{"JR", "Department of Journalism", "Journalism"}}},
}

var coursePrefixes = []string{
	"Introduction to", "Fundamentals of", "Methods of", "History of", "Theory of",
	"Practice of", "Modern", "Applied", "Advanced", "Research in",
	"Seminar on", "Mathematics for", "Tools for", "Ethics in", "Project in",
	"Laboratory of", "Special Topics in", "Models of", "Systems of", "Workshop on",
}

var (
	lastNames = []string{"Shevchenko", "Kovalenko", "Bondarenko", "Tkachenko", "Kravchuk", "Melnychuk",
		"Polishchuk", "Savchuk", "Marchenko", "Lytvynenko", "Hrytsenko", "Pavlenko", "Kyrylenko", "Holovko",
		"Diachuk", "Ostapchuk", "Romanenko", "Yaroshenko", "Yakovenko", "Nesterenko", "Tymoshenko", "Levchenko",
		"Zakharchenko", "Onyshchenko", "Demchenko", "Zinchenko", "Ponomarenko", "Humeniuk", "Petruk", "Karpenko",
		"Moroz", "Melnyk", "Boiko", "Koval", "Oliinyk", "Lysenko", "Rudenko", "Savchenko", "Petrenko", "Kuzmenko"}
	maleNames = []string{"Oleksandr", "Andrii", "Dmytro", "Maksym", "Vladyslav", "Bohdan", "Artem", "Nazar",
		"Taras", "Yevhen", "Ivan", "Mykola", "Serhii", "Yurii", "Pavlo", "Roman", "Viktor", "Oleh", "Denys", "Ihor"}
	femaleNames = []string{"Anna", "Mariia", "Sofiia", "Viktoriia", "Daryna", "Yuliia", "Kateryna", "Olena",
		"Iryna", "Khrystyna", "Nataliia", "Tetiana", "Oksana", "Svitlana", "Alina", "Anastasiia", "Polina",
		"Veronika", "Diana", "Valeriia"}
	fatherNames = []string{"Oleksandr", "Ivan", "Petro", "Serhii", "Mykhailo", "Vasyl", "Andrii", "Volodymyr",
		"Mykola", "Yurii", "Viktor", "Oleh", "Roman", "Stepan", "Ihor"}
	positions = []string{"assistant", "senior_lecturer", "associate_professor", "professor"}
)

// New builds the reference data for a university with the given number of students.
func New(seed uint64, students int) *Dataset {
	r := rand.New(rand.NewPCG(seed, 1))
	d := &Dataset{Seed: seed, curriculum: map[int64][][]int64{}, deptTeachers: map[int64][]int64{}}

	for fi, f := range faculties {
		fid := int64(fi + 1)
		d.Faculties = append(d.Faculties, Faculty{ID: fid, Code: f.code, Name: f.name})
		for _, dep := range f.depts {
			did := int64(len(d.Departments) + 1)
			d.Departments = append(d.Departments, Department{ID: did, FacultyID: fid, Code: dep[0], Name: dep[1], Field: dep[2]})
			d.Specialties = append(d.Specialties, Specialty{ID: did, DepartmentID: did,
				Code: fmt.Sprintf("%03d", 100+int(did)*3), Name: dep[2]})
		}
	}

	for _, dep := range d.Departments {
		for i := 0; i < TeachersPerDepartment; i++ {
			id := int64(len(d.Teachers) + 1)
			male := r.IntN(2) == 0
			first, middle := personName(r, male)
			pos := positions[min(3, int(math.Abs(r.NormFloat64())*1.3))]
			degree := ""
			switch pos {
			case "professor":
				degree = "DSc"
			case "associate_professor", "senior_lecturer":
				degree = "PhD"
			}
			d.Teachers = append(d.Teachers, Teacher{ID: id, DepartmentID: dep.ID, LastName: pick(r, lastNames),
				FirstName: first, MiddleName: middle, Position: pos, AcademicDegree: degree})
			d.deptTeachers[dep.ID] = append(d.deptTeachers[dep.ID], id)
		}
		for i := 0; i < CoursesPerDepartment; i++ {
			id := int64(len(d.Courses) + 1)
			ctrl := "exam"
			switch x := r.Float64(); {
			case x > 0.85:
				ctrl = "diff_credit"
			case x > 0.6:
				ctrl = "credit"
			}
			d.Courses = append(d.Courses, Course{ID: id, DepartmentID: dep.ID,
				Code: fmt.Sprintf("%s-%d%02d", dep.Code, 1+i/5, i%5+1),
				Name: coursePrefixes[i] + " " + dep.Field, Credits: float64(3 + r.IntN(4)),
				Control: ctrl, HasLab: r.IntN(2) == 0})
		}
	}

	for y := 0; y < Cohorts+1; y++ {
		year := FirstAdmissionYear + y
		for term := 1; term <= 2; term++ {
			if y == Cohorts && term == 2 {
				break
			}
			var start, end time.Time
			if term == 1 {
				start = date(year, 9, 1)
				end = date(year, 12, 28)
			} else {
				start = date(year+1, 1, 26)
				end = date(year+1, 6, 28)
			}
			d.Semesters = append(d.Semesters, Semester{ID: int64(len(d.Semesters) + 1),
				AcademicYear: fmt.Sprintf("%d/%d", year, year+1), Term: term, Start: start, End: end})
		}
	}
	d.CurrentSemester = d.Semesters[len(d.Semesters)-1].ID

	// curriculum: per semester 2 courses of the own department, 2 of the same faculty, 2 of other faculties
	for _, sp := range d.Specialties {
		dep := d.Departments[sp.DepartmentID-1]
		used := map[int64]bool{}
		own := coursesWhere(d, func(c Course) bool { return c.DepartmentID == dep.ID })
		faculty := coursesWhere(d, func(c Course) bool {
			return c.DepartmentID != dep.ID && d.Departments[c.DepartmentID-1].FacultyID == dep.FacultyID
		})
		other := coursesWhere(d, func(c Course) bool { return d.Departments[c.DepartmentID-1].FacultyID != dep.FacultyID })
		plan := make([][]int64, 8)
		for s := 0; s < 8; s++ {
			plan[s] = append(plan[s], take(r, own, used, 2)...)
			plan[s] = append(plan[s], take(r, faculty, used, 2)...)
			plan[s] = append(plan[s], take(r, other, used, 2)...)
		}
		d.curriculum[sp.ID] = plan
	}

	// groups: students are spread evenly over specialties and cohorts
	perCell := students / (len(d.Specialties) * Cohorts)
	extra := students % (len(d.Specialties) * Cohorts)
	for _, sp := range d.Specialties {
		dep := d.Departments[sp.DepartmentID-1]
		for c := 0; c < Cohorts; c++ {
			year := FirstAdmissionYear + c
			n := perCell
			if extra > 0 {
				n++
				extra--
			}
			for g := 0; n > 0; g++ {
				size := min(GroupSize, n)
				n -= size
				id := int64(len(d.Groups) + 1)
				teachers := d.deptTeachers[dep.ID]
				d.Groups = append(d.Groups, Group{ID: id, SpecialtyID: sp.ID, AdmissionYear: year,
					Code:      fmt.Sprintf("%s-%02d-%02d", dep.Code, year%100, g+1),
					CuratorID: teachers[r.IntN(len(teachers))]})
				d.groupStudents = append(d.groupStudents, size)
			}
		}
	}
	return d
}

// Batch generates the next n students with their enrollments and grades.
func (d *Dataset) Batch(n int) ([]Student, []Enrollment, []Grade) {
	var (
		students    []Student
		enrollments []Enrollment
		grades      []Grade
	)
	for len(students) < n && d.groupCursor < len(d.Groups) {
		g := d.Groups[d.groupCursor]
		d.nextStudentID++
		sid := d.nextStudentID
		r := rand.New(rand.NewPCG(d.Seed, uint64(sid)+1_000_000))
		st := newStudent(r, sid, g)
		if g.AdmissionYear+4 <= d.Semesters[d.CurrentSemester-1].Start.Year() {
			st.Status = "graduated"
		}
		students = append(students, st)

		ability := clamp(0.78+r.NormFloat64()*0.12, 0.3, 1)
		sp := d.Specialties[g.SpecialtyID-1]
		for _, sem := range d.Semesters {
			semNo := semesterNumber(g.AdmissionYear, sem)
			if semNo < 0 || semNo > 7 {
				continue
			}
			current := sem.ID == d.CurrentSemester
			for _, cid := range d.curriculum[sp.ID][semNo] {
				c := d.Courses[cid-1]
				d.nextEnrollmentID++
				e := Enrollment{ID: d.nextEnrollmentID, StudentID: sid, CourseID: cid, SemesterID: sem.ID, Status: "enrolled"}
				teachers := d.deptTeachers[c.DepartmentID]
				lecturer := teachers[int(g.ID*7+cid)%len(teachers)]
				assistant := teachers[int(g.ID*13+cid*3)%len(teachers)]
				works := 4
				if current {
					works = 2
				}
				var total float64
				for k := 1; k <= works; k++ {
					typ, title := "practice", fmt.Sprintf("Practice %d", k)
					if c.HasLab {
						typ, title = "lab", fmt.Sprintf("Lab %d", k)
					}
					grades = append(grades, d.grade(r, e.ID, assistant, typ, title, 10, ability, sem.Start.AddDate(0, 0, 14*k)))
					total += grades[len(grades)-1].Score
				}
				if !current {
					grades = append(grades, d.grade(r, e.ID, lecturer, "module", "Module test", 20, ability, sem.Start.AddDate(0, 0, 60)))
					total += grades[len(grades)-1].Score
					typ, title := "exam", "Exam"
					if c.Control != "exam" {
						typ, title = "test", "Final test"
					}
					grades = append(grades, d.grade(r, e.ID, lecturer, typ, title, 40, ability, sem.Start.AddDate(0, 0, 110)))
					total += grades[len(grades)-1].Score
					e.TotalScore = &total
					ects, national := ECTS(total), National(total, c.Control)
					e.ECTSGrade, e.NationalGrade = &ects, &national
					e.Status = "completed"
					if total < 60 {
						e.Status = "failed"
					}
				}
				enrollments = append(enrollments, e)
			}
		}

		d.groupFilled++
		if d.groupFilled == d.groupStudents[d.groupCursor] {
			d.groupCursor++
			d.groupFilled = 0
		}
	}
	return students, enrollments, grades
}

func (d *Dataset) grade(r *rand.Rand, enrollmentID, teacherID int64, typ, title string, max float64, ability float64, on time.Time) Grade {
	d.nextGradeID++
	score := math.Round(max * clamp(ability+r.NormFloat64()*0.08, 0, 1))
	return Grade{ID: d.nextGradeID, EnrollmentID: enrollmentID, TeacherID: teacherID, Type: typ, Title: title,
		Score: score, MaxScore: max, GradedOn: on}
}

func newStudent(r *rand.Rand, id int64, g Group) Student {
	male := r.IntN(2) == 0
	first, middle := personName(r, male)
	status := "active"
	switch x := r.Float64(); {
	case x > 0.97:
		status = "expelled"
	case x > 0.94:
		status = "academic_leave"
	}
	funding := "budget"
	if r.Float64() < 0.4 {
		funding = "contract"
	}
	return Student{
		ID: id, GroupID: g.ID,
		RecordBookNo:   fmt.Sprintf("RB%d-%06d", g.AdmissionYear, id),
		LastName:       pick(r, lastNames),
		FirstName:      first,
		MiddleName:     middle,
		BirthDate:      date(g.AdmissionYear-17, 1+r.IntN(12), 1+r.IntN(28)),
		EnrollmentDate: date(g.AdmissionYear, 9, 1),
		Email:          fmt.Sprintf("s%06d@stud.univ.edu.ua", id),
		Phone:          fmt.Sprintf("+38067%07d", r.IntN(10_000_000)),
		Status:         status,
		Funding:        funding,
	}
}

func personName(r *rand.Rand, male bool) (string, string) {
	father := pick(r, fatherNames)
	if male {
		return pick(r, maleNames), patronymic(father, true)
	}
	return pick(r, femaleNames), patronymic(father, false)
}

func patronymic(father string, male bool) string {
	base := father
	switch father {
	case "Serhii", "Andrii", "Yurii":
		base = father[:len(father)-1]
		if male {
			return base + "iovych"
		}
		return base + "ivna"
	case "Mykola":
		if male {
			return "Mykolaiovych"
		}
		return "Mykolaivna"
	}
	if male {
		return base + "ovych"
	}
	return base + "ivna"
}

// ECTS converts a 100-point score to the ECTS scale (same rules as fn_ects_grade in lab 1).
func ECTS(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 82:
		return "B"
	case score >= 74:
		return "C"
	case score >= 64:
		return "D"
	case score >= 60:
		return "E"
	case score >= 35:
		return "FX"
	}
	return "F"
}

// National converts a score to the national scale (same rules as fn_national_grade in lab 1).
func National(score float64, control string) string {
	if control == "credit" {
		if score >= 60 {
			return "passed"
		}
		return "not passed"
	}
	switch {
	case score >= 90:
		return "excellent"
	case score >= 74:
		return "good"
	case score >= 60:
		return "satisfactory"
	}
	return "unsatisfactory"
}

func semesterNumber(admissionYear int, s Semester) int {
	startYear := s.Start.Year()
	if s.Term == 2 {
		startYear--
	}
	return (startYear-admissionYear)*2 + s.Term - 1
}

func coursesWhere(d *Dataset, f func(Course) bool) []int64 {
	var ids []int64
	for _, c := range d.Courses {
		if f(c) {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

func take(r *rand.Rand, pool []int64, used map[int64]bool, n int) []int64 {
	var out []int64
	for len(out) < n {
		id := pool[r.IntN(len(pool))]
		if !used[id] {
			used[id] = true
			out = append(out, id)
		}
	}
	return out
}

func pick(r *rand.Rand, xs []string) string { return xs[r.IntN(len(xs))] }

func clamp(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

func date(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
