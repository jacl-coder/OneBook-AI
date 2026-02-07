package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/services/gateway/internal/authclient"
	"onebookai/services/gateway/internal/bookclient"
	"onebookai/services/gateway/internal/chatclient"
	"onebookai/services/gateway/internal/config"
	"onebookai/services/gateway/internal/server"
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
	bookClient := bookclient.NewClient(cfg.BookServiceURL)
	chatClient := chatclient.NewClient(cfg.ChatServiceURL)
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

	httpServer, err := server.New(server.Config{
		Auth:                       authClient,
		Book:                       bookClient,
		Chat:                       chatClient,
		TokenVerifier:              tokenVerifier,
		RedisAddr:                  cfg.RedisAddr,
		RedisPassword:              cfg.RedisPassword,
		SignupRateLimitPerMinute:   cfg.SignupRateLimitPerMinute,
		LoginRateLimitPerMinute:    cfg.LoginRateLimitPerMinute,
		RefreshRateLimitPerMinute:  cfg.RefreshRateLimitPerMinute,
		PasswordRateLimitPerMinute: cfg.PasswordRateLimitPerMinute,
		MaxUploadBytes:             cfg.MaxUploadBytes,
		AllowedExtensions:          cfg.AllowedExtensions,
	})
	if err != nil {
		log.Fatalf("failed to init gateway server: %v", err)
	}

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
