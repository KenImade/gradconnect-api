package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// addApplicationHandler godoc
// @Summary      Add an opportunity to tracked applications
// @Description  Creates a new application tracker entry for the authenticated user.
// @Tags         ApplicationTrackers
// @Accept       json
// @Produce      json
// @Param        body  body      data.CreateApplicationInput  true  "Application to track"
// @Success      201   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse  "Already tracking this opportunity"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /me/applications [post]
func (app *application) addApplicationHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateApplicationInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateCreateApplicationInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	application, err := app.models.ApplicationTracker.Add(r.Context(), app.db, user.ID, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrDuplicateApplication):
			app.errorResponse(w, r, http.StatusConflict, "you are already tracking this opportunity")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": application}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateApplicationHandler godoc
// @Summary      Update a tracked application
// @Description  Updates the status and/or notes of an existing application tracker entry.
// @Tags         ApplicationTrackers
// @Accept       json
// @Produce      json
// @Param        id    path      string                       true  "Application tracker ID"
// @Param        body  body      data.UpdateApplicationInput  true  "Fields to update"
// @Success      200   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /me/applications/{id} [patch]
func (app *application) updateApplicationHandler(w http.ResponseWriter, r *http.Request) {
	trackerID, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.UpdateApplicationInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateApplicationInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	application, err := app.models.ApplicationTracker.Update(r.Context(), app.db, user.ID, trackerID, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": application}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listApplicationsHandler godoc
// @Summary      List the current user's application tracker entries
// @Description  Returns the authenticated user's tracked applications with pagination and optional status filter.
// @Tags         ApplicationTrackers
// @Produce      json
// @Param        status     query     string  false  "Filter by status"  Enums(interested, applied, assessment, interview, offer, rejected)
// @Param        page       query     int     false  "Page number"       default(1)
// @Param        page_size  query     int     false  "Items per page"    default(50)
// @Param        sort       query     string  false  "Sort field"        Enums(updated_at, -updated_at, created_at, -created_at)
// @Success      200        {object}  envelope
// @Failure      401        {object}  ErrorResponse
// @Failure      403        {object}  ErrorResponse
// @Failure      422        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Router       /me/applications [get]
func (app *application) listApplicationsHandler(w http.ResponseWriter, r *http.Request) {
	var filters data.Filters

	v := validator.New()
	qs := r.URL.Query()

	status := app.readString(qs, "status", "")
	filters.Page = app.readInt(qs, "page", 1, v)
	filters.PageSize = app.readInt(qs, "page_size", 50, v)
	filters.Sort = app.readString(qs, "sort", "-updated_at")
	filters.SortSafeList = []string{"updated_at", "created_at", "-updated_at", "-created_at"}

	data.ValidateApplicationStatusFilter(v, status)
	data.ValidateFilters(v, filters)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	applications, metadata, err := app.models.ApplicationTracker.List(r.Context(), app.db, user.ID, status, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": applications, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
