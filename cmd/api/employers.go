package main

import (
	"errors"
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
	"github.com/julienschmidt/httprouter"
)

func (app *application) createEmployerHandler(w http.ResponseWriter, r *http.Request) {
	// v.Check(validator.IsValidUUID(input.EmployerID), "employer_id", "must be a valid UUID")
}

// @Summary      Show employer
// @Description  Get a full employer hub profile by slug or ID
// @Tags         Employers
// @Produce      json
// @Param        identifier  path  string  true  "Employer slug or UUID"
// @Success      200  {object}  envelope{data=data.Employer}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /employers/{identifier} [get]
func (app *application) showEmployerHandler(w http.ResponseWriter, r *http.Request) {
	params := httprouter.ParamsFromContext(r.Context())
	identifier := params.ByName("identifier")

	var employer *data.Employer
	var err error

	if validator.IsValidUUID(identifier) {
		employer, err = app.models.Employers.GetByID(identifier)
	} else {
		employer, err = app.models.Employers.GetBySlug(identifier)
	}

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

func (app *application) updateEmployerHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) deleteEmployerHandler(w http.ResponseWriter, r *http.Request) {}

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

	if data.ValidationFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	employers, metadata, err := app.models.Employers.GetAll(input.Search, input.Industry, input.IsVerified, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": employers, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
