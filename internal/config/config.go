package config

import (
	"os"
	"strconv"
)

// Config holds the service configuration
type Config struct {
	// Server settings
	Host string
	Port int

	// Database settings
	DBPath string

	// LLM settings
	LLMBaseURL string
	LLMModel   string
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Host:       getEnv("QUERY_API_HOST", "0.0.0.0"),
		Port:       getEnvInt("QUERY_API_PORT", 8081),
		DBPath:     getEnv("DB_PATH", "../whatssapBot/whatsapp_bot.db"),
		LLMBaseURL: getEnv("LLM_BASE_URL", "http://localhost:1234"),
		LLMModel:   getEnv("LLM_MODEL", "llama-3.2-3b-instruct"),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
