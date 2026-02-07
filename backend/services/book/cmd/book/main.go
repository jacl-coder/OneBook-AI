package main

import (
	"log"
	"log/slog"
	"net/http"
	"time"

	"onebookai/internal/servicetoken"
	"onebookai/internal/usertoken"
	"onebookai/internal/util"
	"onebookai/services/book/internal/app"
	"onebookai/services/book/internal/authclient"
	"onebookai/services/book/internal/config"
	"onebookai/services/book/internal/server"
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
	internalVerifyKeys, err := servicetoken.ParseVerifyPublicKeys(cfg.InternalJWTVerifyPublicKeys)
	if err != nil {
		log.Fatalf("failed to parse internal jwt verify public keys: %v", err)
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

	appCore, err := app.New(app.Config{
		DatabaseURL:               cfg.DatabaseURL,
		MinioEndpoint:             cfg.MinioEndpoint,
		MinioAccessKey:            cfg.MinioAccessKey,
		MinioSecretKey:            cfg.MinioSecretKey,
		MinioBucket:               cfg.MinioBucket,
		MinioUseSSL:               cfg.MinioUseSSL,
		IngestURL:                 cfg.IngestURL,
		InternalJWTKeyID:          cfg.InternalJWTKeyID,
		InternalJWTPrivateKeyPath: cfg.InternalJWTPrivateKeyPath,
		MaxUploadBytes:            cfg.MaxUploadBytes,
		AllowedExtensions:         cfg.AllowedExtensions,
	})
	if err != nil {
		log.Fatalf("failed to init app: %v", err)
	}

	httpServer, err := server.New(server.Config{
		App:                         appCore,
		Auth:                        authClient,
		TokenVerifier:               tokenVerifier,
		InternalJWTKeyID:            cfg.InternalJWTKeyID,
		InternalJWTPublicKeyPath:    cfg.InternalJWTPublicKeyPath,
		InternalJWTVerifyPublicKeys: internalVerifyKeys,
		MaxUploadBytes:              cfg.MaxUploadBytes,
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

	slog.Info("book server listening", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
	}
}
