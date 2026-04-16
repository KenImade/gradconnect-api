package main

import (
	"errors"
	"net/http"
	"time"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

func (app *application) createOpportunityHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) showOpportunityHandler(w http.ResponseWriter, r *http.Request) {}

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
		app.serverErrorResponse(w, r, err)
		return
	}

	opportunity, err := app.models.Opportunities.GetBySlug(slug)
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

func (app *application) updateOpportunityHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) deleteOpportunityHanlder(w http.ResponseWriter, r *http.Request) {}

// @Summary      List opportunities
// @Description  List graduate opportunities with filtering, keyword search, and pagination
// @Tags         Opportunities
// @Produce      json
// @Param        q                query  string  false  "Keyword search across title, employer name, location, discipline, industry, requirements, and description"
// @Param        type             query  string  false  "Filter by type"  Enums(graduate_trainee, internship, nysc, industrial_attachment)
// @Param        status           query  string  false  "Filter by computed status"  Enums(upcoming, open, closed, withdrawn, all)  default(open)
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
	input.Status = app.readString(qs, "status", "open")
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

	opportunities, metadata, err := app.models.Opportunities.GetAll(input)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": opportunities, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
