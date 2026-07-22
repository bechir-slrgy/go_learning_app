package main

import "os"

type Config struct {
	Port        string
	DatabaseURL string
}

func LoadConfig() Config {
	return Config{
		Port:        envOr("PORT", "3000"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://taskuser:taskpass@localhost:5433/tasks?sslmode=disable"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
