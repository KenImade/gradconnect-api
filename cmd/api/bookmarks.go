package main

import (
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// getCurrentUserBookmarksHandler godoc
// @Summary      List the current user's bookmarks
// @Description  Returns the authenticated user's bookmarked opportunities with pagination.
// @Tags         Bookmarks
// @Produce      json
// @Param        page       query     int     false  "Page number"          default(1)
// @Param        page_size  query     int     false  "Items per page"       default(20)
// @Param        sort       query     string  false  "Sort field"           Enums(created_at, deadline, -created_at, -deadline)
// @Success      200        {object}  envelope
// @Failure      401        {object}  ErrorResponse
// @Failure      403        {object}  ErrorResponse
// @Failure      422        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Router       /me/bookmarks [get]
func (app *application) getCurrentUserBookmarksHandler(w http.ResponseWriter, r *http.Request) {
	var filters data.Filters

	v := validator.New()

	qs := r.URL.Query()

	filters.Page = app.readInt(qs, "page", 1, v)
	filters.PageSize = app.readInt(qs, "page_size", 20, v)

	filters.Sort = app.readString(qs, "sort", "-created_at")
	filters.SortSafeList = []string{"created_at", "deadline", "-created_at", "-deadline"}

	if data.ValidateFilters(v, filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	bookmarks, metadata, err := app.models.Bookmarks.GetAllForUser(r.Context(), app.db, user.ID, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": bookmarks, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}
