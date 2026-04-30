package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
)

// ErrorResponse represents the standard JSON error structure.
// @Description The standard error response object returned by the API.
type ErrorResponse struct {
	Error any `json:"error"` // Can be a string or a map of validation errors
}

// ValidationErrors represents the structure for 422 Unprocessable Entity errors.
// @Description A map of field names to error messages.
type ValidationErrors struct {
	Error map[string]string `json:"error"`
}

func (app *application) logError(r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)

	app.logger.Error(err.Error(), "method", method, "uri", uri)
}

// errorResponse is the base helper for all JSON error responses.
func (app *application) errorResponse(w http.ResponseWriter, r *http.Request, status int, message any) {
	env := envelope{"error": message}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(500)
	}
}

// serverErrorResponse handles 500 Internal Server Error.
func (app *application) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
		hub.CaptureException(err)
	}

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

// failedValidationResponse handles 422 Unprocessable Entity.
func (app *application) failedValidationResponse(w http.ResponseWriter, r *http.Request, errors map[string]string) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

// notFoundResponse handles 404 Not Found.
func (app *application) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "requested resource was not found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

// badRequestResponse handles 400 Bad Request.
func (app *application) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

// invalidCredentialsResponse handles 401 Unauthorized.
func (app *application) invalidCredentialsResponse(w http.ResponseWriter, r *http.Request) {
	message := "invalid authentication credentials"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) authenticationRequiredResponse(w http.ResponseWriter, r *http.Request) {
	message := "you must be authenticated to access this resource"
	app.errorResponse(w, r, http.StatusUnauthorized, message)
}

func (app *application) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *application) rateLimitExceededResponse(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}

	headers := make(http.Header)
	headers.Set("Retry-After", fmt.Sprintf("%d", seconds))

	app.errorResponseWithHeaders(w, r, http.StatusTooManyRequests, "rate limit exceeded, please try again later", headers)
}

func (app *application) errorResponseWithHeaders(w http.ResponseWriter, r *http.Request, status int, message any, headers http.Header) {
	env := envelope{"error": message}
	err := app.writeJSON(w, status, env, headers)
	if err != nil {
		app.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		w.WriteHeader(500)
	}
}
