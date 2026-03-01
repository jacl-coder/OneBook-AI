package main

import (
	"log"
	"log/slog"
	"net/http"
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
		log.Fatalf("failed to load config: %v", err)
	}

	logger := util.InitLogger(cfg.LogLevel)
	internalVerifyKeys, err := servicetoken.ParseVerifyPublicKeys(cfg.InternalJWTVerifyPublicKeys)
	if err != nil {
		log.Fatalf("failed to parse internal jwt verify public keys: %v", err)
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
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer, err := server.New(server.Config{
		App:                         appCore,
		InternalJWTKeyID:            cfg.InternalJWTKeyID,
		InternalJWTPublicKeyPath:    cfg.InternalJWTPublicKeyPath,
		InternalJWTVerifyPublicKeys: internalVerifyKeys,
	})
	if err != nil {
		log.Fatalf("failed to init server: %v", err)
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
