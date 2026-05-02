package app

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
func (app *App) createAssessmentHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *App) updateAssessmentHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *App) listAssessmentsHandler(w http.ResponseWriter, r *http.Request) {
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

// showAdminAssessmentHandler godoc
// @Summary      Get an assessment (admin)
// @Description  Fetch a single assessment by UUID with employer stub embedded.
// @Tags         Admin
// @Produce      json
// @Security     SessionCookie
// @Param        id   path  string  true  "Assessment UUID"
// @Success      200  {object}  envelope{data=data.Assessment}
// @Failure      401  {object}  envelope{error=string}
// @Failure      403  {object}  envelope{error=string}
// @Failure      404  {object}  envelope{error=string}
// @Failure      500  {object}  envelope{error=object}
// @Router       /admin/assessments/{id} [get]
func (app *App) showAdminAssessmentHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	assessment, err := app.models.Assessments.GetByID(r.Context(), app.db, id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFoundResponse(w, r)
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": assessment}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listAdminAssessmentsHandler godoc
// @Summary      List assessments (admin)
// @Description  List all assessment profiles across all employers, with employer stub embedded.
// @Tags         Admin
// @Produce      json
// @Security     SessionCookie
// @Param        q            query  string  false  "Search programme type or employer name (case-insensitive substring)"
// @Param        employer_id  query  string  false  "Filter to a single employer (UUID)"
// @Param        sort         query  string  false  "Sort field (prefix with - for descending)"  Enums(programme_type, created_at, updated_at, -programme_type, -created_at, -updated_at)  default(-updated_at)
// @Param        page         query  int     false  "Page number"  default(1)
// @Param        page_size    query  int     false  "Items per page (max 50)"  default(50)
// @Success      200  {object}  envelope{data=[]data.Assessment,pagination=data.Metadata}
// @Failure      401  {object}  envelope{error=string}
// @Failure      403  {object}  envelope{error=string}
// @Failure      422  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /admin/assessments [get]
func (app *App) listAdminAssessmentsHandler(w http.ResponseWriter, r *http.Request) {
	var input data.AssessmentFilters

	v := validator.New()
	qs := r.URL.Query()

	input.Search = app.readString(qs, "q", "")
	input.EmployerID = app.readString(qs, "employer_id", "")

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 50, v)
	input.Filters.Sort = app.readString(qs, "sort", "-updated_at")
	input.Filters.SortSafeList = []string{
		"programme_type", "created_at", "updated_at",
		"-programme_type", "-created_at", "-updated_at",
	}

	if input.EmployerID != "" {
		v.Check(validator.IsValidUUID(input.EmployerID), "employer_id", "must be a valid UUID")
	}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	assessments, metadata, err := app.models.Assessments.GetAll(r.Context(), app.db, input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": assessments, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
