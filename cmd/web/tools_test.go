package main

import (
	"bytes"
	"context"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sglmr/gowebstart/db"
	"github.com/sglmr/gowebstart/internal/email"
)

type testServer struct {
	*httptest.Server
}

// newTestServer creates a test server for integration tests.
func newTestServer(t *testing.T) *testServer {
	// Create a new serve mux
	mux := http.NewServeMux()

	ctx := context.Background()

	// Connect to the PostgreSQL database
	dbpool, err := pgxpool.New(ctx, os.Getenv("NOTES_TEST_DB_DSN"))
	if err != nil {
		t.Fatalf("connecting to postgres: %w", err)
	}
	defer dbpool.Close()

	// Ping the database, timeout after 5 seconds
	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second*5)
	defer pingCancel()

	if err = dbpool.Ping(pingCtx); err != nil {
		t.Fatalf("pinging postgres: %w", err)
	}

	// Create a new database queries object
	queries := db.New(dbpool)

	// Create an io.Discard logger for testing
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

	// Initialize a new session manager with the cleanup goroutine disabled
	sessionManager := scs.New()
	sessionManager.Store = memstore.NewWithCleanupInterval(0)
	sessionManager.Cookie.Secure = true

	// Create a test mailer (io.Discard)
	mailer := email.NewLogMailer(logger)

	// Initialize other required vairables for routes
	username := "test@example.com"
	password := `$2a$10$yIdGuTfOlZEA00kpreh2yuTihYQs9WAjeoIu/81AMWTVt9.Ocef5O` // 'password'
	wg := sync.WaitGroup{}

	// Create the httpHandler
	handler := AddRoutes(mux, logger, false, mailer, username, password, &wg, sessionManager, queries)

	// Initialize a new test server
	ts := httptest.NewTLSServer(handler)

	// Create and assign a cookiejar to the test server
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	ts.Client().Jar = jar

	// Disable redirect-following with a custom CheckRedirect function.
	ts.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// http.ErrUseLastResponse error forces the client to return to the received response.
		return http.ErrUseLastResponse
	}
	// TODO: come up with some way of getting the last response and the redirected to response

	return &testServer{ts}
}

type testResponse struct {
	statusCode int
	header     http.Header
	body       string
}

// csrfToken extracts and returns the csrfToken from a testResponse html body
func (tr testResponse) csrfToken(t *testing.T) string {
	t.Helper()

	csrfTokenRX := regexp.MustCompile(`<input type="hidden" name="csrf_token" value="(.+)">`)
	csrfTokenHtmxRX := regexp.MustCompile(`<body hx-headers='{"X-CSRF-TOKEN": "(.+)"}'>`)

	var matches []string
	// Try to find a CSRF token in a form
	matches = csrfTokenRX.FindStringSubmatch(tr.body)
	if len(matches) >= 2 {
		return html.UnescapeString(string(matches[1]))
	}

	// Try to find a CSRF token in the htmx
	matches = csrfTokenHtmxRX.FindStringSubmatch(tr.body)
	if len(matches) >= 2 {
		return html.UnescapeString(string(matches[1]))
	}

	t.Fatal("no csrf token found in body")
	return ""
}

// get issues a GET request and returns a testResponse object
//   - 'path' is the relative url path, like "/about/"
func (ts *testServer) get(t *testing.T, path string) testResponse {
	// Create a new http request
	request, err := http.NewRequest(http.MethodGet, ts.URL+path, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	// Send Http Request
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}

	// Read the body of the http response
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	// Return a testResponse object
	return testResponse{
		statusCode: response.StatusCode,
		header:     response.Header,
		body:       string(body),
	}
}

// post issues a POST request and returns a testResponse object
//   - 'path' is the relative url path, like "/about/"
func (ts *testServer) post(t *testing.T, path string, data url.Values) testResponse {
	// Create a new http POST request.
	request, err := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(data.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the POST request.
	response, err := ts.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}

	// Read the response body from the request.
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	// Return a testResponse object
	return testResponse{
		statusCode: response.StatusCode,
		header:     response.Header,
		body:       string(body),
	}
}
