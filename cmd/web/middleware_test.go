package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sglmr/go-notes/internal/assert"
)

func TestSecureHeadersMW(t *testing.T) {
	t.Parallel()

	// Initialize a new httptest.ResponseRecorder and dummy http.Request.
	rr := httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock HTTP handler that we can pass to our SecureHeadersMW
	// middleware, which writes a 200 status code and an "OK" response body.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Pass the mock HTTP handler to the SecureHeadersMW middleware.
	// Call ServeHTTP to execute it.
	SecureHeadersMW(next).ServeHTTP(rr, r)

	// Get the results of the test
	rs := rr.Result()

	// Check that the middleware has correctly set the Referrer-Policy
	// header on the response.
	want := "origin-when-cross-origin"
	assert.Equal(t, rs.Header.Get("Referrer-Policy"), want)

	// Check that the middleware has correctly set the X-Content-Type-Options
	// header on the response.
	want = "nosniff"
	assert.Equal(t, rs.Header.Get("X-Content-Type-Options"), want)

	// Check that the middleware has correctly set the X-Frame-Options header
	// on the response.
	want = "deny"
	assert.Equal(t, rs.Header.Get("X-Frame-Options"), want)

	// Check that the middleware has correctly set the X-XSS-Protection header
	// on the response
	want = "0"
	assert.Equal(t, rs.Header.Get("X-XSS-Protection"), want)

	// Check that the middleware has correctly called the next handler in line
	// and the response status code and body are as expected.
	assert.Equal(t, rs.StatusCode, http.StatusOK)

	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	assert.Equal(t, string(body), "OK")
}

func TestRecoverPanicMW(t *testing.T) {
	t.Parallel()

	// Create a test logger
	logBuffer := bytes.Buffer{}
	testLogger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	// Initialize a new httptest.ResponseRecorder and dummy http.Request.
	rr := httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock HTTP handler that we can pass to our RecoverPanicMW
	// middleware, which creates a panic
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("Help!")
	})

	// Pass the mock HTTP handler to the RecoverPanicMW middleware.
	// Call ServeHTTP to execute it.
	recoverPanicMW(next, testLogger, false).ServeHTTP(rr, r)

	// Get the results of the test
	rs := rr.Result()

	// Check that the middleware has correctly called the next handler in line
	// and the response status code and body are as expected.
	assert.Equal(t, http.StatusInternalServerError, rs.StatusCode)

	defer rs.Body.Close()
	body, err := io.ReadAll(rs.Body)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.TrimSpace(body)

	want := "The server encountered a problem and could not process your request"
	assert.Equal(t, string(body), want)

	// Check the log message
	logMsg := logBuffer.String()

	assert.StringIn(t, "level=ERROR", logMsg)
	assert.StringIn(t, "status=500", logMsg)
	assert.StringIn(t, "error=Help!", logMsg)
}

func TestBasicAuthMWUnauthorized(t *testing.T) {
	t.Parallel()

	// Create a test logger
	logBuffer := bytes.Buffer{}
	testLogger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	// Initialize a new httptest.ResponseRecorder and dummy http.Request.
	rr := httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a mock HTTP handler that we can pass to our BasicAuthMW
	// middleware, which writes a 200 status code and an "OK" response body.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Pass the mock HTTP handler to the BasicAuthMW middleware.
	// Call ServeHTTP to execute it.
	// Hashed password is 'password'
	mw := basicAuthMW(testUsername, testPasswordHash, testLogger)
	mw(next).ServeHTTP(rr, r)

	// Get the results of the test
	rs := rr.Result()

	// Check that the middleware has correctly called the next handler in line
	// and the response status code and body are as expected.
	assert.Equal(t, http.StatusUnauthorized, rs.StatusCode)

	// Check that the middleware has correctly set the WWW-Authenticate header
	// on the response.
	assert.Equal(t, `Basic realm="restricted", charset="UTF-8"`, rs.Header.Get("WWW-Authenticate"))
}

func TestBasicAuthMWOK(t *testing.T) {
	t.Parallel()

	// Create a test logger
	logBuffer := bytes.Buffer{}
	testLogger := slog.New(slog.NewTextHandler(&logBuffer, nil))

	// Initialize a new httptest.ResponseRecorder and dummy http.Request.
	rr := httptest.NewRecorder()

	r, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Set the basic auth credentials in the request
	r.SetBasicAuth(testUsername, testPassword)

	// Create a mock HTTP handler that we can pass to our BasicAuthMW
	// middleware, which writes a 200 status code and an "OK" response body.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Pass the mock HTTP handler to the BasicAuthMW middleware.
	// Call ServeHTTP to execute it.
	// Hashed password is 'password'
	mw := basicAuthMW(testUsername, testPasswordHash, testLogger)
	mw(next).ServeHTTP(rr, r)

	// Get the results of the test
	rs := rr.Result()

	// Check that the middleware has correctly called the next handler in line
	// and the response status code and body are as expected.
	assert.Equal(t, http.StatusOK, rs.StatusCode)
}
