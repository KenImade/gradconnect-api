package data

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type DBTX interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type Models struct {
	Analytics          AnalyticsModel
	ApplicationTracker ApplicationTrackerModel
	Assessments        AssessmentModel
	Bookmarks          BookmarkModel
	Employers          EmployerModel
	ImportJob          ImportJobModel
	Opportunities      OpportunityModel
	Permissions        PermissionModel
	Reviews            ReviewModel
	Sessions           SessionModel
	Tasks              TaskModel
	Tokens             TokenModel
	Users              UserModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Analytics:          AnalyticsModel{DB: db},
		ApplicationTracker: ApplicationTrackerModel{DB: db},
		Assessments:        AssessmentModel{DB: db},
		Bookmarks:          BookmarkModel{DB: db},
		Employers:          EmployerModel{DB: db},
		ImportJob:          ImportJobModel{DB: db},
		Opportunities:      OpportunityModel{DB: db},
		Permissions:        PermissionModel{DB: db},
		Reviews:            ReviewModel{DB: db},
		Sessions:           SessionModel{DB: db},
		Tasks:              TaskModel{DB: db},
		Tokens:             TokenModel{DB: db},
		Users:              UserModel{DB: db},
	}
}
