package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"onebookai/internal/util"
	"onebookai/services/auth/internal/app"
	"onebookai/services/auth/internal/config"
	"onebookai/services/auth/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger, cleanup := util.InitLogger(cfg.LogLevel, "auth", cfg.LogsDir, "../../logs")
	if cleanup != nil {
		defer cleanup()
	}

	sessionTTL, err := config.ParseSessionTTL(cfg.SessionTTL)
	if err != nil {
		util.Fatal("failed to parse session TTL", "err", err)
	}
	refreshTTL, err := config.ParseRefreshTTL(cfg.RefreshTTL)
	if err != nil {
		util.Fatal("failed to parse refresh TTL", "err", err)
	}
	jwtLeeway, err := config.ParseJWTLeeway(cfg.JWTLeeway)
	if err != nil {
		util.Fatal("failed to parse jwt leeway", "err", err)
	}
	verifyPublicKeys, err := config.ParseVerifyPublicKeys(cfg.JWTVerifyPublicKeys)
	if err != nil {
		util.Fatal("failed to parse jwt verify public keys", "err", err)
	}

	appCore, err := app.New(app.Config{
		DatabaseURL:         cfg.DatabaseURL,
		RedisAddr:           cfg.RedisAddr,
		RedisPassword:       cfg.RedisPassword,
		SessionTTL:          sessionTTL,
		RefreshTTL:          refreshTTL,
		JWTPrivateKeyPath:   cfg.JWTPrivateKeyPath,
		JWTPublicKeyPath:    cfg.JWTPublicKeyPath,
		JWTKeyID:            cfg.JWTKeyID,
		JWTVerifyPublicKeys: verifyPublicKeys,
		JWTIssuer:           cfg.JWTIssuer,
		JWTAudience:         cfg.JWTAudience,
		JWTLeeway:           jwtLeeway,
	})
	if err != nil {
		util.Fatal("failed to init app", "err", err)
	}

	httpServer, err := server.New(server.Config{
		App:                        appCore,
		RedisAddr:                  cfg.RedisAddr,
		RedisPassword:              cfg.RedisPassword,
		TrustedProxyCIDRs:          cfg.TrustedProxyCIDRs,
		SignupRateLimitPerMinute:   cfg.SignupRateLimitPerMinute,
		LoginRateLimitPerMinute:    cfg.LoginRateLimitPerMinute,
		RefreshRateLimitPerMinute:  cfg.RefreshRateLimitPerMinute,
		PasswordRateLimitPerMinute: cfg.PasswordRateLimitPerMinute,
	})
	if err != nil {
		util.Fatal("failed to init auth server", "err", err)
	}

	addr := ":" + cfg.Port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	slog.Info("auth server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
