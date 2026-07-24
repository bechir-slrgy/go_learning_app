package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type WebhookRepo interface {
	List(ctx context.Context, userID uuid.UUID) ([]model.Webhook, error)
	Create(ctx context.Context, userID uuid.UUID, url string) (model.Webhook, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
}

type Poster interface {
	PostJSON(ctx context.Context, url string, body, out any) error
}

type WebhookService struct {
	repo   WebhookRepo
	poster Poster
}

func NewWebhookService(repo WebhookRepo, poster Poster) *WebhookService {
	return &WebhookService{repo: repo, poster: poster}
}

func (s *WebhookService) List(ctx context.Context, userID uuid.UUID) ([]model.Webhook, error) {
	return s.repo.List(ctx, userID)
}

func (s *WebhookService) Create(ctx context.Context, userID uuid.UUID, in model.WebhookInput) (model.Webhook, error) {
	if err := in.Validate(); err != nil {
		return model.Webhook{}, err
	}
	return s.repo.Create(ctx, userID, in.URL)
}

func (s *WebhookService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

const deliveryTimeout = 20 * time.Second

func (s *WebhookService) TaskCreated(ctx context.Context, userID uuid.UUID, t model.Task) {
	event := model.TaskEvent{Event: "task.created", Task: t, SentAt: time.Now()}
	go s.deliver(context.WithoutCancel(ctx), userID, event)
}

func (s *WebhookService) deliver(ctx context.Context, userID uuid.UUID, event model.TaskEvent) {
	ctx, cancel := context.WithTimeout(ctx, deliveryTimeout)
	defer cancel()

	hooks, err := s.repo.List(ctx, userID)
	if err != nil {
		log.Printf("webhook: list for user %s: %v", userID, err)
		return
	}

	for _, h := range hooks {
		if err := s.poster.PostJSON(ctx, h.URL, event, nil); err != nil {
			log.Printf("webhook %s (%s): %v", h.ID, h.URL, err)
			continue
		}
		log.Printf("webhook %s (%s): delivered", h.ID, h.URL)
	}
}
