package app

import (
	"context"
	"net/http"

	"api.gradconnect.com/internal/data"
)

type contextKey string

const userContextKey = contextKey("user")

func (app *App) contextSetUser(r *http.Request, user *data.User) *http.Request {
	ctx := context.WithValue(r.Context(), userContextKey, user)
	return r.WithContext(ctx)
}

func (app *App) contextGetUser(r *http.Request) *data.User {
	user, ok := r.Context().Value(userContextKey).(*data.User)
	if !ok {
		// No auth middleware ran — treat as anonymous.
		// This can happen for requests that short-circuit early,
		// like CORS preflight OPTIONS requests.
		return data.AnonymousUser
	}
	return user
}
