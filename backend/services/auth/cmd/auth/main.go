package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/util"
	"onebookai/services/auth/internal/app"
	"onebookai/services/auth/internal/config"
	"onebookai/services/auth/internal/server"
)

func main() {
	cfg, err := config.Load(config.ConfigPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	sessionTTL, err := config.ParseSessionTTL(cfg.SessionTTL)
	if err != nil {
		log.Fatalf("failed to parse session TTL: %v", err)
	}

	logger := util.InitLogger(cfg.LogLevel)

	appCore, err := app.New(app.Config{
		DatabaseURL:   cfg.DatabaseURL,
		RedisAddr:     cfg.RedisAddr,
		RedisPassword: cfg.RedisPassword,
		SessionTTL:    sessionTTL,
		JWTSecret:     cfg.JWTSecret,
	})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer := server.New(server.Config{
		App: appCore,
	})

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
