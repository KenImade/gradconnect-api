package main

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

func (app *application) authenticate(next http.Handler) http.Handler {
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

func (app *application) requireAuthenticatedUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)

		if user.IsAnonymous() {
			app.authenticationRequiredResponse(w, r)
			return
		}

		next(w, r)
	}
}

func (app *application) requireVerifiedUser(next http.HandlerFunc) http.HandlerFunc {
	return app.requireAuthenticatedUser(func(w http.ResponseWriter, r *http.Request) {
		user := app.contextGetUser(r)
		if !user.EmailVerified {
			app.errorResponse(w, r, http.StatusForbidden, "your email must be verified to access this resource")
			return
		}
		next(w, r)
	})
}

func (app *application) requirePermission(code string, next http.HandlerFunc) http.HandlerFunc {
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
func (app *application) rateLimitByIP(limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
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
func (app *application) rateLimitAll() func(http.Handler) http.Handler {
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

func (app *application) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")

		origin := r.Header.Get("Origin")

		if origin != "" {
			for i := range app.config.cors.trustedOrigins {
				if origin == app.config.cors.trustedOrigins[i] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true") // ← add

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
// Exported-style helper in case other code wants to include it in logs.
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
	// If a handler calls Write without WriteHeader, Go implicitly sends 200.
	// We want to record that.
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
func (app *application) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip the healthcheck — too noisy to be useful.
		if r.URL.Path == "/api/v1/healthcheck" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		reqID := newRequestID()

		// Stash the request ID in context so inner handlers can reference it
		// in their own log lines (e.g. for errors).
		ctx := context.WithValue(r.Context(), requestIDContextKey, reqID)
		r = r.WithContext(ctx)

		// Echo to client for support debugging ("include this header with your report").
		w.Header().Set("X-Request-ID", reqID)

		rw := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		// If WriteHeader was never called and no body was written,
		// default to 200 for accurate logging.
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

		// Only include query if present — cleaner logs for the typical no-query case.
		if r.URL.RawQuery != "" {
			attrs = append(attrs, "query", r.URL.RawQuery)
		}

		// If the auth middleware ran and resolved a user, include their ID.
		// This is safe because contextGetUser always returns a valid User
		// (AnonymousUser if unauthenticated) — no nil check needed.
		user := app.contextGetUser(r)
		if !user.IsAnonymous() {
			attrs = append(attrs, "user_id", user.ID)
		}

		// Level by status: 5xx = error, 4xx = warn, 2xx/3xx = info.
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
	// X-Forwarded-For is a comma-separated list; the first entry is the
	// original client. The rest are the chain of proxies.
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
	// Fallback: RemoteAddr includes the port, strip it if possible.
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
