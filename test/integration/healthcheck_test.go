package integration_test

import (
	"net/http"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/healthcheck", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Status     string `json:"status"`
		SystemInfo struct {
			Environment string `json:"environment"`
			Version     string `json:"version"`
		} `json:"system_info"`
	}
	decodeJSON(t, resp, &body)

	if body.Status != "available" {
		t.Errorf("status = %q, want %q", body.Status, "available")
	}
	if body.SystemInfo.Environment != "testing" {
		t.Errorf("environment = %q, want %q", body.SystemInfo.Environment, "testing")
	}
	if body.SystemInfo.Version == "" {
		t.Error("version is empty")
	}
}
