package main

import "time"

type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type taskInput struct {
	Title string `json:"title"`
	Done  bool   `json:"done"`
}
