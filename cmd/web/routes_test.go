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
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test OK with login
	response = ts.get(t, "/health/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Check the response content type
	assert.Equal(t, "text/plain", response.header.Get("Content-Type"))

	// Check the body contains "OK"
	assert.StringIn(t, "status: OK", response.body)
	assert.StringIn(t, vcs.Version(), response.body)
	assert.StringIn(t, "devMode: false", response.body)
	assert.StringIn(t, "app name:", response.body)
}

func TestListNotes(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/list/", false)
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test OK with login
	response = ts.get(t, "/list/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Has the search form fields
	assert.StringIn(t, `<form method="GET"`, response.body)
	assert.StringIn(t, `<input type="text" name="q" id="q" placeholder="Search notes..." value="">`, response.body)
	assert.StringIn(t, `<select name="tag" id="tag" aria-label="Select a tag...">`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="favorites" name="favorites" />`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="archived" name="archived" />`, response.body)

	// Has the title of a recent note
	assert.StringIn(t, "Weekend Plans", response.body)
	assert.StringIn(t, "/note/n_001/", response.body)

	// Response does not have an archived note
	assert.StringNotIn(t, "PostgreSQL Learning", response.body)
	assert.StringNotIn(t, "n_009", response.body)
}

func TestDeleteNoteGet(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/note/n_001/delete/", false)
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test OK with login
	response = ts.get(t, "/note/n_001/delete/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Has the form fields
	assert.StringIn(t, `<form method="POST" action="/note/n_001/delete/">`, response.body)
	assert.StringIn(t, `<input type="hidden" name="csrf_token" value="`, response.body)
	assert.StringIn(t, `<input type="submit" value="Delete">`, response.body)

	// Has the title of the note
	assert.StringIn(t, "Weekend Plans", response.body)
}

func TestDeleteNotePost(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Validate the post exists
	response := ts.get(t, "/note/n_001/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Test unauthorized without login
	response = ts.post(t, "/note/n_001/delete/", url.Values{}, false)
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test delete requires csrf_token
	response = ts.post(t, "/notes/n_001/delete/", url.Values{}, true)
	assert.Equal(t, http.StatusBadRequest, response.statusCode)

	// Get a CSRF Token then post a delete
	response = ts.get(t, "/note/n_001/delete/", true)
	data := url.Values{}
	data.Add("csrf_token", response.csrfToken(t))

	// Post a response with the csrf token
	response = ts.post(t, "/note/n_001/delete/", data, true)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)
	assert.Equal(t, "/list/", response.header.Get("Location"))

	// Validate the post doesn't exist anymore
	response = ts.get(t, "/note/n_001/", true)
	assert.Equal(t, http.StatusNotFound, response.statusCode)
}

func TestHome(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	response := ts.get(t, "/", true)

	assert.Equal(t, response.statusCode, http.StatusOK)
	assert.StringIn(t, "Example", response.body)
}

func TestNewNoteGET(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/new/", false)
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test OK with login
	response = ts.get(t, "/new/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Has the form fields
	assert.StringIn(t, `<form id="note-form" method="POST">`, response.body)
	assert.StringIn(t, `<input type="hidden" name="csrf_token" value="`, response.body)
	assert.StringIn(t, `<input type="text" id="title" name="title"`, response.body)
	assert.StringIn(t, `<input type="datetime-local" id="created_at" name="created_at"`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="favorite" name="favorite" role="switch"`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="archive" name="archive" role="switch"`, response.body)
	assert.StringIn(t, `textarea id="note" name="note" placeholder="Note content..."`, response.body)
	assert.StringIn(t, `<input type="submit" value="Submit">`, response.body)
}

func TestEditNoteGET(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/note/n_002/edit/", false)
	assert.Equal(t, http.StatusUnauthorized, response.statusCode)

	// Test OK with login
	response = ts.get(t, "/note/n_002/edit/", true)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Has the form fields
	assert.StringIn(t, `<form id="note-form" method="POST">`, response.body)
	assert.StringIn(t, `<input type="hidden" name="csrf_token" value="`, response.body)
	assert.StringIn(t, `<input type="text" id="title" name="title"`, response.body)
	assert.StringIn(t, `<input type="datetime-local" id="created_at" name="created_at"`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="favorite" name="favorite" role="switch"`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="archive" name="archive" role="switch"`, response.body)
	assert.StringIn(t, `textarea id="note" name="note" placeholder="Note content..."`, response.body)
	assert.StringIn(t, `<input type="submit" value="Submit">`, response.body)

	// Test the form has data from the fields
	assert.StringIn(t, `New Recipe`, response.body)
	assert.StringIn(t, `Found an amazing #recipe for pasta`, response.body)
	assert.StringIn(t, `2025-01-20`, response.body)    // created_at is editable
	assert.StringNotIn(t, `2025-02-01`, response.body) // modified_at is not editable
	assert.StringNotIn(t, `checked`, response.body)
}
