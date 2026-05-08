package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
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

func ExtractPDFPageCount(data []byte) int {
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := re.FindSubmatch(data)
	if len(matches) >= 2 {
		count := 0
		var dummy int
		dummy, _ = fmt.Sscanf(string(matches[1]), "%d", &count)
		_ = dummy
		if count > 0 {
			return count
		}
	}
	return 0
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

func LoadPlan(id string) *JSONPlan {
	path := VaultPath("data", "plans", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var p JSONPlan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil
	}
	return &p
}

func SavePlan(p *JSONPlan) error {
	path := VaultPath("data", "plans", p.ID+".json")
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(path, data, 0644)
}