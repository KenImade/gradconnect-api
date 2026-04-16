package main

import (
	"net/http"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
)

// @Summary      List reviews
// @Description  List approved community reviews for an employer
// @Tags         Reviews
// @Produce      json
// @Param        slug       path   string  true   "Employer slug"
// @Param        sort       query  string  false  "Sort field: created_at, difficulty_rating, experience_rating"  default(created_at)
// @Param        page       query  int     false  "Page number"                                                   default(1)
// @Param        page_size  query  int     false  "Items per page"                                                default(20)
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

	var input struct {
		data.Filters
	}

	v := validator.New()
	qs := r.URL.Query()

	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)
	input.Filters.Sort = app.readString(qs, "sort", "-created_at")
	input.Filters.SortSafeList = []string{"difficulty_rating", "experience_rating", "created_at", "-difficulty_rating", "-experience_rating", "-created_at"}

	if data.ValidationFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	reviews, metadata, err := app.models.Reviews.GetAllByEmployerSlug(slug, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": reviews, "pagination": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
