package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every value the service reads from the environment.
// Nothing else in the codebase touches os.Getenv, so there is exactly one
// place to look when a deployment misbehaves.
type Config struct {
	Port         string
	Env          string
	MongoURI     string
	MongoDB      string
	RedisURL     string
	JWTSecret    string
	CORSOrigins  []string
	PublicAppURL string
}

func Load() *Config {
	// .env is a convenience for local dev; in production the platform injects
	// real environment variables and this call simply finds no file.
	_ = godotenv.Load()

	cfg := &Config{
		Port:         get("PORT", "8080"),
		Env:          get("APP_ENV", "development"),
		MongoURI:     get("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:      get("MONGO_DB", "livepolls"),
		RedisURL:     get("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:    get("JWT_SECRET", ""),
		PublicAppURL: get("PUBLIC_APP_URL", "http://localhost:5173"),
	}

	for _, o := range strings.Split(get("CORS_ORIGINS", "http://localhost:5173"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, o)
		}
	}

	if len(cfg.JWTSecret) < 32 {
		if cfg.IsProd() {
			log.Fatal("config: JWT_SECRET must be at least 32 characters in production")
		}
		log.Println("config: warning - weak JWT_SECRET, acceptable for local dev only")
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = "dev-only-insecure-secret-do-not-ship-anywhere"
		}
	}
	return cfg
}

func (c *Config) IsProd() bool { return c.Env == "production" }

func get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
