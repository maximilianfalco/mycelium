package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey        string
	DatabaseURL         string
	EmbeddingModel      string
	ChatModel           string
	MaxEmbeddingBatch   int
	MaxContextTokens    int
	MaxAutoReindexFiles int
	ServerPort          string
}

func Load() (*Config, error) {
	// .env is optional — environment variables take precedence
	_ = godotenv.Load()

	cfg := &Config{
		OpenAIAPIKey:        os.Getenv("OPENAI_API_KEY"),
		DatabaseURL:         getEnvDefault("DATABASE_URL", "postgresql://mycelium:mycelium@localhost:5433/mycelium"),
		EmbeddingModel:      getEnvDefault("EMBEDDING_MODEL", "text-embedding-3-small"),
		ChatModel:           getEnvDefault("CHAT_MODEL", "gpt-4o"),
		MaxEmbeddingBatch:   getEnvInt("MAX_EMBEDDING_BATCH", 2048),
		MaxContextTokens:    getEnvInt("MAX_CONTEXT_TOKENS", 8000),
		MaxAutoReindexFiles: getEnvInt("MAX_AUTO_REINDEX_FILES", 100),
		ServerPort:          getEnvDefault("SERVER_PORT", "8080"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
