package agent

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEmbeddingSerdeRoundtrip(t *testing.T) {
	cases := [][]float32{
		{},
		{0},
		{1.5, -2.25, 0, 1e-9, 1e9},
		{float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())},
	}

	for i, in := range cases {
		bytes := serializeEmbedding(in)
		out, err := deserializeEmbedding(bytes)
		if err != nil {
			t.Fatalf("case %d: deserialize error: %v", i, err)
		}
		if len(out) != len(in) {
			t.Fatalf("case %d: length mismatch got %d want %d", i, len(out), len(in))
		}
		for j := range in {
			a, b := in[j], out[j]
			isNaN := math.IsNaN(float64(a)) && math.IsNaN(float64(b))
			if !isNaN && a != b {
				t.Fatalf("case %d index %d: got %v want %v", i, j, b, a)
			}
		}
	}
}

func TestDeserializeEmbeddingInvalidLength(t *testing.T) {
	if _, err := deserializeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for length-3 buffer, got nil")
	}
	if _, err := deserializeEmbedding([]byte{1}); err == nil {
		t.Fatal("expected error for length-1 buffer, got nil")
	}
}

func TestSerializeEmbeddingByteLength(t *testing.T) {
	got := serializeEmbedding([]float32{1, 2, 3, 4})
	if len(got) != 16 {
		t.Fatalf("expected 16 bytes for 4 floats, got %d", len(got))
	}
}

func TestCosineSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical vectors", []float32{1, 0, 0}, []float32{1, 0, 0}, 1.0},
		{"opposite vectors", []float32{1, 0, 0}, []float32{-1, 0, 0}, -1.0},
		{"orthogonal vectors", []float32{1, 0, 0}, []float32{0, 1, 0}, 0.0},
		{"length mismatch yields zero", []float32{1, 2}, []float32{1, 2, 3}, 0.0},
		{"zero vector yields zero", []float32{0, 0, 0}, []float32{1, 1, 1}, 0.0},
		{"empty vectors yield zero", []float32{}, []float32{}, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CosineSimilarity(tc.a, tc.b)
			if math.Abs(got-tc.want) > 1e-6 {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCosineSimilarityScaledVectorsEqualUnitCosine(t *testing.T) {
	a := []float32{3, 4}
	b := []float32{6, 8} // colinear, so cos = 1
	got := CosineSimilarity(a, b)
	if math.Abs(got-1.0) > 1e-6 {
		t.Fatalf("colinear scaled vectors: got %v, want ~1", got)
	}
}

func TestEmbedText(t *testing.T) {
	cases := []struct {
		name string
		in   Chunk
		want string
	}{
		{"both headings", Chunk{ParentHeading: "Course", Heading: "Topic", Content: "body"}, "Course > Topic\nbody"},
		{"only heading", Chunk{Heading: "Topic", Content: "body"}, "Topic\nbody"},
		{"only parent", Chunk{ParentHeading: "Course", Content: "body"}, "Course\nbody"},
		{"no headings", Chunk{Content: "body"}, "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := embedText(tc.in); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestKeywordSearchMatchesContent(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init vector store: %v", err)
	}
	now := time.Now().Format(time.RFC3339)
	rows := []struct{ path, heading, parent, content, course string }{
		{"a.md", "Replication", "Systems", "Leader-based replication overview", "cs101"},
		{"b.md", "Sharding", "Systems", "Range-based partitioning", "cs101"},
		{"c.md", "STPA", "Safety", "control structure analysis", "biology"},
	}
	for _, r := range rows {
		if _, err := a.DB.Exec(
			`INSERT INTO corpus_chunks (path, heading, parent_heading, content, course_id, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'concept', ?, ?)`,
			r.path, r.heading, r.parent, r.content, r.course, now, now,
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := a.keywordSearch("replication", "", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 || got[0].SourceFile != "a.md" {
		t.Fatalf("expected 1 match (a.md), got %+v", got)
	}

	scoped, err := a.keywordSearch("Systems", "cs101", 10)
	if err != nil {
		t.Fatalf("scoped search: %v", err)
	}
	if len(scoped) != 2 {
		t.Fatalf("expected 2 Systems matches, got %d", len(scoped))
	}

	none, err := a.keywordSearch("nothing", "", 10)
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("expected empty non-nil slice, got %v", none)
	}
}

func TestNeedsReindex(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	corpusDir := a.VaultPath("data", "corpus")
	if err := os.MkdirAll(corpusDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	abs := filepath.Join(corpusDir, "note.md")
	if err := os.WriteFile(abs, []byte("# Title\n\nbody"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Not yet indexed → needs reindex.
	need, err := a.NeedsReindex(abs)
	if err != nil {
		t.Fatalf("needs reindex: %v", err)
	}
	if !need {
		t.Fatal("expected needs reindex when no chunks exist")
	}

	// Insert a chunk with updated_at in the future, with embedding present.
	future := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	if _, err := a.DB.Exec(
		`INSERT INTO corpus_chunks (path, heading, parent_heading, content, embedding, category, created_at, updated_at) VALUES (?, '', '', ?, ?, 'concept', ?, ?)`,
		"note.md", "body", []byte{0, 0, 0, 0}, future, future,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	need, err = a.NeedsReindex(abs)
	if err != nil {
		t.Fatalf("needs reindex (fresh): %v", err)
	}
	if need {
		t.Fatal("expected no reindex when DB row is newer than file mtime")
	}
}
