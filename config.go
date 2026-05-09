package main

import (
	"fmt"
	"os"

	"study-app/agent"
)

const (
	defaultAPIURL         = "https://opencode.ai/zen/go/v1"
	defaultModel          = "qwen3.6-plus"
	defaultEmbeddingModel = "nomic-ai/nomic-embed-text-v1.5"
	defaultListenAddr     = ":8081"
	defaultVaultRoot      = "/workspace"
)

// loadConfig builds a Config from environment variables and validates
// that any required values are present.
func loadConfig() (agent.Config, error) {
	apiKey := firstNonEmpty(os.Getenv("LLM_API_KEY"), os.Getenv("OPENCODE_API_KEY"))
	if apiKey == "" {
		return agent.Config{}, fmt.Errorf("LLM_API_KEY or OPENCODE_API_KEY must be set")
	}

	return agent.Config{
		VaultRoot:      firstNonEmpty(os.Getenv("VAULT_ROOT"), defaultVaultRoot),
		APIKey:         apiKey,
		APIURL:         firstNonEmpty(os.Getenv("LLM_API_URL"), os.Getenv("OPENCODE_API_URL"), defaultAPIURL),
		Model:          firstNonEmpty(os.Getenv("LLM_MODEL"), defaultModel),
		EmbeddingModel: firstNonEmpty(os.Getenv("EMBEDDING_MODEL"), defaultEmbeddingModel),
		ListenAddr:     firstNonEmpty(os.Getenv("LISTEN_ADDR"), defaultListenAddr),
		AuthToken:      os.Getenv("AUTH_TOKEN"),
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
