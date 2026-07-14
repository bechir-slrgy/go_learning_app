package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
)

func mustConnect() *pgxpool.Pool {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://taskuser:taskpass@localhost:5433/tasks"
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("ping: %v", err)
	}
	log.Println("connected to postgres")
	return pool
}
