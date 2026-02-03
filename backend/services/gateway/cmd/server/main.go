package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

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

	authClient := authclient.NewClient(cfg.AuthServiceURL)
	bookClient := bookclient.NewClient(cfg.BookServiceURL)
	chatClient := chatclient.NewClient(cfg.ChatServiceURL)

	httpServer := server.New(server.Config{
		Auth:              authClient,
		Book:              bookClient,
		Chat:              chatClient,
		MaxUploadBytes:    cfg.MaxUploadBytes,
		AllowedExtensions: cfg.AllowedExtensions,
	})

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
