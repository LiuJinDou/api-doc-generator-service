package config

import (
	"os"
)

type Config struct {
	Server  ServerConfig
	Git     GitConfig
	Webhook WebhookConfig
	Apifox  ApifoxConfig
	Storage StorageConfig
}

type ServerConfig struct {
	Port string
}

type GitConfig struct {
	WorkDir string
}

type WebhookConfig struct {
	Secret string
}

type ApifoxConfig struct {
	Token     string
	ProjectID string
	BaseURL   string
}

type StorageConfig struct {
	Enabled bool
}

// Load configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Git: GitConfig{
			WorkDir: getEnv("GIT_WORK_DIR", "/tmp/repos"),
		},
		Webhook: WebhookConfig{
			Secret: getEnv("WEBHOOK_SECRET", ""),
		},
		Apifox: ApifoxConfig{
			Token:     getEnv("APIFOX_TOKEN", ""),
			ProjectID: getEnv("APIFOX_PROJECT_ID", ""),
			BaseURL:   getEnv("APIFOX_BASE_URL", "https://api.apifox.cn"),
		},
		Storage: StorageConfig{
			Enabled: getEnv("STORAGE_ENABLED", "false") == "true",
		},
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
