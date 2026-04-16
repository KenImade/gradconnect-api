package data

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Permissions []string

func (p Permissions) Include(code string) bool {
	return slices.Contains(p, code)
}

type PermissionModel struct {
	DB *pgxpool.Pool
}

func (m PermissionModel) GetAllForUser(userID string) (Permissions, error) {
	query := `
        SELECT DISTINCT permission
        FROM user_permission
        WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return permissions, nil
}

func (m PermissionModel) AddForUser(ctx context.Context, tx pgx.Tx, userID string, codes ...string) error {
	query := `
        INSERT INTO user_permission (user_id, permission)
        SELECT $1, unnest($2::text[])
        ON CONFLICT DO NOTHING` // Prevents errors if double-assigned

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := tx.Exec(ctx, query, userID, codes)
	return err
}

func (m PermissionModel) RemoveForUser(userID string, codes ...string) error {
	query := `
        DELETE FROM user_permission
        WHERE user_id = $1 AND permission = ANY($2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, userID, codes)
	return err
}
