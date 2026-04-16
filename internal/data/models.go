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
	Reviews       ReviewModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Assessments:   AssessmentModel{DB: db},
		Employers:     EmployerModel{DB: db},
		Opportunities: OpportunityModel{DB: db},
		Reviews:       ReviewModel{DB: db},
	}
}
