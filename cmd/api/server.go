package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"task_crud_api/internal/auth"
	"task_crud_api/internal/client"
	"task_crud_api/internal/handler"
	"task_crud_api/internal/middleware"
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
	refreshRepo := repository.NewRefreshTokenRepo(db)

	tokens := auth.NewTokenService(cfg.SignKey, cfg.AccessTTL, cfg.RefreshTTL)

	hooks := service.NewWebhookService(repository.NewWebhookRepo(db), api)
	notes := service.NewNotificationService(repository.NewNotificationRepo(db), userRepo)

	tasks := service.NewTaskService(repository.NewTaskRepo(db), hooks, api, notes)
	users := service.NewUserService(userRepo)
	authService := service.NewAuthService(userRepo, refreshRepo, tokens)

	guard := middleware.NewAuth(tokens)
	h := handler.NewServer(tasks, users, authService, hooks, notes, guard)

	return &Server{cfg: cfg, db: db, router: h.Routes()}
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
