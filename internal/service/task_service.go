package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

type TaskRepo interface {
	ListPaged(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.Task, error)
	CountByUser(ctx context.Context, userID uuid.UUID) (int, error)
	Get(ctx context.Context, userID, id uuid.UUID) (model.Task, error)
	Create(ctx context.Context, userID uuid.UUID, title string) (model.Task, error)
	Update(ctx context.Context, userID, id uuid.UUID, title string) (model.Task, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error

	GetAny(ctx context.Context, id uuid.UUID) (model.Task, error)
	ListByStatus(ctx context.Context, status model.TaskStatus) ([]model.Task, error)
	SetStatus(ctx context.Context, id uuid.UUID, status model.TaskStatus) (model.Task, error)
	SetReviewed(ctx context.Context, id uuid.UUID, status model.TaskStatus, reviewerID uuid.UUID) (model.Task, error)
}

type Alerter interface {
	TaskSubmitted(ctx context.Context, t model.Task, submitter model.User)
	TaskReviewed(ctx context.Context, t model.Task, reviewer model.User)
}

type Notifier interface {
	TaskCreated(ctx context.Context, userID uuid.UUID, t model.Task)
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

func (s *TaskService) List(ctx context.Context, userID uuid.UUID, page, pageSize int) (model.Page[model.Task], error) {
	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return model.Page[model.Task]{}, err
	}

	offset := (page - 1) * pageSize
	items, err := s.repo.ListPaged(ctx, userID, pageSize, offset)
	if err != nil {
		return model.Page[model.Task]{}, err
	}

	return model.NewPage(items, page, pageSize, total), nil
}

func (s *TaskService) Get(ctx context.Context, userID, id uuid.UUID) (model.Task, error) {
	return s.repo.Get(ctx, userID, id)
}

func (s *TaskService) Create(ctx context.Context, userID uuid.UUID, in model.TaskInput) (model.Task, error) {
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

func (s *TaskService) Update(ctx context.Context, userID, id uuid.UUID, in model.TaskInput) (model.Task, error) {
	if err := in.Validate(); err != nil {
		return model.Task{}, err
	}
	return s.repo.Update(ctx, userID, id, in.Title)
}

func (s *TaskService) Submit(ctx context.Context, submitter model.User, id uuid.UUID) (model.Task, error) {
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

func (s *TaskService) Review(ctx context.Context, reviewer model.User, id uuid.UUID, decision model.TaskStatus) (model.Task, error) {
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

func (s *TaskService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

const importSource = "https://jsonplaceholder.typicode.com/todos?_limit=%d"

const importWorkers = 5

const maxCSVRows = 5000

type externalTodo struct {
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type importJob struct {
	idx int
	td  externalTodo
}

type importResult struct {
	idx  int
	task model.Task
	ok   bool
	err  error
}

func (s *TaskService) Import(ctx context.Context, userID uuid.UUID, limit int) ([]model.Task, error) {
	if limit < 1 || limit > 20 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 20", model.ErrInvalid)
	}

	var todos []externalTodo
	if err := s.fetcher.GetJSON(ctx, fmt.Sprintf(importSource, limit), &todos); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrUpstream, err)
	}
	return s.importTodos(ctx, userID, todos)
}

func (s *TaskService) ImportCSV(ctx context.Context, userID uuid.UUID, r io.Reader) ([]model.Task, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: parse csv: %v", model.ErrInvalid, err)
	}

	todos := make([]externalTodo, 0, len(records))
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		title := strings.TrimSpace(rec[0])
		if i == 0 && strings.EqualFold(title, "title") {
			continue
		}
		if title == "" {
			continue
		}
		completed := false
		if len(rec) > 1 {
			completed = parseCSVBool(rec[1])
		}
		todos = append(todos, externalTodo{Title: title, Completed: completed})
		if len(todos) >= maxCSVRows {
			break
		}
	}

	return s.importTodos(ctx, userID, todos)
}

func parseCSVBool(s string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(s))
	return err == nil && b
}

func (s *TaskService) importTodos(ctx context.Context, userID uuid.UUID, todos []externalTodo) ([]model.Task, error) {
	if len(todos) == 0 {
		return []model.Task{}, nil
	}

	jobs := make(chan importJob)
	results := make(chan importResult, len(todos))

	workers := importWorkers
	if len(todos) < workers {
		workers = len(todos)
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				results <- s.importOne(ctx, userID, job)
			}
		}()
	}

	go func() {
		for i, td := range todos {
			jobs <- importJob{idx: i, td: td}
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	ordered := make([]model.Task, len(todos))
	present := make([]bool, len(todos))
	var firstErr error
	for res := range results {
		switch {
		case res.err != nil:
			if firstErr == nil {
				firstErr = res.err
			}
		case res.ok:
			ordered[res.idx] = res.task
			present[res.idx] = true
		}
	}

	imported := make([]model.Task, 0, len(todos))
	for i := range ordered {
		if present[i] {
			imported = append(imported, ordered[i])
		}
	}
	return imported, firstErr
}

func (s *TaskService) importOne(ctx context.Context, userID uuid.UUID, job importJob) importResult {
	in := model.TaskInput{Title: job.td.Title}
	if err := in.Validate(); err != nil {
		return importResult{idx: job.idx}
	}

	t, err := s.repo.Create(ctx, userID, in.Title)
	if err != nil {
		return importResult{idx: job.idx, err: err}
	}

	if job.td.Completed {
		t, err = s.repo.SetStatus(ctx, t.ID, model.StatusSubmitted)
		if err != nil {
			return importResult{idx: job.idx, err: err}
		}
	}

	return importResult{idx: job.idx, task: t, ok: true}
}
