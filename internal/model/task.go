package model

import (
	"fmt"
	"strings"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusSubmitted TaskStatus = "submitted"
	StatusApproved  TaskStatus = "approved"
	StatusRejected  TaskStatus = "rejected"
)

func (s TaskStatus) CanTransitionTo(next TaskStatus) bool {
	switch s {
	case StatusPending:
		return next == StatusSubmitted
	case StatusSubmitted:
		return next == StatusApproved || next == StatusRejected
	case StatusRejected:
		return next == StatusSubmitted
	case StatusApproved:
		return false
	}
	return false
}

func (s TaskStatus) Valid() bool {
	switch s {
	case StatusPending, StatusSubmitted, StatusApproved, StatusRejected:
		return true
	}
	return false
}

type Task struct {
	ID     int        `json:"id"`
	UserID int        `json:"user_id"`
	Title  string     `json:"title"`
	Status TaskStatus `json:"status"`

	ReviewedBy *int       `json:"reviewed_by"`
	ReviewedAt *time.Time `json:"reviewed_at"`

	CreatedAt time.Time `json:"created_at"`
}

type TaskInput struct {
	Title string `json:"title"`
}

func (in *TaskInput) Validate() error {
	in.Title = strings.TrimSpace(in.Title)

	if in.Title == "" {
		return fmt.Errorf("%w: title is required", ErrInvalid)
	}
	if len(in.Title) > 200 {
		return fmt.Errorf("%w: title must be 200 characters or fewer", ErrInvalid)
	}
	return nil
}
