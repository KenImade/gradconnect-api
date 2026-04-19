package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// addReviewHandler godoc
// @Summary      Submit a review for an employer's graduate programme
// @Description  Creates a new review in 'pending' status for moderation.
// @Tags         Reviews
// @Accept       json
// @Produce      json
// @Param        body  body      data.CreateReviewInput  true  "Review details"
// @Success      201   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Employer not found"
// @Failure      409   {object}  ErrorResponse  "Review already exists"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /reviews [post]
func (app *application) addReviewHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateReviewInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateCreateReviewInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	review, err := app.models.Reviews.Insert(r.Context(), app.db, user.ID, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrDuplicateReview):
			app.errorResponse(w, r, http.StatusConflict, "you have already submitted a review for this programme")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	response := data.ReviewSubmissionResponse{
		ID:            review.ID,
		EmployerID:    review.EmployerID,
		ProgrammeName: review.ProgrammeName,
		Status:        review.Status,
		CreatedAt:     review.CreatedAt,
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateReviewHandler godoc
// @Summary      Update an existing review
// @Description  Updates a review belonging to the authenticated user. Edited reviews revert to 'pending' status for re-moderation.
// @Tags         Reviews
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "Review ID"
// @Param        body  body      data.UpdateReviewInput  true  "Fields to update"
// @Success      200   {object}  envelope
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /reviews/{id} [patch]
func (app *application) updateReviewHandler(w http.ResponseWriter, r *http.Request) {
	reviewID, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.UpdateReviewInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateReviewInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	user := app.contextGetUser(r)

	review, err := app.models.Reviews.Update(r.Context(), app.db, user.ID, reviewID, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": review}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listReviewsHandler godoc
// @Summary      List reviews
// @Description  List approved community reviews for an employer
// @Tags         Reviews
// @Produce      json
// @Param        slug       path   string  true   "Employer slug"
// @Param        sort       query  string  false  "Sort field"    default(-created_at)
// @Param        page       query  int     false  "Page number"   default(1)
// @Param        page_size  query  int     false  "Items per page" default(20)
// @Success      200  {object}  envelope{data=[]data.Review,pagination=data.Metadata}
// @Failure      422  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /employers/{slug}/reviews [get]
func (app *application) listReviewsHandler(w http.ResponseWriter, r *http.Request) {
	slug, err := app.readSlugParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var filters data.Filters

	v := validator.New()
	qs := r.URL.Query()

	filters.Page = app.readInt(qs, "page", 1, v)
	filters.PageSize = app.readInt(qs, "page_size", 20, v)
	filters.Sort = app.readString(qs, "sort", "-created_at")
	filters.SortSafeList = []string{"difficulty_rating", "experience_rating", "created_at", "-difficulty_rating", "-experience_rating", "-created_at"}

	if data.ValidateFilters(v, filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	reviews, metadata, err := app.models.Reviews.GetAllByEmployerSlug(r.Context(), app.db, slug, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": reviews, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// moderateReviewHandler godoc
// @Summary      Moderate a community review (admin only)
// @Description  Approves or rejects a pending review. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Review ID"
// @Param        body  body      data.ModerateReviewInput  true  "Moderation decision"
// @Success      200   {object}  envelope{data=data.Review}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/reviews/{id} [patch]
func (app *application) moderateReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.ModerateReviewInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateModerateReviewInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	review, err := app.models.Reviews.Moderate(r.Context(), app.db, id, input.Status)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Queue aggregate recalc if the review was approved
	if input.Status == "approved" {
		taskPayload := map[string]any{
			"employer_id": review.EmployerID,
		}
		if err := app.models.Tasks.Insert(r.Context(), app.db, "employer:recalc_ratings", taskPayload); err != nil {
			app.logger.Error("failed to queue rating recalc", "error", err, "employer_id", review.EmployerID)
			// Don't fail the request — moderation succeeded; recalc can be retried
		}
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": review}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listReviewsForModerationHandler godoc
// @Summary      List reviews in the moderation queue (admin only)
// @Description  Returns reviews filtered by status for moderation workflow. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Param        status     query  string  false  "Filter by status"  Enums(pending, approved, rejected)  default(pending)
// @Param        sort       query  string  false  "Sort field"        Enums(created_at, -created_at)      default(created_at)
// @Param        page       query  int     false  "Page number"       default(1)
// @Param        page_size  query  int     false  "Items per page"    default(20)
// @Success      200  {object}  envelope{data=[]data.AdminReview,pagination=data.Metadata}
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      422  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/reviews [get]
func (app *application) listReviewsForModerationHandler(w http.ResponseWriter, r *http.Request) {
	var filters data.Filters

	v := validator.New()
	qs := r.URL.Query()

	status := app.readString(qs, "status", "pending")
	filters.Page = app.readInt(qs, "page", 1, v)
	filters.PageSize = app.readInt(qs, "page_size", 20, v)
	filters.Sort = app.readString(qs, "sort", "created_at")
	filters.SortSafeList = []string{"created_at", "-created_at"}

	data.ValidateReviewStatusFilter(v, status)
	data.ValidateFilters(v, filters)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	reviews, metadata, err := app.models.Reviews.GetAllForModeration(r.Context(), app.db, status, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": reviews, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
