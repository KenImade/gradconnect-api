package integration_test

import (
	"net/http"
	"testing"
)

func TestListBookmarks_RequiresVerifiedUser(t *testing.T) {
	ts := newTestServer(t)

	// Unauthenticated.
	resp := ts.do(t, nil, http.MethodGet, "/api/v1/me/bookmarks", nil)
	drainClose(resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated: status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	// Authenticated but email not verified.
	client := ts.newClient()
	regResp := ts.do(t, client, http.MethodPost, "/api/v1/auth/register", registerPayload)
	drainClose(regResp)

	resp2 := ts.do(t, client, http.MethodGet, "/api/v1/me/bookmarks", nil)
	drainClose(resp2)
	if resp2.StatusCode != http.StatusForbidden {
		t.Errorf("unverified: status = %d, want %d", resp2.StatusCode, http.StatusForbidden)
	}
}

func TestListBookmarks_Empty(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "bookmarks-empty@test.com")

	resp := ts.do(t, user.client, http.MethodGet, "/api/v1/me/bookmarks", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body struct {
		Data []any `json:"data"`
	}
	decodeJSON(t, resp, &body)
	if body.Data == nil {
		t.Error("expected empty data array, got nil")
	}
}

func TestAddBookmark_Success(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "add-bookmark@test.com")
	emp := seedEmployer(t, "bookmark-employer")
	opp := seedOpportunity(t, emp.id, "bookmark-opp")

	resp := ts.do(t, user.client, http.MethodPost, "/api/v1/me/bookmarks", map[string]any{
		"opportunity_id": opp.id,
	})
	defer drainClose(resp)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var body struct {
		Data struct {
			ID            string `json:"id"`
			OpportunityID string `json:"opportunity_id"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if body.Data.OpportunityID != opp.id {
		t.Errorf("opportunity_id = %q, want %q", body.Data.OpportunityID, opp.id)
	}
	if body.Data.ID == "" {
		t.Error("expected non-empty bookmark ID")
	}
}

func TestAddBookmark_Duplicate(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "dup-bookmark@test.com")
	emp := seedEmployer(t, "dup-bm-employer")
	opp := seedOpportunity(t, emp.id, "dup-bm-opp")

	payload := map[string]any{"opportunity_id": opp.id}

	resp := ts.do(t, user.client, http.MethodPost, "/api/v1/me/bookmarks", payload)
	drainClose(resp)

	resp2 := ts.do(t, user.client, http.MethodPost, "/api/v1/me/bookmarks", payload)
	defer drainClose(resp2)

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusConflict)
	}
}

func TestAddBookmark_InvalidOpportunity(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "invalid-bm@test.com")

	resp := ts.do(t, user.client, http.MethodPost, "/api/v1/me/bookmarks", map[string]any{
		"opportunity_id": "00000000-0000-0000-0000-000000000000",
	})
	defer drainClose(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRemoveBookmark_Success(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "remove-bm@test.com")
	emp := seedEmployer(t, "remove-bm-employer")
	opp := seedOpportunity(t, emp.id, "remove-bm-opp")

	// Add bookmark.
	addResp := ts.do(t, user.client, http.MethodPost, "/api/v1/me/bookmarks", map[string]any{
		"opportunity_id": opp.id,
	})
	var addBody struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	decodeJSON(t, addResp, &addBody)
	bookmarkID := addBody.Data.ID

	// Remove it.
	delResp := ts.do(t, user.client, http.MethodDelete, "/api/v1/me/bookmarks/"+bookmarkID, nil)
	defer drainClose(delResp)

	if delResp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want %d", delResp.StatusCode, http.StatusNoContent)
	}
}

func TestRemoveBookmark_NotFound(t *testing.T) {
	ts := newTestServer(t)

	user := seedVerifiedUser(t, ts, "rm-notfound@test.com")

	resp := ts.do(t, user.client, http.MethodDelete, "/api/v1/me/bookmarks/00000000-0000-0000-0000-000000000000", nil)
	defer drainClose(resp)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
