# go-notes

A lightweight, feature-rich Go web application template for taking markdown notes.

## Features

- **Complete Web Server**: HTTP server with graceful shutdown
- **Middleware Stack**:
  - Panic recovery
  - Secure headers
  - Request logging
  - CSRF protection
  - Authentication
  - Static asset caching
  - Session management
- **Email Support**: Send emails with configurable SMTP
- **Form Validation**: Comprehensive validation helpers
- **Flash Messages**: Session-based notifications system
- **Templating**: HTML template rendering with data context
- **Static File Serving**: Embedded static file handling
- **Development Mode**: Enhanced debugging with stack traces and additional logging.

## Getting Started

### Postgres setup

```sql
-- Create a database table
create database notes_test;

-- Connect to the database
\c notes_test

-- Create a user for the database
create role notes_user with login password 'pa55word';

-- Change the owner of the database
alter database notes owner to notes_user;

-- Grant access to public schema (might not need this one)
grant create on schema public to notes_user;

-- Grant create on database
grant create on database notes_test to notes_user;

-- Exit with \q
\q
```

Now try to connect to the database

```sh
psql --host=localhost --dbname=notes_test --username=notes_user
```

Then we'll want to add the database dsn as an environment variable(s) in `~/.profile`

```txt
export NOTES_DB_DSN='postgres://notes_user:password@localhost/notes'
export NOTES_TEST_DB_DSN='postgres://notes_user:password@localhost/notes'
```

Restart your computer or run `source $HOME/.profile` to load the environment variables.

You can also now connect directly to psql with `psql $NOTES_DB_DSN `

### Prerequisites

- Go 1.22 or higher
- [Task](https://taskfile.dev/) for project management commands.

### Installation

1. Clone the repository:

```bash
git clone https://github.com/sglmr/go-notes.git
cd gowebstart
```

2. Replace "gowebstart" with your new project name.
3. Build the project:

```bash
task build
```

### Environment Variables

The required environment variables to work with the applicaiton:

```sh

AUTH_EMAIL='...'
AUTH_PASSWORD_HASH='...'
NOTES_DB_DSN='....' # Probably have to add a '?sslmode=disable' suffix if the database is on the same server as the application
```



### Running the Server

Basic usage:

```bash
# Run the app
task run

# Run the app with live reload
task run:live
```

This will start the server on the default address `0.0.0.0:8000`.

### Command-Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | `0.0.0.0` | Server host address |
| `-port` | `""` | Server port number |
| `-dev` | `false` | Development mode - displays stack traces and enables verbose logging |
| `-auth-email` | `$AUTH_EMAIL` env var | Email address for authentication |
| `-auth-password-hash` | `$AUTH_PASSWORD_HASH` env var | Password hash for authentication |
| `-db-dsn` | `$NOTES_DB_DSN` env var | PostgreSQL database connection string |
| `-automigrate` | `true` | Automatically run pending database migrations on startup |
| `-time-location` | `America/Los_Angeles` | Time zone location |

## Email/SMTP Configuration (not currently used)

| Flag | Default | Description |
|------|---------|-------------|
| `-smtp-host` | `""` | SMTP server hostname |
| `-smtp-port` | `25` | SMTP server port |
| `-smtp-username` | `""` | SMTP authentication username |
| `-smtp-password` | `""` | SMTP authentication password |
| `-smtp-from` | `Example Name <no-reply@example.com>` | Email sender name and address |

### Example Usage

```bash
go run ./cmd/web -host=127.0.0.1 -port=8080 -dev=true -db-dsn="postgres://user:pass@localhost:5432/dbname"
```

### Environment Variables

Some flags can be set via environment variables:
- `AUTH_EMAIL` - Used if `-auth-email` is not provided
- `AUTH_PASSWORD_HASH` - Used if `-auth-password-hash` is not provided
- `NOTES_DB_DSN` - Used if `-db-dsn` is not provided

## SMTP Emails

The application includes methods for sending SMTP Emails. Email templates are configuratble in the `assets/emails` directory.

```go
err = mailer.Send(recipient string, replyTo string, data any, templates ...string)
```

## Background Tasks

The application includes a system for running asynchornous tasks using the `BackgroundTask` function.

```go
BackgroundTask(wg *sync.WaitGroup, logger *slog.Logger, fn func() error)
```

Background task system features:

- **Panic Recovery**: Tasks are isolated so panics don't crash the server
- **Logging**: Automatic error logging with the function name
- **WaitGroup Integration**: Proper shutdown handling with sync.WaitGroup
- **Graceful Shutdown**: Tasks tracked during server shutdown

Example usage:

```go
// Send an email in the background
BackgroundTask(
    wg, logger, 
    func() error {
        return mailer.Send("recipient@example.com", "reply-to@example.com", emailData, "email-template.tmpl")
    })

// Continue processing the request without waiting
```

This pattern is useful for operations like:

- Sending emails
- Processing uploaded files
- Running reports
- Performing database maintenance
- Any long-running task that shouldn't block the request handler

## Architecture

### Application Structure

- `assets/`: Folder for all project embedded files
  - `emails/`: Email templates
  - `migrations/`: Database migrations
  - `static/`: Static files like CSS, Javascript, etc
  - `templates/`: Templates to render to HTML pages for the application
    - `pages/`: Main web page content to load, like "home.tmpl" or "about.tmpl"
    - `partials/`: Page partials, like a nav bar, footer, etc
    - `base.tmpl`: Base template for all pages and partials
- `cmd/`
  - `hash/`:
    - `main.go`: Helper app to generate argon2id password hashes.
  - `web/`
    - `helpers.go`: authentication, flash message, and template helper functions
    - `main.go`: Entry point and server configuration
    - `middleware.go`: Middleware for the project
    - `routes.go`: Routes & handlers for the project.
- `db/`: sqlc generated database code
- `internal/`:
  - `argon2id/`: Password hashing & functinos
  - `assert/`: Testing assert functions
  - `email/`: SMTP email functionality
  - `funcs/`: Template functions
  - `render/`: Template rendering helpers
  - `validator/`: Helpers for validating form data
  - `vcs/`: Version information

### Middleware

The application uses a composable middleware pattern:

```go
handler = RecoverPanicMW(mux, logger, devMode)
handler = SecureHeadersMW(handler)
handler = LogRequestMW(logger)(handler)
handler = sessionManager.LoadAndSave(handler)
```

### Authentication

The project has authentication for a single user. There are flags for `-auth-email` and `-auth-password-hash` that will by default read from environment variables for `AUTH_EMAIL` and `AUTH_PASSWORD_HASH`. Password haches can be generated with the `go run ./cmd/hash` program.

There are dedicated handlers and templates for `/login/` and `/logout/` to log into or out of the application.

There are two middlewares related to authentication:

1. `requireLoginMW` - This middleware checks if a user is authenticated. If they are not, it redirects the user to _/login/?next=/page/they/tried/to/visit_.

2. `authenticateMW` - This middleware checks if a user is authenticated, and if so, sets an `isAuthenticatedContextKey` to `true`. Setting any user-related data as context keys in this middleware helps reduce session or other user-specific queries throughout the handlers.

**Templates**: Templates have access to a `{{.IsAuthenticated}} value to chec if the requester is signed in.

## Request Handlers

Request handlers follow a standardized pattern:

```go
func handlerName(dependencies...) http.HandlerFunc {
    // Handler specific type, constant, or variable definitions
    return func(w http.ResponseWriter, r *http.Request) {
        // Handler logic
    }
}
```

### Background Tasks

Tasks like sending emails are handled in the background:

```go
BackgroundTask(wg, logger, func() error {
    return mailer.Send(...)
})
```

## Form Validation

The application includes a comprehensive validation system with the `Validator` struct.

```go
type Validator struct {
    Errors map[string]string
}
```

`Validator` includes methods for managing errors and validation. For Example:


```go
// Example ContactForm validation with Validator

type contactForm struct {
    Name    string
    Message string
    Validator
}

form := contactForm{}
form.Check(IsEmail(form.Email), "Email", "Email must be a valid email address.")
form.Check(NotBlank(form.Message), "Message", "Message is required.")

if form.HasErrors() { 
    // Do something with errors
}
// Do something with no errors
```

Available validators:
- `NotBlank`: Ensures string is not empty
- `MinRunes`/`MaxRunes`: Length validation
- `Between`: Range validation
- `Matches`: Regex validation
- `In`/`NotIn`: Value presence validation
- `NoDuplicates`: Uniqueness validation
- `IsEmail`: Email validation
- `IsURL`: URL validation

## Flash Messages

The application supports various flash message types. Flash messages are formatted and rendered in the `assets/templates/partials/flashMessages.tmpl` template.

```go
putFlashMessage(r, LevelSuccess, "Welcome!", sessionManager)
```

Message levels:
- `LevelSuccess`
- `LevelError`
- `LevelWarning`
- `LevelInfo`

## Customization

### Adding New Routes and Middleware

Add new routes and middleware in the `AddRoutes` function. This project takes advantage of the [Go 1.22 Routing Enhancements](https://go.dev/blog/routing-enhancements).

```go
func AddRoutes(mux *http.ServeMux, ...) http.Handler {
    // Existing routes...
    
    // Add your new route
    mux.Handle("GET /your-path", yourHandler(dependencies...))
    
    // Middleware...
    handler := middleware1(mux)
    handler = middleware2(handler)

    return handler
}
```

### Rendering pages from templates

Templates are rendered using the `render.Page` function. Template pages live in the `assets/templates/pages` directory.

A `newTemplateData` function prefills a map with commony used template data

```go
data := newTemplateData(r, sessionManager)
err := render.Page(w, http.StatusOK, data, "your-template.tmpl")
```

Template functions are managed in the `internal/funcs` package.

## License

[MIT License](LICENSE)

## External Dependencies

- github.com/alexedwards/scs/v2
- github.com/justinas/nosurf
- github.com/wneessen/go-mail