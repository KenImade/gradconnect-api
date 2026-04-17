package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID        string
	UserID    string
	IPAddress string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionModel struct {
	DB *pgxpool.Pool
}

func (m SessionModel) Create(ctx context.Context, db DBTX, userID, ipAddress, userAgent string) (*Session, error) {
	query := `
        INSERT INTO session (user_id, ip_address, user_agent)
        VALUES ($1, $2, $3)
        RETURNING id, user_id, ip_address, user_agent, created_at, expires_at`

	session := &Session{}
	err := db.QueryRow(ctx, query, userID, ipAddress, userAgent).Scan(
		&session.ID,
		&session.UserID,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (m SessionModel) GetByID(ctx context.Context, db DBTX, id string) (*Session, error) {
	query := `
        SELECT id, user_id, ip_address, user_agent, created_at, expires_at
        FROM session
        WHERE id = $1 AND expires_at > now()`

	session := &Session{}
	err := db.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return session, nil
}

func (m SessionModel) Delete(ctx context.Context, db DBTX, id string) error {
	_, err := db.Exec(ctx, "DELETE FROM session WHERE id = $1", id)
	return err
}
