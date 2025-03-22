package main

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/sglmr/go-notes/internal/assert"
	"github.com/sglmr/go-notes/internal/vcs"
)

func TestHealth(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test Unauthorized without login
	response := ts.get(t, "/health/", false)
	assert.Equal(t, response.statusCode, http.StatusUnauthorized)

	// Test OK with login
	response = ts.get(t, "/health/", true)
	assert.Equal(t, response.statusCode, http.StatusOK)

	// Check the response content type
	assert.Equal(t, response.header.Get("Content-Type"), "text/plain")

	// Check the body contains "OK"
	assert.StringContains(t, response.body, "status: OK")
	assert.StringContains(t, response.body, vcs.Version())
	assert.StringContains(t, response.body, "devMode: false")
	assert.StringContains(t, response.body, "app name:")
}

func TestListNotes(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/list/", false)
	assert.Equal(t, response.statusCode, http.StatusUnauthorized)

	// Test OK with login
	response = ts.get(t, "/list/", true)
	assert.Equal(t, response.statusCode, http.StatusOK)

	// Has the search form fields
	assert.StringContains(t, response.body, `<form method="GET"`)
	assert.StringContains(t, response.body, `<input type="text" name="q" id="q" placeholder="Search notes..." value="">`)
	assert.StringContains(t, response.body, `<select name="tag" id="tag" aria-label="Select a tag...">`)
	assert.StringContains(t, response.body, `<input type="checkbox" id="favorites" name="favorites" />`)
	assert.StringContains(t, response.body, `<input type="checkbox" id="archived" name="archived" />`)

	// Has the title of a recent note
	assert.StringContains(t, response.body, "Weekend Plans")
	assert.StringContains(t, response.body, "/note/n_001/")

	// Response does not have an archived note
	assert.StringNotContains(t, response.body, "PostgreSQL Learning")
	assert.StringNotContains(t, response.body, "n_009")
}

func TestDeleteNoteGet(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/note/n_001/delete/", false)
	assert.Equal(t, response.statusCode, http.StatusUnauthorized)

	// Test OK with login
	response = ts.get(t, "/note/n_001/delete/", true)
	assert.Equal(t, response.statusCode, http.StatusOK)

	// Has the form fields
	assert.StringContains(t, response.body, `<form method="POST" action="/note/n_001/delete/">`)
	assert.StringContains(t, response.body, `<input type="hidden" name="csrf_token" value="`)
	assert.StringContains(t, response.body, `<input type="submit" value="Delete">`)

	// Has the title of the note
	assert.StringContains(t, response.body, "Weekend Plans")
}

func TestDeleteNotePost(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Validate the post exists
	response := ts.get(t, "/note/n_001/", true)
	assert.Equal(t, response.statusCode, http.StatusOK)

	// Test unauthorized without login
	response = ts.post(t, "/note/n_001/delete/", url.Values{}, false)
	assert.Equal(t, response.statusCode, http.StatusUnauthorized)

	// Test delete requires csrf_token
	response = ts.post(t, "/notes/n_001/delete/", url.Values{}, true)
	assert.Equal(t, response.statusCode, http.StatusBadRequest)

	// Get a CSRF Token then post a delete
	response = ts.get(t, "/note/n_001/delete/", true)
	data := url.Values{}
	data.Add("csrf_token", response.csrfToken(t))

	// Post a response with the csrf token
	response = ts.post(t, "/note/n_001/delete/", data, true)
	assert.Equal(t, response.statusCode, http.StatusSeeOther)
	next := response.header.Get("Location")
	assert.Equal(t, next, "/list/")

	// Validate the post doesn't exist anymore
	response = ts.get(t, "/note/n_001/", true)
	assert.Equal(t, response.statusCode, http.StatusNotFound)
}

func TestHome(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	response := ts.get(t, "/", true)

	assert.Equal(t, http.StatusOK, response.statusCode)
	assert.StringContains(t, response.body, "Example")
}
