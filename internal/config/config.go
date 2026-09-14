package config

import (
	"os"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port         string
	DatabaseURL  string
	AdminToken   string
	AllowOrigins string // comma-separated list of allowed CORS origins
}

// Load reads configuration from the environment. It does not read .env files
// itself — call godotenv.Load() earlier in main() for local development.
func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		AdminToken:   getEnv("ADMIN_TOKEN", ""),
		AllowOrigins: getEnv("ALLOW_ORIGINS", "*"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
