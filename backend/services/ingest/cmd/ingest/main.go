package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/internal/util"
	"onebookai/services/ingest/internal/app"
	"onebookai/services/ingest/internal/config"
	"onebookai/services/ingest/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, cleanup := util.InitLogger(cfg.LogLevel, "ingest", cfg.LogsDir)
	if cleanup != nil {
		defer cleanup()
	}

	internalVerifyKeys, err := servicetoken.ParseVerifyPublicKeys(cfg.InternalJWTVerifyPublicKeys)
	if err != nil {
		util.Fatal("failed to parse internal jwt verify public keys", "err", err)
	}

	appCore, err := app.New(app.Config{
		DatabaseURL:               cfg.DatabaseURL,
		BookServiceURL:            cfg.BookServiceURL,
		IndexerURL:                cfg.IndexerURL,
		InternalJWTPrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		InternalJWTKeyID:          cfg.InternalJWTKeyID,
		RedisAddr:                 cfg.RedisAddr,
		RedisPassword:             cfg.RedisPassword,
		QueueName:                 cfg.QueueName,
		QueueGroup:                cfg.QueueGroup,
		QueueConcurrency:          cfg.QueueConcurrency,
		QueueMaxRetries:           cfg.QueueMaxRetries,
		QueueRetryDelaySeconds:    cfg.QueueRetryDelaySeconds,
		ChunkSize:                 cfg.ChunkSize,
		ChunkOverlap:              cfg.ChunkOverlap,
		OCREnabled:                cfg.OCREnabled,
		OCRCommand:                cfg.OCRCommand,
		OCRDevice:                 cfg.OCRDevice,
		OCRTimeoutSeconds:         cfg.OCRTimeoutSeconds,
		PDFMinPageRunes:           cfg.PDFMinPageRunes,
		PDFMinPageScore:           cfg.PDFMinPageScore,
		PDFOCRMinScoreDelta:       cfg.PDFOCRMinScoreDelta,
	})
	if err != nil {
		util.Fatal("failed to init app", "err", err)
	}

	httpServer, err := server.New(server.Config{
		App:                         appCore,
		InternalJWTKeyID:            cfg.InternalJWTKeyID,
		InternalJWTPublicKeyPath:    cfg.InternalJWTPublicKeyPath,
		InternalJWTVerifyPublicKeys: internalVerifyKeys,
	})
	if err != nil {
		util.Fatal("failed to init server", "err", err)
	}

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
