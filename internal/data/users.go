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

type User struct {
	ID                 string          `json:"id"`
	Email              string          `json:"email"`
	Password           password        `json:"-"`
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

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

func (p *password) Matches(plaintextPassword string) (bool, error) {
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

func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.FirstName != "", "first_name", "first name must be provided")
	v.Check(user.LastName != "", "last_name", "last name must be provided")
	v.Check(len(user.FirstName) <= 255, "first_name", "must not be more than 255 characters")
	v.Check(len(user.LastName) <= 255, "last_name", "must not be more than 255 characters")

	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	if user.AuthProvider == "email" && user.Password.hash == nil {
		v.AddError("password", "must be provided for email users")
	}

	v.Check(validator.PermittedValue(user.AuthProvider, "email", "google"), "auth_provider", "must be email or google")

	if user.GraduationYear != nil {
		currentYear := time.Now().Year()
		v.Check(*user.GraduationYear >= 1990, "graduation_year", "must be after 1990")
		v.Check(*user.GraduationYear <= currentYear+6, "graduation_year", "must not be more than 6 years in the future")
	}

	if user.DegreeDiscipline != nil {
		v.Check(len(*user.DegreeDiscipline) <= 255, "degree_discipline", "must not be more than 255 characters")
	}

	v.Check(len(user.TargetIndustries) <= 20, "target_industries", "must not have more than 20 industries")
	v.Check(len(user.PreferredLocations) <= 20, "preferred_locations", "must not have more than 20 locations")
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

func (m UserModel) GetByEmail(email string) (*User, error) { return nil, nil }

func (m UserModel) Update(user *User) error { return nil }

func (m UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) { return nil, nil }
