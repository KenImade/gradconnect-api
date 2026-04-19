package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// --- Domain types ---

type Assessment struct {
	ID                   string          `json:"id"`
	EmployerID           string          `json:"employer_id"`
	ProgrammeType        string          `json:"programme_type"`
	Stages               json.RawMessage `json:"stages"`
	AptitudeTestProvider *string         `json:"aptitude_test_provider"`
	InterviewFormat      *string         `json:"interview_format"`
	TimelineWeeks        *int            `json:"timeline_weeks"`
	PrepGuide            *string         `json:"prep_guide"`
	Version              int             `json:"version"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}

// --- Inputs ---

type CreateAssessmentInput struct {
	EmployerID           string          `json:"employer_id"`
	ProgrammeType        string          `json:"programme_type"`
	Stages               json.RawMessage `json:"stages"`
	AptitudeTestProvider *string         `json:"aptitude_test_provider"`
	InterviewFormat      *string         `json:"interview_format"`
	TimelineWeeks        *int            `json:"timeline_weeks"`
	PrepGuide            *string         `json:"prep_guide"`
}

type UpdateAssessmentInput struct {
	ProgrammeType        *string         `json:"programme_type"`
	Stages               json.RawMessage `json:"stages"`
	AptitudeTestProvider *string         `json:"aptitude_test_provider"`
	InterviewFormat      *string         `json:"interview_format"`
	TimelineWeeks        *int            `json:"timeline_weeks"`
	PrepGuide            *string         `json:"prep_guide"`
	Version              int             `json:"version"`
}

// --- Validators ---

func validateStages(v *validator.Validator, raw json.RawMessage) {
	if len(raw) == 0 {
		v.AddError("stages", "must be provided")
		return
	}

	var stages []map[string]any
	if err := json.Unmarshal(raw, &stages); err != nil {
		v.AddError("stages", "must be a valid JSON array")
		return
	}

	v.Check(len(stages) >= 1, "stages", "must contain at least one stage")
	v.Check(len(stages) <= 15, "stages", "must not contain more than 15 stages")

	for i, stage := range stages {
		name, ok := stage["stage_name"].(string)
		if !ok || name == "" {
			v.AddError(fmt.Sprintf("stages[%d].stage_name", i), "must be provided")
		}
		stageType, ok := stage["stage_type"].(string)
		if !ok || stageType == "" {
			v.AddError(fmt.Sprintf("stages[%d].stage_type", i), "must be provided")
		}
	}
}

func ValidateCreateAssessmentInput(v *validator.Validator, input CreateAssessmentInput) {
	v.Check(validator.IsValidUUID(input.EmployerID), "employer_id", "must be a valid UUID")
	v.Check(input.ProgrammeType != "", "programme_type", "must be provided")
	v.Check(len(input.ProgrammeType) <= 100, "programme_type", "must not be more than 100 characters")

	validateStages(v, input.Stages)

	if input.TimelineWeeks != nil {
		v.Check(*input.TimelineWeeks > 0, "timeline_weeks", "must be a positive number")
		v.Check(*input.TimelineWeeks <= 52, "timeline_weeks", "must not be more than 52")
	}
	if input.AptitudeTestProvider != nil {
		v.Check(len(*input.AptitudeTestProvider) <= 100, "aptitude_test_provider", "must not be more than 100 characters")
	}
	if input.InterviewFormat != nil {
		v.Check(len(*input.InterviewFormat) <= 255, "interview_format", "must not be more than 255 characters")
	}
	if input.PrepGuide != nil {
		v.Check(len(*input.PrepGuide) <= 10000, "prep_guide", "must not be more than 10000 characters")
	}
}

func ValidateUpdateAssessmentInput(v *validator.Validator, input UpdateAssessmentInput) {
	v.Check(input.Version > 0, "version", "must be provided")

	if input.ProgrammeType != nil {
		v.Check(*input.ProgrammeType != "", "programme_type", "must not be empty")
		v.Check(len(*input.ProgrammeType) <= 100, "programme_type", "must not be more than 100 characters")
	}
	if len(input.Stages) > 0 {
		validateStages(v, input.Stages)
	}
	if input.TimelineWeeks != nil {
		v.Check(*input.TimelineWeeks > 0, "timeline_weeks", "must be a positive number")
		v.Check(*input.TimelineWeeks <= 52, "timeline_weeks", "must not be more than 52")
	}
	if input.AptitudeTestProvider != nil {
		v.Check(len(*input.AptitudeTestProvider) <= 100, "aptitude_test_provider", "must not be more than 100 characters")
	}
	if input.InterviewFormat != nil {
		v.Check(len(*input.InterviewFormat) <= 255, "interview_format", "must not be more than 255 characters")
	}
	if input.PrepGuide != nil {
		v.Check(len(*input.PrepGuide) <= 10000, "prep_guide", "must not be more than 10000 characters")
	}
}

// --- Model ---

type AssessmentModel struct {
	DB *pgxpool.Pool
}

func (m AssessmentModel) Insert(ctx context.Context, db DBTX, input CreateAssessmentInput) (*Assessment, error) {
	query := `
		INSERT INTO assessment_profile
			(employer_id, programme_type, stages, aptitude_test_provider,
			 interview_format, timeline_weeks, prep_guide)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, employer_id, programme_type, stages, aptitude_test_provider,
		          interview_format, timeline_weeks, prep_guide,
		          version, created_at, updated_at`

	assessment := &Assessment{}
	err := db.QueryRow(ctx, query,
		input.EmployerID,
		input.ProgrammeType,
		input.Stages,
		input.AptitudeTestProvider,
		input.InterviewFormat,
		input.TimelineWeeks,
		input.PrepGuide,
	).Scan(
		&assessment.ID,
		&assessment.EmployerID,
		&assessment.ProgrammeType,
		&assessment.Stages,
		&assessment.AptitudeTestProvider,
		&assessment.InterviewFormat,
		&assessment.TimelineWeeks,
		&assessment.PrepGuide,
		&assessment.Version,
		&assessment.CreatedAt,
		&assessment.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrRecordNotFound // employer doesn't exist
		}
		return nil, err
	}

	return assessment, nil
}

func (m AssessmentModel) GetByID(ctx context.Context, db DBTX, id string) (*Assessment, error) {
	if !validator.IsValidUUID(id) {
		return nil, ErrRecordNotFound
	}

	query := `
		SELECT id, employer_id, programme_type, stages, aptitude_test_provider,
		       interview_format, timeline_weeks, prep_guide,
		       version, created_at, updated_at
		FROM assessment_profile
		WHERE id = $1`

	var assessment Assessment
	err := db.QueryRow(ctx, query, id).Scan(
		&assessment.ID,
		&assessment.EmployerID,
		&assessment.ProgrammeType,
		&assessment.Stages,
		&assessment.AptitudeTestProvider,
		&assessment.InterviewFormat,
		&assessment.TimelineWeeks,
		&assessment.PrepGuide,
		&assessment.Version,
		&assessment.CreatedAt,
		&assessment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &assessment, nil
}

func (m AssessmentModel) Update(ctx context.Context, db DBTX, id string, input UpdateAssessmentInput) (*Assessment, error) {
	current, err := m.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}

	if current.Version != input.Version {
		return nil, ErrEditConflict
	}

	if input.ProgrammeType != nil {
		current.ProgrammeType = *input.ProgrammeType
	}
	if len(input.Stages) > 0 {
		current.Stages = input.Stages
	}
	if input.AptitudeTestProvider != nil {
		current.AptitudeTestProvider = input.AptitudeTestProvider
	}
	if input.InterviewFormat != nil {
		current.InterviewFormat = input.InterviewFormat
	}
	if input.TimelineWeeks != nil {
		current.TimelineWeeks = input.TimelineWeeks
	}
	if input.PrepGuide != nil {
		current.PrepGuide = input.PrepGuide
	}

	query := `
		UPDATE assessment_profile
		SET programme_type = $1, stages = $2, aptitude_test_provider = $3,
		    interview_format = $4, timeline_weeks = $5, prep_guide = $6,
		    version = version + 1
		WHERE id = $7 AND version = $8
		RETURNING version, updated_at`

	err = db.QueryRow(ctx, query,
		current.ProgrammeType,
		current.Stages,
		current.AptitudeTestProvider,
		current.InterviewFormat,
		current.TimelineWeeks,
		current.PrepGuide,
		id,
		input.Version,
	).Scan(&current.Version, &current.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrEditConflict
		}
		return nil, err
	}

	return current, nil
}

func (m AssessmentModel) Delete(ctx context.Context, db DBTX, id string) error {
	result, err := db.Exec(ctx, `DELETE FROM assessment_profile WHERE id = $1`, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m AssessmentModel) GetAllByEmployerSlug(ctx context.Context, db DBTX, slug string, filters Filters) ([]*Assessment, Metadata, error) {
	query := fmt.Sprintf(`
		SELECT count(*) OVER(),
		       a.id, a.employer_id, a.programme_type, a.stages,
		       a.aptitude_test_provider, a.interview_format,
		       a.timeline_weeks, a.prep_guide,
		       a.version, a.created_at, a.updated_at
		FROM assessment_profile a
		INNER JOIN employer e ON e.id = a.employer_id
		WHERE e.slug = $1
		ORDER BY a.%s %s, a.id ASC
		LIMIT $2 OFFSET $3`,
		filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	args := []any{slug, filters.limit(), filters.offset()}

	rows, err := db.Query(ctx, query, args...)
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
			&assessment.CreatedAt,
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
