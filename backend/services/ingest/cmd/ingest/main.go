package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/util"
	"onebookai/services/ingest/internal/app"
	"onebookai/services/ingest/internal/config"
	"onebookai/services/ingest/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := util.InitLogger(cfg.LogLevel)

	appCore, err := app.New(app.Config{
		DatabaseURL:    cfg.DatabaseURL,
		BookServiceURL: cfg.BookServiceURL,
		IndexerURL:     cfg.IndexerURL,
		InternalToken:  cfg.InternalToken,
		ChunkSize:      cfg.ChunkSize,
		ChunkOverlap:   cfg.ChunkOverlap,
	})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer := server.New(server.Config{
		App:           appCore,
		InternalToken: cfg.InternalToken,
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("ingest server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
