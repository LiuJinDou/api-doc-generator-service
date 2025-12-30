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
	Port      string
	PublicURL string // 服务器的公网访问地址，用于生成docs的URL
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
	SyncMode  string // "string" 或 "url"，决定同步方式
}

type StorageConfig struct {
	Enabled bool
}

// Load configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:      getEnv("SERVER_PORT", "8080"),
			PublicURL: getEnv("SERVER_PUBLIC_URL", "http://localhost:8080"),
		},
		Git: GitConfig{
			WorkDir: getEnv("GIT_WORK_DIR", "/tmp/repos"),
		},
		Webhook: WebhookConfig{
			Secret: getEnv("WEBHOOK_SECRET", ""),
		},
		Apifox: ApifoxConfig{
			Token:     getEnv("APIFOX_TOKEN", "APS-TumcW0q4M0qKwZTHnVsqQt4uqYJNF2Hk"),
			ProjectID: getEnv("APIFOX_PROJECT_ID", "7606578"),
			BaseURL:   getEnv("APIFOX_BASE_URL", "https://api.apifox.com"),
			SyncMode:  getEnv("APIFOX_SYNC_MODE", "string"), // 默认string方式
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
