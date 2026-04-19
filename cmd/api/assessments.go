package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// createAssessmentHandler godoc
// @Summary      Create an assessment profile (admin only)
// @Description  Creates an assessment profile for an employer. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body      data.CreateAssessmentInput  true  "Assessment details"
// @Success      201   {object}  envelope{data=data.Assessment}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Employer not found"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/assessments [post]
func (app *application) createAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateAssessmentInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateCreateAssessmentInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	assessment, err := app.models.Assessments.Insert(r.Context(), app.db, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": assessment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateAssessmentHandler godoc
// @Summary      Update an assessment profile (admin only)
// @Description  Updates an assessment profile. Requires admin:full permission and current version.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        id    path      string                      true  "Assessment ID"
// @Param        body  body      data.UpdateAssessmentInput  true  "Fields to update"
// @Success      200   {object}  envelope{data=data.Assessment}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse  "Version mismatch"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/assessments/{id} [patch]
func (app *application) updateAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.UpdateAssessmentInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateAssessmentInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	assessment, err := app.models.Assessments.Update(r.Context(), app.db, id, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrEditConflict):
			app.errorResponse(w, r, http.StatusConflict, "version mismatch, please refresh and retry")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": assessment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// @Summary      List assessments
// @Description  List assessment profiles for an employer
// @Tags         Assessments
// @Produce      json
// @Param        slug       path   string  true   "Employer slug"
// @Param        sort       query  string  false  "Sort field: programme_type, updated_at"  default(programme_type)
// @Param        page       query  int     false  "Page number"                              default(1)
// @Param        page_size  query  int     false  "Items per page"                           default(20)
// @Success      200  {object}  envelope{data=[]data.Assessment,pagination=data.Metadata}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /employers/{slug}/assessments [get]
func (app *application) listAssessmentsHandler(w http.ResponseWriter, r *http.Request) {
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
	filters.Sort = app.readString(qs, "sort", "programme_type")
	filters.SortSafeList = []string{"programme_type", "updated_at", "-programme_type", "-updated_at"}

	if data.ValidateFilters(v, filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	assessments, metadata, err := app.models.Assessments.GetAllByEmployerSlug(r.Context(), app.db, slug, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": assessments, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
