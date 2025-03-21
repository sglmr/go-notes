package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/justinas/nosurf"
	"github.com/sglmr/gowebstart/internal/argon2id"
)

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
		Secure:   os.Getenv("DOKKU_APP_NAME") != "",
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
