package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"registre-admin/internal/auth"
	"registre-admin/internal/config"
	"registre-admin/internal/database"
	"registre-admin/internal/server"

	_ "registre-admin/docs"
)

// @title           RareQC Registre Admin API
// @version         1.0
// @description     REST API pour le registre québécois de patients atteints de maladies rares — administration.

// @BasePath  /api
//
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                Jeton Bearer Keycloak (voir rareqc-infra/scripts/api-token.sh). Format : "Bearer <token>". Les sessions cookies du portail fonctionnent aussi automatiquement.
func main() {
	cfg, err := config.Load()
	setupLogger(cfg.Production)
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	authCfg, err := auth.LoadConfig(cfg.Production)
	if err != nil {
		slog.Error("invalid auth configuration", "error", err)
		os.Exit(1)
	}

	pgDB, err := database.NewPostgresDB(cfg.DB)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}

	if err := database.Migrate(cfg.DB); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Timeouts bound slow or idle clients (Slowloris); the write budget
	// covers the 10 MB uploads and the cart exports.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.SetupRouter(pgDB, cfg, authCfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	go func() {
		slog.Info("server listening", "port", cfg.Port, "production", cfg.Production)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: on SIGINT/SIGTERM, stop accepting connections and
	// drain in-flight requests (10s budget) before exiting.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutdown signal received, draining in-flight requests")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
		database.Close(pgDB)
		os.Exit(1)
	}
	database.Close(pgDB)
	slog.Info("server stopped")
}

// setupLogger routes slog output as JSON in production, human-readable text
// in dev.
func setupLogger(production bool) {
	if production {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
}
