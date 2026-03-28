// Package main demonstrates a REST API server using chi and go-kit libraries
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/text/language"

	"github.com/wahrwelt-kit/go-cachekit"
	"github.com/wahrwelt-kit/go-httpkit/httputil"
	"github.com/wahrwelt-kit/go-httpkit/httputil/middleware"
	"github.com/wahrwelt-kit/go-jwtkit"
	"github.com/wahrwelt-kit/go-logkit"
	"github.com/wahrwelt-kit/go-pgkit/pgutil"
	"github.com/wahrwelt-kit/go-pgkit/postgres"
)

type User struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

// AppConfig is a server-wide configuration loaded once per minute via CachedValue.
type AppConfig struct {
	MaintenanceMode bool   `json:"maintenance_mode"`
	Version         string `json:"version"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	log, err := logkit.New(
		logkit.WithLevel(logkit.InfoLevel),
		logkit.WithOutput(logkit.ConsoleOutput),
		logkit.WithServiceName("chi-rest"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}

	pool, err := postgres.New(ctx, &postgres.Config{
		URL:      env("DATABASE_URL", "postgres://app:app@localhost:5432/app?sslmode=disable"),
		MaxConns: 10,
	})
	if err != nil {
		log.Fatal("postgres", logkit.Error(err))
	}
	defer pool.Close()

	rdb, err := cachekit.NewRedisClient(ctx, &cachekit.RedisConfig{
		Host: env("REDIS_HOST", "localhost"),
		Port: 6379,
	})
	if err != nil {
		log.Fatal("redis", logkit.Error(err))
	}
	defer rdb.Close()

	cache := cachekit.New(rdb)

	// LRFUCache - in-memory L1 cache (hot users), 1 000 entries max.
	// Falls through to Redis+DB on miss via GetOrLoad.
	lrfu := cachekit.NewLRFUCache[uuid.UUID, User](1000)

	// CachedValue - single app config reloaded every minute with singleflight.
	appConfig := cachekit.NewCachedValue[AppConfig](ctx, "app:config", time.Minute,
		cachekit.WithLoadTimeout(5*time.Second),
	)
	defer appConfig.Stop()

	jwtSvc, err := jwtkit.NewJWTService(jwtkit.Config{
		AccessKeys:  []jwtkit.KeyEntry{{Kid: "1", Secret: []byte(env("JWT_SECRET", "change-me-32-bytes-long-secret!!"))}},
		RefreshKeys: []jwtkit.KeyEntry{{Kid: "1", Secret: []byte(env("JWT_SECRET", "change-me-32-bytes-long-secret!!"))}},
		AccessTTL:   15 * time.Minute,
		RefreshTTL:  7 * 24 * time.Hour,
		Issuer:      "chi-rest",
	})
	if err != nil {
		log.Fatal("jwt", logkit.Error(err))
	}

	// i18n bundle - flat JSON format, English default + Spanish.
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if _, err = bundle.ParseMessageFileBytes(
		[]byte(`{"greeting":"Hello","user_not_found":"User not found"}`), "en.json",
	); err != nil {
		log.Fatal("i18n en", logkit.Error(err))
	}
	if _, err = bundle.ParseMessageFileBytes(
		[]byte(`{"greeting":"Hola","user_not_found":"Usuario no encontrado"}`), "es.json",
	); err != nil {
		log.Fatal("i18n es", logkit.Error(err))
	}

	// ClientIP: nil = trust no proxy headers (safe default); pass CIDRs for trusted proxies.
	clientIP, err := middleware.ClientIP(nil)
	if err != nil {
		log.Fatal("clientip middleware", logkit.Error(err))
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID())
	r.Use(clientIP)
	r.Use(middleware.Logger(log, nil))
	r.Use(middleware.Recoverer(log))
	r.Use(middleware.SecurityHeaders(false))
	// Prometheus: http_requests_total + http_request_duration_seconds.
	// ChiPathFromRequest returns stable route pattern (/users/{id}) to avoid cardinality explosion.
	r.Use(middleware.Metrics(nil, httputil.ChiPathFromRequest, log))
	// Per-request timeout - handlers that exceed 10 s get a 503.
	r.Use(middleware.Timeout(10*time.Second, log))
	// Language resolution: cookie "lang" -> ?lang= -> Accept-Language header -> English default.
	r.Use(middleware.I18n(bundle,
		middleware.WithLanguageCookie("lang"),
		middleware.WithLanguageQueryParam("lang"),
	))

	r.Get("/health", httputil.HealthHandler(nil))
	// Expose Prometheus metrics.
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Localised greeting - shows I18n middleware in action.
	r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		greeting := middleware.Localize(r.Context(), &i18n.LocalizeConfig{
			MessageID:      "greeting",
			DefaultMessage: &i18n.Message{ID: "greeting", Other: "Hello"},
		})
		httputil.RenderJSON(w, r, http.StatusOK, map[string]string{"message": greeting})
	})

	r.Post("/auth/login", loginHandler(jwtSvc))

	r.Group(func(r chi.Router) {
		r.Use(jwtkit.JWTAuth(jwtSvc, jwtkit.WithLogger(log)))

		// GET /config - CachedValue: result is singleflighted and refreshed every minute.
		r.Get("/config", configHandler(appConfig))

		// GET /users/{id} - L1 LRFUCache -> L2 Redis (GetOrLoad) -> Postgres.
		r.Get("/users/{id}", getUserHandler(pool, cache, lrfu))

		// GET /users/{id}/export - streams the user record as a downloadable JSON file.
		r.Get("/users/{id}/export", exportUserHandler(pool, cache))
	})

	srv := &http.Server{Addr: ":8080", Handler: r}

	go func() {
		log.Info("listening on :8080")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server", logkit.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}

func loginHandler(jwtSvc *jwtkit.JWTService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputil.RenderError(w, r, http.StatusBadRequest, "invalid request body")
			return
		}
		uid, err := uuid.Parse(req.UserID)
		if err != nil {
			httputil.RenderError(w, r, http.StatusBadRequest, "invalid user_id")
			return
		}
		pair, err := jwtSvc.GenerateTokenPair(r.Context(), uid, req.Role)
		if err != nil {
			httputil.RenderError(w, r, http.StatusInternalServerError, "token generation failed")
			return
		}
		httputil.RenderJSON(w, r, http.StatusOK, pair)
	}
}

func configHandler(cfg *cachekit.CachedValue[AppConfig]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		config, err := cfg.Get(r.Context(), func(ctx context.Context) (AppConfig, error) {
			// In production: SELECT maintenance_mode, version FROM settings LIMIT 1
			return AppConfig{MaintenanceMode: false, Version: "1.0.0"}, nil
		})
		if err != nil {
			httputil.RenderError(w, r, http.StatusInternalServerError, "failed to load config")
			return
		}
		httputil.RenderJSON(w, r, http.StatusOK, config)
	}
}

func getUserHandler(pool *pgxpool.Pool, cache *cachekit.Cache, lrfu *cachekit.LRFUCache[uuid.UUID, User]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.ParseUUIDField(w, r, chi.URLParam(r, "id"), "id")
		if !ok {
			return
		}

		// L1: in-memory LRFU - O(1) hit, no network hop.
		if user, ok := lrfu.Get(id); ok {
			httputil.RenderJSON(w, r, http.StatusOK, user)
			return
		}

		// L2: Redis -> Postgres with singleflight coalescing.
		user, err := cachekit.GetOrLoad(cache, r.Context(), fmt.Sprintf("user:%s", id), 5*time.Minute, func(ctx context.Context) (User, error) {
			var u User
			err := pool.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id = $1", id).Scan(&u.ID, &u.Name, &u.Email)
			return u, err
		})
		if err != nil {
			notFoundMsg := middleware.Localize(r.Context(), &i18n.LocalizeConfig{
				MessageID:      "user_not_found",
				DefaultMessage: &i18n.Message{ID: "user_not_found", Other: "User not found"},
			})
			if pgutil.IsNoRows(err) {
				httputil.RenderError(w, r, http.StatusNotFound, notFoundMsg)
				return
			}
			httputil.RenderError(w, r, http.StatusInternalServerError, "failed to load user")
			return
		}

		lrfu.Set(id, user)
		httputil.RenderJSON(w, r, http.StatusOK, user)
	}
}

func exportUserHandler(pool *pgxpool.Pool, cache *cachekit.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := httputil.ParseUUIDField(w, r, chi.URLParam(r, "id"), "id")
		if !ok {
			return
		}
		user, err := cachekit.GetOrLoad(cache, r.Context(), fmt.Sprintf("user:%s", id), 5*time.Minute, func(ctx context.Context) (User, error) {
			var u User
			err := pool.QueryRow(ctx, "SELECT id, name, email FROM users WHERE id = $1", id).Scan(&u.ID, &u.Name, &u.Email)
			return u, err
		})
		if err != nil {
			if pgutil.IsNoRows(err) {
				httputil.RenderError(w, r, http.StatusNotFound, "user not found")
				return
			}
			httputil.RenderError(w, r, http.StatusInternalServerError, "failed to load user")
			return
		}
		// Sends Content-Disposition: attachment; filename="user-<id>.json"
		if err := httputil.RenderJSONAttachment(w, user, fmt.Sprintf("user-%s.json", id)); err != nil {
			httputil.RenderError(w, r, http.StatusInternalServerError, "export failed")
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
