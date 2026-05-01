package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool runs background tasks from the task_queue table. It polls for
// pending jobs, dispatches them via the JobHandler, and manages retries
// with exponential backoff.
type Pool struct {
	db       *pgxpool.Pool
	logger   *slog.Logger
	handler  JobHandler
	pollWait time.Duration
}

// New constructs a worker Pool. Call Run in a goroutine to start polling.
func New(db *pgxpool.Pool, logger *slog.Logger, handler JobHandler) *Pool {
	return &Pool{
		db:       db,
		logger:   logger,
		handler:  handler,
		pollWait: 5 * time.Second,
	}
}

// Run polls the task queue and processes jobs until ctx is cancelled.
// Designed to be called in a goroutine; blocks for the pool's lifetime.
func (p *Pool) Run(ctx context.Context) {
	p.logger.Info("worker pool started")
	defer p.logger.Info("worker pool stopped")

	for {
		// Honor cancellation between iterations.
		select {
		case <-ctx.Done():
			return
		default:
		}

		taskID, jobType, payload, err := p.claimNext(ctx)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// No work — sleep with cancellation awareness.
				if !p.sleep(ctx, p.pollWait) {
					return
				}
				continue
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			p.logger.Error("worker poll error", "err", err)
			if !p.sleep(ctx, p.pollWait) {
				return
			}
			continue
		}

		// Dispatch with a per-job context that's still cancellable on shutdown.
		if err := p.handler.Handle(ctx, jobType, payload); err != nil {
			p.markFailed(ctx, taskID, err)
			continue
		}
		p.markCompleted(ctx, taskID)
	}
}

// claimNext atomically picks the next pending task and marks it processing.
// FOR UPDATE SKIP LOCKED makes this safe across multiple worker processes.
func (p *Pool) claimNext(ctx context.Context) (id, jobType string, payload []byte, err error) {
	const query = `
        UPDATE task_queue
        SET status = 'processing', locked_at = now()
        WHERE id = (
            SELECT id FROM task_queue
            WHERE status = 'pending' AND run_at <= now()
            ORDER BY run_at ASC
            LIMIT 1
            FOR UPDATE SKIP LOCKED
        )
        RETURNING id, job_type, payload`

	err = p.db.QueryRow(ctx, query).Scan(&id, &jobType, &payload)
	return
}

// markCompleted records a successful task. Errors here are logged but
// not returned — the task already succeeded; bookkeeping shouldn't fail
// the work that's already done.
func (p *Pool) markCompleted(ctx context.Context, taskID string) {
	_, err := p.db.Exec(ctx, `
        UPDATE task_queue
        SET status = 'completed', completed_at = now()
        WHERE id = $1
    `, taskID)
	if err != nil {
		p.logger.Error("marking task completed", "err", err, "task_id", taskID)
	}
}

// markFailed increments attempts, schedules an exponential backoff retry,
// or marks the task dead if max_attempts is exhausted.
func (p *Pool) markFailed(ctx context.Context, taskID string, taskErr error) {
	p.logger.Error("task failed", "task_id", taskID, "err", taskErr)

	const query = `
        UPDATE task_queue
        SET
            status = CASE
                WHEN attempts + 1 >= max_attempts THEN 'dead'::task_status_type
                ELSE 'pending'::task_status_type
            END,
            attempts = attempts + 1,
            last_error = $1,
            run_at = CASE
                WHEN attempts + 1 >= max_attempts THEN run_at
                ELSE now() + (pow(2, attempts + 1) * interval '1 minute')
            END
        WHERE id = $2`

	if _, err := p.db.Exec(ctx, query, taskErr.Error(), taskID); err != nil {
		p.logger.Error("marking task failed", "err", err, "task_id", taskID)
	}
}

// sleep waits for d or until ctx cancels. Returns false if ctx cancelled.
func (p *Pool) sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// RunDeadlineRemindersNow runs the reminder enqueue immediately,
// bypassing the time-of-day check. Daily idempotency via cron_run
// table is still enforced.
func (p *Pool) RunDeadlineRemindersNow(ctx context.Context, baseURL, frontendURL string) (int, error) {
	lagos, err := time.LoadLocation("Africa/Lagos")
	if err != nil {
		return 0, fmt.Errorf("loading timezone: %w", err)
	}
	today := time.Now().In(lagos).Format("2006-01-02")

	var runID string
	err = p.db.QueryRow(ctx, `
        INSERT INTO cron_run (job_name, run_date)
        VALUES ($1, $2::date)
        ON CONFLICT (job_name, run_date) DO NOTHING
        RETURNING id
    `, cronJobName, today).Scan(&runID)

	if err == pgx.ErrNoRows {
		return 0, ErrAlreadyRanToday
	}
	if err != nil {
		return 0, fmt.Errorf("claiming cron run: %w", err)
	}

	enqueued := p.enqueueDeadlineReminders(ctx, baseURL, frontendURL)

	_, err = p.db.Exec(ctx, `
        UPDATE cron_run
        SET completed_at = now(), enqueued_count = $1
        WHERE id = $2
    `, enqueued, runID)
	if err != nil {
		p.logger.Error("marking cron run complete", "err", err)
	}

	return enqueued, nil
}
