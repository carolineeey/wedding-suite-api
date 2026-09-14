package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/config"
	"github.com/carolineeey/wedding-suite-api/internal/db"
	"github.com/carolineeey/wedding-suite-api/internal/handlers"
	"github.com/carolineeey/wedding-suite-api/internal/middleware"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// Best-effort local .env load; on Render, env vars come from the
	// service's environment settings instead and this file won't exist.
	_ = godotenv.Load()

	cfg := config.Load()

	conn, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer conn.Close()

	router := buildRouter(conn, cfg)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   splitAndTrim(cfg.AllowOrigins),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      corsHandler.Handler(middleware.Logging(router)),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("wedding-suite-api listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM (Render sends SIGTERM on deploy).
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func buildRouter(conn *sql.DB, cfg config.Config) *mux.Router {
	r := mux.NewRouter()
	r.StrictSlash(false)

	// -- public --
	r.HandleFunc("/health", handlers.Health(conn)).Methods(http.MethodGet)

	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/wedding", handlers.GetWedding(conn)).Methods(http.MethodGet)
	api.HandleFunc("/guests/{code}", handlers.GetGuestByCode(conn)).Methods(http.MethodGet)
	api.HandleFunc("/guests/{code}/rsvp", handlers.SubmitRSVP(conn)).Methods(http.MethodPost)
	api.HandleFunc("/wishes", handlers.ListWishes(conn, false)).Methods(http.MethodGet)
	api.HandleFunc("/wishes", handlers.CreateWish(conn)).Methods(http.MethodPost)

	// -- admin (Bearer ADMIN_TOKEN required) --
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(middleware.RequireAdmin(cfg.AdminToken))

	admin.HandleFunc("/wedding", handlers.UpsertWedding(conn)).Methods(http.MethodPut)
	admin.HandleFunc("/events", handlers.CreateEvent(conn)).Methods(http.MethodPost)
	admin.HandleFunc("/events/{id}", handlers.DeleteEvent(conn)).Methods(http.MethodDelete)
	admin.HandleFunc("/guests", handlers.ListGuests(conn)).Methods(http.MethodGet)
	admin.HandleFunc("/guests", handlers.CreateGuest(conn)).Methods(http.MethodPost)
	admin.HandleFunc("/guests/{id}", handlers.UpdateGuest(conn)).Methods(http.MethodPut)
	admin.HandleFunc("/guests/{id}", handlers.DeleteGuest(conn)).Methods(http.MethodDelete)
	admin.HandleFunc("/rsvp-summary", handlers.GetRSVPSummary(conn)).Methods(http.MethodGet)
	admin.HandleFunc("/wishes", handlers.ListWishes(conn, true)).Methods(http.MethodGet)
	admin.HandleFunc("/wishes/{id}/approval", handlers.SetWishApproval(conn)).Methods(http.MethodPut)
	admin.HandleFunc("/wishes/{id}", handlers.DeleteWish(conn)).Methods(http.MethodDelete)

	return r
}

func splitAndTrim(s string) []string {
	if s == "" || s == "*" {
		return []string{"*"}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
