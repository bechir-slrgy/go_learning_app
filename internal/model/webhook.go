package model

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Webhook struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type WebhookInput struct {
	URL string `json:"url"`
}

func (in *WebhookInput) Validate() error {
	in.URL = strings.TrimSpace(in.URL)

	if in.URL == "" {
		return fmt.Errorf("%w: url is required", ErrInvalid)
	}

	u, err := url.ParseRequestURI(in.URL)
	if err != nil {
		return fmt.Errorf("%w: url is not a valid absolute URL", ErrInvalid)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: url must start with http:// or https://", ErrInvalid)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: url must include a host", ErrInvalid)
	}
	return nil
}

type TaskEvent struct {
	Event  string    `json:"event"`
	Task   Task      `json:"task"`
	SentAt time.Time `json:"sent_at"`
}
