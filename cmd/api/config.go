package main

import (
	"crypto/rsa"
	"log"
	"os"
	"time"

	"task_crud_api/internal/auth"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
	SignKey     *rsa.PrivateKey
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
}

func LoadConfig() Config {
	return Config{
		Port:        envOr("PORT", "3000"),
		DatabaseURL: envOr("DATABASE_URL", "postgres://taskuser:taskpass@localhost:5433/tasks?sslmode=disable"),
		RedisAddr:   envOr("REDIS_ADDR", "localhost:6379"),
		SignKey:     loadSignKey(),
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
	}
}

func loadSignKey() *rsa.PrivateKey {
	path := os.Getenv("JWT_PRIVATE_KEY_PATH")
	if path == "" {
		log.Println("WARNING: JWT_PRIVATE_KEY_PATH is not set, generating an ephemeral RS256 dev key. Tokens will not survive a restart. Set JWT_PRIVATE_KEY_PATH in production.")
		key, err := auth.GenerateDevKey()
		if err != nil {
			log.Fatalf("generate dev signing key: %v", err)
		}
		return key
	}

	pemBytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read JWT_PRIVATE_KEY_PATH %q: %v", path, err)
	}
	key, err := auth.ParsePrivateKeyPEM(pemBytes)
	if err != nil {
		log.Fatalf("load signing key from %q: %v", path, err)
	}
	return key
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
