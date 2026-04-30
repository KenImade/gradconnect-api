package main

import (
	"errors"
	"fmt"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
	"github.com/julienschmidt/httprouter"
)

// createEmployerHandler godoc
// @Summary      Create a new employer (admin only)
// @Description  Creates a new employer profile. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body      data.CreateEmployerInput  true  "Employer details"
// @Success      201   {object}  envelope{data=data.Employer}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/employers [post]
func (app *application) createEmployerHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateEmployerInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateCreateEmployerInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	employer, err := app.models.Employers.Insert(r.Context(), app.db, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmployerSlug):
			v.AddError("slug", "an employer with this slug already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/api/v1/employers/%s", employer.Slug))

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": employer}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showEmployerBySlugHandler godoc
// @Summary      Show employer
// @Description  Get a full employer hub profile by slug
// @Tags         Employers
// @Produce      json
// @Param        slug  path  string  true  "Employer slug"
// @Success      200  {object}  envelope{data=data.Employer}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /employers/{slug} [get]
func (app *application) showEmployerBySlugHandler(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	slug := params.ByName("slug")

	employer, err := app.models.Employers.GetBySlug(r.Context(), app.db, slug)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": employer}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showEmployerByIDHandler godoc
// @Summary      Show employer
// @Description  Get a full employer hub profile by id
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Employer id"
// @Success      200  {object}  envelope{data=data.Employer}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /admin/employers/{id} [get]
func (app *application) showEmployerByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	employer, err := app.models.Employers.GetByID(r.Context(), app.db, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": employer}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateEmployerHandler godoc
// @Summary      Update an employer (admin only)
// @Description  Updates an existing employer profile. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Employer ID"
// @Param        body  body      data.UpdateEmployerInput  true  "Fields to update"
// @Success      200   {object}  envelope{data=data.Employer}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/employers/{id} [patch]
func (app *application) updateEmployerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.UpdateEmployerInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateEmployerInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	employer, err := app.models.Employers.Update(r.Context(), app.db, id, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrEditConflict):
			app.errorResponse(w, r, http.StatusConflict, "edit conflict, please retry")
		case errors.Is(err, data.ErrDuplicateEmployerSlug):
			v.AddError("slug", "an employer with this slug already exists")
			app.failedValidationResponse(w, r, v.Errors)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": employer}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteEmployerHandler godoc
// @Summary      Delete an employer (admin only)
// @Description  Deletes an employer and cascades to related records. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Employer ID"
// @Success      204
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/employers/{id} [delete]
func (app *application) deleteEmployerHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Employers.Delete(r.Context(), app.db, id)
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

// listEmployersHandler godoc
// @Summary      List employers
// @Description  List employer hub profiles with filtering, search, and pagination
// @Tags         Employers
// @Accept       json
// @Produce      json
// @Param        q            query  string  false  "Full-text search across name, overview, industry, and location"
// @Param        industry     query  string  false  "Filter by industry (exact match)"
// @Param        is_verified  query  bool    false  "Filter by verified status"
// @Param        sort         query  string  false  "Sort field: name, created_at (prefix with - for descending)"  default(name)
// @Param        page         query  int     false  "Page number"                                                   default(1)
// @Param        page_size    query  int     false  "Items per page (max 100)"                                       default(20)
// @Success      200  {object}  envelope{data=[]data.Employer,pagination=data.Metadata}
// @Failure      422  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /employers [get]
func (app *application) listEmployersHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Search     string
		Industry   string
		IsVerified *bool
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Search = app.readString(qs, "q", "")
	input.Industry = app.readString(qs, "industry", "")
	input.IsVerified = app.readBool(qs, "is_verified", nil)

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "name")
	input.Filters.SortSafeList = []string{"name", "created_at", "-name", "-created_at"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	employers, metadata, err := app.models.Employers.GetAll(r.Context(), app.db, input.Search, input.Industry, input.IsVerified, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": employers, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
