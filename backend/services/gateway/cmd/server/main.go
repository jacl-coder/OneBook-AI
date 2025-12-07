package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"onebookai/services/gateway/internal/app"
	"onebookai/services/gateway/internal/server"
)

func main() {
	port := env("PORT", "8080")
	dataDir := env("DATA_DIR", "./data")

	appCore, err := app.New(app.Config{StorageDir: dataDir})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer := server.New(server.Config{
		App: appCore,
	})

	addr := ":" + port
	srv := &http.Server{
		Addr:         addr,
		Handler:      httpServer.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("server listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func env(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
