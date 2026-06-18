// Command seed-memory imports a directory of markdown memory files into the
// agent_memory SQLite table. Point -source at your memory directory.
// Idempotent: deletes all rows for the user before reseeding.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"study-app/agent"
)

var userID = "default"

func main() {
	source := flag.String("source", "./memory", "source memory directory")
	userFlag := flag.String("user", userID, "user id to seed memory under")
	dbPath := flag.String("db", "data/study.db", "study.db path")
	dryRun := flag.Bool("dry-run", false, "print what would be inserted; do not write")
	flag.Parse()
	userID = *userFlag

	db, err := agent.OpenDB(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	rows, err := collect(*source)
	if err != nil {
		log.Fatalf("collect: %v", err)
	}
	log.Printf("collected %d candidate rows from %s", len(rows), *source)

	if *dryRun {
		for _, r := range rows {
			fmt.Printf("[%s] course=%q title=%q (%d bytes)\n", r.Kind, r.CourseID, r.Title, len(r.Body))
		}
		return
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("begin tx: %v", err)
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.Printf("rollback: %v", rollbackErr)
			}
		}
	}()
	if _, err := tx.Exec(`DELETE FROM agent_memory WHERE user_id = ?`, userID); err != nil {
		log.Fatalf("clear: %v", err)
	}
	stmt, err := tx.Prepare(`INSERT INTO agent_memory (user_id, course_id, kind, title, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatalf("prepare: %v", err)
	}
	defer func() { _ = stmt.Close() }()
	for _, r := range rows {
		r.UserID = userID
		var courseID interface{}
		if r.CourseID != "" {
			courseID = r.CourseID
		}
		if _, err := stmt.Exec(r.UserID, courseID, r.Kind, r.Title, r.Body, r.CreatedAt, r.CreatedAt); err != nil {
			log.Fatalf("insert %q: %v", r.Title, err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	committed = true
	log.Printf("seeded %d rows", len(rows))
}

func collect(root string) ([]agent.Memory, error) {
	var out []agent.Memory
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		if filepath.Base(path) == "MEMORY.md" {
			return nil
		}
		fm, body, err := parseFile(path)
		if err != nil {
			return nil
		}
		kind := mapKind(fm["type"])
		course := deriveCourseID(path, root)
		if course == "" && kind != "profile" && kind != "feedback" {
			return nil
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		out = append(out, agent.Memory{
			CourseID:  course,
			Kind:      kind,
			Title:     fm["name"],
			Body:      strings.TrimSpace(body),
			CreatedAt: info.ModTime().Unix(),
		})
		return nil
	})
	return out, err
}

func mapKind(t string) string {
	switch t {
	case "user":
		return "profile"
	case "feedback":
		return "feedback"
	case "project":
		return "project"
	case "reference":
		return "reference"
	default:
		return "project"
	}
}

func deriveCourseID(path, root string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 2 && parts[0] == "courses" {
		return parts[1]
	}
	return ""
}

func parseFile(path string) (map[string]string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024), 1024*1024)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, "", fmt.Errorf("no frontmatter in %s", path)
	}
	fm := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		fm[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	if len(fm) == 0 {
		return nil, "", fmt.Errorf("empty frontmatter in %s", path)
	}
	var body strings.Builder
	for sc.Scan() {
		body.WriteString(sc.Text())
		body.WriteString("\n")
	}
	if err := sc.Err(); err != nil {
		return nil, "", err
	}
	return fm, body.String(), nil
}
