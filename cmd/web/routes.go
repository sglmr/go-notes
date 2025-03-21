package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5"
	"github.com/justinas/nosurf"
	"github.com/sglmr/gowebstart/assets"
	"github.com/sglmr/gowebstart/db"
	"github.com/sglmr/gowebstart/internal/email"
	"github.com/sglmr/gowebstart/internal/render"
	"github.com/sglmr/gowebstart/internal/validator"
	"github.com/sglmr/gowebstart/internal/vcs"
)

// AddRoutes adds all the routes to the mux
func AddRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	devMode bool,
	mailer email.MailerInterface,
	username, passwordHash string,
	wg *sync.WaitGroup,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.Handler {
	// Set up file server for embedded static files
	// fileserver := http.FileServer(http.FS(assets.EmbeddedFiles))
	fileServer := http.FileServer(http.FS(staticFileSystem{assets.EmbeddedFiles}))
	mux.Handle("GET /static/", CacheControlMW("31536000")(fileServer))

	mux.Handle("GET /", home(logger, devMode, sessionManager))
	mux.Handle("GET /list/", listNotes(logger, devMode, sessionManager, queries))
	mux.Handle("GET /note/{id}/", viewNote(logger, devMode, sessionManager, queries))
	mux.Handle("GET /new/", noteFormGet(logger, devMode, sessionManager, queries))
	mux.Handle("GET /note/{id}/edit/", noteFormGet(logger, devMode, sessionManager, queries))

	mux.Handle("POST /new/", noteFormPOST(logger, devMode, sessionManager, queries))
	mux.Handle("POST /note/{id}/edit/", noteFormPOST(logger, devMode, sessionManager, queries))

	mux.Handle("GET /health/", health(devMode))

	// TODO: Remove these
	mux.Handle("GET /import/", importNote(queries))
	mux.Handle("POST /import/", importNote(queries))

	handler := RecoverPanicMW(mux, logger, devMode)
	if os.Getenv("DOKKU_APP_NAME") != "" {
		handler = SecureHeadersMW(handler)
	}
	handler = LogRequestMW(logger)(handler)
	handler = CsrfMW(handler)
	handler = sessionManager.LoadAndSave(handler)

	// Use Basic auth for everything
	return BasicAuthMW(username, passwordHash, logger)(handler)
}

// health handles a healthcheck response "OK"
func health(devMode bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "status: OK")
		fmt.Fprintln(w, "ver: ", vcs.Version())
		fmt.Fprintln(w, "devMode: ", devMode)
		fmt.Fprintln(w, "app name: ", os.Getenv("DOKKU_APP_NAME"))
	}
}

// home handles the root route
func home(
	logger *slog.Logger,
	showTrace bool,
	sessionManager *scs.SessionManager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Redirect non-root paths to root
		// TODO: write a test for this someday
		if r.URL.Path != "/" {
			NotFound(w, r)
			return
		}
		putFlashMessage(r, LevelSuccess, "Welcome!", sessionManager)
		putFlashMessage(r, LevelSuccess, "You made it!", sessionManager)

		data := newTemplateData(r, sessionManager)

		if err := render.Page(w, http.StatusOK, data, "home.tmpl"); err != nil {
			ServerError(w, r, err, logger, showTrace)
			return
		}
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
		// Create a new template data file
		data := newTemplateData(r, sessionManager)

		// Check if there is a search query parameter
		q := r.URL.Query().Get("q")
		tag := r.URL.Query().Get("tag")
		favorites := len(r.URL.Query().Get("favorites")) > 0
		archived := len(r.URL.Query().Get("archived")) > 0
		data["Search"] = map[string]any{
			"Q":         q,
			"Tag":       tag,
			"Favorites": favorites,
			"Archived":  archived,
		}
		logger.Debug("list notes search", "q", q, "tag", tag, "favorites", favorites, "archived", archived)

		var notes []db.Note
		if len(q) == 0 && len(tag) == 0 && !favorites && !archived {
			// List of all notes
			n, err := queries.ListNotes(r.Context())
			if err != nil {
				ServerError(w, r, err, logger, showTrace)
				return
			}
			notes = n
		} else {
			// Search for notes
			params := db.SearchNotesParams{
				Query:     q,
				Tags:      []string{tag},
				Archived:  archived,
				Favorites: favorites,
			}
			logger.Debug("tag search params", "params", params)
			n, err := queries.SearchNotes(r.Context(), params)
			if err != nil {
				ServerError(w, r, err, logger, showTrace)
				return
			}
			notes = n
		}

		logger.Debug("list notes", "count", len(notes))

		// Add the notes data to the template data map
		data["Notes"] = notes

		// Query for a list of tags
		tagList, err := queries.GetTagsWithCounts(r.Context())
		if err != nil {
			ServerError(w, r, err, logger, showTrace)
			return
		}
		data["TagList"] = tagList

		// Render the page
		if err := render.Page(w, http.StatusOK, data, "listNotes.tmpl"); err != nil {
			ServerError(w, r, err, logger, showTrace)
			return
		}
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
			NotFound(w, r)
			return
		} else if err != nil {
			ServerError(w, r, err, logger, showTrace)
			return
		}

		// Add the note data to the template data map
		data["Note"] = note

		// Render the page
		if err := render.Page(w, http.StatusOK, data, "viewNote.tmpl"); err != nil {
			ServerError(w, r, err, logger, showTrace)
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
			CreatedAt: time.Now(),
		}

		// Check if there is an id value in the url path
		id := r.PathValue("id")

		// Query for a single note if there is an id
		if id != "" {
			note, err := queries.GetNote(r.Context(), id)
			if errors.Is(err, pgx.ErrNoRows) {
				NotFound(w, r)
				return
			} else if err != nil {
				ServerError(w, r, err, logger, showTrace)
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
			ServerError(w, r, err, logger, showTrace)
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
			BadRequest(w, r, fmt.Errorf("parse import post data: %w", err))
			return
		}

		noteID := r.FormValue("note_id")
		if noteID == "" {
			BadRequest(w, r, errors.New("missing note_id"))
			return
		}
		title := r.FormValue("title")
		if title == "" {
			BadRequest(w, r, errors.New("missing title"))
			return
		}
		note := r.FormValue("note")
		if note == "" {
			BadRequest(w, r, errors.New("missing note content"))
			return
		}
		archive := len(r.FormValue("archive")) > 0
		favorite := len(r.FormValue("favorite")) > 0

		// Convert the value to time.Time
		createdAt, err := time.Parse("2006-01-02T15:04", r.FormValue("created_at"))
		if err != nil {
			BadRequest(w, r, err)
			return
		}
		// Convert the value to time.Time
		modifiedAt, err := time.Parse("2006-01-02T15:04", r.FormValue("modified_at"))
		if err != nil {
			BadRequest(w, r, err)
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
			BadRequest(w, r, err)
			return
		}

		// Create a new template data for a future response
		data := newTemplateData(r, sessionManager)

		// Check if there is an id value in the url path
		id := r.PathValue("id")

		if len(id) > 0 {
			// Query for a single note if there is an id

			_, err := queries.GetNote(r.Context(), id)
			if errors.Is(err, pgx.ErrNoRows) {
				NotFound(w, r)
				return
			} else if err != nil {
				ServerError(w, r, err, logger, showTrace)
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
		form.CreatedAt, err = time.Parse("2006-01-02T15:04", r.FormValue("created_at"))
		if err != nil {
			form.AddError("CreatedAt", "invalid date time")
			form.CreatedAt = time.Now()
		}

		// If title is blank, use the first line of the note content
		if form.Title == "" {
			t := strings.SplitN(form.Note, "\n", 1)[0]
			form.Title = strings.TrimSpace(t)
		}

		// Validate the form fields
		form.Check(validator.NotBlank(form.Title), "Title", "title is required")
		form.Check(validator.NotBlank(form.Note), "Note", "note content is required")
		form.Check(!form.CreatedAt.IsZero(), "CreatedAt", "must be a valid date time")

		// Return the form data and re-render the form page if there are any errors
		if form.HasErrors() {
			data["Form"] = form
			if err := render.Page(w, http.StatusBadRequest, data, "noteForm.tmpl"); err != nil {
				ServerError(w, r, err, logger, showTrace)
				return
			}
		}

		if len(id) > 0 {
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
				ServerError(w, r, err, logger, showTrace)
				return
			}
		} else {
			// Create an ID for the note
			id, err = db.GenerateID("n")
			if err != nil {
				ServerError(w, r, err, logger, showTrace)
				return
			}
			// Create a new note
			params := db.CreateNoteParams{
				ID:        id,
				Title:     form.Title,
				Note:      form.Note,
				Favorite:  form.Favorite,
				CreatedAt: form.CreatedAt,
				Tags:      extractTags(form.Note),
			}
			logger.Debug("creating a note", "params", params)
			note, err = queries.CreateNote(r.Context(), params)
			if err != nil {
				ServerError(w, r, err, logger, showTrace)
				return
			}
		}

		// Note created or updated successfully, redirect to view the note
		url := fmt.Sprintf("/note/%v/", note.ID)
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}
