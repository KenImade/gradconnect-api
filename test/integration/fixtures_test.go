package integration_test

import (
	"context"
	"net/http"
	"testing"
)

// verifiedUser is returned by seedVerifiedUser.
type verifiedUser struct {
	id     string
	client *http.Client // carries a valid session cookie
}

// seedVerifiedUser registers a user via the HTTP API (to get a real session
// cookie), then promotes them to verified with full permissions directly in
// the DB. This mirrors what email-activation would do, without needing a
// working mail server in the test flow.
func seedVerifiedUser(t *testing.T, ts *testServer, email string) verifiedUser {
	t.Helper()

	client := ts.newClient()

	payload := map[string]any{
		"first_name": "Test",
		"last_name":  "User",
		"email":      email,
		"password":   "pa55word123",
	}

	resp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", payload)
	drainClose(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seedVerifiedUser register: status = %d", resp.StatusCode)
	}

	var userID string
	err := testDB.QueryRow(context.Background(),
		`UPDATE app_user SET email_verified = true WHERE email = $1 RETURNING id`,
		email,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("seedVerifiedUser activate: %v", err)
	}

	_, err = testDB.Exec(context.Background(), `
		INSERT INTO user_permission (user_id, permission)
		VALUES ($1, 'review:submit'), ($1, 'review:edit')
		ON CONFLICT DO NOTHING
	`, userID)
	if err != nil {
		t.Fatalf("seedVerifiedUser permissions: %v", err)
	}

	return verifiedUser{id: userID, client: client}
}

type seededEmployer struct {
	id   string
	slug string
}

// seedEmployer inserts a minimal employer row directly into the DB.
func seedEmployer(t *testing.T, slug string) seededEmployer {
	t.Helper()

	var id string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO employer (name, slug, industry)
		VALUES ($1, $2, 'Technology')
		RETURNING id
	`, slug+" Corp", slug).Scan(&id)
	if err != nil {
		t.Fatalf("seedEmployer: %v", err)
	}
	return seededEmployer{id: id, slug: slug}
}

type seededOpportunity struct {
	id   string
	slug string
}

// seedOpportunity inserts a minimal opportunity row for the given employer.
func seedOpportunity(t *testing.T, employerID, slug string) seededOpportunity {
	t.Helper()

	var id string
	err := testDB.QueryRow(context.Background(), `
		INSERT INTO opportunity
			(employer_id, title, slug, type, intake_year, description, location, application_url, is_active)
		VALUES
			($1, $2, $3, 'graduate_trainee', 2025, 'A great role', 'Lagos', 'https://apply.example.com', true)
		RETURNING id
	`, employerID, slug+" Graduate Trainee", slug).Scan(&id)
	if err != nil {
		t.Fatalf("seedOpportunity: %v", err)
	}
	return seededOpportunity{id: id, slug: slug}
}
