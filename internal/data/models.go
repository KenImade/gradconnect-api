package data

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Assessments   AssessmentModel
	Employers     EmployerModel
	Opportunities OpportunityModel
	Permissions   PermissionModel
	Reviews       ReviewModel
	Sessions      SessionModel
	Tasks         TaskModel
	Tokens        TokenModel
	Users         UserModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Assessments:   AssessmentModel{DB: db},
		Employers:     EmployerModel{DB: db},
		Opportunities: OpportunityModel{DB: db},
		Permissions:   PermissionModel{DB: db},
		Reviews:       ReviewModel{DB: db},
		Sessions:      SessionModel{DB: db},
		Tasks:         TaskModel{DB: db},
		Tokens:        TokenModel{DB: db},
		Users:         UserModel{DB: db},
	}
}
