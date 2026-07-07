package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"artifact/internal/config"
	"artifact/internal/handler"
	"artifact/internal/service"
	"artifact/internal/storage"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config yaml")
	flag.Parse()
	cfg, err := config.LoadConfigFromFile(*configPath)
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	store, err := storage.NewMinioStorage(cfg.Storage)
	if err != nil {
		slog.Error("minio client failed", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := store.EnsureBucketExists(ctx); err != nil {
		slog.Error("ensure bucket failed", "err", err)
		os.Exit(1)
	}
	slog.Info("minio ok", "bucket", cfg.Storage.Bucket)
	svc := service.NewArtifactService(store, cfg)
	e := handler.NewRouter(svc, cfg.Server.ApiKey)
	slog.Info("starting server", "addr", cfg.Server.Addr())
	if err := e.Start(cfg.Server.Addr()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
