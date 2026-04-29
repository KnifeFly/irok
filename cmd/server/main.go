package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	kiroauth "orik/internal/auth/kiro"
	"orik/internal/config"
	"orik/internal/httpapi"
	"orik/internal/pool"
	"orik/internal/prompt"
	kiroprovider "orik/internal/provider/kiro"
)

func main() {
	configPath := flag.String("config", filepath.Join("config", "config.toml"), "path to config.toml")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger, closeLog, err := newLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer closeLog()
	slog.SetDefault(logger)

	pools, err := pool.NewStore(cfg.Files.PoolsPath)
	if err != nil {
		logger.Error("load pools", "error", err)
		os.Exit(1)
	}
	prompts, err := prompt.NewStore(cfg.Files.PromptsPath)
	if err != nil {
		logger.Error("load prompts", "error", err)
		os.Exit(1)
	}

	kiroService := kiroprovider.New(cfg, pools, logger)
	authManager := kiroauth.NewManager(cfg, pools, logger)
	api, err := httpapi.New(cfg, pools, prompts, kiroService, authManager, logger)
	if err != nil {
		logger.Error("init http api", "error", err)
		os.Exit(1)
	}

	server := &http.Server{
		Addr:              cfg.Address(),
		Handler:           api.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	logger.Info("orik server starting", "addr", cfg.Address())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newLogger(cfg config.Config) (*slog.Logger, func(), error) {
	if err := os.MkdirAll(cfg.Logging.Dir, 0o755); err != nil {
		return nil, func() {}, err
	}
	file, err := os.OpenFile(filepath.Join(cfg.Logging.Dir, "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, func() {}, err
	}
	writer := io.MultiWriter(os.Stdout, file)
	level := slog.LevelInfo
	if cfg.Logging.Level == "debug" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level}))
	return logger, func() { _ = file.Close() }, nil
}
