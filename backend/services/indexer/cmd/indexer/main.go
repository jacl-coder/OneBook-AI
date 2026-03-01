package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/internal/util"
	"onebookai/services/indexer/internal/app"
	"onebookai/services/indexer/internal/config"
	"onebookai/services/indexer/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, cleanup := util.InitLogger(cfg.LogLevel, "indexer", cfg.LogsDir)
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
		InternalJWTPrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		InternalJWTKeyID:          cfg.InternalJWTKeyID,
		RedisAddr:                 cfg.RedisAddr,
		RedisPassword:             cfg.RedisPassword,
		QueueName:                 cfg.QueueName,
		QueueGroup:                cfg.QueueGroup,
		QueueConcurrency:          cfg.QueueConcurrency,
		QueueMaxRetries:           cfg.QueueMaxRetries,
		QueueRetryDelaySeconds:    cfg.QueueRetryDelaySeconds,
		EmbeddingProvider:         cfg.EmbeddingProvider,
		EmbeddingBaseURL:          cfg.EmbeddingBaseURL,
		EmbeddingModel:            cfg.EmbeddingModel,
		EmbeddingDim:              cfg.EmbeddingDim,
		EmbeddingBatchSize:        cfg.EmbeddingBatchSize,
		EmbeddingConcurrency:      cfg.EmbeddingConcurrency,
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

	slog.Info("indexer server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
