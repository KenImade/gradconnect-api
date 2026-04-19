package data

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TaskModel struct {
	DB *pgxpool.Pool
}

func (m TaskModel) Insert(ctx context.Context, db DBTX, jobType string, payload any) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	query := `
        INSERT INTO task_queue (job_type, payload, status, run_at)
        VALUES ($1, $2::jsonb, 'pending', $3)
    `

	_, err = db.Exec(ctx, query, jobType, jsonPayload, time.Now())
	return err
}
