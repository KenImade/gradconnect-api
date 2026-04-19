package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"api.gradconnect.com/internal/data"
	"github.com/jackc/pgx/v5"
)

func (app *application) processImport(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	job, err := app.models.ImportJob.GetByID(ctx, app.db, jobID)
	if err != nil {
		return fmt.Errorf("fetch import job: %w", err)
	}

	if err := app.models.ImportJob.MarkProcessing(ctx, app.db, jobID); err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}

	// Always clean up the uploaded file when done
	defer os.Remove(job.FilePath)

	file, err := os.Open(job.FilePath)
	if err != nil {
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, 0, fmt.Sprintf("open file: %v", err))
		return err
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
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
	rowsTotal := len(rows)

	// Process everything in a single transaction — any row failure rolls back all inserts
	var rowsImported int
	err = app.inTransaction(ctx, func(tx pgx.Tx) error {
		var err error
		switch job.ImportType {
		case "employers":
			rowsImported, err = app.importEmployers(ctx, tx, header, rows)
		case "opportunities":
			rowsImported, err = app.importOpportunities(ctx, tx, header, rows)
		case "assessments":
			rowsImported, err = app.importAssessments(ctx, tx, header, rows)
		default:
			return fmt.Errorf("unknown import type: %s", job.ImportType)
		}
		return err
	})

	if err != nil {
		_ = app.models.ImportJob.MarkFailed(ctx, app.db, jobID, rowsTotal, err.Error())
		return err
	}

	return app.models.ImportJob.MarkCompleted(ctx, app.db, jobID, rowsTotal, rowsImported)
}

// importEmployers processes an employer CSV.
// Expected columns: name, slug, industry, size, hq_location, logo_url, overview, website
func (app *application) importEmployers(ctx context.Context, tx pgx.Tx, header []string, rows [][]string) (int, error) {
	colIdx := indexColumns(header)

	required := []string{"name", "slug", "industry"}
	for _, c := range required {
		if _, ok := colIdx[c]; !ok {
			return 0, fmt.Errorf("missing required column: %s", c)
		}
	}

	for i, row := range rows {
		input := data.CreateEmployerInput{
			Name:     row[colIdx["name"]],
			Slug:     row[colIdx["slug"]],
			Industry: row[colIdx["industry"]],
		}
		if idx, ok := colIdx["size"]; ok && row[idx] != "" {
			v := row[idx]
			input.Size = &v
		}
		if idx, ok := colIdx["hq_location"]; ok && row[idx] != "" {
			v := row[idx]
			input.HQLocation = &v
		}
		if idx, ok := colIdx["logo_url"]; ok && row[idx] != "" {
			v := row[idx]
			input.LogoURL = &v
		}
		if idx, ok := colIdx["overview"]; ok && row[idx] != "" {
			v := row[idx]
			input.Overview = &v
		}
		if idx, ok := colIdx["website"]; ok && row[idx] != "" {
			v := row[idx]
			input.Website = &v
		}

		if _, err := app.models.Employers.Insert(ctx, tx, input); err != nil {
			return 0, fmt.Errorf("row %d (%s): %w", i+2, input.Slug, err)
		}
	}

	return len(rows), nil
}

// importOpportunities processes an opportunity CSV.
// Expected columns: employer_slug, title, slug, type, intake_year, description, location,
// application_url, requirements, discipline_tags (pipe-separated), opens_at, deadline, source_url
func (app *application) importOpportunities(ctx context.Context, tx pgx.Tx, header []string, rows [][]string) (int, error) {
	colIdx := indexColumns(header)

	required := []string{"employer_slug", "title", "slug", "type", "intake_year", "description", "location", "application_url"}
	for _, c := range required {
		if _, ok := colIdx[c]; !ok {
			return 0, fmt.Errorf("missing required column: %s", c)
		}
	}

	for i, row := range rows {
		employerSlug := row[colIdx["employer_slug"]]
		employer, err := app.models.Employers.GetBySlug(ctx, tx, employerSlug)
		if err != nil {
			return 0, fmt.Errorf("row %d: employer '%s' not found", i+2, employerSlug)
		}

		intakeYear, err := strconv.Atoi(row[colIdx["intake_year"]])
		if err != nil {
			return 0, fmt.Errorf("row %d: invalid intake_year: %w", i+2, err)
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
		}

		if idx, ok := colIdx["requirements"]; ok && row[idx] != "" {
			v := row[idx]
			input.Requirements = &v
		}
		if idx, ok := colIdx["discipline_tags"]; ok && row[idx] != "" {
			input.DisciplineTags = splitAndTrim(row[idx], "|")
		}
		if idx, ok := colIdx["opens_at"]; ok && row[idx] != "" {
			t, err := time.Parse("2006-01-02", row[idx])
			if err != nil {
				return 0, fmt.Errorf("row %d: invalid opens_at: %w", i+2, err)
			}
			input.OpensAt = &t
		}
		if idx, ok := colIdx["deadline"]; ok && row[idx] != "" {
			t, err := time.Parse("2006-01-02", row[idx])
			if err != nil {
				return 0, fmt.Errorf("row %d: invalid deadline: %w", i+2, err)
			}
			input.Deadline = &t
		}
		if idx, ok := colIdx["source_url"]; ok && row[idx] != "" {
			v := row[idx]
			input.SourceURL = &v
		}

		if _, err := app.models.Opportunities.Insert(ctx, tx, input); err != nil {
			return 0, fmt.Errorf("row %d (%s): %w", i+2, input.Slug, err)
		}
	}

	return len(rows), nil
}

// importAssessments — skeleton only. Stages field is complex JSON so CSV isn't a great fit.
// Implement when you actually need it, or consider accepting JSON for this one.
func (app *application) importAssessments(ctx context.Context, tx pgx.Tx, header []string, rows [][]string) (int, error) {
	return 0, errors.New("assessment import not yet implemented — use the API directly")
}

// --- helpers ---

func indexColumns(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, name := range header {
		idx[name] = i
	}
	return idx
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
