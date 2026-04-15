package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"api.gradconnect.com/internal/validator"
	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

func (app *application) writeJSON(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
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

func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)

	if s == "" {
		return defaultValue
	}

	return s
}

func (app *application) readInt(qs url.Values, key string, defaultValue int, v *validator.Validator) int {
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

func (app *application) readBool(qs url.Values, key string, defaultValue *bool) *bool {
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

func (app *application) readSlugParam(r *http.Request) (string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	slug := params.ByName("slug")
	if slug == "" {
		return "", errors.New("missing slug parameter")
	}

	return slug, nil
}

func (app *application) readIDParam(r *http.Request) (string, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id := params.ByName("id")
	if !validator.IsValidUUID(id) {
		return "", errors.New("invalid id parameter")
	}

	return id, nil
}
