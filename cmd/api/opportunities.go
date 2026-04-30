package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// createOpportunityHandler godoc
// @Summary      Create a new opportunity (admin only)
// @Description  Creates a new opportunity listing. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        body  body      data.CreateOpportunityInput  true  "Opportunity details"
// @Success      201   {object}  envelope{data=data.Opportunity}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse  "Employer not found"
// @Failure      409   {object}  ErrorResponse  "Slug already exists"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/opportunities [post]
func (app *application) createOpportunityHandler(w http.ResponseWriter, r *http.Request) {
	var input data.CreateOpportunityInput

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateCreateOpportunityInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	opportunity, err := app.models.Opportunities.Insert(r.Context(), app.db, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrDuplicateOpportunitySlug):
			app.errorResponse(w, r, http.StatusConflict, "an opportunity with this slug already exists")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/api/v1/opportunities/%s", opportunity.Slug))

	err = app.writeJSON(w, http.StatusCreated, envelope{"data": opportunity}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// showOpportunityByIDHandler godoc
// @Summary      Show opportunity
// @Description  Get a full opportunity profile by id
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Opportunity id"
// @Success      200  {object}  envelope{data=data.Opportunity}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /admin/opportunity/{id} [get]
func (app *application) showOpportunityByIDHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	opportunity, err := app.models.Opportunities.GetByID(r.Context(), app.db, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunity}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// updateOpportunityHandler godoc
// @Summary      Update an opportunity (admin only)
// @Description  Updates an existing opportunity. Requires admin:full permission.
// @Tags         Admin
// @Accept       json
// @Produce      json
// @Param        id    path      string                       true  "Opportunity ID"
// @Param        body  body      data.UpdateOpportunityInput  true  "Fields to update"
// @Success      200   {object}  envelope{data=data.Opportunity}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      404   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/opportunities/{id} [patch]
func (app *application) updateOpportunityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input data.UpdateOpportunityInput

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	data.ValidateUpdateOpportunityInput(v, input)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	opportunity, err := app.models.Opportunities.Update(r.Context(), app.db, id, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrDuplicateOpportunitySlug):
			app.errorResponse(w, r, http.StatusConflict, "an opportunity with this slug already exists")
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunity}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteOpportunityHandler godoc
// @Summary      Delete an opportunity (admin only)
// @Description  Deletes an opportunity. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Opportunity ID"
// @Success      204
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/opportunities/{id} [delete]
func (app *application) deleteOpportunityHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	err = app.models.Opportunities.Delete(r.Context(), app.db, id)
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

// showOpportunityBySlugHandler godoc
// @Summary      Show opportunity
// @Description  Get a full opportunity profile by slug
// @Tags         Opportunities
// @Produce      json
// @Param        slug  path  string  true  "Opportunity slug"
// @Success      200  {object}  envelope{data=data.Opportunity}
// @Failure      404  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /opportunities/{slug} [get]
func (app *application) showOpportunityBySlugHandler(w http.ResponseWriter, r *http.Request) {
	slug, err := app.readSlugParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	opportunity, err := app.models.Opportunities.GetBySlug(r.Context(), app.db, slug)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunity}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listOpportunitiesHandler godoc
// @Summary      List opportunities (public)
// @Description  List graduate opportunities visible to the public — open and upcoming only.
// @Description  Closed and withdrawn listings are hidden; admins should use /admin/opportunities for the full ledger view.
// @Tags         Opportunities
// @Produce      json
// @Param        q                query  string  false  "Keyword search across title, employer name, location, discipline, industry, requirements, and description"
// @Param        type             query  string  false  "Filter by type"  Enums(graduate_trainee, internship, nysc, industrial_attachment)
// @Param        status           query  string  false  "Filter by computed status. Public consumers may only request open or upcoming listings (or both via 'open_or_upcoming')."  Enums(open, upcoming, open_or_upcoming)  default(open_or_upcoming)
// @Param        intake_year      query  int     false  "Filter by programme intake year"
// @Param        industry         query  string  false  "Filter by employer's industry (exact match)"
// @Param        location         query  string  false  "Filter by location (case-insensitive substring match)"
// @Param        discipline       query  string  false  "Filter by discipline tag (array contains)"
// @Param        deadline_before  query  string  false  "Show opportunities with deadline before this date (YYYY-MM-DD)"
// @Param        deadline_after   query  string  false  "Show opportunities with deadline after this date (YYYY-MM-DD)"
// @Param        sort             query  string  false  "Sort field (prefix with - for descending)"  Enums(deadline, opens_at, created_at, title, -deadline, -opens_at, -created_at, -title)  default(deadline)
// @Param        page             query  int     false  "Page number"  default(1)
// @Param        page_size        query  int     false  "Items per page (max 50)"  default(20)
// @Success      200  {object}  envelope{data=[]data.Opportunity,pagination=data.Metadata}
// @Failure      422  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /opportunities [get]
func (app *application) listOpportunitiesHandler(w http.ResponseWriter, r *http.Request) {
	var input data.OpportunityFilters

	v := validator.New()
	qs := r.URL.Query()

	input.Search = app.readString(qs, "q", "")
	input.Type = app.readString(qs, "type", "")
	input.Status = app.readString(qs, "status", "open_or_upcoming")

	// Public consumers can only see open or upcoming opportunities
	publicAllowed := []string{"open", "upcoming", "open_or_upcoming"}
	v.Check(
		validator.PermittedValue(input.Status, publicAllowed...),
		"status",
		"must be one of: open, upcoming, open_or_upcoming",
	)

	input.IntakeYear = app.readInt(qs, "intake_year", 0, v)
	input.Industry = app.readString(qs, "industry", "")
	input.Location = app.readString(qs, "location", "")
	input.Discipline = app.readString(qs, "discipline", "")
	input.DeadlineBefore = app.readDate(qs, "deadline_before", time.Time{}, v)
	input.DeadlineAfter = app.readDate(qs, "deadline_after", time.Time{}, v)

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "deadline")
	input.Filters.SortSafeList = []string{"deadline", "opens_at", "created_at", "title", "-deadline", "-opens_at", "-created_at", "-title"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	opportunities, metadata, err := app.models.Opportunities.GetAll(r.Context(), app.db, input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunities, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listAdminOpportunitiesHandler godoc
// @Summary      List opportunities (admin)
// @Description  List all graduate opportunities including closed and withdrawn.
// @Description  Admin-only ledger view of every record on the platform regardless of status.
// @Tags         Admin
// @Produce      json
// @Security     SessionCookie
// @Param        q                query  string  false  "Keyword search across title, employer name, location, discipline, industry, requirements, and description"
// @Param        type             query  string  false  "Filter by type"  Enums(graduate_trainee, internship, nysc, industrial_attachment)
// @Param        status           query  string  false  "Filter by computed status. Admins may request any status, including closed and withdrawn, or 'all' to see every status."  Enums(all, open, upcoming, closed, withdrawn)  default(all)
// @Param        intake_year      query  int     false  "Filter by programme intake year"
// @Param        industry         query  string  false  "Filter by employer's industry (exact match)"
// @Param        location         query  string  false  "Filter by location (case-insensitive substring match)"
// @Param        discipline       query  string  false  "Filter by discipline tag (array contains)"
// @Param        deadline_before  query  string  false  "Show opportunities with deadline before this date (YYYY-MM-DD)"
// @Param        deadline_after   query  string  false  "Show opportunities with deadline after this date (YYYY-MM-DD)"
// @Param        sort             query  string  false  "Sort field (prefix with - for descending). Defaults to newest-first for admin moderation workflows."  Enums(deadline, opens_at, created_at, title, -deadline, -opens_at, -created_at, -title)  default(-created_at)
// @Param        page             query  int     false  "Page number"  default(1)
// @Param        page_size        query  int     false  "Items per page (max 50). Higher default than the public endpoint for bulk admin operations."  default(50)
// @Success      200  {object}  envelope{data=[]data.Opportunity,pagination=data.Metadata}
// @Failure      401  {object}  envelope{error=string}
// @Failure      403  {object}  envelope{error=string}
// @Failure      422  {object}  envelope{error=object}
// @Failure      500  {object}  envelope{error=object}
// @Router       /admin/opportunities [get]
func (app *application) listAdminOpportunitiesHandler(w http.ResponseWriter, r *http.Request) {
	var input data.OpportunityFilters

	v := validator.New()
	qs := r.URL.Query()

	input.Search = app.readString(qs, "q", "")
	input.Type = app.readString(qs, "type", "")
	input.Status = app.readString(qs, "status", "all")

	adminAllowed := []string{"all", "open", "upcoming", "closed", "withdrawn"}
	v.Check(
		validator.PermittedValue(input.Status, adminAllowed...),
		"status",
		"must be one of: all, open, upcoming, closed, withdrawn",
	)

	input.IntakeYear = app.readInt(qs, "intake_year", 0, v)
	input.Industry = app.readString(qs, "industry", "")
	input.Location = app.readString(qs, "location", "")
	input.Discipline = app.readString(qs, "discipline", "")
	input.DeadlineBefore = app.readDate(qs, "deadline_before", time.Time{}, v)
	input.DeadlineAfter = app.readDate(qs, "deadline_after", time.Time{}, v)

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 50, v)   // ← higher default for admin
	input.Filters.Sort = app.readString(qs, "sort", "-created_at") // ← newest first for admin
	input.Filters.SortSafeList = []string{"deadline", "opens_at", "created_at", "title", "-deadline", "-opens_at", "-created_at", "-title"}

	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	opportunities, metadata, err := app.models.Opportunities.GetAll(r.Context(), app.db, input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunities, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
