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
	"github.com/carolineeey/wedding-suite-api/internal/repository"
	"github.com/carolineeey/wedding-suite-api/internal/usecase"
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

	// Request work is cut off at 5s, inside the 10s WriteTimeout, so a slow
	// query still gets an error response written.
	handler := middleware.Logging(middleware.Timeout(5 * time.Second)(router))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      corsHandler.Handler(handler),
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

// buildRouter wires repositories (SQL) into usecases (business rules) into
// handlers (HTTP), then mounts the handlers on routes.
func buildRouter(conn *sql.DB, cfg config.Config) *mux.Router {
	weddingRepo := repository.NewWeddingRepository(conn)
	eventRepo := repository.NewEventRepository(conn)
	guestRepo := repository.NewGuestRepository(conn)
	wishRepo := repository.NewWishRepository(conn)
	adminRepo := repository.NewAdminRepository(conn)
	sessionRepo := repository.NewSessionRepository(conn)
	giftRepo := repository.NewGiftRepository(conn)

	// scope turns the {slug} in a route into the wedding ID the usecases
	// work from. It is the only lookup from address to wedding.
	scope := usecase.NewWeddingScope(weddingRepo)

	weddings := usecase.NewWeddingUsecase(weddingRepo, eventRepo)
	events := usecase.NewEventUsecase(eventRepo)
	guests := usecase.NewGuestUsecase(guestRepo)
	rsvp := usecase.NewRSVPUsecase(guestRepo)
	wishes := usecase.NewWishUsecase(wishRepo, cfg.WishesRequireApproval)
	auth := usecase.NewAuthUsecase(adminRepo, weddingRepo, sessionRepo, scope)
	gifts := usecase.NewGiftUsecase(giftRepo)
	invitations := usecase.NewInvitationUsecase(usecase.InvitationStores{
		Guests:   guestRepo,
		Weddings: weddingRepo,
		Events:   eventRepo,
		Gifts:    giftRepo,
	})

	r := mux.NewRouter()
	r.StrictSlash(false)

	// -- public --
	r.HandleFunc("/health", handlers.Health(conn)).Methods(http.MethodGet)

	// Invite-code lookups and public writes are rate limited per client IP.
	// Limits are generous because guests at the venue may share one Wi-Fi IP.
	lookupLimit := middleware.RateLimit(60, 10*time.Minute)
	writeLimit := middleware.RateLimit(30, 10*time.Minute)
	// Login is tight: it is the one route where guessing pays off.
	loginLimit := middleware.RateLimit(10, 10*time.Minute)
	requireAdmin := middleware.RequireAdmin(auth)

	api := r.PathPrefix("/api/v1").Subrouter()

	// Everything that belongs to one wedding hangs off its slug. Invite codes
	// stay global: a code is unique across weddings and already identifies
	// the guest, so asking the guest for the slug too would be friction.
	pub := api.PathPrefix("/w/{slug}").Subrouter()
	pub.HandleFunc("/wedding", handlers.GetWedding(weddings)).Methods(http.MethodGet)
	pub.HandleFunc("/wishes", handlers.ListWishes(scope, wishes, false)).Methods(http.MethodGet)
	pub.Handle("/wishes", writeLimit(handlers.CreateWish(scope, wishes))).Methods(http.MethodPost)

	api.Handle("/invitations/{code}", lookupLimit(handlers.GetInvitation(invitations))).Methods(http.MethodGet)
	api.Handle("/guests/{code}", lookupLimit(handlers.GetGuestByCode(rsvp))).Methods(http.MethodGet)
	api.Handle("/guests/{code}/rsvp", writeLimit(handlers.SubmitRSVP(rsvp))).Methods(http.MethodPost)

	// -- auth: login hands out the session token admin routes require --
	api.Handle("/auth/login", loginLimit(handlers.Login(auth))).Methods(http.MethodPost)
	api.Handle("/auth/logout", requireAdmin(handlers.Logout(auth))).Methods(http.MethodPost)

	// -- admin (Bearer session token required) --
	admin := api.PathPrefix("/admin").Subrouter()
	admin.Use(requireAdmin)

	admin.HandleFunc("/me", handlers.Me(auth)).Methods(http.MethodGet)
	// Creating a wedding is the one admin route without a slug: it is what
	// mints one, and its creator becomes its first admin.
	admin.HandleFunc("/weddings", handlers.CreateWedding(weddings)).Methods(http.MethodPost)

	// Each admin manages only the weddings they were granted.
	adminW := admin.PathPrefix("/w/{slug}").Subrouter()
	adminW.Use(middleware.RequireWeddingAccess(auth))
	adminW.HandleFunc("/wedding", handlers.UpdateWedding(weddings)).Methods(http.MethodPut)
	adminW.HandleFunc("/events", handlers.CreateEvent(scope, events)).Methods(http.MethodPost)
	adminW.HandleFunc("/events/{id}", handlers.DeleteEvent(scope, events)).Methods(http.MethodDelete)
	adminW.HandleFunc("/gifts", handlers.ListGifts(scope, gifts)).Methods(http.MethodGet)
	adminW.HandleFunc("/gifts", handlers.CreateGift(scope, gifts)).Methods(http.MethodPost)
	adminW.HandleFunc("/gifts/{id}", handlers.DeleteGift(scope, gifts)).Methods(http.MethodDelete)
	adminW.HandleFunc("/guests", handlers.ListGuests(scope, guests)).Methods(http.MethodGet)
	adminW.HandleFunc("/guests", handlers.CreateGuest(scope, guests)).Methods(http.MethodPost)
	adminW.HandleFunc("/guests/{id}", handlers.UpdateGuest(scope, guests)).Methods(http.MethodPut)
	adminW.HandleFunc("/guests/{id}", handlers.DeleteGuest(scope, guests)).Methods(http.MethodDelete)
	adminW.HandleFunc("/rsvp-summary", handlers.GetRSVPSummary(scope, guests)).Methods(http.MethodGet)
	adminW.HandleFunc("/wishes", handlers.ListWishes(scope, wishes, true)).Methods(http.MethodGet)
	adminW.HandleFunc("/wishes/{id}/approval", handlers.SetWishApproval(scope, wishes)).Methods(http.MethodPut)
	adminW.HandleFunc("/wishes/{id}", handlers.DeleteWish(scope, wishes)).Methods(http.MethodDelete)

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
