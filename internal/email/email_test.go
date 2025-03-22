package email

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/sglmr/go-notes/internal/assert"
)

func TestLogMailer_Send(t *testing.T) {
	// Create a buffer to capture log output
	var logBuffer bytes.Buffer

	// Create a test logger that writes to our buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create the LogMailer with our test logger
	logMailer := NewLogMailer(logger)

	// Test data
	recipient := "test@example.com"
	replyTo := "reply@example.com"
	testData := map[string]string{"name": "Test User", "message": "Hello World"}
	patterns := []string{"welcome.tmpl", "notification.tmpl"}

	// Call the Send method
	err := logMailer.Send(recipient, replyTo, testData, patterns...)

	// Assert no error was returned
	assert.NoError(t, err)

	// Assert the log StringIn the expected information
	logOutput := logBuffer.String()
	assert.StringIn(t, "send email", logOutput)
	assert.StringIn(t, "recipient=test@example.com", logOutput)
	assert.StringIn(t, "name", logOutput)
	assert.StringIn(t, "Test User", logOutput)
	assert.StringIn(t, "message", logOutput)
	assert.StringIn(t, "Hello World", logOutput)
	assert.StringIn(t, "welcome.tmpl", logOutput)
	assert.StringIn(t, "notification.tmpl", logOutput)
}

// TestLogMailerImplementsInterface ensures that LogMailer correctly implements MailerInterface
func TestLogMailerImplementsInterface(t *testing.T) {
	t.Parallel()
	// This is a compile time check that LogMailer implements the MailerInterface
	var _ MailerInterface = (*LogMailer)(nil)
}

// TestMailerImplementsInterface ensures that Mailer correctly implements MailerInterface
func TestMailerImplementsInterface(t *testing.T) {
	t.Parallel()
	var _ MailerInterface = (*Mailer)(nil)
}
