package service

import (
	"context"
	"fmt"

	"task_crud_api/internal/model"
)

type TaskRepo interface {
	List(ctx context.Context, userID int) ([]model.Task, error)
	Get(ctx context.Context, userID, id int) (model.Task, error)
	Create(ctx context.Context, userID int, title string) (model.Task, error)
	Update(ctx context.Context, userID, id int, title string) (model.Task, error)
	Delete(ctx context.Context, userID, id int) error

	GetAny(ctx context.Context, id int) (model.Task, error)
	ListByStatus(ctx context.Context, status model.TaskStatus) ([]model.Task, error)
	SetStatus(ctx context.Context, id int, status model.TaskStatus) (model.Task, error)
	SetReviewed(ctx context.Context, id int, status model.TaskStatus, reviewerID int) (model.Task, error)
}

type Alerter interface {
	TaskSubmitted(ctx context.Context, t model.Task, submitter model.User)
	TaskReviewed(ctx context.Context, t model.Task, reviewer model.User)
}

type Notifier interface {
	TaskCreated(ctx context.Context, userID int, t model.Task)
}

type Getter interface {
	GetJSON(ctx context.Context, url string, out any) error
}

type TaskService struct {
	repo     TaskRepo
	notifier Notifier
	fetcher  Getter
	alerts   Alerter
}

func NewTaskService(repo TaskRepo, notifier Notifier, fetcher Getter, alerts Alerter) *TaskService {
	return &TaskService{repo: repo, notifier: notifier, fetcher: fetcher, alerts: alerts}
}

func (s *TaskService) List(ctx context.Context, userID int) ([]model.Task, error) {
	return s.repo.List(ctx, userID)
}

func (s *TaskService) Get(ctx context.Context, userID, id int) (model.Task, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *TaskService) Create(ctx context.Context, userID int, in model.TaskInput) (model.Task, error) {
	if err := in.Validate(); err != nil {
		return model.Task{}, err
	}
	t, err := s.repo.Create(ctx, userID, in.Title)
	if err != nil {
		return model.Task{}, err
	}
	if s.notifier != nil {
		s.notifier.TaskCreated(ctx, userID, t)
	}
	return t, nil
}

func (s *TaskService) Update(ctx context.Context, userID, id int, in model.TaskInput) (model.Task, error) {
	if err := in.Validate(); err != nil {
		return model.Task{}, err
	}
	return s.repo.Update(ctx, userID, id, in.Title)
}

func (s *TaskService) Submit(ctx context.Context, submitter model.User, id int) (model.Task, error) {
	t, err := s.repo.Get(ctx, submitter.ID, id)
	if err != nil {
		return model.Task{}, err
	}
	if !t.Status.CanTransitionTo(model.StatusSubmitted) {
		return model.Task{}, fmt.Errorf("%w: cannot submit a task that is %s", model.ErrConflict, t.Status)
	}

	t, err = s.repo.SetStatus(ctx, id, model.StatusSubmitted)
	if err != nil {
		return model.Task{}, err
	}
	s.alerts.TaskSubmitted(ctx, t, submitter)
	return t, nil
}

func (s *TaskService) Review(ctx context.Context, reviewer model.User, id int, decision model.TaskStatus) (model.Task, error) {
	if decision != model.StatusApproved && decision != model.StatusRejected {
		return model.Task{}, fmt.Errorf("%w: decision must be approved or rejected", model.ErrInvalid)
	}
	if !reviewer.Role.IsAdmin() {
		return model.Task{}, model.ErrForbidden
	}

	t, err := s.repo.GetAny(ctx, id)
	if err != nil {
		return model.Task{}, err
	}
	if !t.Status.CanTransitionTo(decision) {
		return model.Task{}, fmt.Errorf("%w: cannot %s a task that is %s", model.ErrConflict, decision, t.Status)
	}

	t, err = s.repo.SetReviewed(ctx, id, decision, reviewer.ID)
	if err != nil {
		return model.Task{}, err
	}
	s.alerts.TaskReviewed(ctx, t, reviewer)
	return t, nil
}

func (s *TaskService) ReviewQueue(ctx context.Context, reviewer model.User, status model.TaskStatus) ([]model.Task, error) {
	if !reviewer.Role.IsAdmin() {
		return nil, model.ErrForbidden
	}
	if !status.Valid() {
		return nil, fmt.Errorf("%w: unknown status %q", model.ErrInvalid, status)
	}
	return s.repo.ListByStatus(ctx, status)
}

func (s *TaskService) Delete(ctx context.Context, userID, id int) error {
	return s.repo.Delete(ctx, userID, id)
}

const importSource = "https://jsonplaceholder.typicode.com/todos?_limit=%d"

type externalTodo struct {
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

func (s *TaskService) Import(ctx context.Context, userID, limit int) ([]model.Task, error) {
	if limit < 1 || limit > 20 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 20", model.ErrInvalid)
	}

	var todos []externalTodo
	if err := s.fetcher.GetJSON(ctx, fmt.Sprintf(importSource, limit), &todos); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrUpstream, err)
	}

	imported := []model.Task{}
	for _, td := range todos {
		in := model.TaskInput{Title: td.Title}
		if err := in.Validate(); err != nil {
			continue
		}

		t, err := s.repo.Create(ctx, userID, in.Title)
		if err != nil {
			return imported, err
		}

		if td.Completed {
			if t, err = s.repo.SetStatus(ctx, t.ID, model.StatusSubmitted); err != nil {
				return imported, err
			}
		}
		imported = append(imported, t)
	}
	return imported, nil
}
