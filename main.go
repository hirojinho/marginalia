// Command marginalia is the main entrypoint for the study app HTTP server.
// It loads config from environment variables, opens the SQLite database,
// builds the agent.App, registers handler routes, and starts the server
// with graceful shutdown on SIGINT/SIGTERM.
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

	"marginalia/agent"
	"marginalia/handler"
)

//go:embed static/*
var staticFiles embed.FS

const (
	httpReadTimeout     = 30 * time.Second
	httpWriteTimeout    = 5 * time.Minute // accommodates streaming chat responses
	httpIdleTimeout     = 120 * time.Second
	shutdownGracePeriod = 15 * time.Second
	sandboxIdleDays     = 7
)

// dataSubdirs are the working subdirectories under <vault>/data the
// app expects to exist. Created idempotently at startup.
var dataSubdirs = []string{
	"pdf-files",
	"plans",
	"pdf-texts",
	"agent-sessions",
	"agent-out",
	filepath.Join("corpus", "study-methods"),
	filepath.Join("corpus", "courses"),
	filepath.Join("corpus", "meta"),
}

var (
	buildCommit    = "unknown"
	buildTimestamp = "unknown"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	cfg.BuildCommit = buildCommit
	cfg.BuildTimestamp = buildTimestamp

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

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			n, err := app.Sandbox.Sweep(sandboxIdleDays)
			if err != nil {
				slog.Error("sandbox sweep", "err", err)
			} else if n > 0 {
				slog.Info("sandbox sweep removed stale dirs", "count", n)
			}
		}
	}()

	go func() {
		cutoff := time.Now().Add(-90 * 24 * time.Hour)
		if n, err := app.PruneOldEvents(cutoff); err != nil {
			slog.Warn("prune old events", "err", err)
		} else if n > 0 {
			slog.Info("pruned old events", "count", n)
		}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-90 * 24 * time.Hour)
			if n, err := app.PruneOldEvents(cutoff); err != nil {
				slog.Warn("prune old events", "err", err)
			} else if n > 0 {
				slog.Info("pruned old events", "count", n)
			}
		}
	}()

	app.LoadActiveSessionID()
	if n, err := app.MigratePhase3Sessions(); err != nil {
		slog.Warn("phase3 session migration", "err", err)
	} else if n > 0 {
		slog.Info("phase3 session migration applied", "rows", n)
	}
	if n, err := app.MigrateSessionMode(); err != nil {
		slog.Warn("session mode migration", "err", err)
	} else if n > 0 {
		slog.Info("session mode migration applied", "rows", n)
	}

	llm := agent.NewLLMClient(cfg.APIKey, cfg.APIURL, cfg.Model)

	mux := http.NewServeMux()
	h := handler.New(app, llm, staticFiles)
	h.Register(mux)

	handler.LogStartupHealth(app)

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      h.AuthMiddleware(mux),
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
