package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"task_crud_api/internal/middleware"
	"task_crud_api/internal/service"
)

type Server struct {
	tasks *service.TaskService
	users *service.UserService
	auth  *service.AuthService
	hooks *service.WebhookService
	notes *service.NotificationService
	guard *middleware.Auth
}

func NewServer(
	tasks *service.TaskService,
	users *service.UserService,
	auth *service.AuthService,
	hooks *service.WebhookService,
	notes *service.NotificationService,
	guard *middleware.Auth,
) *Server {
	return &Server{
		tasks: tasks,
		users: users,
		auth:  auth,
		hooks: hooks,
		notes: notes,
		guard: guard,
	}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(5 * time.Second))

	r.Get("/health", Health)

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", s.login)
			r.Post("/refresh", s.refresh)
			r.Post("/logout", s.logout)
		})

		r.Route("/users", func(r chi.Router) {
			r.Post("/", s.userCreate)

			r.Group(func(r chi.Router) {
				r.Use(s.guard.RequireUser)
				r.Get("/", s.userList)
				r.Get("/me", s.userMe)
				r.Get("/{id}", s.userGet)
				r.Put("/{id}", s.userUpdate)
				r.Delete("/{id}", s.userDelete)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(s.guard.RequireUser)

			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", s.taskList)
				r.Post("/", s.taskCreate)
				r.Post("/import", s.taskImport)
				r.Get("/{id}", s.taskGet)
				r.Put("/{id}", s.taskUpdate)
				r.Post("/{id}/submit", s.taskSubmit)
				r.Delete("/{id}", s.taskDelete)
			})

			r.Route("/webhooks", func(r chi.Router) {
				r.Get("/", s.webhookList)
				r.Post("/", s.webhookCreate)
				r.Delete("/{id}", s.webhookDelete)
			})

			r.Route("/notifications", func(r chi.Router) {
				r.Get("/", s.noteList)
				r.Post("/{id}/read", s.noteMarkRead)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Use(s.guard.RequireAdmin)
				r.Get("/tasks", s.adminQueue)
				r.Post("/tasks/{id}/approve", s.adminApprove)
				r.Post("/tasks/{id}/reject", s.adminReject)
			})
		})
	})

	return r
}
