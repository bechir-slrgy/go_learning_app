package service

import (
	"context"
	"fmt"
	"log"

	"task_crud_api/internal/model"
)

type NotificationRepo interface {
	List(ctx context.Context, userID int) ([]model.Notification, error)
	Create(ctx context.Context, userID, taskID int, message string) (model.Notification, error)
	MarkRead(ctx context.Context, userID, id int) error
}

type UserLister interface {
	ListByRole(ctx context.Context, role model.Role) ([]model.User, error)
}

type NotificationService struct {
	repo  NotificationRepo
	users UserLister
}

func NewNotificationService(repo NotificationRepo, users UserLister) *NotificationService {
	return &NotificationService{repo: repo, users: users}
}

func (s *NotificationService) List(ctx context.Context, userID int) ([]model.Notification, error) {
	return s.repo.List(ctx, userID)
}

func (s *NotificationService) MarkRead(ctx context.Context, userID, id int) error {
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *NotificationService) TaskSubmitted(ctx context.Context, t model.Task, submitter model.User) {
	admins, err := s.users.ListByRole(ctx, model.RoleAdmin)
	if err != nil {
		log.Printf("notify: list admins: %v", err)
		return
	}

	msg := fmt.Sprintf("%s submitted %q for review", submitter.Name, t.Title)
	for _, admin := range admins {
		if _, err := s.repo.Create(ctx, admin.ID, t.ID, msg); err != nil {
			log.Printf("notify admin %d: %v", admin.ID, err)
		}
	}
}

func (s *NotificationService) TaskReviewed(ctx context.Context, t model.Task, reviewer model.User) {
	msg := fmt.Sprintf("%s %s your task %q", reviewer.Name, t.Status, t.Title)
	if _, err := s.repo.Create(ctx, t.UserID, t.ID, msg); err != nil {
		log.Printf("notify owner %d: %v", t.UserID, err)
	}
}
