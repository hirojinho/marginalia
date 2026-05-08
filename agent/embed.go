package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type embeddingResponse struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

func EmbedText(text string) ([]float32, error) {
	results, err := EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return results[0], nil
}

func EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	apiKey := getAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key for embeddings")
	}

	apiURL := getAPIURL()
	model := getEmbeddingModel()

	body := map[string]interface{}{
		"model": model,
		"input": texts,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding api error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding api status %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	results := make([][]float32, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(results) {
			results[d.Index] = d.Embedding
		}
	}

	if len(texts) > 1 {
		time.Sleep(100 * time.Millisecond)
	}

	return results, nil
}

func getEmbeddingModel() string {
	if m := os.Getenv("EMBEDDING_MODEL"); m != "" {
		return m
	}
	return "nomic-ai/nomic-embed-text-v1.5"
}

func getAPIURL() string {
	if u := os.Getenv("LLM_API_URL"); u != "" {
		return u
	}
	if u := os.Getenv("OPENCODE_API_URL"); u != "" {
		return u
	}
	return "https://opencode.ai/zen/go/v1"
}

func getAPIKey() string {
	if k := os.Getenv("LLM_API_KEY"); k != "" {
		return k
	}
	return os.Getenv("OPENCODE_API_KEY")
}
