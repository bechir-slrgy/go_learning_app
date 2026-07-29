package service

import (
	"context"
	"log"
	"sync"
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

const (
	deliveryTimeout    = 10 * time.Second
	webhookConcurrency = 8
)

type deliveryResult struct {
	hook model.Webhook
	err  error
}

func (s *WebhookService) TaskCreated(ctx context.Context, userID uuid.UUID, t model.Task) {
	event := model.TaskEvent{Event: "task.created", Task: t, SentAt: time.Now()}
	go s.deliver(context.WithoutCancel(ctx), userID, event)
}

func (s *WebhookService) deliver(ctx context.Context, userID uuid.UUID, event model.TaskEvent) {
	hooks, err := s.repo.List(ctx, userID)
	if err != nil {
		log.Printf("webhook: list for user %s: %v", userID, err)
		return
	}
	if len(hooks) == 0 {
		return
	}

	sem := make(chan struct{}, webhookConcurrency)
	results := make(chan deliveryResult, len(hooks))
	var wg sync.WaitGroup

	for _, h := range hooks {
		wg.Add(1)
		go func(h model.Webhook) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			hookCtx, cancel := context.WithTimeout(ctx, deliveryTimeout)
			defer cancel()

			results <- deliveryResult{hook: h, err: s.poster.PostJSON(hookCtx, h.URL, event, nil)}
		}(h)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var delivered, failed int
	for res := range results {
		if res.err != nil {
			failed++
			log.Printf("webhook %s (%s): %v", res.hook.ID, res.hook.URL, res.err)
			continue
		}
		delivered++
		log.Printf("webhook %s (%s): delivered", res.hook.ID, res.hook.URL)
	}
	log.Printf("webhook: user %s event %s -> %d delivered, %d failed of %d", userID, event.Event, delivered, failed, len(hooks))
}
