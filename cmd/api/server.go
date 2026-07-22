package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"task_crud_api/internal/client"
	"task_crud_api/internal/handler"
	"task_crud_api/internal/repository"
	"task_crud_api/internal/service"
)

type Server struct {
	cfg    Config
	db     *sql.DB
	router http.Handler
}

func NewServer(cfg Config) *Server {
	db := repository.MustConnect(cfg.DatabaseURL)

	api := client.New(10 * time.Second)

	userRepo := repository.NewUserRepo(db)

	hooks := service.NewWebhookService(repository.NewWebhookRepo(db), api)
	notes := service.NewNotificationService(repository.NewNotificationRepo(db), userRepo)

	tasks := service.NewTaskService(repository.NewTaskRepo(db), hooks, api, notes)
	users := service.NewUserService(userRepo)

	auth := handler.NewAuth(users)
	taskHandler := handler.NewTaskHandler(tasks, auth)
	userHandler := handler.NewUserHandler(users, auth)
	webhookHandler := handler.NewWebhookHandler(hooks, auth)
	adminHandler := handler.NewAdminHandler(tasks, auth)
	noteHandler := handler.NewNotificationHandler(notes, auth)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Use(middleware.Timeout(5 * time.Second))

	r.Get("/health", handler.Health)
	r.Mount("/api/tasks", taskHandler.Router())
	r.Mount("/api/users", userHandler.Router())
	r.Mount("/api/webhooks", webhookHandler.Router())
	r.Mount("/api/notifications", noteHandler.Router())
	r.Mount("/api/admin", adminHandler.Router())

	return &Server{cfg: cfg, db: db, router: r}
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{Addr: ":" + s.cfg.Port, Handler: s.router}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("listening on http://localhost:%s", s.cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	s.db.Close()
	log.Println("stopped cleanly")
	return nil
}
