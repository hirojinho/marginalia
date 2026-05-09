package agent

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SearchResult struct {
	SourceFile    string
	Heading       string
	ParentHeading string
	Content       string
	Score         float64
}

// InitVectorStore creates the corpus_chunks table and its indexes.
// Idempotent.
func (a *App) InitVectorStore() error {
	_, err := a.DB.Exec(`
		CREATE TABLE IF NOT EXISTS corpus_chunks (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			path           TEXT NOT NULL,
			heading        TEXT NOT NULL DEFAULT '',
			parent_heading TEXT NOT NULL DEFAULT '',
			content        TEXT NOT NULL,
			embedding      BLOB,
			course_id      TEXT,
			category       TEXT NOT NULL DEFAULT 'concept',
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_corpus_chunks_course ON corpus_chunks(course_id);
		CREATE INDEX IF NOT EXISTS idx_corpus_chunks_path ON corpus_chunks(path);
	`)
	if err != nil {
		return fmt.Errorf("create corpus_chunks: %w", err)
	}
	return nil
}

// IndexCorpus walks the corpus directory and (re)indexes any markdown
// files whose mtime is newer than their stored chunks. Stale paths
// (files removed on disk) are deleted from the index.
func (a *App) IndexCorpus() error {
	corpusDir := a.VaultPath("data", "corpus")
	if _, err := os.Stat(corpusDir); os.IsNotExist(err) {
		return nil
	}

	var filesToIndex []string
	if err := filepath.WalkDir(corpusDir, func(absPath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(absPath, ".md") {
			return nil
		}
		needsIndex, err := a.NeedsReindex(absPath)
		if err != nil {
			slog.Warn("reindex check", "path", absPath, "err", err)
			return nil
		}
		if needsIndex {
			filesToIndex = append(filesToIndex, absPath)
		}
		return nil
	}); err != nil {
		slog.Warn("walk corpus dir", "dir", corpusDir, "err", err)
	}

	if len(filesToIndex) == 0 {
		slog.Info("corpus up to date")
		return nil
	}

	for _, f := range filesToIndex {
		if err := a.IndexFile(f); err != nil {
			slog.Error("index file failed", "path", f, "err", err)
		} else {
			slog.Info("indexed file", "path", f)
		}
	}

	a.deleteStalePaths()
	return nil
}

func (a *App) IndexFile(absPath string) error {
	corpusDir := a.VaultPath("data", "corpus")
	relPath, err := filepath.Rel(corpusDir, absPath)
	if err != nil {
		return fmt.Errorf("rel path: %w", err)
	}

	chunks, err := ChunkFile(absPath)
	if err != nil {
		return fmt.Errorf("chunk file: %w", err)
	}
	if len(chunks) == 0 {
		return nil
	}

	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = embedText(c)
	}

	embeddings, err := a.EmbedBatch(texts)
	if err != nil {
		// Fall back to per-text on batch failure.
		embeddings = make([][]float32, len(chunks))
		for i, t := range texts {
			emb, eErr := a.EmbedText(t)
			if eErr == nil {
				embeddings[i] = emb
			}
		}
	}

	now := time.Now().Format(time.RFC3339)
	for i, c := range chunks {
		var embedBytes []byte
		if i < len(embeddings) && len(embeddings[i]) > 0 {
			embedBytes = serializeEmbedding(embeddings[i])
		}
		if _, err := a.DB.Exec(
			`INSERT INTO corpus_chunks (path, heading, parent_heading, content, embedding, course_id, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			relPath, c.Heading, c.ParentHeading, c.Content, embedBytes, c.CourseID, c.Category, now, now,
		); err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
	}

	var embeddingsCount int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM corpus_chunks WHERE path = ? AND embedding IS NOT NULL", relPath).Scan(&embeddingsCount); err != nil {
		slog.Warn("count embeddings", "path", relPath, "err", err)
	}
	slog.Info("indexed chunks", "path", relPath, "chunks", len(chunks), "with_embeddings", embeddingsCount)
	return nil
}

func embedText(c Chunk) string {
	var parts []string
	if c.ParentHeading != "" {
		parts = append(parts, c.ParentHeading)
	}
	if c.Heading != "" {
		parts = append(parts, c.Heading)
	}
	header := strings.Join(parts, " > ")
	if header != "" {
		return header + "\n" + c.Content
	}
	return c.Content
}

func (a *App) NeedsReindex(absPath string) (bool, error) {
	fi, err := os.Stat(absPath)
	if err != nil {
		return false, err
	}
	fileMtime := fi.ModTime()

	corpusDir := a.VaultPath("data", "corpus")
	relPath, _ := filepath.Rel(corpusDir, absPath)

	var maxUpdatedAt string
	if err := a.DB.QueryRow("SELECT MAX(updated_at) FROM corpus_chunks WHERE path = ?", relPath).Scan(&maxUpdatedAt); err != nil || maxUpdatedAt == "" {
		return true, nil
	}

	var nullEmbeds int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM corpus_chunks WHERE path = ? AND embedding IS NULL", relPath).Scan(&nullEmbeds); err != nil {
		slog.Warn("count null embeddings", "path", relPath, "err", err)
	}
	if nullEmbeds > 0 {
		return true, nil
	}

	dbTime, err := time.Parse(time.RFC3339, maxUpdatedAt)
	if err != nil {
		return true, nil
	}
	return fileMtime.After(dbTime), nil
}

func (a *App) deleteStalePaths() {
	corpusDir := a.VaultPath("data", "corpus")
	rows, err := a.DB.Query("SELECT DISTINCT path FROM corpus_chunks")
	if err != nil {
		slog.Warn("list corpus paths for stale check", "err", err)
		return
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			slog.Warn("scan corpus path", "err", err)
			continue
		}
		paths = append(paths, p)
	}
	if err := rows.Err(); err != nil {
		slog.Warn("iterate corpus paths", "err", err)
	}

	for _, p := range paths {
		fullPath := filepath.Join(corpusDir, p)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if _, err := a.DB.Exec("DELETE FROM corpus_chunks WHERE path = ?", p); err != nil {
				slog.Warn("delete stale corpus chunks", "path", p, "err", err)
				continue
			}
			slog.Info("removed stale corpus chunks", "path", p)
		}
	}
}

const cosineSimilarityFloor = 0.3

func (a *App) Search(query string, course string, topK int) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 3
	}

	queryEmbed, err := a.EmbedText(query)
	if err != nil {
		return a.keywordSearch(query, course, topK)
	}

	sql := "SELECT path, heading, parent_heading, content, embedding FROM corpus_chunks"
	var args []interface{}
	if course != "" {
		sql += " WHERE course_id = ?"
		args = append(args, course)
	}

	rows, err := a.DB.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query corpus_chunks: %w", err)
	}
	defer rows.Close()

	type scored struct {
		result SearchResult
		score  float64
	}
	var scoredResults []scored
	for rows.Next() {
		var sourceFile, heading, parentHeading, content string
		var embedBytes []byte
		if err := rows.Scan(&sourceFile, &heading, &parentHeading, &content, &embedBytes); err != nil {
			continue
		}
		if len(embedBytes) == 0 {
			continue
		}
		embed, err := deserializeEmbedding(embedBytes)
		if err != nil {
			continue
		}
		score := CosineSimilarity(queryEmbed, embed)
		if score < cosineSimilarityFloor {
			continue
		}
		scoredResults = append(scoredResults, scored{
			result: SearchResult{
				SourceFile:    sourceFile,
				Heading:       heading,
				ParentHeading: parentHeading,
				Content:       content,
				Score:         score,
			},
			score: score,
		})
	}

	if len(scoredResults) == 0 {
		return a.keywordSearch(query, course, topK)
	}

	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})
	if topK > len(scoredResults) {
		topK = len(scoredResults)
	}
	results := make([]SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = scoredResults[i].result
	}
	return results, nil
}

func (a *App) keywordSearch(query, course string, topK int) ([]SearchResult, error) {
	sql := "SELECT path, heading, parent_heading, content FROM corpus_chunks WHERE (content LIKE ? OR heading LIKE ? OR parent_heading LIKE ?)"
	pattern := "%" + query + "%"
	args := []interface{}{pattern, pattern, pattern}
	if course != "" {
		sql += " AND course_id = ?"
		args = append(args, course)
	}
	sql += " LIMIT ?"
	args = append(args, topK)

	rows, err := a.DB.Query(sql, args...)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SourceFile, &r.Heading, &r.ParentHeading, &r.Content); err != nil {
			continue
		}
		r.Score = 0.5
		results = append(results, r)
	}
	if results == nil {
		return []SearchResult{}, nil
	}
	return results, nil
}

func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		va, vb := float64(a[i]), float64(b[i])
		dot += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func serializeEmbedding(embed []float32) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, embed)
	return buf.Bytes()
}

func deserializeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding data length: %d", len(data))
	}
	embed := make([]float32, len(data)/4)
	buf := bytes.NewReader(data)
	if err := binary.Read(buf, binary.LittleEndian, &embed); err != nil {
		return nil, err
	}
	return embed, nil
}
