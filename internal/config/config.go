package config

import (
	"os"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port         string
	DatabaseURL  string
	AllowOrigins string // comma-separated list of allowed CORS origins

	// WishesRequireApproval hides new guestbook messages until an admin
	// approves them. Off by default, so messages appear immediately.
	WishesRequireApproval bool
}

// Load reads configuration from the environment. It does not read .env files
// itself — call godotenv.Load() earlier in main() for local development.
func Load() Config {
	return Config{
		Port:         getEnv("PORT", "8080"),
		DatabaseURL:  getEnv("DATABASE_URL", ""),
		AllowOrigins: getEnv("ALLOW_ORIGINS", "*"),

		WishesRequireApproval: getEnv("WISHES_REQUIRE_APPROVAL", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
