package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/justinas/nosurf"
	"github.com/sglmr/go-notes/assets"
	"github.com/sglmr/go-notes/db"
	"github.com/sglmr/go-notes/internal/argon2id"
	"github.com/sglmr/go-notes/internal/render"
	"github.com/sglmr/go-notes/internal/validator"
	"github.com/sglmr/go-notes/internal/vcs"
)

// addRoutes adds all the routes to the mux
func addRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	devMode bool,
	authEmail, passwordHash string,
	wg *sync.WaitGroup,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) {
	// Set up file server for embedded static files
	fileServer := http.FileServer(http.FS(staticFileSystem{assets.EmbeddedFiles}))
	mux.Handle("GET /static/", cacheControlMW("31536000")(fileServer))
	mux.Handle("GET /health/", health(devMode))

	// These routes are not protected
	dynamic := func(next http.Handler) http.Handler {
		return csrfMW(next)
	}
	mux.Handle("GET /login/", dynamic(login(logger, sessionManager, devMode, authEmail, passwordHash)))
	mux.Handle("POST /login/", dynamic(login(logger, sessionManager, devMode, authEmail, passwordHash)))

	// These routes are protected
	protected := func(next http.Handler) http.Handler {
		return requireLoginMW()(dynamic(next))
	}
	mux.Handle("GET /", protected(home(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /notes/list/", protected(listNotes(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /notes/search/", protected(listNotes(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /notes/refresh-tags/", protected(refreshNoteTags(logger, wg, devMode, sessionManager, queries)))
	mux.Handle("GET /note/{id}/", protected(viewNote(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /note/{id}/print/", protected(viewNote(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /notes/new/", protected(noteFormGet(logger, devMode, sessionManager, queries)))
	mux.Handle("POST /notes/new/", protected(noteFormPOST(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /note/{id}/delete/", protected(deleteNote(logger, devMode, sessionManager, queries)))
	mux.Handle("POST /note/{id}/delete/", protected(deleteNote(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /note/{id}/edit/", protected(noteFormGet(logger, devMode, sessionManager, queries)))
	mux.Handle("POST /note/{id}/edit/", protected(noteFormPOST(logger, devMode, sessionManager, queries)))
	mux.Handle("GET /time/", protected(timeZone(logger, devMode, sessionManager)))
	mux.Handle("POST /time/", protected(timeZone(logger, devMode, sessionManager)))
	mux.Handle("GET /import/", protected(importNote(queries)))
	mux.Handle("POST /import/", protected(importNote(queries)))
	mux.Handle("GET /logout/", protected(logout(logger, sessionManager, devMode)))
	mux.Handle("POST /logout/", protected(logout(logger, sessionManager, devMode)))
}

// health handles a healthcheck response "OK"
func health(devMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "status: OK")
		fmt.Fprintln(w, "devMode:", devMode)
		fmt.Fprintln(w, "ver: ", vcs.Version())
		fmt.Fprintln(w, "time location: ", timeLocation)
	}
}

// home handles the root route
func home(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Redirect non-root paths to root
		// TODO: write a test for this someday
		if r.URL.Path != "/" {
			clientError(w, http.StatusNotFound)
			return
		}

		// Query for a random note
		note, err := queries.RandomNote(r.Context())
		if errors.Is(err, pgx.ErrNoRows) {
			note = db.Note{}
		} else if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		// Set up template data
		data := newTemplateData(r, sessionManager)
		data["Note"] = note

		if err := render.Page(w, http.StatusOK, data, "home.tmpl"); err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}
	}
}

// login handles logins
func login(
	logger *slog.Logger,
	sessionManager *scs.SessionManager,
	showTrace bool,
	authEmail, passwordHash string,
) http.HandlerFunc {
	// Login form object
	type loginForm struct {
		Email    string
		Password string
		validator.Validator
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the "next" url parameter for the page to redirect to on successful login
		nextURL := r.URL.Query().Get("next")
		logger.Debug("login next", "next", nextURL)
		if len(nextURL) == 0 {
			// Set to home if there was not next url
			nextURL = "/"
		}

		// Render form for a GET request
		if r.Method == http.MethodGet {
			data := newTemplateData(r, sessionManager)
			data["Form"] = loginForm{}

			// Render the login page
			if err := render.Page(w, http.StatusOK, data, "login.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		// Parse the form data
		err := r.ParseForm()
		if err != nil {
			clientError(w, http.StatusBadRequest)
			return
		}

		// Create a form with the data
		form := loginForm{
			Email:    r.FormValue("email"),
			Password: r.FormValue("password"),
		}

		// Validate the form data
		form.Check("Email", validator.NotBlank(form.Email), "This field cannot be blank.")
		form.Check("Email", validator.MaxRunes(form.Email, 50), "This field cannot be more than 100 characters.")
		form.Check("Email", validator.IsEmail(form.Email), "Email must be a valid email.")
		form.Check("Password", validator.NotBlank(form.Password), "This field cannot be blank.")
		form.Check("Password", validator.MaxRunes(form.Password, 100), "This field cannot be more than 150 characters.")

		// Return form errors if the form is not valid
		if form.HasErrors() {
			putFlashMessage(r, flashError, "please correct the form errors", sessionManager)
			data := newTemplateData(r, sessionManager)
			data["Form"] = form

			// Render the login page
			if err := render.Page(w, http.StatusUnprocessableEntity, data, "login.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		// Check if the email matches and if not, send back to the login page
		if subtle.ConstantTimeCompare([]byte(authEmail), []byte(form.Email)) == 0 {
			putFlashMessage(r, flashError, "Email or password is incorrect", sessionManager)

			data := newTemplateData(r, sessionManager)
			data["Form"] = form

			// re-render the login page
			if err := render.Page(w, http.StatusUnprocessableEntity, data, "login.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		// Check whether the hashed pasword for the user and the plain text password provided match
		match, err := argon2id.ComparePasswordAndHash(form.Password, passwordHash)
		switch {
		case err != nil:
			serverError(w, r, err, logger, showTrace)
			return
		case !match:
			putFlashMessage(r, flashError, "Email or password is incorrect", sessionManager)

			data := newTemplateData(r, sessionManager)
			data["Form"] = form

			// re-render the login page
			if err := render.Page(w, http.StatusUnprocessableEntity, data, "login.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		// Renew token after login to change the session ID
		err = sessionManager.RenewToken(r.Context())
		if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		// Set the authenticated session key
		sessionManager.Put(r.Context(), "authenticated", true)
		putFlashMessage(r, flashSuccess, "You are in!", sessionManager)

		// Redirect to the next page.
		http.Redirect(w, r, nextURL, http.StatusSeeOther)
	}
}

// logout handles logging out
func logout(
	logger *slog.Logger,
	sessionManager *scs.SessionManager,
	showTrace bool,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the "next" url parameter for the page to redirect to on successful login
		nextURL := r.URL.Query().Get("next")
		logger.Debug("login next", "next", nextURL)
		if len(nextURL) == 0 {
			// Set to home if there was not next url
			nextURL = "/"
		}

		// Render form for a GET request
		if r.Method == http.MethodGet {
			data := newTemplateData(r, sessionManager)

			// Render the login page
			if err := render.Page(w, http.StatusOK, data, "logout.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		// Renew token after login to change the session ID
		err := sessionManager.RenewToken(r.Context())
		if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		// Remove the authenticated session key
		sessionManager.Remove(r.Context(), "authenticated")
		putFlashMessage(r, flashSuccess, "You've been logged out!", sessionManager)

		// Redirect to the next page.
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// listNotes displays a list of all the notes
func listNotes(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := db.SearchNotesParams{
			Query:     r.URL.Query().Get("q"),
			Tags:      []string{r.URL.Query().Get("tag")},
			Archived:  len(r.URL.Query().Get("archived")) > 0,
			Favorites: len(r.URL.Query().Get("favorites")) > 0,
		}

		logger.Debug("notes params", "urlPath", r.URL.Path, "params", params)

		// Query the database for the notes
		notes, err := queries.SearchNotes(r.Context(), params)
		if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		// Query for a list of tags
		tagList, err := queries.GetTagsWithCounts(r.Context())
		if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		logger.Debug("query counts", "notes", len(notes), "tags", len(tagList))

		// Prepare template data
		data := newTemplateData(r, sessionManager)
		data["Q"] = r.URL.Query().Get("q")
		data["Tag"] = r.URL.Query().Get("tag")
		data["Favorites"] = params.Favorites
		data["Archived"] = params.Archived
		data["Notes"] = notes
		data["TagList"] = tagList

		// Render the page
		if err := render.Page(w, http.StatusOK, data, "listNotes.tmpl"); err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}
	}
}

// refreshNoteTags refreshes all the note tags on a get request
func refreshNoteTags(
	logger *slog.Logger,
	wg *sync.WaitGroup,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Update the note tags in the background
		backgroundTask(
			wg, logger,
			func() error {
				ctx, ctxCancel := context.WithTimeout(context.Background(), time.Minute*2)
				defer ctxCancel()

				// Get a list of all the notes
				notes, err := queries.ListAllNotes(ctx)
				if err != nil {
					return err
				}

				// update the tag for each note
				for _, note := range notes {
					params := db.UpdateNoteTagsParams{
						ID:   note.ID,
						Tags: extractTags(note.Note),
					}
					_, err := queries.UpdateNoteTags(ctx, params)
					if err != nil {
						logger.Error("update note tags error", "note", note.Title, "note_id", note.ID, "error", err)
					} else {
						logger.Debug("updated note tags", "note", note.Title, "note_id", note.ID)
					}
				}
				return nil
			})

		putFlashMessage(r, flashSuccess, "Queued background task to update notes.", sessionManager)

		http.Redirect(w, r, "/notes/list/", http.StatusSeeOther)
	}
}

// viewNote displays a single note
func viewNote(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new template data file
		data := newTemplateData(r, sessionManager)

		// Check if there is an id value for the note
		id := r.PathValue("id")

		// Query for a single note
		note, err := queries.GetNote(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			clientError(w, http.StatusNotFound)
			return
		} else if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		// Add the note data to the template data map
		data["Note"] = note

		// Choose print vs regular view
		switch {
		// Render the page without the base template
		case strings.HasSuffix(r.URL.Path, "/print/"):
			if err := render.NamedTemplate(w, http.StatusOK, data, "base", "printBase.tmpl", "pages/viewNote.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
		default:
			// Render the page
			if err := render.Page(w, http.StatusOK, data, "viewNote.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}

		}
	}
}

// deleteNote deletes a note
func deleteNote(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if there is an id value for the note
		id := r.PathValue("id")

		// Query for a single note
		note, err := queries.GetNote(r.Context(), id)
		if errors.Is(err, pgx.ErrNoRows) {
			clientError(w, http.StatusNotFound)
			return
		} else if err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// Create a new template data file
			data := newTemplateData(r, sessionManager)
			data["Note"] = note

			// Render the page
			if err := render.Page(w, http.StatusOK, data, "deleteNote.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		case http.MethodPost:
			err := queries.DeleteNote(r.Context(), id)
			if err != nil {
				serverError(w, r, err, logger, showTrace)
			}

			http.Redirect(w, r, "/notes/list/", http.StatusSeeOther)
			return

		default:
			clientError(w, http.StatusNotFound)
			return
		}
	}
}

// noteFormGet displays an editor for creating or updating notes
func noteFormGet(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	type noteForm struct {
		Title     string
		Note      string
		Favorite  bool
		Archive   bool
		CreatedAt time.Time
		validator.Validator
	}
	return func(w http.ResponseWriter, r *http.Request) {
		data := newTemplateData(r, sessionManager)
		form := noteForm{
			Title:     "",
			Note:      "",
			Favorite:  false,
			Archive:   false,
			CreatedAt: time.Now().In(timeLocation),
		}

		// Check if there is an id value in the url path
		id := r.PathValue("id")

		// Query for a single note if there is an id
		if id != "" {
			note, err := queries.GetNote(r.Context(), id)
			if errors.Is(err, pgx.ErrNoRows) {
				clientError(w, http.StatusNotFound)
				return
			} else if err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}

			data["Note"] = note

			// Fill in the form with the Note data
			form = noteForm{
				Title:     note.Title,
				Note:      note.Note,
				Favorite:  note.Favorite,
				Archive:   note.Archive,
				CreatedAt: note.CreatedAt,
			}
		}

		// Populate the Form Data
		data["Form"] = form

		// Render the page
		if err := render.Page(w, http.StatusOK, data, "noteForm.tmpl"); err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}
	}
}

// importNote handles POST requests to insert a note
func importNote(
	queries *db.Queries,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// Set content type to plain text
			w.Header().Set("Content-Type", "text/plain")

			// Write a text response
			fmt.Fprint(w, nosurf.Token(r))
			return
		}

		// Return Bad Request if the form data is not parseable
		err := r.ParseForm()
		if err != nil {
			clientError(w, http.StatusBadRequest)
			return
		}

		noteID := r.FormValue("note_id")
		if noteID == "" {
			fmt.Fprintln(w, "missing note_id")
			clientError(w, http.StatusUnprocessableEntity)
			return
		}
		title := r.FormValue("title")
		if title == "" {
			fmt.Fprintln(w, "missing title")
			clientError(w, http.StatusUnprocessableEntity)
			return
		}
		note := r.FormValue("note")
		if note == "" {
			fmt.Fprintln(w, "missing note")
			clientError(w, http.StatusUnprocessableEntity)
			return
		}
		archive := len(r.FormValue("archive")) > 0
		favorite := len(r.FormValue("favorite")) > 0

		// Convert the value to time.Time
		createdAt, err := time.ParseInLocation(time.RFC3339, r.FormValue("created_at"), timeLocation)
		if err != nil || createdAt.IsZero() {
			fmt.Fprintln(w, "missing or invalid created_at")
			clientError(w, http.StatusUnprocessableEntity)
			return
		}
		// Convert the value to time.Time
		modifiedAt, err := time.ParseInLocation(time.RFC3339, r.FormValue("modified_at"), timeLocation)
		if err != nil || modifiedAt.IsZero() {
			fmt.Fprintln(w, "missing or invalid modified_at")
			clientError(w, http.StatusUnprocessableEntity)
			return
		}

		params := db.ImportNoteParams{
			ID:         noteID,
			Title:      title,
			Note:       note,
			Archive:    archive,
			Favorite:   favorite,
			CreatedAt:  createdAt,
			ModifiedAt: modifiedAt,
			Tags:       extractTags(note),
		}

		n, err := queries.ImportNote(r.Context(), params)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "error importing %s: %s", noteID, err.Error())
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "created note: %v", n.ID)
	}
}

// noteFormPost handles POST requests to update or create notes
func noteFormPOST(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.HandlerFunc {
	type noteForm struct {
		Title     string
		Note      string
		Favorite  bool
		Archive   bool
		CreatedAt time.Time
		validator.Validator
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		var note db.Note

		// Return Bad Request if the form data is not parseable
		if err = r.ParseForm(); err != nil {
			clientError(w, http.StatusBadRequest)
			return
		}

		// Check if there is an id value in the url path
		id := r.PathValue("id")

		if len(id) > 0 {
			// Query for a single note if there is an id
			_, err := queries.GetNote(r.Context(), id)
			if errors.Is(err, pgx.ErrNoRows) {
				clientError(w, http.StatusNotFound)
				return
			} else if err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
		}

		// Parse out the note form data
		form := noteForm{}
		form.Title = r.FormValue("title")
		form.Archive = r.FormValue("archive") != ""
		form.Favorite = r.FormValue("favorite") != ""
		form.Note = r.FormValue("note")

		// Convert the value to time.Time
		form.CreatedAt, err = time.ParseInLocation("2006-01-02T15:04", r.FormValue("created_at"), timeLocation)
		if err != nil {
			form.AddError("CreatedAt", "invalid date time")
			form.CreatedAt = time.Now().In(timeLocation)
		}

		// If title is blank, use the first line of the note content
		if form.Title == "" {
			before, _, found := strings.Cut(form.Note, "\n")
			if found {
				form.Title = strings.TrimSpace(before)
			}

		}

		// Validate the form fields
		form.Check("Title", validator.NotBlank(form.Title), "title is required")
		form.Check("Note", validator.NotBlank(form.Note), "note content is required")
		form.Check("CreatedAt", !form.CreatedAt.IsZero(), "must be a valid date time")

		// Return the form data and re-render the form page if there are any errors
		if form.HasErrors() {
			// Create a new template data for a future response
			data := newTemplateData(r, sessionManager)
			data["Form"] = form
			if err := render.Page(w, http.StatusUnprocessableEntity, data, "noteForm.tmpl"); err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			return
		}

		switch {
		case len(id) > 0:
			// Update an existing Note
			params := db.UpdateNoteParams{
				ID:        id,
				Title:     form.Title,
				Note:      form.Note,
				Archive:   form.Archive,
				Favorite:  form.Favorite,
				CreatedAt: form.CreatedAt,
				Tags:      extractTags(form.Note),
			}
			logger.Debug("updating a note", "params", params)
			note, err = queries.UpdateNote(r.Context(), params)
			if err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}

		default:
			// Create a new note

			// Create an ID for the note
			id, err = db.GenerateID("n")
			if err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
			// Create a new note
			params := db.CreateNoteParams{
				ID:        id,
				Title:     form.Title,
				Note:      form.Note,
				Favorite:  form.Favorite,
				CreatedAt: form.CreatedAt,
				Archive:   form.Archive,
				Tags:      extractTags(form.Note),
			}
			logger.Debug("creating a note", "params", params)
			note, err = queries.CreateNote(r.Context(), params)
			if err != nil {
				serverError(w, r, err, logger, showTrace)
				return
			}
		}

		// Note created or updated successfully, redirect to view the note
		url := fmt.Sprintf("/note/%v/", note.ID)
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

// timeZone allows for changing the global time. This information probably needs to be on
// a user profile and in the session at some point in the future.
func timeZone(logger *slog.Logger, showTrace bool, sessionManager *scs.SessionManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		if r.Method == http.MethodPost {
			// Parse the form, bad request if there is an error
			if err := r.ParseForm(); err != nil {
				clientError(w, http.StatusBadRequest)
				return
			}

			// New Time Location
			currentLocation := timeLocation.String()
			newLocation := r.FormValue("time_location")

			// Load the time location
			timeLocation, err = time.LoadLocation(newLocation)
			if err != nil {
				putFlashMessage(r, flashError, err.Error(), sessionManager)
				// reload the previous location
				timeLocation, _ = time.LoadLocation(currentLocation)
			}
		}

		// Create a new template data file
		data := newTemplateData(r, sessionManager)
		data["CurrentTimeLocation"] = timeLocation.String()
		data["CurrentTime"] = time.Now().In(timeLocation)

		// Render the page
		if err := render.Page(w, http.StatusOK, data, "timeLocation.tmpl"); err != nil {
			serverError(w, r, err, logger, showTrace)
			return
		}
	}
}
