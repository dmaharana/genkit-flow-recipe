package config

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Port     string
}

// Load initializes the configuration from .env file, environment variables, and command-line flags.
func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables or flags")
	}

	c := &Config{}

	// Define flags
	providerFlag := flag.String("provider", getEnv("LLM_PROVIDER", "ollama"), "LLM provider (e.g., ollama, openai)")
	baseURLFlag := flag.String("base-url", getEnv("LLM_BASE_URL", "http://127.0.0.1:11434/v1"), "LLM base URL")
	apiKeyFlag := flag.String("api-key", getEnv("LLM_API_KEY", "ollama"), "LLM API key")
	modelFlag := flag.String("model", getEnv("LLM_MODEL", "qwen3:0.6b"), "LLM model name")
	portFlag := flag.String("port", getEnv("PORT", "3400"), "Server port")
	flag.Parse()

	c.Provider = *providerFlag
	c.BaseURL = *baseURLFlag
	c.APIKey = *apiKeyFlag
	c.Model = *modelFlag
	c.Port = *portFlag

	return c
}

// LLMModel returns the full model identifier (e.g., "ollama/qwen3:0.6b").
func (c *Config) LLMModel() string {
	return fmt.Sprintf("%s/%s", c.Provider, c.Model)
}

// Addr returns the server address string.
func (c *Config) Addr() string {
	return fmt.Sprintf("0.0.0.0:%s", c.Port)
}

// getEnv retrieves the value of the environment variable named by the key.
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
