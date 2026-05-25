package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
)

type PDFEntry struct {
	ID           int     `json:"id"`
	OriginalName string  `json:"original_name"`
	CourseID     *string `json:"course_id"`
	CourseName   string  `json:"course_name"`
	Pages        int     `json:"pages"`
	LastPage     int     `json:"last_page"`
	LastReadAt   *string `json:"last_read_at"`
	UploadedAt   string  `json:"uploaded_at"`
}

type JSONPlan struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Phases   []Phase       `json:"phases"`
	Sessions []PlanSession `json:"sessions,omitempty"`
}

type Phase struct {
	Title    string    `json:"title"`
	Clusters []Cluster `json:"clusters,omitempty"`
	Tasks    []Task    `json:"tasks,omitempty"`
}

type Cluster struct {
	Title string `json:"title"`
	Tasks []Task `json:"tasks"`
}

type Task struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type PlanSession struct {
	Date  string `json:"date"`
	Topic string `json:"topic"`
	Time  string `json:"time"`
	Notes string `json:"notes,omitempty"`
}

type PlanSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	HasPlan bool   `json:"hasPlan"`
}

type Session struct {
	ID        int64  `json:"id"`
	CourseID  string `json:"course_id"`
	Topic     string `json:"topic"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	LastPdfID *int64 `json:"last_pdf_id"`
	LastPage  int    `json:"last_page"`
	PdfName   string `json:"pdf_name,omitempty"`
	Summary   string `json:"summary"`
	SummaryAt int    `json:"summary_at"`
}

// ExtractPDFPageCount returns the page count parsed from the PDF
// /Count entry, or 0 if it cannot be determined.
func ExtractPDFPageCount(data []byte) int {
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := re.FindSubmatch(data)
	if len(matches) < 2 {
		return 0
	}
	count, err := strconv.Atoi(string(matches[1]))
	if err != nil || count <= 0 {
		return 0
	}
	return count
}

func CountTasks(p *JSONPlan) (done, total int) {
	if p == nil {
		return 0, 0
	}
	for _, phase := range p.Phases {
		for _, t := range phase.Tasks {
			total++
			if t.Done {
				done++
			}
		}
		for _, cluster := range phase.Clusters {
			for _, t := range cluster.Tasks {
				total++
				if t.Done {
					done++
				}
			}
		}
	}
	return
}

// LoadPlan reads the plan JSON for the given id from the vault.
// Returns nil if the plan file is missing or malformed.
// assignMissingTaskIDs walks the plan and assigns a new UUID to any task
// with an empty ID. Returns true if any IDs were assigned (plan is dirty).
func assignMissingTaskIDs(p *JSONPlan) bool {
	dirty := false
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			if p.Phases[i].Tasks[j].ID == "" {
				p.Phases[i].Tasks[j].ID = newTaskID()
				dirty = true
			}
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				if p.Phases[i].Clusters[k].Tasks[j].ID == "" {
					p.Phases[i].Clusters[k].Tasks[j].ID = newTaskID()
					dirty = true
				}
			}
		}
	}
	return dirty
}

func (a *App) LoadPlan(id string) *JSONPlan {
	path := a.VaultPath("data", "plans", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var p JSONPlan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil
	}
	if assignMissingTaskIDs(&p) {
		if err := a.SavePlan(&p); err != nil {
			slog.Warn("failed to persist migrated task IDs", "plan_id", id, "err", err)
		}
	}
	return &p
}

func (a *App) SavePlan(p *JSONPlan) error {
	path := a.VaultPath("data", "plans", p.ID+".json")
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
