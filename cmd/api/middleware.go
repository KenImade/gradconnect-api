package main

import (
	"net/http"

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
