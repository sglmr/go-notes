package main

import (
	"context"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/sglmr/go-notes/db"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexedwards/scs/v2"
	"github.com/sglmr/go-notes/internal/email"
)

//=============================================================================
// Top level application functions
//=============================================================================

var timeLocation *time.Location

func init() {
	gob.Register(FlashMessage{})
	gob.Register([]FlashMessage{})
}

func main() {
	// Get the background context to use throughout the application
	ctx := context.Background()

	// Run the application
	if err := runApp(ctx, os.Stdout, os.Args, os.Getenv); err != nil {
		log.Fatal("error runnning app: ", err.Error())
	}
}

// newServer is a constructor that takes in all dependencies as arguments
func newServer(
	logger *slog.Logger,
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

	// Add routes the ServeMux
	addRoutes(mux, logger, devMode, mailer, username, passwordHash, wg, sessionManager, queries)

	// Add middleare chain for all the routes
	var handler http.Handler = mux
	handler = recoverPanicMW(mux, logger, devMode)
	handler = SecureHeadersMW(handler)
	handler = csrfMW(handler)
	handler = authenticateMW(sessionManager)(handler)

	// Always apply session middlewaare last in the chain (first to execute)
	handler = sessionManager.LoadAndSave(handler)
	handler = LogRequestMW(logger)(handler)

	// Return everything
	return handler
}

func runApp(
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
	location := fs.String("time-location", "America/Los_Angeles", "Time Location (default: America/Los_Angeles)")
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

	// Load the time location
	timeLocation, err = time.LoadLocation(*location)
	if err != nil {
		return fmt.Errorf("load location error: %w", err)
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
	if !*devMode || os.Getenv("DOKKU_APP_NAME") != "" {
		sessionManager.Cookie.Secure = true
	}

	// Set up router
	srv := newServer(logger, *devMode, mailer, *username, *passwordHash, &wg, sessionManager, queries)

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

// backgroundTask executes a function in a background goroutine with proper error handling.
func backgroundTask(wg *sync.WaitGroup, logger *slog.Logger, fn func() error) {
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
