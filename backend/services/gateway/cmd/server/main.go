package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
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
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, cleanup := util.InitLogger(cfg.LogLevel, "gateway", cfg.LogsDir)
	if cleanup != nil {
		defer cleanup()
	}

	jwtLeeway, err := config.ParseJWTLeeway(cfg.JWTLeeway)
	if err != nil {
		util.Fatal("failed to parse jwt leeway", "err", err)
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
		util.Fatal("failed to init jwks verifier", "err", err)
	}

	httpServer, err := server.New(server.Config{
		Auth:                       authClient,
		Book:                       bookClient,
		Chat:                       chatClient,
		TokenVerifier:              tokenVerifier,
		AccessCookieName:           cfg.AccessCookieName,
		AccessCookieDomain:         cfg.AccessCookieDomain,
		AccessCookiePath:           cfg.AccessCookiePath,
		AccessCookieSecure:         cfg.AccessCookieSecure,
		AccessCookieSameSite:       cfg.AccessCookieSameSite,
		AccessCookieMaxAge:         time.Duration(cfg.AccessCookieMaxAgeSeconds) * time.Second,
		RefreshCookieName:          cfg.RefreshCookieName,
		RefreshCookieDomain:        cfg.RefreshCookieDomain,
		RefreshCookiePath:          cfg.RefreshCookiePath,
		RefreshCookieSecure:        cfg.RefreshCookieSecure,
		RefreshCookieSameSite:      cfg.RefreshCookieSameSite,
		RefreshCookieMaxAge:        time.Duration(cfg.RefreshCookieMaxAgeSeconds) * time.Second,
		RedisAddr:                  cfg.RedisAddr,
		RedisPassword:              cfg.RedisPassword,
		TrustedProxyCIDRs:          cfg.TrustedProxyCIDRs,
		SignupRateLimitPerMinute:   cfg.SignupRateLimitPerMinute,
		LoginRateLimitPerMinute:    cfg.LoginRateLimitPerMinute,
		RefreshRateLimitPerMinute:  cfg.RefreshRateLimitPerMinute,
		PasswordRateLimitPerMinute: cfg.PasswordRateLimitPerMinute,
		MaxUploadBytes:             cfg.MaxUploadBytes,
		AllowedExtensions:          cfg.AllowedExtensions,
	})
	if err != nil {
		util.Fatal("failed to init gateway server", "err", err)
	}

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 150 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
