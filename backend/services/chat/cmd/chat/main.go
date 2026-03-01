package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/services/chat/internal/app"
	"onebookai/services/chat/internal/authclient"
	"onebookai/services/chat/internal/bookclient"
	"onebookai/services/chat/internal/config"
	"onebookai/services/chat/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := util.InitLogger(cfg.LogLevel)
	jwtLeeway, err := config.ParseJWTLeeway(cfg.JWTLeeway)
	if err != nil {
		log.Fatalf("failed to parse jwt leeway: %v", err)
	}
	authClient := authclient.NewClient(cfg.AuthServiceURL)
	tokenVerifier, err := usertoken.NewVerifier(usertoken.Config{
		JWKSURL:    cfg.AuthJWKSURL,
		Issuer:     cfg.JWTIssuer,
		Audience:   cfg.JWTAudience,
		Leeway:     jwtLeeway,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
	if err != nil {
		log.Fatalf("failed to init jwks verifier: %v", err)
	}
	bookClient := bookclient.NewClient(cfg.BookServiceURL)

	appCore, err := app.New(app.Config{
		DatabaseURL:        cfg.DatabaseURL,
		GenerationProvider: cfg.GenerationProvider,
		GenerationBaseURL:  cfg.GenerationBaseURL,
		GenerationAPIKey:   cfg.GenerationAPIKey,
		GenerationModel:    cfg.GenerationModel,
		EmbeddingProvider:  cfg.EmbeddingProvider,
		EmbeddingBaseURL:   cfg.EmbeddingBaseURL,
		EmbeddingModel:     cfg.EmbeddingModel,
		EmbeddingDim:       cfg.EmbeddingDim,
		TopK:               cfg.TopK,
		HistoryLimit:       cfg.HistoryLimit,
	})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer := server.New(server.Config{
		App:           appCore,
		Auth:          authClient,
		Books:         bookClient,
		TokenVerifier: tokenVerifier,
	})

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("chat server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
