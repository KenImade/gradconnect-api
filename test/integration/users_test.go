package integration_test

import (
	"net/http"
	"testing"
)

func TestUpdateUser_Success(t *testing.T) {
	ts := newTestServer(t)
	client := ts.newClient()

	resp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(resp)

	update := map[string]any{
		"first_name": "Updated",
		"last_name":  "Name",
	}
	resp2 := ts.do(t, client, http.MethodPatch, "/api/v1/me", update)
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	var body struct {
		Data struct {
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
		} `json:"data"`
	}
	decodeJSON(t, resp2, &body)

	if body.Data.FirstName != "Updated" {
		t.Errorf("first_name = %q, want %q", body.Data.FirstName, "Updated")
	}
	if body.Data.LastName != "Name" {
		t.Errorf("last_name = %q, want %q", body.Data.LastName, "Name")
	}
}

func TestUpdateUser_Unauthenticated(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodPatch, "/api/v1/me", map[string]any{"first_name": "X"})
	defer drainClose(resp)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
