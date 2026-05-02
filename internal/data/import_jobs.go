package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImportJobRowError struct {
	RowNumber int    `json:"row_number"`
	Message   string `json:"message"`
	RawData   string `json:"raw_data,omitempty"`
}

type ImportJob struct {
	ID           string              `json:"id"`
	UserID       string              `json:"-"`
	ImportType   string              `json:"import_type"`
	FilePath     string              `json:"-"`
	Status       string              `json:"status"`
	RowsTotal    *int                `json:"rows_total"`
	RowsImported *int                `json:"rows_imported"`
	RowsFailed   *int                `json:"rows_failed"`
	RowErrors    []ImportJobRowError `json:"row_errors,omitempty"`
	ErrorMessage *string             `json:"error_message"`
	CreatedAt    time.Time           `json:"created_at"`
	CompletedAt  *time.Time          `json:"completed_at"`
}

var permittedImportTypes = []string{"employers", "opportunities", "assessments"}

func IsPermittedImportType(t string) bool {
	for _, p := range permittedImportTypes {
		if p == t {
			return true
		}
	}
	return false
}

type ImportJobModel struct {
	DB *pgxpool.Pool
}

func (m ImportJobModel) Insert(ctx context.Context, db DBTX, userID, importType, filePath string) (*ImportJob, error) {
	query := `
		INSERT INTO import_job (user_id, import_type, file_path)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, import_type, file_path, status, created_at`

	job := &ImportJob{}
	err := db.QueryRow(ctx, query, userID, importType, filePath).Scan(
		&job.ID,
		&job.UserID,
		&job.ImportType,
		&job.FilePath,
		&job.Status,
		&job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (m ImportJobModel) GetByID(ctx context.Context, db DBTX, id string) (*ImportJob, error) {
	query := `
        SELECT id, user_id, import_type, file_path, status,
               rows_total, rows_imported, error_message, row_errors, created_at, completed_at
        FROM import_job
        WHERE id = $1`

	job := &ImportJob{}
	var rowErrorsJSON []byte
	err := db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.ImportType,
		&job.FilePath,
		&job.Status,
		&job.RowsTotal,
		&job.RowsImported,
		&job.ErrorMessage,
		&rowErrorsJSON,
		&job.CreatedAt,
		&job.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	if len(rowErrorsJSON) > 0 {
		if err := json.Unmarshal(rowErrorsJSON, &job.RowErrors); err != nil {
			return nil, fmt.Errorf("unmarshaling row_errors: %w", err)
		}
	}

	if job.RowsTotal != nil && job.RowsImported != nil {
		failed := *job.RowsTotal - *job.RowsImported
		job.RowsFailed = &failed
	}

	return job, nil
}

func (m ImportJobModel) MarkProcessing(ctx context.Context, db DBTX, id string) error {
	_, err := db.Exec(ctx, `UPDATE import_job SET status = 'processing' WHERE id = $1`, id)
	return err
}

func (m ImportJobModel) MarkCompleted(
	ctx context.Context,
	db DBTX,
	id string,
	rowsTotal, rowsImported int,
	rowErrors []ImportJobRowError,
) error {
	errorsJSON, err := json.Marshal(rowErrors)
	if err != nil {
		return fmt.Errorf("marshaling row_errors: %w", err)
	}

	query := `
        UPDATE import_job
        SET status = 'completed',
            rows_total = $1,
            rows_imported = $2,
            row_errors = $3,
            completed_at = now()
        WHERE id = $4`
	_, err = db.Exec(ctx, query, rowsTotal, rowsImported, errorsJSON, id)
	return err
}

func (m ImportJobModel) MarkFailed(ctx context.Context, db DBTX, id string, rowsTotal int, errMsg string) error {
	query := `
		UPDATE import_job
		SET status = 'failed', rows_total = $1, error_message = $2, completed_at = now()
		WHERE id = $3`
	_, err := db.Exec(ctx, query, rowsTotal, errMsg, id)
	return err
}

func (m ImportJobModel) GetRecent(ctx context.Context, db DBTX, limit int) ([]*ImportJob, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	query := `
        SELECT id, user_id, import_type, file_path, status,
               rows_total, rows_imported, error_message, row_errors, created_at, completed_at
        FROM import_job
        ORDER BY created_at DESC
        LIMIT $1`

	rows, err := db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []*ImportJob{}
	for rows.Next() {
		job := &ImportJob{}
		var rowErrorsJSON []byte
		err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.ImportType,
			&job.FilePath,
			&job.Status,
			&job.RowsTotal,
			&job.RowsImported,
			&job.ErrorMessage,
			&rowErrorsJSON,
			&job.CreatedAt,
			&job.CompletedAt,
		)
		if err != nil {
			return nil, err
		}

		if len(rowErrorsJSON) > 0 {
			if err := json.Unmarshal(rowErrorsJSON, &job.RowErrors); err != nil {
				return nil, fmt.Errorf("unmarshaling row_errors for job %s: %w", job.ID, err)
			}
		}

		if job.RowsTotal != nil && job.RowsImported != nil {
			failed := *job.RowsTotal - *job.RowsImported
			job.RowsFailed = &failed
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}
