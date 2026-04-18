package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"api.gradconnect.com/internal/validator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var AnonymousUser = &User{}

// DB Model
type User struct {
	ID                 string          `json:"id"`
	Email              string          `json:"email"`
	Password           Password        `json:"-"`
	FirstName          string          `json:"first_name"`
	LastName           string          `json:"last_name"`
	AuthProvider       string          `json:"auth_provider"`
	EmailVerified      bool            `json:"email_verified"`
	DegreeDiscipline   *string         `json:"degree_discipline"`
	GraduationYear     *int            `json:"graduation_year"`
	TargetIndustries   []string        `json:"target_industries"`
	PreferredLocations []string        `json:"preferred_locations"`
	Preferences        json.RawMessage `json:"preferences"`
	Version            int             `json:"version"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type Password struct {
	plaintext *string
	hash      []byte
}

// Inputs

type CreateUserInput struct {
	FirstName          string          `json:"first_name" example:"John"`
	LastName           string          `json:"last_name" example:"Doe"`
	Email              string          `json:"email" example:"john@example.com"`
	Password           string          `json:"password" example:"pa55word"`
	DegreeDiscipline   *string         `json:"degree_discipline" example:"Computer Science"`
	GraduationYear     *int            `json:"graduation_year" example:"2025"`
	TargetIndustries   []string        `json:"target_industries" example:"[\"Finance\", \"Tech\"]"`
	PreferredLocations []string        `json:"preferred_locations" example:"[\"Lagos\", \"Abuja\"]"`
	Preferences        json.RawMessage `json:"preferences" swaggertype:"object"`
}

type LoginUserInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UpdateUserInput struct {
	FirstName          *string         `json:"first_name"`
	LastName           *string         `json:"last_name"`
	DegreeDiscipline   *string         `json:"degree_discipline"`
	GraduationYear     *int            `json:"graduation_year"`
	TargetIndustries   *[]string       `json:"target_industries"`
	PreferredLocations *[]string       `json:"preferred_locations"`
	Preferences        json.RawMessage `json:"preferences"`
}

type GoogleAuthInput struct {
	Code string `json:"code"`
}

type ForgotPasswordInput struct {
	Email string `json:"email"`
}

type ResetPasswordInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// Responses
type CurrentUserResponse struct {
	ID                 string          `json:"id"`
	Email              string          `json:"email"`
	Name               string          `json:"name"`
	AuthProvider       string          `json:"auth_provider"`
	EmailVerified      bool            `json:"email_verified"`
	DegreeDiscipline   *string         `json:"degree_discipline"`
	GraduationYear     *int            `json:"graduation_year"`
	TargetIndustries   []string        `json:"target_industries"`
	PreferredLocations []string        `json:"preferred_locations"`
	Preferences        json.RawMessage `json:"preferences"`
	Version            int             `json:"version"`
	Permissions        []string        `json:"permissions"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

func (p *Password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *Password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

func ValidateCreateUserInput(v *validator.Validator, input CreateUserInput) {
	v.Check(input.FirstName != "", "first_name", "must be provided")
	v.Check(input.LastName != "", "last_name", "must be provided")
	v.Check(len(input.FirstName) <= 255, "first_name", "must not be more than 255 characters")
	v.Check(len(input.LastName) <= 255, "last_name", "must not be more than 255 characters")

	ValidateEmail(v, input.Email)
	ValidatePasswordPlaintext(v, input.Password)

	if input.GraduationYear != nil {
		currentYear := time.Now().Year()
		v.Check(*input.GraduationYear >= 1990, "graduation_year", "must be after 1990")
		v.Check(*input.GraduationYear <= currentYear+6, "graduation_year", "must not be more than 6 years in the future")
	}

	if input.DegreeDiscipline != nil {
		v.Check(len(*input.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}

	v.Check(len(input.TargetIndustries) <= 20, "target_industries", "must not have more than 20 industries")
	v.Check(len(input.PreferredLocations) <= 20, "preferred_locations", "must not have more than 20 locations")
}

func ValidateUpdateUserInput(v *validator.Validator, input UpdateUserInput) {
	if input.FirstName != nil {
		v.Check(*input.FirstName != "", "first_name", "must not be empty")
		v.Check(len(*input.FirstName) <= 255, "first_name", "must not be more than 255 characters")
	}
	if input.LastName != nil {
		v.Check(*input.LastName != "", "last_name", "must not be empty")
		v.Check(len(*input.LastName) <= 255, "last_name", "must not be more than 255 characters")
	}
	if input.GraduationYear != nil {
		currentYear := time.Now().Year()
		v.Check(*input.GraduationYear >= 1990, "graduation_year", "must be after 1990")
		v.Check(*input.GraduationYear <= currentYear+6, "graduation_year", "must not be more than 6 years in the future")
	}
	if input.DegreeDiscipline != nil {
		v.Check(len(*input.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}
	if input.TargetIndustries != nil {
		v.Check(len(*input.TargetIndustries) <= 20, "target_industries", "must not have more than 20 industries")
	}
	if input.PreferredLocations != nil {
		v.Check(len(*input.PreferredLocations) <= 20, "preferred_locations", "must not have more than 20 locations")
	}
}

func ValidateLoginUserInput(v *validator.Validator, user LoginUserInput) {
	ValidateEmail(v, user.Email)
	ValidatePasswordPlaintext(v, user.Password)
}

var (
	ErrDuplicateEmail = errors.New("duplicate email")
)

type UserModel struct {
	DB *pgxpool.Pool
}

func (m UserModel) Insert(ctx context.Context, tx pgx.Tx, user *User) error {
	query := `
        INSERT INTO app_user (
            email, password_hash, first_name, last_name, auth_provider, email_verified,
            degree_discipline, graduation_year, target_industries, preferred_locations,
            preferences
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        RETURNING id, created_at, version`

	args := []any{
		user.Email,
		user.Password.hash,
		user.FirstName,
		user.LastName,
		user.AuthProvider,
		user.EmailVerified,
		user.DegreeDiscipline,
		user.GraduationYear,
		user.TargetIndustries,
		user.PreferredLocations,
		user.Preferences,
	}

	err := tx.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Version,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrDuplicateEmail
		}
		return err
	}

	return nil
}

func (m UserModel) GetByEmail(ctx context.Context, db DBTX, email string) (*User, error) {
	query := `
        SELECT id, email, password_hash, first_name, last_name, auth_provider, email_verified,
               degree_discipline, graduation_year, target_industries, preferred_locations,
               preferences, version, created_at, updated_at
        FROM app_user
        WHERE email = $1`

	user := &User{}
	err := db.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&user.FirstName,
		&user.LastName,
		&user.AuthProvider,
		&user.EmailVerified,
		&user.DegreeDiscipline,
		&user.GraduationYear,
		&user.TargetIndustries,
		&user.PreferredLocations,
		&user.Preferences,
		&user.Version,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return user, nil
}

func (m UserModel) GetByID(ctx context.Context, db DBTX, id string) (*User, error) {
	query := `
        SELECT id, email, password_hash, first_name, last_name, auth_provider, email_verified,
               degree_discipline, graduation_year, target_industries, preferred_locations,
               preferences, version, created_at, updated_at
        FROM app_user
        WHERE id = $1`

	user := &User{}
	err := db.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.Password.hash,
		&user.FirstName,
		&user.LastName,
		&user.AuthProvider,
		&user.EmailVerified,
		&user.DegreeDiscipline,
		&user.GraduationYear,
		&user.TargetIndustries,
		&user.PreferredLocations,
		&user.Preferences,
		&user.Version,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return user, nil
}

func (m UserModel) Update(ctx context.Context, db DBTX, user *User) error {
	query := `
        UPDATE app_user
        SET first_name = $1, last_name = $2, email_verified = $3,
            degree_discipline = $4, graduation_year = $5,
            target_industries = $6, preferred_locations = $7,
            preferences = $8, version = version + 1
        WHERE id = $9 AND version = $10
        RETURNING version`

	args := []any{
		user.FirstName,
		user.LastName,
		user.EmailVerified,
		user.DegreeDiscipline,
		user.GraduationYear,
		user.TargetIndustries,
		user.PreferredLocations,
		user.Preferences,
		user.ID,
		user.Version,
	}

	err := db.QueryRow(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrDuplicateEmail
			}
			return err
		}
	}

	return nil
}

func (m UserModel) Activate(ctx context.Context, db DBTX, userID string) error {
	query := `
        UPDATE app_user
        SET email_verified = true, version = version + 1
        WHERE id = $1`

	_, err := db.Exec(ctx, query, userID)
	return err
}

func (m UserModel) UpdatePassword(ctx context.Context, db DBTX, userID string, hash []byte) error {
	query := `
        UPDATE app_user
        SET password_hash = $1, version = version + 1
        WHERE id = $2`

	_, err := db.Exec(ctx, query, hash, userID)
	return err
}

func HashPassword(plaintext string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plaintext), 12)
}
