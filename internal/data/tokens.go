package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ScopeActivation     = "activation"
	ScopePasswordReset  = "password-reset"
	ScopeAuthentication = "authentication"
)

type Token struct {
	Plaintext string    `json:"token"`
	Hash      string    `json:"-"` // hex-encoded SHA-256
	UserID    string    `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID string, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	randomBytes := make([]byte, 16)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// Encode to base32 to make it URL-safe and easy to read/type
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hex.EncodeToString(hash[:])

	return token, nil
}

type TokenModel struct {
	DB *pgxpool.Pool
}

func (m TokenModel) GetUserForToken(ctx context.Context, db DBTX, scope, tokenPlaintext string) (string, error) {
	hash := sha256.Sum256([]byte(tokenPlaintext))
	tokenHash := hex.EncodeToString(hash[:])

	tableName := "email_verification_token"
	if scope == ScopePasswordReset {
		tableName = "password_reset_token"
	}

	query := fmt.Sprintf(`
        SELECT user_id
        FROM %s
        WHERE token_hash = $1 AND expires_at > now()`, tableName)

	var userID string
	err := db.QueryRow(ctx, query, tokenHash).Scan(&userID)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return "", ErrRecordNotFound
		default:
			return "", err
		}
	}

	return userID, nil
}

func (m TokenModel) New(ctx context.Context, db DBTX, userID string, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	// Determine target table
	tableName := "email_verification_token"
	if scope == ScopePasswordReset {
		tableName = "password_reset_token"
	}

	// 1. Delete any existing tokens for this user (Lifecycle Rule)
	// 2. Insert the new token
	// We do this in one multi-statement string or separate calls within the tx
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", tableName)
	_, err = db.Exec(ctx, deleteQuery, userID)
	if err != nil {
		return nil, err
	}

	insertQuery := fmt.Sprintf(`
        INSERT INTO %s (user_id, token_hash, expires_at)
        VALUES ($1, $2, $3)`, tableName)

	_, err = db.Exec(ctx, insertQuery, token.UserID, token.Hash, token.Expiry)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func (m TokenModel) DeleteAllForUser(ctx context.Context, db DBTX, scope, userID string) error {

	tableName := "email_verification_token"
	if scope == ScopePasswordReset {
		tableName = "password_reset_token"
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", tableName)
	_, err := db.Exec(ctx, query, userID)
	return err
}
