package agent

var KnownCourses = []struct {
	ID   string
	Name string
}{
	{"ce297", "Safety Models and Techniques (CE-297)"},
	{"ddia", "Designing Data-Intensive Applications"},
	{"dsa-interview", "DSA Interview Prep"},
	{"software-arch", "Software Architecture"},
	{"thesis", "Thesis — Phase 1 Survey"},
	{"guitar", "🎸 Guitar — Motivation-First Consistency"},
}

// CourseName returns the display name for a course ID.
// Prefer App.CourseName for new callers.
func CourseName(id string) string {
	for _, c := range KnownCourses {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// AppCourseName returns the display name for a course ID via DB lookup.
func (a *App) AppCourseName(id string) string {
	c, _ := a.GetCourse(id)
	return c.Name
}
