package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"api.gradconnect.com/internal/app"
	"api.gradconnect.com/internal/imagegen"
	"api.gradconnect.com/internal/mailer"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testDB is the shared pool for all integration tests. It is initialised once
// in TestMain and reused across test functions for speed.
var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("GRADCONNECT_TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://gradconnect:gradconnect_dev_pw@localhost:5433/gradconnect_test?sslmode=disable"
	}

	var err error
	testDB, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		panic("could not connect to test database: " + err.Error())
	}
	defer testDB.Close()

	if err := testDB.Ping(context.Background()); err != nil {
		panic("test database unreachable — is docker-compose up? " + err.Error())
	}

	os.Exit(m.Run())
}

// truncateAll removes all application data between tests. Table order respects
// FK constraints; CASCADE handles any remaining dependencies.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(context.Background(), `
		TRUNCATE
			session,
			email_verification_token,
			password_reset_token,
			task_queue,
			user_permission,
			bookmark,
			application_track,
			review,
			import_job,
			cron_run,
			opportunity,
			employer,
			app_user
		CASCADE
	`)
	if err != nil {
		t.Fatalf("truncateAll: %v", err)
	}
}

// newTestServer spins up an httptest.Server backed by a real App wired to the
// test database. Tables are truncated after each test via t.Cleanup.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	m, err := mailer.New(
		"localhost",
		1025,
		"",
		"",
		"test@gradconnect.ng",
		"support-test@gradconnect.ng",
		false,
		"")
	if err != nil {
		t.Fatalf("mailer.New: %v", err)
	}

	cfg := app.Config{
		Env:         "testing",
		FrontendURL: "http://localhost:3000",
		BaseURL:     "http://localhost:4000",
	}

	ig, err := imagegen.New()
	if err != nil {
		t.Fatalf("imagegen.New: %v", err)
	}
	a := app.New(cfg, testDB, ig, logger, m, &noopStorage{}, nil, "")
	srv := httptest.NewServer(a.Routes())

	t.Cleanup(func() {
		srv.Close()
		truncateAll(t)
	})

	return &testServer{Server: srv}
}

// testServer wraps httptest.Server with convenience methods.
type testServer struct {
	*httptest.Server
}

// newClient returns an HTTP client that persists cookies across requests (for
// session-based auth flows). Redirects are not followed automatically.
func (ts *testServer) newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// do makes a JSON request to the test server and returns the raw response.
// Pass body=nil for requests with no body.
func (ts *testServer) do(t *testing.T, client *http.Client, method, path string, body any) *http.Response {
	t.Helper()

	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, ts.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if client == nil {
		client = ts.newClient()
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// decodeJSON decodes the response body into dst.
func decodeJSON(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// drainClose reads and discards the response body so the connection can be reused.
func drainClose(resp *http.Response) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

// noopStorage satisfies storage.Storage without performing any I/O.
type noopStorage struct{}

func (n *noopStorage) Upload(_ context.Context, _, _ string, _ io.Reader) (string, error) {
	return "https://example.com/test-upload.jpg", nil
}
func (n *noopStorage) Download(_ context.Context, _ string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (n *noopStorage) Delete(_ context.Context, _ string) error { return nil }
func (n *noopStorage) PublicURL(_ string) string                { return "https://example.com/test.jpg" }
