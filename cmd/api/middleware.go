package main

import (
	"net"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
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

// rateLimitBySession returns middleware that rate limits requests per session.
// Falls back to IP-based limiting if no session cookie is present.
func (app *application) rateLimitBySession(limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var key string
			if cookie, err := r.Cookie("session_id"); err == nil {
				key = "session:" + r.URL.Path + ":" + cookie.Value
			} else {
				ip, _, _ := net.SplitHostPort(r.RemoteAddr)
				key = "ip:" + r.URL.Path + ":" + ip
			}

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
