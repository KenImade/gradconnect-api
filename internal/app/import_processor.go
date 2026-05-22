package app

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api.gradconnect.com/internal/data"
	"github.com/jackc/pgx/v5"
)

// rowProcessor is the per-type function signature. Each implementation
// processes one row in its own transaction and returns a typed error
// on failure (which becomes part of the row_errors report).
type rowProcessor func(ctx context.Context, tx pgx.Tx, row []string, colIdx map[string]int) error

func (app *App) processImport(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	job, err := app.models.ImportJob.GetByID(ctx, app.db, jobID)
	if err != nil {
		return fmt.Errorf("fetch import job: %w", err)
	}

	if err := app.models.ImportJob.MarkProcessing(ctx, app.db, jobID); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	// Always clean up the uploaded file from storage on completion,
	// success or failure. The job record retains all the info we need.
	defer func() {
		if err := app.storage.Delete(context.Background(), job.FilePath); err != nil {
			app.logger.Warn("cleanup uploaded file", "err", err, "key", job.FilePath)
		}
	}()

	body, err := app.storage.Download(ctx, job.FilePath)
	if err != nil {
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, 0, fmt.Sprintf("download file: %v", err))
		return err
	}
	defer body.Close()

	reader := csv.NewReader(body)
	reader.FieldsPerRecord = -1 // permit variable-width rows; per-row processors validate

	records, err := reader.ReadAll()
	if err != nil {
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, 0, fmt.Sprintf("parse csv: %v", err))
		return err
	}

	if len(records) < 2 {
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, 0, "csv must contain a header row and at least one data row")
		return errors.New("empty csv")
	}

	header := records[0]
	rows := records[1:]
	colIdx := indexColumns(header)

	// Pick the per-row processor for this import type. Failing here
	// means the type is unrecognised — we mark the entire job failed
	// since there are no per-row results to report.
	var processor rowProcessor
	switch job.ImportType {
	case "employers":
		if err := requireColumns(colIdx, "name", "slug", "industry"); err != nil {
			_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, len(rows), err.Error())
			return err
		}
		processor = app.processEmployerRow
	case "opportunities":
		if err := requireColumns(colIdx, "employer_id", "title", "slug", "type", "intake_year", "description", "location", "application_url"); err != nil {
			_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, len(rows), err.Error())
			return err
		}
		processor = app.processOpportunityRow
	case "assessments":
		if err := requireColumns(colIdx, "employer_slug", "programme_type"); err != nil {
			_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, len(rows), err.Error())
			return err
		}
		processor = app.processAssessmentRow
	default:
		msg := fmt.Sprintf("unknown import type: %s", job.ImportType)
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, len(rows), msg)
		return errors.New(msg)
	}

	// Process each row in its own transaction. Per-row errors collect
	// into rowErrors; valid rows persist regardless of failures elsewhere.
	rowErrors := make([]data.ImportJobRowError, 0)
	rowsImported := 0

	for i, row := range rows {
		rowNumber := i + 2 // +1 for 0-index, +1 for header row

		if len(row) < len(header) {
			rowErrors = append(rowErrors, data.ImportJobRowError{
				RowNumber: rowNumber,
				Message:   fmt.Sprintf("row has %d columns, expected %d (check for missing commas or unescaped quotes)", len(row), len(header)),
				RawData:   truncate(joinRow(row), 200),
			})
			continue
		}

		err := app.inTransaction(ctx, func(tx pgx.Tx) error {
			return processor(ctx, tx, row, colIdx)
		})

		if err != nil {
			rowErrors = append(rowErrors, data.ImportJobRowError{
				RowNumber: rowNumber,
				Message:   err.Error(),
				RawData:   truncate(joinRow(row), 200),
			})
			continue
		}
		rowsImported++
	}

	return app.models.ImportJob.MarkCompleted(ctx, app.db, jobID, len(rows), rowsImported, rowErrors)
}

// --- per-row processors ---

func (app *App) processEmployerRow(ctx context.Context, tx pgx.Tx, row []string, colIdx map[string]int) error {
	input := data.CreateEmployerInput{
		Name:     row[colIdx["name"]],
		Slug:     row[colIdx["slug"]],
		Industry: row[colIdx["industry"]],
	}
	if idx, ok := colIdx["size"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.Size = &v
	}
	if idx, ok := colIdx["hq_location"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.HQLocation = &v
	}
	if idx, ok := colIdx["logo_url"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.LogoURL = &v
	}
	if idx, ok := colIdx["overview"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.Overview = &v
	}
	if idx, ok := colIdx["website"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.Website = &v
	}

	_, err := app.models.Employers.Upsert(ctx, tx, input)
	return err
}

func (app *App) processOpportunityRow(ctx context.Context, tx pgx.Tx, row []string, colIdx map[string]int) error {
	employerSlug := row[colIdx["employer_slug"]]
	employer, err := app.models.Employers.GetBySlug(ctx, tx, employerSlug)
	if err != nil {
		return fmt.Errorf("employer %q not found", employerSlug)
	}

	intakeYear, err := strconv.Atoi(row[colIdx["intake_year"]])
	if err != nil {
		return fmt.Errorf("invalid intake_year: %w", err)
	}

	input := data.CreateOpportunityInput{
		EmployerID:     employer.ID,
		Title:          row[colIdx["title"]],
		Slug:           row[colIdx["slug"]],
		Type:           row[colIdx["type"]],
		IntakeYear:     intakeYear,
		Description:    row[colIdx["description"]],
		Location:       row[colIdx["location"]],
		ApplicationURL: row[colIdx["application_url"]],
		IsActive:       true, // CSV imports default to active; can be overridden via column
	}

	if idx, ok := colIdx["requirements"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.Requirements = &v
	}
	if idx, ok := colIdx["discipline_tags"]; ok && idx < len(row) && row[idx] != "" {
		input.DisciplineTags = splitAndTrim(row[idx], "|")
	}
	if idx, ok := colIdx["opens_at"]; ok && idx < len(row) && row[idx] != "" {
		d, err := data.ParseDate(row[idx])
		if err != nil {
			return fmt.Errorf("invalid opens_at: %w", err)
		}
		input.OpensAt = &d
	}
	if idx, ok := colIdx["deadline"]; ok && idx < len(row) && row[idx] != "" {
		d, err := data.ParseDate(row[idx])
		if err != nil {
			return fmt.Errorf("invalid deadline: %w", err)
		}
		input.Deadline = &d
	}
	if idx, ok := colIdx["source_url"]; ok && idx < len(row) && row[idx] != "" {
		v := row[idx]
		input.SourceURL = &v
	}

	// Optional: if "is_active" is in the CSV, respect it
	if idx, ok := colIdx["is_active"]; ok && idx < len(row) && row[idx] != "" {
		input.IsActive = strings.ToLower(row[idx]) == "true"
	}

	_, err = app.models.Opportunities.Upsert(ctx, tx, input)
	return err
}

func (app *App) processAssessmentRow(ctx context.Context, tx pgx.Tx, row []string, colIdx map[string]int) error {
	return errors.New("assessment import not yet implemented — use the API directly")
}

// --- helpers ---

func indexColumns(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[name] = i
	}
	return idx
}

func requireColumns(colIdx map[string]int, required ...string) error {
	for _, c := range required {
		if _, ok := colIdx[c]; !ok {
			return fmt.Errorf("missing required column: %s", c)
		}
	}
	return nil
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func joinRow(row []string) string {
	return strings.Join(row, ",")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
