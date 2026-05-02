package integration_test

import (
	"net/http"
	"testing"
)

func TestListEmployers_Empty(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/employers", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Data []any `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if body.Data == nil {
		t.Error("expected data array, got nil")
	}
}

func TestListEmployers_WithData(t *testing.T) {
	ts := newTestServer(t)

	seedEmployer(t, "acme")
	seedEmployer(t, "globex")

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/employers", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if len(body.Data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(body.Data))
	}
}

func TestShowEmployerBySlug_Found(t *testing.T) {
	ts := newTestServer(t)

	emp := seedEmployer(t, "testco")

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/employers/"+emp.slug, nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Data struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if body.Data.Slug != emp.slug {
		t.Errorf("slug = %q, want %q", body.Data.Slug, emp.slug)
	}
	if body.Data.ID != emp.id {
		t.Errorf("id = %q, want %q", body.Data.ID, emp.id)
	}
}

func TestShowEmployerBySlug_NotFound(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/employers/does-not-exist", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
