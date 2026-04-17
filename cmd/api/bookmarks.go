package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// listBookmarksHandler godoc
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
func (app *application) listBookmarksHandler(w http.ResponseWriter, r *http.Request) {
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

type addBookmarkInput struct {
	OpportunityID string `json:"opportunity_id"`
}

// addBookmarkForCurrentUserHandler godoc
// @Summary      Bookmark an opportunity
// @Description  Add an opportunity to the authenticated user's bookmarks.
// @Tags         Bookmarks
// @Accept       json
// @Produce      json
// @Param        body  body      addBookmarkInput  true  "Opportunity to bookmark"
// @Success      201   {object}  data.Bookmark
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse  "Already bookmarked"
// @Failure      500   {object}  ErrorResponse
// @Router       /me/bookmarks [post]
func (app *application) addBookmarkHandler(w http.ResponseWriter, r *http.Request) {
	var input addBookmarkInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	v.Check(input.OpportunityID != "", "opportunity_id", "must be provided")
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	bookmark, err := app.models.Bookmarks.Create(r.Context(), app.db, user.ID, input.OpportunityID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrDuplicateBookmark):
			app.errorResponse(w, r, http.StatusConflict, "you have already bookmarked this opportunity")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := data.BookmarkCreateResponse{
		ID:            bookmark.ID,
		OpportunityID: input.OpportunityID,
		CreatedAt:     bookmark.CreatedAt,
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// removeBookmarkHandler godoc
// @Summary      Remove a bookmark
// @Description  Delete a bookmark belonging to the authenticated user.
// @Tags         Bookmarks
// @Produce      json
// @Param        id  path  string  true  "Bookmark ID"
// @Success      204
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /me/bookmarks/{id} [delete]
func (app *application) removeBookmarkHandler(w http.ResponseWriter, r *http.Request) {
	bookmarkID, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	user := app.contextGetUser(r)

	err = app.models.Bookmarks.Delete(r.Context(), app.db, bookmarkID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
