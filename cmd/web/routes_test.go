package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglmr/go-notes/db"
	"github.com/sglmr/go-notes/internal/assert"
	"github.com/sglmr/go-notes/internal/vcs"
)

func TestLoginLogout(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Test logout unauthorized without login
	response := ts.get(t, "/logout/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test login without login
	response = ts.get(t, "/login/")
	assert.Equal(t, http.StatusOK, response.statusCode)
	assert.StringIn(t, `<input type="hidden" name="csrf_token"`, response.body)
	assert.StringIn(t, `<input type="text" id="email" name="email"`, response.body)
	assert.StringIn(t, `<input type="password" id="password" name="password"`, response.body)
	assert.StringNotIn(t, `/logout/`, response.body)

	// Try login with fake username
	data := url.Values{}
	data.Set("csrf_token", response.csrfToken(t))
	data.Set("email", "fake@example.com")
	data.Set("password", testPassword)
	response = ts.post(t, "/login/", data)
	assert.Equal(t, http.StatusUnprocessableEntity, response.statusCode)
	assert.StringIn(t, "Email or password is incorrect", response.body)
	assert.StringNotIn(t, "You are in!", response.body)

	// Try login with a fake password
	data.Set("email", testEmail)
	data.Set("password", "wrong-password")
	response = ts.post(t, "/login/", data)
	assert.Equal(t, http.StatusUnprocessableEntity, response.statusCode)
	assert.StringIn(t, "Email or password is incorrect", response.body)
	assert.StringNotIn(t, "You are in!", response.body)

	// Try login with real password and email
	data.Set("email", testEmail)
	data.Set("password", testPassword)
	response = ts.post(t, "/login/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Check flash message on next page
	response = ts.get(t, "/")
	assert.StringIn(t, "You are in!", response.body)
	assert.StringNotIn(t, "Email or password is incorrect", response.body)

	// Try logout get after login
	response = ts.get(t, "/logout/")
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Try posting logout to log out
	data = url.Values{}
	data.Set("csrf_token", response.csrfToken(t))
	response = ts.post(t, "/logout/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Logout get should redirect to login page now
	response = ts.get(t, "/logout/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)
}

func TestHealth(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test ok without login
	response := ts.get(t, "/health/")
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/health/")
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Check the response content type
	assert.Equal(t, "text/plain", response.header.Get("Content-Type"))

	// Check the body contains "OK"
	assert.StringIn(t, "status: OK", response.body)
	assert.StringIn(t, vcs.Version(), response.body)
	assert.StringIn(t, "devMode: false", response.body)
	assert.StringIn(t, "time location:  America/Los_Angeles", response.body)
}

func TestListNotes(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/notes/list/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/notes/list/")
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Has the search form fields
	assert.StringIn(t, `<form method="GET"`, response.body)
	assert.StringIn(t, `<input type="text" name="q" id="q" placeholder="Search notes..." value="">`, response.body)
	assert.StringIn(t, `<select name="tag" id="tag" aria-label="Select a tag...">`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="favorites" name="favorites"`, response.body)
	assert.StringIn(t, `<input type="checkbox" id="archived" name="archived"`, response.body)

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
	response := ts.get(t, "/note/n_001/delete/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/note/n_001/delete/")
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

	// Test redirect to login
	response := ts.post(t, "/note/n_001/delete/", url.Values{})
	assert.Equal(t, http.StatusSeeOther, response.statusCode)
	assert.StringIn(t, "/login/?next=", response.header.Get("Location"))

	// Validate the post exists
	ts.login(t)
	response = ts.get(t, "/note/n_001/")
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Get a CSRF Token then post a delete
	response = ts.get(t, "/note/n_001/delete/")
	assert.Equal(t, http.StatusOK, response.statusCode)
	token := response.csrfToken(t)

	// Test delete requires csrf_token
	data := url.Values{}
	response = ts.post(t, "/note/n_001/delete/", data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Post a response with the csrf token
	data.Set("csrf_token", token)
	response = ts.post(t, "/note/n_001/delete/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)
	assert.Equal(t, "/notes/list/", response.header.Get("Location"))

	// Validate the post doesn't exist anymore
	response = ts.get(t, "/note/n_001/")
	assert.Equal(t, http.StatusNotFound, response.statusCode)
}

func TestHome(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Try getting the page without login
	response := ts.get(t, "/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Try again with login
	ts.login(t)
	response = ts.get(t, "/")
	assert.Equal(t, http.StatusOK, response.statusCode)
	assert.StringIn(t, "Home", response.body)
}

func TestNewNoteGET(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/notes/new/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/notes/new/")
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

func TestNewNotePOST(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Create a new database connection for queries
	queries := db.NewTestDatabase(t, context.Background(), os.Getenv("NOTES_TEST_DB_DSN"), false)

	data := url.Values{}

	// Test unauthorized without login
	response := ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test bad request with login (but missing csrf)
	ts.login(t)
	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Get a CSRF token for testing
	response = ts.get(t, "/notes/new/")
	csrfToken := response.csrfToken(t)

	// Try a full request without the csrf token
	data.Set("title", "A Shiny New Post")
	data.Set("created_at", time.Now().In(timeLocation).Format("2006-01-02T15:04"))
	data.Set("favorite", "on")
	data.Set("archive", "on")
	data.Set("note", `just #testing with #fishing and not [link](#link) or href="#that"`)

	// Post should fail without a csrf token
	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Post should succeed with a csrf token
	data.Set("csrf_token", csrfToken)
	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Check the reditect location
	nextLocation := response.header.Get("Location")
	assert.StringIn(t, "/note/n_", nextLocation)

	// Get the postID from the new post
	newPostID := strings.Split(nextLocation, "/")[2]
	assert.StringIn(t, "n_", newPostID)

	// Query the new post from the database
	note, err := queries.GetNote(context.Background(), newPostID)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the new post data in the database
	assert.Equal(t, "A Shiny New Post", note.Title)
	assert.EqualTime(t, time.Now().In(timeLocation), note.CreatedAt, time.Second*61)
	assert.EqualTime(t, time.Now().In(timeLocation), note.ModifiedAt, time.Second*61)
	assert.Equal(t, `just #testing with #fishing and not [link](#link) or href="#that"`, note.Note)
	assert.Equal(t, true, note.Archive)
	assert.Equal(t, true, note.Favorite)
	assert.EqualSlices(t, []string{"testing", "fishing"}, note.Tags)

	// Try another new post without the created_at, it should fail
	data.Del("created_at")
	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusUnprocessableEntity, response.statusCode)

	// Try another without any note content, it should fail
	data.Set("created_at", time.Now().In(timeLocation).Format("2006-01-02T15:04"))
	data.Del("note")

	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusUnprocessableEntity, response.statusCode)

	// Try a new note with more minimal data
	data.Del("title")
	data.Set("note", "This note is out of control\n\nor not")
	data.Del("favorite")
	data.Del("archive")

	// Should be okay
	response = ts.post(t, "/notes/new/", data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Check the reditect location
	nextLocation = response.header.Get("Location")
	assert.StringIn(t, "/note/n_", nextLocation)

	// Get the postID from the new post
	newPostID = strings.Split(nextLocation, "/")[2]
	assert.StringIn(t, "n_", newPostID)

	// Query the new post from the database
	note, err = queries.GetNote(context.Background(), newPostID)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the contents of the note

	assert.Equal(t, "This note is out of control", note.Title)
	assert.EqualTime(t, time.Now().In(timeLocation), note.CreatedAt, time.Second*61)
	assert.EqualTime(t, time.Now().In(timeLocation), note.ModifiedAt, time.Second*61)
	assert.Equal(t, "This note is out of control\n\nor not", note.Note)
	assert.Equal(t, false, note.Archive)
	assert.Equal(t, false, note.Favorite)
	assert.Equal(t, 0, len(note.Tags))
}

func TestEditNoteGET(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/note/n_002/edit/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/note/n_002/edit/")
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

func TestEditNotePOST(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Create a new database connection for queries
	queries := db.NewTestDatabase(t, context.Background(), os.Getenv("NOTES_TEST_DB_DSN"), false)

	data := url.Values{}

	note, err := queries.RandomNote(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("/note/%s/edit/", note.ID)

	// Test unauthorized without login
	response := ts.post(t, url, data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test bad request with login (no csrf)
	ts.login(t)
	response = ts.post(t, url, data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Get a CSRF token for testing
	response = ts.get(t, "/notes/new/")
	csrfToken := response.csrfToken(t)

	// Try a full request without the csrf token
	data.Set("title", "It's different now")
	data.Set("created_at", time.Now().In(timeLocation).Format("2006-01-02T15:04"))
	switch note.Favorite {
	case true:
		data.Set("favorite", "")
	default:
		data.Set("favorite", "true")
	}
	switch note.Archive {
	case true:
		data.Set("archive", "")
	default:
		data.Set("archive", "true")
	}
	data.Set("note", "It's not the same anymore")

	// Test bad request with csrf token
	response = ts.post(t, url, data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Add the csrf token and try again
	data.Set("csrf_token", csrfToken)

	// Test request OK with csrf token
	response = ts.post(t, url, data)
	assert.Equal(t, http.StatusSeeOther, response.statusCode)
	assert.Equal(t, fmt.Sprintf("/note/%s/", note.ID), response.header.Get("Location"))

	// Get the updated note's data
	updatedNote, err := queries.GetNote(context.Background(), note.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Validate the updated note's data
	assert.Equal(t, note.ID, updatedNote.ID)
	assert.Equal(t, "It's different now", updatedNote.Title)
	assert.Equal(t, "It's not the same anymore", updatedNote.Note)
	assert.NotEqual(t, note.Archive, updatedNote.Archive)
	assert.NotEqual(t, note.Favorite, updatedNote.Favorite)
	assert.EqualTime(t, time.Now().In(timeLocation), updatedNote.CreatedAt, time.Second*61)
	assert.EqualTime(t, time.Now().In(timeLocation), updatedNote.ModifiedAt, time.Second*61)
}

func TestTimeLocationGET(t *testing.T) {
	// Create a new test server
	ts := newTestServer(t)
	defer ts.Close()

	// Test unauthorized without login
	response := ts.get(t, "/time/")
	assert.Equal(t, http.StatusSeeOther, response.statusCode)

	// Test OK with login
	ts.login(t)
	response = ts.get(t, "/time/")
	assert.Equal(t, http.StatusOK, response.statusCode)
	assert.StringIn(t, "America/Los_Angeles", response.body)
	assert.Equal(t, "America/Los_Angeles", timeLocation.String())

	// Make up some data to change the time location
	data := url.Values{}
	data.Set("time_location", "America/New_York")

	csrfToken := response.csrfToken(t)

	// Try to update the location. Should fail without login
	response = ts.post(t, "/time/", data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Try to update the location. Should fail without csrf
	response = ts.post(t, "/time/", data)
	assert.Equal(t, http.StatusForbidden, response.statusCode)

	// Update the timezone
	data.Set("csrf_token", csrfToken)
	response = ts.post(t, "/time/", data)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Get the time page again to check the update data
	response = ts.get(t, "/time/")
	assert.StringIn(t, "America/New_York", response.body)
	assert.StringNotIn(t, "America/Los_Angeles", response.body)
	assert.Equal(t, "America/New_York", timeLocation.String())

	// change the timezone to an invalid timezone
	data.Del("time_location")
	data.Set("time_location", "America/NOOOOOOO")

	response = ts.post(t, "/time/", data)
	assert.Equal(t, http.StatusOK, response.statusCode)

	// Get the time page again to the data didn't change
	response = ts.get(t, "/time/")
	assert.StringIn(t, "America/New_York", response.body)
	assert.StringNotIn(t, "America/Los_Angeles", response.body)
	assert.Equal(t, "America/New_York", timeLocation.String())
}
