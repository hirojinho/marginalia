// claw-cli is the agent's command-line surface into claw-study state.
// It is invoked by Pi via the bash tool. All subcommands write JSON
// (or markdown for `memory load`) to stdout. Errors go to stderr with
// non-zero exit codes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"study-app/agent"
)

const defaultUserID = "eduardo"

func main() {
	dbPath := os.Getenv("CLAW_STUDY_DB")
	if dbPath == "" {
		dbPath = "data/study.db"
	}
	os.Exit(run(os.Args, os.Stdout, os.Stderr, dbPath))
}

func run(args []string, stdout, stderr io.Writer, dbPath string) int {
	return runWithStdin(args, os.Stdin, stdout, stderr, dbPath)
}

func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: claw-cli <subcommand> [args]")
		return 2
	}
	switch args[1] {
	case "memory":
		return runMemory(args[2:], stdin, stdout, stderr, dbPath)
	default:
		fmt.Fprintf(stderr, "unknown subcommand: %q\n", args[1])
		return 2
	}
}

func runMemory(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: claw-cli memory <save|search|load> [args]")
		return 2
	}
	switch args[0] {
	case "save":
		return memorySave(args[1:], stdin, stdout, stderr, dbPath)
	case "search":
		return memorySearch(args[1:], stdout, stderr, dbPath)
	default:
		fmt.Fprintf(stderr, "unknown memory subcommand: %q\n", args[0])
		return 2
	}
}

type searchResult struct {
	ID       int64  `json:"id"`
	Kind     string `json:"kind"`
	CourseID string `json:"course_id,omitempty"`
	Title    string `json:"title,omitempty"`
	Snippet  string `json:"snippet"`
}

func memorySearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "search query (required)")
	course := fs.String("course", "", "course id (optional)")
	limit := fs.Int("limit", 20, "max results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		fmt.Fprintln(stderr, "memory search: --query is required")
		return 2
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		fmt.Fprintf(stderr, "init schema: %v\n", err)
		return 1
	}
	store := agent.NewMemoryStore(db)
	rows, err := store.Search(defaultUserID, *query, *course, *limit)
	if err != nil {
		fmt.Fprintf(stderr, "search: %v\n", err)
		return 1
	}
	out := make([]searchResult, 0, len(rows))
	for _, m := range rows {
		snippet := m.Body
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		out = append(out, searchResult{
			ID: m.ID, Kind: m.Kind, CourseID: m.CourseID,
			Title: m.Title, Snippet: snippet,
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"results": out})
	return 0
}

func memorySave(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kind := fs.String("kind", "", "memory kind (profile|feedback|project|reference)")
	course := fs.String("course", "", "course id (optional)")
	title := fs.String("title", "", "memory title")
	body := fs.String("body", "", "memory body, or `-` to read from stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *kind == "" || *body == "" {
		fmt.Fprintln(stderr, "memory save: --kind and --body are required")
		return 2
	}

	bodyText := *body
	if bodyText == "-" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read stdin: %v\n", err)
			return 1
		}
		bodyText = string(raw)
	}

	db, err := agent.OpenDB(dbPath)
	if err != nil {
		fmt.Fprintf(stderr, "open db: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := agent.InitSchema(db); err != nil {
		fmt.Fprintf(stderr, "init schema: %v\n", err)
		return 1
	}
	store := agent.NewMemoryStore(db)
	saved, err := store.Save(agent.Memory{
		UserID:   defaultUserID,
		CourseID: *course,
		Kind:     *kind,
		Title:    *title,
		Body:     bodyText,
	})
	if err != nil {
		fmt.Fprintf(stderr, "save: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{
		"id":    saved.ID,
		"kind":  saved.Kind,
		"title": saved.Title,
	})
	return 0
}
