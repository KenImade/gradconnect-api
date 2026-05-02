package integration_test

import (
	"net/http"
	"testing"
)

// registerPayload is the default valid registration body used across tests.
var registerPayload = map[string]any{
	"first_name": "Ada",
	"last_name":  "Lovelace",
	"email":      "ada@test.com",
	"password":   "pa55word123",
}

func TestRegister_Success(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", registerPayload)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var body struct {
		Data struct {
			ID            string `json:"id"`
			Email         string `json:"email"`
			FirstName     string `json:"first_name"`
			EmailVerified bool   `json:"email_verified"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if body.Data.ID == "" {
		t.Error("expected non-empty user ID")
	}
	if body.Data.Email != registerPayload["email"] {
		t.Errorf("email = %q, want %q", body.Data.Email, registerPayload["email"])
	}
	if body.Data.EmailVerified {
		t.Error("email_verified should be false immediately after registration")
	}

	// A session cookie must be set.
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == "session_id" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("session_id cookie not set after registration")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	ts := newTestServer(t)

	// First registration — must succeed.
	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("first register: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	// Second registration with the same email — must fail.
	resp2 := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", registerPayload)
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("duplicate register: status = %d, want %d", resp2.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestRegister_InvalidInput(t *testing.T) {
	ts := newTestServer(t)

	cases := []struct {
		name    string
		payload map[string]any
	}{
		{
			name:    "missing email",
			payload: map[string]any{"first_name": "Ada", "last_name": "L", "password": "pa55word123"},
		},
		{
			name:    "short password",
			payload: map[string]any{"first_name": "Ada", "last_name": "L", "email": "short@test.com", "password": "abc"},
		},
		{
			name:    "invalid email format",
			payload: map[string]any{"first_name": "Ada", "last_name": "L", "email": "not-an-email", "password": "pa55word123"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", tc.payload)
			drainClose(resp)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want %d", tc.name, resp.StatusCode, http.StatusUnprocessableEntity)
			}
		})
	}
}

func TestLogin_Success(t *testing.T) {
	ts := newTestServer(t)

	// Register first.
	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)

	// Login.
	resp2 := ts.do(t, nil, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    registerPayload["email"],
		"password": registerPayload["password"],
	})
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	found := false
	for _, c := range resp2.Cookies() {
		if c.Name == "session_id" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("session_id cookie not set after login")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)

	resp2 := ts.do(t, nil, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    registerPayload["email"],
		"password": "wrongpassword",
	})
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusUnauthorized)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/login", map[string]any{
		"email":    "nobody@test.com",
		"password": "pa55word123",
	})
	defer drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLogout_Success(t *testing.T) {
	ts := newTestServer(t)
	client := ts.newClient()

	// Register — client stores the session cookie automatically.
	resp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)

	// Logout using the same client (cookie is sent automatically).
	resp2 := ts.do(t, client, http.MethodPost, "/api/v1/auth/logout", nil)
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusNoContent)
	}
}

func TestLogout_Unauthenticated(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodPost, "/api/v1/auth/logout", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
