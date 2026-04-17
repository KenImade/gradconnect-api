package main

import (
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

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
	var status string

	v := validator.New()
	qs := r.URL.Query()

	status = app.readString(qs, "status", "")
	filters.Page = app.readInt(qs, "page", 1, v)
	filters.PageSize = app.readInt(qs, "page_size", 50, v)
	filters.Sort = app.readString(qs, "sort", "-updated_at")
	filters.SortSafeList = []string{"updated_at", "created_at", "-updated_at", "-created_at"}

	if status != "" {
		v.Check(validator.PermittedValue(data.ApplicationStatus(status),
			data.StatusInterested, data.StatusApplied, data.StatusAssessment,
			data.StatusInterview, data.StatusOffer, data.StatusRejected,
		), "status", "invalid status")
	}

	if data.ValidateFilters(v, filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	applications, metadata, err := app.models.ApplicationTracker.List(r.Context(), app.db, user.ID, status, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := envelope{"data": applications, "pagination": metadata}

	err = app.writeJSON(w, http.StatusOK, response, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
