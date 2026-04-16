package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Assessment struct {
	ID                   string          `json:"id"`
	EmployerID           string          `json:"employer_id"`
	ProgrammeType        string          `json:"programme_type"`
	Stages               json.RawMessage `json:"stages"`
	AptitudeTestProvider *string         `json:"aptitude_test_provider"`
	InterviewFormat      *string         `json:"interview_format"`
	TimelineWeeks        *Timeline       `json:"timeline_weeks"`
	PrepGuide            *string         `json:"prep_guide"`
	Version              int             `json:"version"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

func ValidateAssessment(v *validator.Validator, assessment *Assessment) {
	v.Check(validator.IsValidUUID(assessment.EmployerID), "employer_id", "must be a valid UUID")
	v.Check(assessment.ProgrammeType != "", "programme_type", "must be provided")
	v.Check(len(assessment.ProgrammeType) <= 100, "programme_type", "must not be more than 100 characters")
	v.Check(len(assessment.Stages) > 0, "stages", "must be provided")
	if assessment.TimelineWeeks != nil {
		v.Check(*assessment.TimelineWeeks > 0, "timeline_weeks", "must be a positive number")
	}
}

type AssessmentModel struct {
	DB *pgxpool.Pool
}

func (m AssessmentModel) Insert(assessment *Assessment) error { return nil }

func (m AssessmentModel) Get(id string) (*Assessment, error) {
	if !validator.IsValidUUID(id) {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, employer_id, programme_type, stages, aptitude_test_provider,
				interview_format, timeline_weeks, prep_guide, version, updated_at
		FROM assessment_profile
		WHERE id = $1
	`

	var assessment Assessment

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, id).Scan(
		&assessment.ID,
		&assessment.EmployerID,
		&assessment.ProgrammeType,
		&assessment.Stages,
		&assessment.AptitudeTestProvider,
		&assessment.InterviewFormat,
		&assessment.TimelineWeeks,
		&assessment.PrepGuide,
		&assessment.Version,
		&assessment.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &assessment, nil
}

func (m AssessmentModel) Update(assessment *Assessment) error { return nil }

func (m AssessmentModel) Delete(id string) error { return nil }

func (m AssessmentModel) GetAllByEmployerSlug(slug string, filters Filters) ([]*Assessment, Metadata, error) {
	query := fmt.Sprintf(`
        SELECT count(*) OVER(),
            a.id, a.employer_id, a.programme_type, a.stages,
            a.aptitude_test_provider, a.interview_format,
            a.timeline_weeks, a.prep_guide, a.version, a.updated_at
        FROM assessment_profile a
        INNER JOIN employer e ON e.id = a.employer_id
        WHERE e.slug = $1
        ORDER BY %s %s
        LIMIT $2 OFFSET $3`,
		filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{slug, filters.limit(), filters.offset()}

	rows, err := m.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	assessments := []*Assessment{}

	for rows.Next() {
		var assessment Assessment

		err := rows.Scan(
			&totalRecords,
			&assessment.ID,
			&assessment.EmployerID,
			&assessment.ProgrammeType,
			&assessment.Stages,
			&assessment.AptitudeTestProvider,
			&assessment.InterviewFormat,
			&assessment.TimelineWeeks,
			&assessment.PrepGuide,
			&assessment.Version,
			&assessment.UpdatedAt,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		assessments = append(assessments, &assessment)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return assessments, metadata, nil
}
