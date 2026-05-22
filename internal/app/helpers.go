package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *App) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	js, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	js = append(js, '\n')

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

func (app *App) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Limit the size of the request body to 1,048,576 bytes (1MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1_048_576)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		default:
			return err
		}
	}

	// prevents additional JSON objects in request
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

func (app *App) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

func (app *App) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

func (app *App) readBool(qs url.Values, key string, defaultValue *bool) *bool {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}

	return &b
}

func (app *App) readDate(qs url.Values, key string, defaultValue time.Time, v *validator.Validator) time.Time {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}

	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		v.AddError(key, "invalid date format, expected YYYY-MM-DD")
		return defaultValue
	}

	return t
}

func (app *App) readSlugParam(r *http.Request) (string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	slug := params.ByName("slug")
	if slug == "" {
		return "", errors.New("missing slug parameter")
	}

	return slug, nil
}

func (app *App) readIDParam(r *http.Request) (string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id := params.ByName("id")
	if !validator.IsValidUUID(id) {
		return "", errors.New("invalid id parameter")
	}

	return id, nil
}

// inTransaction is a helper that starts a transaction and handles rollback/commit.
func (app *App) inTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := app.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// sendIfDeliverable looks up the recipient's deliverability status and only
// invokes the mailer if the address is active. Returns nil for suppressed
// addresses — from the worker's perspective, "we correctly chose not to send"
// is success, not a failure to retry.
//
// Returns the mailer's error directly if a send is attempted.
//
// This is a defensive guard. The reminder cron also filters at enqueue time,
// but tasks can sit in the queue long enough that an address's status
// changes between enqueue and process. This check is the authoritative one.
func (app *App) sendIfDeliverable(ctx context.Context, recipient, templateFile string, data any) error {
	var status string
	err := app.db.QueryRow(ctx, `
        SELECT email_status FROM app_user WHERE email = $1
    `, recipient).Scan(&status)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// No user with this address — most likely a system or admin
		// recipient (notifications, internal alerts). Allow the send;
		// the bounce subscriber will create the suppression record if
		// needed.
		return app.mailer.Send(recipient, templateFile, data)
	case err != nil:
		return fmt.Errorf("checking email status for %s: %w", recipient, err)
	}

	if status != "active" {
		app.logger.Info("skipping send to suppressed address",
			"recipient", recipient, "status", status, "template", templateFile)
		return nil
	}

	return app.mailer.Send(recipient, templateFile, data)
}
