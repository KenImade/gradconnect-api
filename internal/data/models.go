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
	Employers     EmployerModel
	Assessments   AssessmentModel
	Reviews       ReviewModel
	Opportunities OpportunityModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Employers:     EmployerModel{DB: db},
		Assessments:   AssessmentModel{DB: db},
		Reviews:       ReviewModel{DB: db},
		Opportunities: OpportunityModel{DB: db},
	}
}
