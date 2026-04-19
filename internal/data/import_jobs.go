package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ImportJob struct {
	ID           string     `json:"id"`
	UserID       string     `json:"-"`
	ImportType   string     `json:"import_type"`
	FilePath     string     `json:"-"`
	Status       string     `json:"status"`
	RowsTotal    *int       `json:"rows_total"`
	RowsImported *int       `json:"rows_imported"`
	ErrorMessage *string    `json:"error_message"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"`
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
		       rows_total, rows_imported, error_message, created_at, completed_at
		FROM import_job
		WHERE id = $1`

	job := &ImportJob{}
	err := db.QueryRow(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.ImportType,
		&job.FilePath,
		&job.Status,
		&job.RowsTotal,
		&job.RowsImported,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return job, nil
}

func (m ImportJobModel) MarkProcessing(ctx context.Context, db DBTX, id string) error {
	_, err := db.Exec(ctx, `UPDATE import_job SET status = 'processing' WHERE id = $1`, id)
	return err
}

func (m ImportJobModel) MarkCompleted(ctx context.Context, db DBTX, id string, rowsTotal, rowsImported int) error {
	query := `
		UPDATE import_job
		SET status = 'completed', rows_total = $1, rows_imported = $2, completed_at = now()
		WHERE id = $3`
	_, err := db.Exec(ctx, query, rowsTotal, rowsImported, id)
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
