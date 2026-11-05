package model

// Debtor is a student of a group with failed courses.
type Debtor struct {
	StudentID int64
	LastName  string
	FirstName string
	Failed    int
}

// CourseStat is the result of a course in a semester.
type CourseStat struct {
	Code     string
	Name     string
	Students int
	AvgScore float64
	PassRate float64
}
