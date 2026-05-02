package integration_test

import (
	"net/http"
	"testing"
)

func TestGetCurrentUser_Unauthenticated(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/me", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetCurrentUser_Authenticated(t *testing.T) {
	ts := newTestServer(t)
	client := ts.newClient()

	// Register — client captures the session cookie.
	resp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	// GET /me with the same client.
	resp2 := ts.do(t, client, http.MethodGet, "/api/v1/me", nil)
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	var body struct {
		Data struct {
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
		} `json:"data"`
	}
	decodeJSON(t, resp2, &body)

	if body.Data.Email != registerPayload["email"] {
		t.Errorf("email = %q, want %q", body.Data.Email, registerPayload["email"])
	}
	if body.Data.FirstName != registerPayload["first_name"] {
		t.Errorf("first_name = %q, want %q", body.Data.FirstName, registerPayload["first_name"])
	}
}

func TestGetCurrentUser_AfterLogout(t *testing.T) {
	ts := newTestServer(t)
	client := ts.newClient()

	// Register + logout.
	resp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)

	resp2 := ts.do(t, client, http.MethodPost, "/api/v1/auth/logout", nil)
	drainClose(resp2)

	// GET /me after logout — session no longer valid.
	resp3 := ts.do(t, client, http.MethodGet, "/api/v1/me", nil)
	defer drainClose(resp3)

	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp3.StatusCode, http.StatusUnauthorized)
	}
}
