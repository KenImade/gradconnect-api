package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"api.gradconnect.com/internal/data"
	"github.com/getsentry/sentry-go"
)

func (app *App) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Cookie")

		cookie, err := r.Cookie("session_id")
		if err != nil {
			// No cookie — anonymous user
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		session, err := app.models.Sessions.GetByID(r.Context(), app.db, cookie.Value)
		if err != nil {
			// Invalid or expired session — treat as anonymous
			r = app.contextSetUser(r, data.AnonymousUser)
			next.ServeHTTP(w, r)
			return
		}

		user, err := app.models.Users.GetByID(r.Context(), app.db, session.UserID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		r = app.contextSetUser(r, user)
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.Scope().SetUser(sentry.User{
				ID:    user.ID,
				Email: user.Email,
			})
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next(w, r)
	}
}

func (app *App) requireVerifiedUser(next http.HandlerFunc) http.HandlerFunc {
	return app.requireAuthenticatedUser(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if !user.EmailVerified {
			app.errorResponse(w, r, http.StatusForbidden, "your email must be verified to access this resource")
			return
		}
		next(w, r)
	})
}

func (app *App) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
	return app.requireVerifiedUser(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		permissions, err := app.models.Permissions.GetAllForUser(r.Context(), app.db, user.ID)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		hasPermission := false
		for _, p := range permissions {
			if p == code {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			app.errorResponse(w, r, http.StatusForbidden, "your user account does not have the necessary permissions to access this resource")
			return
		}

		next(w, r)
	})
}

// Rate Limiters

// rateLimitByIP returns middleware that rate limits requests per IP address per path.
// Use for unauthenticated endpoints where the IP is the natural unit of identification.
func (app *App) rateLimitByIP(limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			key := "ip:" + r.URL.Path + ":" + ip

			allowed, retryAfter := app.limiter.Allow(key, limit, window)
			if !allowed {
				app.rateLimitExceededResponse(w, r, retryAfter)
				return
			}
			next(w, r)
		}
	}
}

// rateLimitAll applies a global rate limit to all requests except those with
// their own specific limiter (register, login, forgot-password, resend-verification).
// Uses session ID if present, falls back to IP.
func (app *App) rateLimitAll() func(http.Handler) http.Handler {
	// Paths with their own inline or middleware-specific rate limits
	exemptPaths := map[string]bool{
		"/api/v1/auth/register":            true,
		"/api/v1/auth/login":               true,
		"/api/v1/auth/forgot-password":     true,
		"/api/v1/auth/resend-verification": true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if exemptPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			var key string
			if cookie, err := r.Cookie("session_id"); err == nil {
				key = "global:session:" + cookie.Value
			} else {
				ip, _, _ := net.SplitHostPort(r.RemoteAddr)
				key = "global:ip:" + ip
			}

			allowed, retryAfter := app.limiter.Allow(key, 100, time.Minute)
			if !allowed {
				app.rateLimitExceededResponse(w, r, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (app *App) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		if origin != "" {
			for i := range app.config.CORS.TrustedOrigins {
				if origin == app.config.CORS.TrustedOrigins[i] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")

					if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
						w.Header().Set("Access-Control-Allow-Methods", "OPTIONS, GET, POST, PUT, PATCH, DELETE")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

						w.WriteHeader(http.StatusOK)
						return
					}

					break
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

const requestIDContextKey contextKey = "request_id"

// newRequestID returns a short hex-encoded random string for correlating
// log entries to a single request lifecycle.
func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// requestIDFromContext returns the request ID for the current request, or empty.
func requestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDContextKey).(string); ok {
		return id
	}
	return ""
}

// responseWriter wraps http.ResponseWriter to capture status code and byte
// count for access logging.
type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// logRequests logs every incoming HTTP request with method, path, status,
// duration, client IP, user agent, and (if authenticated) the user ID.
// Healthcheck requests are skipped to keep logs clean.
func (app *App) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip the healthcheck — too noisy to be useful.
		if r.URL.Path == "/api/v1/healthcheck" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		reqID := newRequestID()

		ctx := context.WithValue(r.Context(), requestIDContextKey, reqID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", reqID)

		rw := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}

		attrs := []any{
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"bytes", rw.bytes,
			"remote_ip", clientIP(r),
			"user_agent", r.UserAgent(),
		}

		if r.URL.RawQuery != "" {
			attrs = append(attrs, "query", r.URL.RawQuery)
		}

		user := app.contextGetUser(r)
		if !user.IsAnonymous() {
			attrs = append(attrs, "user_id", user.ID)
		}

		switch {
		case status >= 500:
			app.logger.Error("request completed", attrs...)
		case status >= 400:
			app.logger.Warn("request completed", attrs...)
		default:
			app.logger.Info("request completed", attrs...)
		}
	})
}

// clientIP returns the best-effort original client IP, respecting proxy
// headers from Railway, Fly.io, and similar PaaS platforms.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i, c := range xff {
			if c == ',' {
				return strings.TrimSpace(xff[:i])
			}
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
