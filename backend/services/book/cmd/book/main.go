package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/util"
	"onebookai/services/book/internal/app"
	"onebookai/services/book/internal/config"
	"onebookai/services/book/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := util.InitLogger(cfg.LogLevel)

	appCore, err := app.New(app.Config{
		DatabaseURL:       cfg.DatabaseURL,
		MinioEndpoint:     cfg.MinioEndpoint,
		MinioAccessKey:    cfg.MinioAccessKey,
		MinioSecretKey:    cfg.MinioSecretKey,
		MinioBucket:       cfg.MinioBucket,
		MinioUseSSL:       cfg.MinioUseSSL,
		IngestURL:         cfg.IngestURL,
		InternalToken:     cfg.InternalToken,
		MaxUploadBytes:    cfg.MaxUploadBytes,
		AllowedExtensions: cfg.AllowedExtensions,
	})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer := server.New(server.Config{
		App:            appCore,
		InternalToken:  cfg.InternalToken,
		MaxUploadBytes: cfg.MaxUploadBytes,
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("book server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
