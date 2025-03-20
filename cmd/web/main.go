package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/sglmr/gowebstart/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/sglmr/gowebstart/assets"
	"github.com/sglmr/gowebstart/internal/argon2id"
	"github.com/sglmr/gowebstart/internal/email"
	"github.com/sglmr/gowebstart/internal/render"
	"github.com/sglmr/gowebstart/internal/vcs"
	"golang.org/x/exp/constraints"
)

//=============================================================================
// Top level application functions
//=============================================================================

func main() {
	// Get the background context to pass through the application
	ctx := context.Background()

	// Run the application
	if err := RunApp(ctx, os.Stdout, os.Args, os.Getenv); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
		return
	}
}

// NewServer is a constructor that takes in all dependencies as arguments
func NewServer(
	logger *slog.Logger,
	useAuth bool,
	devMode bool,
	mailer email.MailerInterface,
	username, passwordHash string,
	wg *sync.WaitGroup,
	sessionManager *scs.SessionManager,
	queries *db.Queries,
) http.Handler {
	// Create a serve mux
	logger.Debug("creating server")
	mux := http.NewServeMux()

	// Register the home handler for the root route
	httpHandler := AddRoutes(mux, logger, useAuth, devMode, mailer, username, passwordHash, wg, sessionManager, queries)

	return httpHandler
}

func RunApp(
	ctx context.Context,
	w io.Writer,
	args []string,
	getenv func(string) string,
) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Create a waitgroup with 1 item for handling shutdown
	wg := sync.WaitGroup{}
	wg.Add(1)

	// New Flag set
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)

	host := fs.String("host", "0.0.0.0", "Server host")
	port := fs.String("port", "", "Server port")
	devMode := fs.Bool("dev", false, "Development mode. Displays stack trace & more verbose logging")
	username := fs.String("username", os.Getenv("BASIC_AUTH_USERNAME"), "Username basic auth")
	passwordHash := fs.String("password-hash", os.Getenv("BASIC_AUTH_PASSWORD"), "Password for basic auth ('password' by default)")
	pgdsn := fs.String("db-dsn", os.Getenv("NOTES_DB_DSN"), "PostgreSQL DSN")
	migrate := fs.Bool("automigrate", true, "Automatically perform up migrations on startup")
	_ = fs.String("smtp-host", "", "Email smtp host")
	_ = fs.Int("smtp-port", 25, "Email smtp port")
	_ = fs.String("smtp-username", "", "Email smtp username")
	_ = fs.String("smtp-password", "", "Email smtp password")
	_ = fs.String("smtp-from", "Eample Name <no-reply@example.com>", "Email smtp Sender")

	// Parse the flags
	err := fs.Parse(args[1:])
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	// Connect to the PostgreSQL database
	dbpool, err := pgxpool.New(ctx, *pgdsn)
	if err != nil {
		return fmt.Errorf("connecting to postgres: %w", err)
	}
	defer dbpool.Close()

	// Ping the database, timeout after 5 seconds
	pingCtx, pingCancel := context.WithTimeout(ctx, time.Second*5)
	defer pingCancel()

	if err = dbpool.Ping(pingCtx); err != nil {
		return fmt.Errorf("pinging postgres: %w", err)
	}

	// Create a new database queries object
	queries := db.New(dbpool)

	// Perform up migrations
	if *migrate {
		err = db.MigrateUp(dbpool)
		if err != nil {
			return fmt.Errorf("migrate up failed: %w", err)
		}
	}

	// Get port from environment
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *port == "" {
		*port = "8000"
	}

	// Create a new logger
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Check username and password hash
	useAuth := !*devMode
	// Check username
	if useAuth && len(*username) < 5 {
		// validate username is there
		return errors.New("missing basic auth username")
	}
	// Check passwordHash works
	if useAuth {
		_, _, _, err := argon2id.DecodeHash(*passwordHash)
		if err != nil {
			return fmt.Errorf("invalid argon2id decode: %w", err)
		}
	}

	// Create a mailer for sending emails
	var mailer email.MailerInterface
	switch {
	case *devMode:
		// Change log level to debug
		logLevel.Set(slog.LevelDebug)

		// Configure email to send to log
		mailer = email.NewLogMailer(logger)
	default:
		logLevel.Set(slog.LevelInfo)
		mailer = email.NewLogMailer(logger)
		// TODO: Configure a mailer to send real emails
		// mailer, err = email.NewMailer(*smtpHost, *smtpPort, *smtpUsername, *smtpPassword, *smtpFrom)
		// if err != nil {
		// logger.Error("smtp configuration error", "error", err)
		// return fmt.Errorf("smtp mailer setup failed: %w", err)
		// }
	}

	// Session manager configuration
	sessionManager := scs.New()
	sessionManager.Lifetime = 24 * time.Hour

	// Set up router
	srv := NewServer(logger, useAuth, *devMode, mailer, *username, *passwordHash, &wg, sessionManager, queries)

	// Configure an http server
	httpServer := &http.Server{
		Addr:         net.JoinHostPort(*host, *port),
		Handler:      srv,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// This pattern is starts a server background while the main program continues with other tasks.
	// The main program can later stop the server using httpServer.Shutdown().
	go func() {
		logger.Info("application running (press ctrl+C to quit)", "address", fmt.Sprintf("http://%s", httpServer.Addr))

		// httpServer.ListenAndServe() begins listening for HTTP requests
		// This method blocks (runs forever) until the server is shut down
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Print an error if any error other than http.ErrServerclosed shows up
			logger.Error("listen and serve error", "error", err)
			// Send SIGTERM to self to shutdown the application
			p, _ := os.FindProcess(os.Getpid())
			p.Signal(syscall.SIGTERM)
		}
	}()

	// Start a goroutine to handle server shutdown
	go func() {
		// The waitgroup counter will decrement and signal complete at
		// the end of this function
		defer wg.Done()

		// This blocks the goroutine until the ctx context is cancelled
		<-ctx.Done()
		logger.Info("waiting for application to shutdown")

		// Create an empty context for the shutdown process with a 10 second timer
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Initiate a graceful shutdown of the server and handle any errors
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("error shutting down http server: %s\n", "error", err)
		}
	}()
	// Makes the goroutine wait until shutdown starts
	wg.Wait()
	logger.Info("application shutdown complete")
	return nil
}

// BackgroundTask executes a function in a background goroutine with proper error handling.
func BackgroundTask(wg *sync.WaitGroup, logger *slog.Logger, fn func() error) {
	// Increment waitgroup to track whether this background task is complete or not
	wg.Add(1)

	// Launch a goroutine to run the task in
	go func() {
		// decrement the waitgroup after the task completes
		defer wg.Done()

		// Get the name of the function
		funcName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

		// Recover any panics in the task function so that
		// a panic doesn't kill the whole application
		defer func() {
			err := recover()
			if err != nil {
				logger.Error("task", "name", funcName, "error", fmt.Errorf("%s", err))
			}
		}()

		// Execute the provided function, logging any errors
		err := fn()
		if err != nil {
			logger.Error("task", "name", funcName, "error", err)
		}
	}()
}

//=============================================================================
// Helper functions
//=============================================================================

// AddRoutes adds all the routes to the mux
func AddRoutes(
	mux *http.ServeMux,
	logger *slog.Logger,
	useAuth bool,
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

	mux.Handle("GET /health/", health())

	// Add recoverPanic middleware
	handler := RecoverPanicMW(mux, logger, devMode)
	// handler = SecureHeadersMW(handler)
	handler = LogRequestMW(logger)(handler)
	handler = sessionManager.LoadAndSave(handler)

	// Wrap everything in basic auth middleware if the useAuth flag is set
	if useAuth {
		// handler = BasicAuthMW(username, passwordHash, logger)(handler)
	}

	// Return the handler
	return handler
}

// ServerError handles server error http responses.
func ServerError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger, showTrace bool) {
	// TODO: find some way of reporting the server error
	// app.reportServerError(r, err)

	message := "The server encountered a problem and could not process your request"

	// Display the stack trace on the web page if env is development is on
	if showTrace {
		body := fmt.Sprintf("%s\n\n%s", err, string(debug.Stack()))
		http.Error(w, body, http.StatusInternalServerError)
		return
	}
	logger.Error("server error", "status", http.StatusInternalServerError, "error", err)

	http.Error(w, message, http.StatusInternalServerError)
}

// NotFound handles not found http responses.
func NotFound(w http.ResponseWriter, r *http.Request) {
	message := "The requested resource could not be found"
	http.Error(w, message, http.StatusNotFound)
}

// BadRequest hadles bad request http responses.
func BadRequest(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

//=============================================================================
// Routes/Views/HTTP handlers
//=============================================================================

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
		data["Search"] = map[string]string{
			"Q":   q,
			"Tag": tag,
		}
		logger.Debug("list notes search", "q", q, "tag", tag)

		var notes []db.Note
		if len(q) == 0 && len(tag) == 0 {
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
				Query: q,
				Tags:  []string{tag},
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
		Validator
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
		Validator
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
		form.Check(NotBlank(form.Title), "Title", "title is required")
		form.Check(NotBlank(form.Note), "Note", "note content is required")
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

// health handles a healthcheck response "OK"
func health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "status: OK")
		fmt.Fprintln(w, "ver: ", vcs.Version())
	}
}

// protected handles a page protected by basic authentication.
func protected() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "You're visiting a protected page!")
	}
}

//=============================================================================
// Helpers
//=============================================================================

// extractTags extracts hashtags from the input text and returns them as a slice of strings
// The hashtags are extracted without the # symbol.
func extractTags(text string) []string {
	// Compile the regular expression
	re := regexp.MustCompile(`(\s|.)#([-a-z0-9]*[a-z0-9])`)

	// Find all matches of the regex in the input text
	// The second argument -1 means return all matches
	matches := re.FindAllStringSubmatch(text, -1)

	// Initialize the results slice
	result := []string{}

	// Extract the capture group (the hashtag without #) from each match
	for _, match := range matches {
		switch {
		case len(match) == 0:
			continue
		case match[1] == "(":
			// Exclude links to ids in markdown
			// ex: [link](#heading-link)
			continue
		case match[1] == `"`:
			// Exclude links to ids in a href tags
			// ex: <a href="#heading-link">link</a>
			continue
		default:
			result = append(result, match[2])
		}
	}

	return result
}

// newTemplateData constructs a map of data to pass into templates
func newTemplateData(r *http.Request, sessionManager *scs.SessionManager) map[string]any {
	messages, ok := sessionManager.Pop(r.Context(), "messages").([]FlashMessage)
	if !ok {
		messages = []FlashMessage{}
	}

	return map[string]any{
		"CSRFToken": nosurf.Token(r),
		"Messages":  messages,
		"Version":   vcs.Version(),
	}
}

//=============================================================================
// Middleware functions
//=============================================================================

// staticFileSystem is a custom type that embeds the standard http.FileSystem for serving static files
type staticFileSystem struct {
	fs fs.FS
}

// Open is a method on the staticFileSystem to only serve files in the
// static embedded file folder without directory listings
func (sfs staticFileSystem) Open(path string) (fs.File, error) {
	// If the file isn't in the /static directory, don't return it
	if !strings.HasPrefix(path, "static") {
		return nil, fs.ErrNotExist
	}

	// Try to open the file
	f, err := sfs.fs.Open(path)
	if err != nil {
		return nil, err
	}

	// os.Stat to determine if the path is a file or directory
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}

	// If the file is a directory, check for an index.html file
	if s.IsDir() {
		index := filepath.Join(path, "index.html")
		if _, err := sfs.fs.Open(index); err != nil {
			closeErr := f.Close()
			if closeErr != nil {
				return nil, closeErr
			}
			return nil, err
		}
	}

	return f, nil
}

// CacheControlMW sets the Cache-Control header
func CacheControlMW(age string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%s", age))
			next.ServeHTTP(w, r)
		})
	}
}

// RecoverPanicMW recovers from panics to avoid crashing the whole server
func RecoverPanicMW(next http.Handler, logger *slog.Logger, showTrace bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			err := recover()
			if err != nil {
				ServerError(w, r, fmt.Errorf("%s", err), logger, showTrace)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// SecureHeadersMW sets security headers for the whole application
func SecureHeadersMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "origin-when-cross-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "deny")
		w.Header().Set("X-XSS-Protection", "0")

		next.ServeHTTP(w, r)
	})
}

// LogRequestMW logs the http request
func LogRequestMW(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var (
				ip     = r.RemoteAddr
				proto  = r.Proto
				method = r.Method
				uri    = r.URL.RequestURI()
			)
			logger.Info("request", "ip", ip, "proto", proto, "method", method, "uri", uri)
			next.ServeHTTP(w, r)
		})
	}
}

// CsrfMW protects specific routes against CSRF.
func CsrfMW(next http.Handler) http.Handler {
	csrfHandler := nosurf.New(next)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
	})
	return csrfHandler
}

// BasicAuthMW restricts routes for basic authentication
func BasicAuthMW(username, passwordHash string, logger *slog.Logger) func(http.Handler) http.Handler {
	authError := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)

		message := "You must be authenticated to access this resource"
		http.Error(w, message, http.StatusUnauthorized)
	})

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get basic auth credentials from the request
			requestUsername, requestPassword, ok := r.BasicAuth()
			if !ok {
				authError(w, r)
				return
			}

			// Check if the username matches the request
			if username != requestUsername {
				authError(w, r)
				return
			}

			match, err := argon2id.ComparePasswordAndHash(requestPassword, passwordHash)
			if err != nil {
				logger.Error("ComparePasswordAndHash error", "error", err)
				authError(w, r)
				return
			} else if !match {
				authError(w, r)
				return
			}
			// Serve the next http request
			next.ServeHTTP(w, r)
		})
	}
}

//=============================================================================
// Validator (validation) functions
//=============================================================================

// Validator is a type with helper functions for Validation
type Validator struct {
	Errors map[string]string
}

// Valid returns 'true' when there are no errors in the map
func (v Validator) Valid() bool {
	return !v.HasErrors()
}

// HasErrors returns 'true' when there are errors in the map
func (v Validator) HasErrors() bool {
	return len(v.Errors) != 0
}

// AddError adds a message for a given key to the map of errors.
func (v *Validator) AddError(key, message string) {
	if v.Errors == nil {
		v.Errors = map[string]string{}
	}

	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// Check will add an error message to the specified key if ok is 'false'.
func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// -------------- Validation checks functions --------------------

var RgxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// NotBlank returns true when a string is not empty.
func NotBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

// MinRunes returns true when the string is longer than n runes.
func MinRunes(value string, n int) bool {
	return utf8.RuneCountInString(value) >= n
}

// MaxRunes returns true when the string is <= n runes.
func MaxRunes(value string, n int) bool {
	return utf8.RuneCountInString(value) <= n
}

// Between returns true when the value is between (inclusive) two values.
func Between[T constraints.Ordered](value, min, max T) bool {
	return value >= min && value <= max
}

// Matches returns true when the string matches a given regular expression.
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// In returns true when a value is in the safe list of values.
func In[T comparable](value T, safelist ...T) bool {
	for i := range safelist {
		if value == safelist[i] {
			return true
		}
	}
	return false
}

// AllIn returns true if all the values are in the safelist of values.
func AllIn[T comparable](values []T, safelist ...T) bool {
	for i := range values {
		if !In(values[i], safelist...) {
			return false
		}
	}
	return true
}

// NotIn returns true when the value is not in the blocklist of values.
func NotIn[T comparable](value T, blocklist ...T) bool {
	for i := range blocklist {
		if value == blocklist[i] {
			return false
		}
	}
	return true
}

// NoDuplicates returns true when there are no duplicates in the values
func NoDuplicates[T comparable](values []T) bool {
	uniqueValues := make(map[T]bool)

	for _, value := range values {
		uniqueValues[value] = true
	}

	return len(values) == len(uniqueValues)
}

// IsEmail returns true when the string value passes an email regular expression pattern.
func IsEmail(value string) bool {
	if len(value) > 254 {
		return false
	}

	return RgxEmail.MatchString(value)
}

// IsURL returns true if the value is a valid URL
func IsURL(value string) bool {
	u, err := url.ParseRequestURI(value)
	if err != nil {
		return false
	}

	return u.Scheme != "" && u.Host != ""
}

//=============================================================================
// Flash Message functions
//=============================================================================

type contextKey string

const flashMessageKey = "messages"

type FlashMessageLevel string

const (
	// Different FlashMessageLevel types
	LevelSuccess FlashMessageLevel = "success"
	LevelError   FlashMessageLevel = "error"
	LevelWarning FlashMessageLevel = "warning"
	LevelInfo    FlashMessageLevel = "info"
)

type FlashMessage struct {
	Level   FlashMessageLevel
	Message string
}

// putFlashMessage adds a flash message into the session manager
func putFlashMessage(r *http.Request, level FlashMessageLevel, message string, sessionManager *scs.SessionManager) {
	newMessage := FlashMessage{
		Level:   level,
		Message: message,
	}

	// Create a new flashMessageKey context key if one doesn't exist and add the message
	messages, ok := sessionManager.Get(r.Context(), flashMessageKey).([]FlashMessage)
	if !ok {
		sessionManager.Put(r.Context(), flashMessageKey, []FlashMessage{newMessage})
		return
	}

	// Add a flash message to an existing flashMessageKey context key
	messages = append(messages, newMessage)
	sessionManager.Put(r.Context(), flashMessageKey, messages)
}
