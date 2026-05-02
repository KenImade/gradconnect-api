package main

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"api.gradconnect.com/internal/data"
	"api.gradconnect.com/internal/validator"
	"api.gradconnect.com/internal/worker"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
func (app *application) startImportHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *application) getImportJobHandler(w http.ResponseWriter, r *http.Request) {
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
func (app *application) listImportJobsHandler(w http.ResponseWriter, r *http.Request) {
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

// triggerDeadlineRemindersHandler manually fires the deadline reminder
// enqueue job. Useful for testing in staging, recovery from a missed
// cron, or admin-initiated re-runs.
//
// Bypasses the time-of-day check but still respects the daily idempotency
// guard via the cron_run table — call it twice on the same day and the
// second call returns "already run today".
func (app *application) triggerDeadlineRemindersHandler(w http.ResponseWriter, r *http.Request) {
	enqueued, err := app.worker.RunDeadlineRemindersNow(r.Context(), app.config.baseURL, app.config.frontendURL)
	if err != nil {
		if errors.Is(err, worker.ErrAlreadyRanToday) {
			app.errorResponse(w, r, http.StatusConflict, "deadline reminders already ran today")
			return
		}
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{
		"data": map[string]any{
			"enqueued": enqueued,
			"message":  "deadline reminders enqueued",
		},
	}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
