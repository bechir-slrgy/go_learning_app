package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	pool := mustConnect()
	defer pool.Close()

	repo := NewRepo(pool)
	handler := NewHandler(repo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Fatal(http.ListenAndServe(":"+port, handler.Router()))
}
