package main

import (
	"log"
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

const devJWTSecret = "dev-only-insecure-secret-change-me"

func LoadConfig() Config {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		log.Println("WARNING: JWT_SECRET is not set, using the insecure dev default. Set JWT_SECRET in production.")
		secret = devJWTSecret
	}

	return Config{
		Port:        envOr("PORT", "3000"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://taskuser:taskpass@localhost:5433/tasks?sslmode=disable"),
		JWTSecret:   secret,
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
