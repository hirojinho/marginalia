package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ToolStudySkill dispatches a named study skill (orientation, study_notes,
// self_test, review, grill_me) for the given topic and course.
func (a *App) ToolStudySkill(args json.RawMessage) string {
	var p struct {
		Skill  string            `json:"skill"`
		Params map[string]string `json:"params"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}

	params := p.Params
	if params == nil {
		params = map[string]string{}
	}

	topic := params["topic"]
	courseID := params["course_id"]
	courseName := CourseName(courseID)

	courseInterests := a.loadCourseInterests(courseID)
	corpusContent := a.loadCorpusContext(topic, courseID)

	switch p.Skill {
	case "orientation":
		return generateOrientation(topic, courseID, courseName, courseInterests, corpusContent)
	case "study_notes":
		return generateStudyNotes(topic, courseID, courseName, params["content"], courseInterests, corpusContent)
	case "self_test":
		count := 5
		if c, err := strconv.Atoi(params["count"]); err == nil && c > 0 && c <= 20 {
			count = c
		}
		return generateSelfTest(topic, courseID, courseName, count, courseInterests, corpusContent)
	case "review":
		return generateReview(topic, courseID, courseName, courseInterests, corpusContent)
	case "grill_me":
		return generateGrillMe(topic, courseID, courseName, courseInterests, corpusContent)
	default:
		return "error: unknown skill '" + p.Skill + "'. Available skills: orientation, study_notes, self_test, review, grill_me"
	}
}

func (a *App) loadCourseInterests(courseID string) string {
	if courseID == "" {
		return ""
	}
	data := readFileWithLog(a.VaultPath("data", "courses", courseID, "interests.md"))
	if data == "" {
		return ""
	}
	return "\n\nCourse interests and focus areas:\n" + data
}

func (a *App) loadCorpusContext(topic, courseID string) string {
	query := topic
	if courseID != "" {
		query = courseID + " " + topic
	}

	results, err := a.Search(query, courseID, 3)
	if err == nil && len(results) > 0 {
		var b strings.Builder
		for _, r := range results {
			heading := r.Heading
			if heading == "" {
				heading = r.ParentHeading
			}
			fmt.Fprintf(&b, "\n\n--- %s (%s) ---\n%s", r.SourceFile, heading, r.Content)
		}
		return b.String()
	}

	// Fallback 1: study-methods corpus dir
	if content := readDirAsCorpus(a.VaultPath("data", "corpus", "study-methods"), ""); content != "" {
		return content
	}
	// Fallback 2: course-specific corpus dir
	if courseID != "" {
		return readDirAsCorpus(a.VaultPath("data", "corpus", "courses", courseID), "course:"+courseID+"/")
	}
	return ""
}

func readDirAsCorpus(dir, sourcePrefix string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n\n--- %s%s ---\n%s", sourcePrefix, strings.TrimSuffix(f.Name(), ".md"), string(data))
	}
	return b.String()
}

// GetSessionSystemPrompt builds the per-session system prompt by
// appending course-specific interests, the most recent fleeting note
// (for ce297), and a PDF excerpt if one is associated with the
// session.
func (a *App) GetSessionSystemPrompt(sessionID int64, basePrompt string) string {
	if sessionID == 0 {
		return basePrompt
	}
	courseID, topic, err := a.GetSessionCourseAndTopic(sessionID)
	if err != nil {
		return basePrompt
	}
	if courseID == "" {
		return basePrompt + "\n\n---\n\nYou are in a general study session (no specific course)."
	}

	courseName := CourseName(courseID)
	extra := "\n\n---\n\nYou are in a study session for **" + courseName + "** (course ID: " + courseID + ")."
	if topic != "" && topic != "General" {
		extra += " Topic: " + topic + "."
	}

	if data := readFileWithLog(a.VaultPath("data", "courses", courseID, "interests.md")); data != "" {
		extra += "\n\n" + courseName + " interests:\n" + data
	}

	if courseID == "ce297" {
		fleetingGlob := a.VaultPath("data", "courses", "ce297", "fleeting", "*.md")
		if matches, _ := filepath.Glob(fleetingGlob); len(matches) > 0 {
			lastFleeting := matches[len(matches)-1]
			if data, err := os.ReadFile(lastFleeting); err == nil {
				extra += "\n\nLatest fleeting note:\n" + string(data)
			} else {
				slog.Info("fleeting note unread", "path", lastFleeting)
			}
		}
	}

	if pdfID, _ := a.GetSessionLastPDFID(sessionID); pdfID > 0 {
		cachePath := a.VaultPath("data", "pdf-texts", fmt.Sprintf("%d.txt", pdfID))
		if data, err := os.ReadFile(cachePath); err == nil {
			text := string(data)
			if len(text) > 2000 {
				text = text[:2000] + "\n...[truncated, use pdf_extract for full content]"
			}
			pdfName, _ := a.PDFOriginalName(pdfID)
			extra += fmt.Sprintf("\n\n---\n\nCurrent PDF: **%s**\n\nExcerpt:\n%s", pdfName, text)
		}
	}

	return basePrompt + extra
}

// ---------- Skill prompt templates ----------

func generateOrientation(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Study Orientation: " + topic,
		Body: `You are a study orientation assistant. Based on the topic and course context below, produce a practical pre-reading guide:

1. **Prerequisites** — What should the student already know before starting?
2. **Key Concepts** — 3-5 core ideas to focus on while reading
3. **Watch Points** — Common misconceptions or tricky parts to be aware of
4. **Study Approach** — Suggested method (examples-first, read-then-solve, etc.)
5. **Questions to Ask While Reading** — 3-5 questions to keep in mind during the study session

Be specific and practical. No generic advice.`,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateStudyNotes(topic, courseID, courseName, content, courseInterests, corpusContent string) string {
	body := `Generate structured study notes using this format:

## [Topic]

### Summary (2-3 sentences capturing the essence)

### Key Concepts
- Concept 1: brief explanation
- Concept 2: brief explanation

### Formulas / Definitions (if applicable)
- Formula/definition with context

### Connections to Other Topics
- How this relates to broader concepts

### Questions for Review
1. Question that tests understanding
2. Another question

Keep notes concise and exam-focused.`

	if content != "" {
		body += "\n\n### Source material to process:\n" + content
	}

	return buildSkillPrompt(skillTemplate{
		Title: "Study Notes Template: " + topic,
		Body:  body,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateSelfTest(topic, courseID, courseName string, count int, courseInterests, corpusContent string) string {
	body := fmt.Sprintf(`Generate %d exam-style questions about this topic. Mix these types:
- Conceptual understanding (explain in your own words)
- Calculation/application (solve a problem)
- Compare and contrast (differences and similarities)
- Identify the error (spot the mistake)

For each question, provide:
1. The question
2. A hint (in parentheses)
3. The expected answer

Format as a numbered quiz. Keep questions practical and exam-relevant.`, count)

	return buildSkillPrompt(skillTemplate{
		Title: "Self-Test: " + topic,
		Body:  body,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateReview(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Spaced Repetition Review: " + topic,
		Body: `Assess the student's understanding of this topic through spaced repetition review:

1. Start with 2-3 quick recall questions (one at a time)
2. Based on how well they answer:
   - If strong: suggest the next topic and mark for later review
   - If shaky: provide a focused refresher on weak areas
   - If new: recommend starting with the orientation skill

Keep it conversational. Ask one question at a time.`,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateGrillMe(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Grill Me: " + topic,
		Body: `You are in "grill me" mode. Interview the student relentlessly about their study plan, design decisions, or understanding of this topic until you reach a shared understanding.

Rules:
1. Walk down each branch of the decision tree, resolving dependencies between decisions one-by-one
2. Ask questions ONE AT A TIME — do not batch them
3. For each question, provide your recommended answer or perspective
4. If a question can be answered by exploring the course material or corpus, do so instead of asking the student
5. Be thorough but conversational — this is a dialogue, not an interrogation
6. Push back gently when answers are vague or hand-wavy
7. Surface assumptions the student hasn't articulated
8. When all branches are resolved, summarize what was learned and any remaining open questions`,
		Footer: "Start by asking the first question.",
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

type skillTemplate struct {
	Title  string
	Body   string
	Footer string
}

func buildSkillPrompt(t skillTemplate, topic, courseID, courseName, courseInterests, corpusContent string) string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(t.Title)
	if courseName != "" {
		fmt.Fprintf(&b, " (%s)", courseName)
	}
	b.WriteString("\n\n")
	b.WriteString(t.Body)
	if courseInterests != "" {
		b.WriteString(courseInterests)
	}
	if corpusContent != "" {
		b.WriteString("\n\n### Relevant reference material:")
		b.WriteString(corpusContent)
	}
	fmt.Fprintf(&b, "\n\nTopic: %s\nCourse: %s (ID: %s)", topic, courseName, courseID)
	if t.Footer != "" {
		b.WriteString("\n\n")
		b.WriteString(t.Footer)
	}
	return b.String()
}
