package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"npms/backend/internal/api"
	"npms/backend/internal/config"
	"npms/backend/internal/database"
	"npms/backend/internal/discovery"
	"npms/backend/internal/ingestion"
	"npms/backend/internal/logging"
	"npms/backend/internal/profile"
	"npms/backend/internal/repository"
	"npms/backend/internal/security"
	"npms/backend/internal/snmp"
)

func main() {
	configPath := flag.String("config", "./config.yaml", "YAML configuration path")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fail(err)
	}
	closeLog, err := logging.Setup(cfg.Logging.ErrorFile, cfg.Logging.Daily, cfg.Logging.MaxSizeMB)
	if err != nil {
		fail(fmt.Errorf("setup error log: %w", err))
	}
	defer func() { _ = closeLog() }()
	profiles, err := profile.LoadDir(cfg.Profiles.Path)
	if err != nil {
		fail(err)
	}
	slog.Info("profiles loaded", "directory", cfg.Profiles.Path, "count", len(profiles))

	db, err := database.OpenSQLite(cfg.Database.Path)
	if err != nil {
		fail(err)
	}
	defer db.Close()
	if err := db.Migrate(context.Background()); err != nil {
		fail(err)
	}
	store, err := repository.NewSQLiteRepository(db.DB)
	if err != nil {
		fail(err)
	}
	server, err := api.NewServer(store, cfg.Server.APIToken, cfg.Server.AllowedOrigin)
	if err != nil {
		fail(err)
	}
	server.SetProfiles(profiles)
	key, err := security.ParseKey(cfg.Security.EncryptionKey)
	if err != nil {
		fail(err)
	}
	box, err := security.NewSecretBox(key)
	if err != nil {
		fail(err)
	}
	server.SetSecretBox(box)
	server.SetDiscoveryProbe(func(ctx context.Context, config snmp.Config) (discovery.Result, error) {
		return discovery.Probe(ctx, config, ingestion.ConnectSNMP)
	})
	poller, err := ingestion.NewStatusPoller(store, ingestion.ConfigResolver{Store: store, Box: box}, ingestion.ConnectSNMP)
	if err != nil {
		fail(err)
	}
	server.SetStatusPoller(poller)

	httpServer := &http.Server{Addr: cfg.Server.Listen, Handler: server.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	slog.Info("api server listening", "address", cfg.Server.Listen, "database", cfg.Database.Path)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
