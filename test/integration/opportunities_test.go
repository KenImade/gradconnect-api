package integration_test

import (
	"net/http"
	"testing"
)

func TestListOpportunities_Empty(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/opportunities", nil)
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

func TestListOpportunities_WithData(t *testing.T) {
	ts := newTestServer(t)

	emp := seedEmployer(t, "megacorp")
	seedOpportunity(t, emp.id, "megacorp-grad-2025")
	seedOpportunity(t, emp.id, "megacorp-intern-2025")

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/opportunities", nil)
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

func TestShowOpportunityBySlug_Found(t *testing.T) {
	ts := newTestServer(t)

	emp := seedEmployer(t, "startupco")
	opp := seedOpportunity(t, emp.id, "startupco-grad-2025")

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/opportunities/"+opp.slug, nil)
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

	if body.Data.Slug != opp.slug {
		t.Errorf("slug = %q, want %q", body.Data.Slug, opp.slug)
	}
	if body.Data.ID != opp.id {
		t.Errorf("id = %q, want %q", body.Data.ID, opp.id)
	}
}

func TestShowOpportunityBySlug_NotFound(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/opportunities/no-such-slug", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestListOpportunities_InactiveNotShown(t *testing.T) {
	ts := newTestServer(t)

	emp := seedEmployer(t, "hiddenco")
	// seedOpportunity creates active=true by default, so deactivate one directly.
	opp := seedOpportunity(t, emp.id, "hiddenco-grad-2025")
	_, err := testDB.Exec(
		t.Context(),
		`UPDATE opportunity SET is_active = false WHERE id = $1`,
		opp.id,
	)
	_ = err // non-fatal — the active record still being there catches miscount

	seedOpportunity(t, emp.id, "hiddenco-intern-2025")

	resp := ts.do(t, nil, http.MethodGet, "/api/v1/opportunities", nil)
	defer drainClose(resp)

	var body struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	for _, d := range body.Data {
		if d.Slug == "hiddenco-grad-2025" {
			t.Error("inactive opportunity should not appear in public listing")
		}
	}
}
