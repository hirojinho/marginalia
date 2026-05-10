// Command claw-cli is the agent's command-line surface into claw-study state.
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
	"path/filepath"

	"study-app/agent"
)

const defaultUserID = "eduardo"

// resolveStudyRoot returns CLAW_STUDY_ROOT or empty.
func resolveStudyRoot() string {
	return os.Getenv("CLAW_STUDY_ROOT")
}

// resolveDBPath returns the effective study.db path. Order of precedence:
// explicit override > fallback (from main) > CLAW_STUDY_DB > CLAW_STUDY_ROOT/data/study.db.
// Returns an error if the final path is empty or does not exist on disk.
// claw-cli never bootstraps a fresh DB — seed-memory is the canonical first-run path.
func resolveDBPath(override, fallback string) (string, error) {
	resolved := override
	if resolved == "" {
		resolved = fallback
	}
	if resolved == "" {
		resolved = os.Getenv("CLAW_STUDY_DB")
	}
	if resolved == "" {
		if root := resolveStudyRoot(); root != "" {
			resolved = filepath.Join(root, "data", "study.db")
		}
	}
	if resolved == "" {
		return "", fmt.Errorf("no database path: pass --db, set CLAW_STUDY_DB, or set CLAW_STUDY_ROOT")
	}
	if _, err := os.Stat(resolved); err != nil {
		return "", fmt.Errorf("database not found at %q: %w", resolved, err)
	}
	return resolved, nil
}

// resolveSkillsDir returns the skills directory path. Order of precedence:
// explicit override > CLAW_STUDY_ROOT/skills > "skills". Does not Stat — a
// defaulted missing dir is OK (Phase 3 has not populated skills/ yet);
// callers must Stat themselves if they want to enforce existence on an
// explicit override.
func resolveSkillsDir(override string) string {
	if override != "" {
		return override
	}
	if root := resolveStudyRoot(); root != "" {
		return filepath.Join(root, "skills")
	}
	return "skills"
}

// newAppFromEnv constructs a fully-configured *agent.App from environment
// variables, mirroring the server's loadConfig. The dbPath argument is the
// already-resolved absolute path from resolveDBPath; this helper does NOT
// re-resolve. requireAPIKey controls whether a missing LLM_API_KEY/
// OPENCODE_API_KEY is a hard error: read-only subcommands pass false; RAG
// and any subcommand that hits the embedding API pass true.
func newAppFromEnv(dbPath string, requireAPIKey bool) (*agent.App, error) {
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENCODE_API_KEY")
	}
	if apiKey == "" && requireAPIKey {
		return nil, fmt.Errorf("LLM_API_KEY or OPENCODE_API_KEY must be set")
	}
	vaultRoot := os.Getenv("VAULT_ROOT")
	if vaultRoot == "" {
		vaultRoot = resolveStudyRoot()
	}
	if vaultRoot == "" {
		vaultRoot = "/workspace"
	}
	apiURL := os.Getenv("LLM_API_URL")
	if apiURL == "" {
		apiURL = os.Getenv("OPENCODE_API_URL")
	}
	if apiURL == "" {
		apiURL = "https://opencode.ai/zen/go/v1"
	}
	model := os.Getenv("LLM_MODEL")
	if model == "" {
		model = "qwen3.6-plus"
	}
	embeddingModel := os.Getenv("EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "nomic-ai/nomic-embed-text-v1.5"
	}
	cfg := agent.Config{
		VaultRoot:      vaultRoot,
		APIKey:         apiKey,
		APIURL:         apiURL,
		Model:          model,
		EmbeddingModel: embeddingModel,
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := agent.InitSchema(db); err != nil {
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return agent.NewApp(cfg, db), nil
}

func main() {
	dbPath := os.Getenv("CLAW_STUDY_DB")
	if dbPath == "" {
		if root := os.Getenv("CLAW_STUDY_ROOT"); root != "" {
			dbPath = filepath.Join(root, "data", "study.db")
		} else {
			dbPath = "data/study.db"
		}
	}
	os.Exit(run(os.Args, os.Stdout, os.Stderr, dbPath))
}

func run(args []string, stdout, stderr io.Writer, dbPath string) int {
	return runWithStdin(args, os.Stdin, stdout, stderr, dbPath)
}

func runWithStdin(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 2 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli <subcommand> [args]")
		return 2
	}
	switch args[1] {
	case "memory":
		return runMemory(args[2:], stdin, stdout, stderr, dbPath)
	case "rag":
		return runRag(args[2:], stdout, stderr, dbPath)
	case "plan":
		return runPlan(args[2:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown subcommand: %q\n", args[1])
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
	case "load":
		return memoryLoad(args[1:], stdout, stderr, dbPath)
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
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		fmt.Fprintln(stderr, "memory search: --query is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	db, err := agent.OpenDB(resolvedDB)
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
		out = append(out, searchResult{
			ID: m.ID, Kind: m.Kind, CourseID: m.CourseID,
			Title: m.Title, Snippet: agent.TruncateRunes(m.Body, 200),
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"results": out})
	return 0
}

func memoryLoad(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory load", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id")
	user := fs.String("user", defaultUserID, "user id")
	skillsDirFlag := fs.String("skills-dir", "", "directory containing SKILL.md files (default: $CLAW_STUDY_ROOT/skills, then 'skills')")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	_ = fs.String("session", "", "session id (informational; unused in v1)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	resolvedSkills := resolveSkillsDir(*skillsDirFlag)
	if *skillsDirFlag != "" {
		if _, statErr := os.Stat(resolvedSkills); statErr != nil {
			_, _ = fmt.Fprintf(stderr, "skills directory not found at %q: %v\n", resolvedSkills, statErr)
			return 1
		}
	}

	db, err := agent.OpenDB(resolvedDB)
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
	scope, err := store.LoadByScope(*user, *course)
	if err != nil {
		fmt.Fprintf(stderr, "load scope: %v\n", err)
		return 1
	}
	var recent []agent.SessionDigest
	if *course != "" {
		recent, err = agent.RecentSessionsForCourse(db, *course, 2)
		if err != nil {
			fmt.Fprintf(stderr, "recent sessions: %v\n", err)
			return 1
		}
	}
	skills, err := agent.ParseSkillsDir(resolvedSkills)
	if err != nil {
		fmt.Fprintf(stderr, "parse skills: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, agent.AssembleAgentsMD(scope, recent, skills, *course))
	return 0
}

func memorySave(args []string, stdin io.Reader, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("memory save", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kind := fs.String("kind", "", "memory kind (profile|feedback|project|reference)")
	course := fs.String("course", "", "course id (optional)")
	title := fs.String("title", "", "memory title")
	body := fs.String("body", "", "memory body, or `-` to read from stdin")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
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

	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	db, err := agent.OpenDB(resolvedDB)
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

func runRag(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli rag <search> [args]")
		return 2
	}
	switch args[0] {
	case "search":
		return ragSearch(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown rag subcommand: %q\n", args[0])
		return 2
	}
}

func runPlan(args []string, stdout, stderr io.Writer, dbPath string) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "usage: claw-cli plan <show|toggle> [args]")
		return 2
	}
	switch args[0] {
	case "show":
		return planShow(args[1:], stdout, stderr, dbPath)
	case "toggle":
		return planToggle(args[1:], stdout, stderr, dbPath)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown plan subcommand: %q\n", args[0])
		return 2
	}
}

func planShow(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id (required)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" {
		_, _ = fmt.Fprintln(stderr, "plan show: --course is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	plan := app.LoadPlan(*course)
	if plan == nil {
		_, _ = fmt.Fprintf(stderr, "plan not found for course %q\n", *course)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(plan)
	return 0
}

func planToggle(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan toggle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id / plan id (required)")
	taskIndex := fs.Int("task", -1, "task index in plan (required, ≥0)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" || *taskIndex < 0 {
		_, _ = fmt.Fprintln(stderr, "plan toggle: --course and --task (≥0) are required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	argsJSON, _ := json.Marshal(map[string]any{ // Marshal of string/int values cannot fail
		"plan_id":    *course,
		"action":     "toggle",
		"task_index": *taskIndex,
	})
	_, _ = fmt.Fprintln(stdout, app.ToolUpdatePlan(argsJSON))
	return 0
}

func ragSearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("rag search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	query := fs.String("query", "", "search query (required)")
	course := fs.String("course", "", "course id filter (optional)")
	topK := fs.Int("top-k", 3, "number of results (1-10)")
	dbOverride := fs.String("db", "", "path to study.db (default: $CLAW_STUDY_DB or $CLAW_STUDY_ROOT/data/study.db)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *query == "" {
		_, _ = fmt.Fprintln(stderr, "rag search: --query is required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, true)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	argsJSON, _ := json.Marshal(map[string]any{ // Marshal of string/int values cannot fail
		"query":  *query,
		"course": *course,
		"top_k":  *topK,
	})
	_, _ = fmt.Fprintln(stdout, app.ToolRAGSearch(argsJSON))
	return 0
}
