package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("ECHO_PORT")
	if port == "" {
		port = "9999"
	}

	http.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		fmt.Printf("\n--- %s %s ---\nContent-Type: %s\n%s\n",
			r.Method, r.URL.Path, r.Header.Get("Content-Type"), body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true}`)
	})

	log.Printf("echo receiver listening on http://localhost:%s/hook", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
