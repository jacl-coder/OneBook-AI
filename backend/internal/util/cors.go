package util

import (
	"net/http"
	"os"
	"strings"
	"sync"
)

var (
	corsOnce sync.Once
	corsCfg  corsConfig
)

type corsConfig struct {
	allowAll         bool
	allowCredentials bool
	allowedOrigins   map[string]struct{}
}

// WithCORS adds strict CORS headers based on configured allowlist.
// Set CORS_ALLOWED_ORIGINS to comma-separated origins or "*".
func WithCORS(next http.Handler) http.Handler {
	corsOnce.Do(func() {
		corsCfg = loadCORSConfig()
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			allowed := corsCfg.allowAll
			if !allowed {
				_, allowed = corsCfg.allowedOrigins[origin]
			}
			if allowed {
				if corsCfg.allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				if corsCfg.allowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			} else if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loadCORSConfig() corsConfig {
	cfg := corsConfig{allowedOrigins: make(map[string]struct{})}
	rawOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if rawOrigins == "*" {
		cfg.allowAll = true
	} else if rawOrigins != "" {
		for _, origin := range strings.Split(rawOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				continue
			}
			cfg.allowedOrigins[origin] = struct{}{}
		}
	}
	cfg.allowCredentials = strings.EqualFold(strings.TrimSpace(os.Getenv("CORS_ALLOW_CREDENTIALS")), "true")
	return cfg
}
