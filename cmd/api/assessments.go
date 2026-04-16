package main

import (
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

func (app *application) createAssessmentHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) showAssessmentHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) updateAssessmentHandler(w http.ResponseWriter, r *http.Request) {}

func (app *application) deleteAssessmentHandler(w http.ResponseWriter, r *http.Request) {}

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

	var input struct {
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "programme_type")
	input.Filters.SortSafeList = []string{"programme_type", "updated_at", "-programme_type", "-updated_at"}

	if data.ValidationFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	assessments, metadata, err := app.models.Assessments.GetAllByEmployerSlug(slug, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": assessments, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
