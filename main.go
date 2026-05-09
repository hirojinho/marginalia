package main

import (
	"context"
	"embed"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"study-app/agent"
	"study-app/handler"
)

//go:embed static/*
var staticFiles embed.FS

const (
	httpReadTimeout     = 30 * time.Second
	httpWriteTimeout    = 5 * time.Minute // accommodates streaming chat responses
	httpIdleTimeout     = 120 * time.Second
	shutdownGracePeriod = 15 * time.Second
)

// dataSubdirs are the working subdirectories under <vault>/data the
// app expects to exist. Created idempotently at startup.
var dataSubdirs = []string{
	"pdf-files",
	"plans",
	"pdf-texts",
	filepath.Join("corpus", "study-methods"),
	filepath.Join("corpus", "courses"),
	filepath.Join("corpus", "meta"),
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	if err := ensureDataDirs(cfg.VaultRoot); err != nil {
		log.Fatalf("ensure data dirs: %v", err)
	}

	dbPath := filepath.Join(cfg.VaultRoot, "data", "study.db")
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		log.Fatalf("init schema: %v", err)
	}
	slog.Info("sqlite initialized", "path", dbPath)

	app := agent.NewApp(cfg, db)
	defer app.Close()

	if err := app.InitVectorStore(); err != nil {
		slog.Warn("vector store init failed", "err", err)
	} else {
		go func() {
			if err := app.IndexCorpus(); err != nil {
				slog.Error("corpus indexing failed", "err", err)
			} else {
				slog.Info("corpus indexed")
			}
		}()
	}

	app.LoadActiveSessionID()

	llm := agent.NewLLMClient(cfg.APIKey, cfg.APIURL, cfg.Model)

	mux := http.NewServeMux()
	h := handler.New(app, llm, staticFiles)
	h.Register(mux)

	handler.LogStartupHealth(app)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  httpReadTimeout,
		WriteTimeout: httpWriteTimeout,
		IdleTimeout:  httpIdleTimeout,
	}

	slog.Info("study app listening", "addr", cfg.ListenAddr, "model", cfg.Model, "api", cfg.APIURL)

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-serveErr:
		log.Fatalf("server error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("graceful shutdown failed", "err", err)
	} else {
		slog.Info("shutdown complete")
	}
}

func ensureDataDirs(vaultRoot string) error {
	dataDir := filepath.Join(vaultRoot, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	for _, sub := range dataSubdirs {
		if err := os.MkdirAll(filepath.Join(dataDir, sub), 0755); err != nil {
			return err
		}
	}
	return nil
}
