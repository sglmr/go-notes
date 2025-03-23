package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"

	"github.com/alexedwards/scs/v2"
	"github.com/justinas/nosurf"
	"github.com/sglmr/go-notes/internal/vcs"
)

//=============================================================================
// Response Helpers
//=============================================================================

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
	body := fmt.Sprint(http.StatusText(http.StatusBadRequest), ": ", err.Error())
	http.Error(w, body, http.StatusBadRequest)
}

//=============================================================================
// Template Helpers
//=============================================================================

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
