package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"api.gradconnect.com/internal/data"
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

	// Limit upload size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		app.errorResponse(w, r, http.StatusRequestEntityTooLarge, "file too large (max 10MB)")
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
	if err := os.MkdirAll(app.config.import_.storageDir, 0755); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	filename := fmt.Sprintf("%s-%s.csv", importType, uuid.NewString())
	fullPath := filepath.Join(app.config.import_.storageDir, filename)

	dst, err := os.Create(fullPath)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	var job *data.ImportJob

	err = app.inTransaction(r.Context(), func(tx pgx.Tx) error {
		var err error
		job, err = app.models.ImportJob.Insert(r.Context(), tx, user.ID, importType, fullPath)
		if err != nil {
			return err
		}

		taskPayload := map[string]any{
			"import_job_id": job.ID,
		}
		return app.models.Tasks.Insert(r.Context(), tx, "admin:import", taskPayload)
	})

	if err != nil {
		// Clean up the file if the transaction failed
		os.Remove(fullPath)
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
