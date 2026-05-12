package app

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/imagegen"
	"api.gradconnect.com/internal/validator"
	"api.gradconnect.com/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

// startImportHandler godoc
// @Summary      Start a bulk data import (admin only)
// @Description  Accepts a CSV file and queues an async import job. Requires admin:full permission.
// @Tags         Admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        type  query     string  true  "Import type"  Enums(employers, opportunities, assessments)
// @Param        file  formData  file    true  "CSV file"
// @Success      202   {object}  envelope{data=data.ImportJob}
// @Failure      400   {object}  ErrorResponse
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      413   {object}  ErrorResponse  "File too large"
// @Failure      422   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/import [post]
func (app *App) startImportHandler(w http.ResponseWriter, r *http.Request) {
	importType := r.URL.Query().Get("type")
	if !data.IsPermittedImportType(importType) {
		app.errorResponse(w, r, http.StatusUnprocessableEntity, "invalid import type; must be one of: employers, opportunities, assessments")
		return
	}

	// Limit upload size to 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		app.errorResponse(w, r, http.StatusRequestEntityTooLarge, "file too large (max 5MB)")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		app.badRequestResponse(w, r, errors.New("missing 'file' field in form data"))
		return
	}
	defer file.Close()

	if filepath.Ext(header.Filename) != ".csv" {
		app.errorResponse(w, r, http.StatusUnprocessableEntity, "file must have .csv extension")
		return
	}

	// Save to disk with a unique name
	// Upload to R2 with a unique key. The key is what gets stored in
	// import_job.file_path; processImport reads it back via app.storage.Download.
	storageKey := fmt.Sprintf("imports/%s-%s.csv", importType, uuid.NewString())

	if _, err := app.storage.Upload(r.Context(), storageKey, "text/csv", file); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	var job *data.ImportJob

	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		var err error
		job, err = app.models.ImportJob.Insert(r.Context(), tx, user.ID, importType, storageKey)
		if err != nil {
			return err
		}

		taskPayload := map[string]any{
			"import_job_id": job.ID,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "admin:import", taskPayload)
	})

	if err != nil {
		// Clean up the file from storage if the transaction failed
		_ = app.storage.Delete(r.Context(), storageKey)
		app.serverErrorResponse(w, r, err)
		return
	}

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/api/v1/admin/import/%s", job.ID))

	err = app.writeJSON(w, http.StatusAccepted, envelope{"data": job}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getImportJobHandler godoc
// @Summary      Get import job status (admin only)
// @Description  Returns the current status and results of an import job. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Param        id  path  string  true  "Import job ID"
// @Success      200  {object}  envelope{data=data.ImportJob}
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/import/{id} [get]
func (app *App) getImportJobHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	job, err := app.models.ImportJob.GetByID(r.Context(), app.db, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": job}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// listImportJobsHandler godoc
// @Summary      List recent import jobs
// @Description  Returns the most recent import jobs, newest first. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Param        limit  query  int  false  "Max results (1-100, default 50)"
// @Success      200    {object}  envelope{data=[]data.ImportJob}
// @Failure      401    {object}  ErrorResponse
// @Failure      403    {object}  ErrorResponse
// @Failure      422    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /admin/import [get]
func (app *App) listImportJobsHandler(w http.ResponseWriter, r *http.Request) {
	qs := r.URL.Query()
	v := validator.New()

	limit := app.readInt(qs, "limit", 50, v)
	v.Check(limit >= 1, "limit", "must be at least 1")
	v.Check(limit <= 100, "limit", "must be at most 100")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	jobs, err := app.models.ImportJob.GetRecent(r.Context(), app.db, limit)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"data": jobs}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// triggerDeadlineRemindersHandler godoc
// @Summary      Trigger the deadline reminder enqueue job
// @Description  Manually fires the deadline reminder enqueue job. Useful for testing in staging,
// @Description  recovery from a missed cron, or admin-initiated re-runs. Returns the count of
// @Description  reminder jobs queued for execution. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Success      202   {object}  envelope{data=data.DeadlineReminderResponse}
// @Failure      401   {object}  ErrorResponse
// @Failure      403   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /admin/jobs/deadline-reminders [post]
func (app *App) triggerDeadlineRemindersHandler(w http.ResponseWriter, r *http.Request) {
	enqueued, err := app.worker.RunDeadlineRemindersNow(r.Context(), app.config.BaseURL, app.config.FrontendURL)
	if err != nil {
		if errors.Is(err, worker.ErrAlreadyRanToday) {
			app.errorResponse(w, r, http.StatusConflict, "deadline reminders already ran today")
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	resp := data.DeadlineReminderResponse{
		Enqueued: enqueued,
		Message:  "deadline reminders enqueued",
	}

	err = app.writeJSON(w, http.StatusAccepted, envelope{"data": resp}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// getAnalyticsHandler godoc
// @Summary      Admin dashboard analytics
// @Description  Returns aggregated counts, 30-day time-series data, top employers,
// @Description  top opportunities, and recent job runs in a single response.
// @Description  Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Success      200  {object}  envelope{data=data.AnalyticsResponse}
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/analytics [get]
func (app *App) getAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp := data.AnalyticsResponse{}

	// Six independent queries run in parallel. errgroup cancels in-flight
	// queries if any one fails, so we don't waste compute on a request
	// the client will never see.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		c, err := app.models.Analytics.GetCounts(gctx)
		if err != nil {
			return err
		}
		resp.Counts = c
		return nil
	})

	g.Go(func() error {
		s, err := app.models.Analytics.RegistrationsTimeSeries(gctx)
		if err != nil {
			return err
		}
		resp.TimeSeries.Registrations = s
		return nil
	})

	g.Go(func() error {
		s, err := app.models.Analytics.BookmarksTimeSeries(gctx)
		if err != nil {
			return err
		}
		resp.TimeSeries.Bookmarks = s
		return nil
	})

	g.Go(func() error {
		s, err := app.models.Analytics.ReviewsTimeSeries(gctx)
		if err != nil {
			return err
		}
		resp.TimeSeries.ReviewsSubmitted = s
		return nil
	})

	g.Go(func() error {
		t, err := app.models.Analytics.TopEmployers(gctx, 10)
		if err != nil {
			return err
		}
		resp.TopEmployers = t
		return nil
	})

	g.Go(func() error {
		t, err := app.models.Analytics.TopOpportunities(gctx, 10)
		if err != nil {
			return err
		}
		resp.TopOpportunities = t
		return nil
	})

	g.Go(func() error {
		j, err := app.models.Analytics.RecentJobs(gctx)
		if err != nil {
			return err
		}
		resp.RecentJobs = j
		return nil
	})

	if err := g.Wait(); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if err := app.writeJSON(w, http.StatusOK, envelope{"data": resp}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type RecalcRatingsResponse struct {
	Recalculated int    `json:"recalculated"`
	Message      string `json:"message"`
}

// triggerRecalcRatingsHandler godoc
// @Summary      Trigger employer ratings recalculation
// @Description  Recomputes cached avg_difficulty_rating, avg_experience_rating, and
// @Description  review_count for every employer. Returns the number of employers processed.
// @Description  Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Success      202  {object}  envelope{data=RecalcRatingsResponse}
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      409  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/jobs/recalculate-ratings [post]
func (app *App) triggerRecalcRatingsHandler(w http.ResponseWriter, r *http.Request) {
	count, err := app.worker.RunRecalcRatingsNow(r.Context())
	if err != nil {
		if errors.Is(err, worker.ErrAlreadyRanToday) {
			app.errorResponse(w, r, http.StatusConflict, "ratings recalculation already ran today")
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	resp := RecalcRatingsResponse{
		Recalculated: count,
		Message:      "employer ratings recalculated",
	}

	if err := app.writeJSON(w, http.StatusAccepted, envelope{"data": resp}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

type CleanupSessionsResponse struct {
	Deleted int    `json:"deleted"`
	Message string `json:"message"`
}

// triggerCleanupSessionsHandler godoc
// @Summary      Trigger expired session cleanup
// @Description  Deletes all expired sessions from the database. Returns the count deleted.
// @Description  Safe to run multiple times. Requires admin:full permission.
// @Tags         Admin
// @Produce      json
// @Success      202  {object}  envelope{data=CleanupSessionsResponse}
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/jobs/cleanup-sessions [post]
func (app *App) triggerCleanupSessionsHandler(w http.ResponseWriter, r *http.Request) {
	deleted, err := app.worker.RunCleanupSessionsNow(r.Context())
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	resp := CleanupSessionsResponse{
		Deleted: deleted,
		Message: "expired sessions cleaned up",
	}

	if err := app.writeJSON(w, http.StatusAccepted, envelope{"data": resp}, nil); err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// generateOpportunityImageHandler godoc
// @Summary      Generate social media post for opportunity
// @Description  Generates a PNG image of a job opportunity for Social Media posts.
// @Description  Safe to run multiple times. Requires admin:full permission.
// @Tags         Admin
// @Produce      png
// @Success      200  {file}  binary  "PNG image of the opportunity card"
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /admin/opportunities/{id}/image [get]
// @Param        id      path      string  true   "Opportunity ID"
// @Param        format  query     string  false  "Image format" Enums(twitter, instagram_square, instagram_portrait, story) default(twitter)
func (app *App) generateOpportunityImageHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}
	formatStr := r.URL.Query().Get("format")
	if formatStr == "" {
		formatStr = string(imagegen.FormatTwitter)
	}
	format := imagegen.Format(formatStr)

	switch format {
	case imagegen.FormatTwitter,
		imagegen.FormatInstagramPortrait,
		imagegen.FormatInstagramSquare,
		imagegen.FormatStory:
	default:
		app.badRequestResponse(w, r, fmt.Errorf("unknown format %q", formatStr))
		return
	}

	op, err := app.models.Opportunities.GetByID(r.Context(), app.db, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	card := imagegen.OpportunityCard{
		Title:        op.Title,
		EmployerName: op.Employer.Name,
		Location:     op.Location,
		Deadline:     op.Deadline,
	}

	img, err := app.imagegen.GenerateOpportunityCard(card, format)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	filename := fmt.Sprintf("opportunity-%s-%s.png", op.ID, format)
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(img)))

	if _, err := w.Write(img); err != nil {
		app.logger.Error("writing image response", "err", err)
	}
}
