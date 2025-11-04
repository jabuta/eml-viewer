# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

EML Viewer is a **local-only** email viewer for .eml files with full-text search. It's designed to run from a USB drive or local directory with zero installation. The application is built with Go + HTMX + SQLite and uses embedded templates and static assets for true portability.

**Critical Context**: This is a **local thumbdrive application**, not a network-exposed service. Security priorities differ from typical web applications.

## Build and Development Commands

### Build
```bash
# Development build (current platform)
go build -o eml-viewer

# Production build with optimizations
go build -ldflags="-s -w" -o eml-viewer

# Cross-compile for all platforms
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-windows.exe
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-macos-intel
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o eml-viewer-macos-apple
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o eml-viewer-linux
```

### Testing
```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/parser/...
go test ./internal/db/...

# Run with race detection
go test -race ./...
```

### Running
```bash
# Default (scans ./emails, starts on port 8787)
./eml-viewer

# Specify custom paths
./eml-viewer -scan /path/to/emails -port 8080
```

## Architecture

### High-Level Design

```
┌─────────────────────────────────────────────┐
│  main.go - Entry point and HTTP server      │
│  - Embeds web assets (go:embed)             │
│  - Chi router with middleware               │
│  - Graceful shutdown handling               │
└─────────────────────────────────────────────┘
                    │
        ┌───────────┴───────────┐
        ▼                       ▼
┌──────────────────┐    ┌──────────────────┐
│  Scanner/Indexer │    │   HTTP Handlers  │
│                  │    │                  │
│  - Walk dir tree │    │  - Index view    │
│  - Find .eml     │    │  - Email view    │
│  - Queue parsing │    │  - Search        │
│  - Batch insert  │    │  - Attachments   │
└────────┬─────────┘    │  - Scan trigger  │
         │              └────────┬─────────┘
         ▼                       │
┌──────────────────┐             │
│   EML Parser     │             │
│                  │             │
│  - MIME parsing  │             │
│  - Charset conv  │             │
│  - HTML/Text     │             │
│  - Attachments   │             │
└────────┬─────────┘             │
         │                       │
         └───────────┬───────────┘
                     ▼
         ┌────────────────────────┐
         │   SQLite Database      │
         │                        │
         │  - emails (metadata)   │
         │  - emails_fts (FTS5)   │
         │  - attachments (blobs) │
         └────────────────────────┘
```

### Key Architectural Patterns

**1. Metadata-Only Database Storage**
- Database stores ONLY metadata (subject, sender, dates, file paths)
- Full email content (HTML, attachments) parsed on-demand from .eml files
- Reason: Keeps DB small, avoids data duplication, enables easy re-indexing

**2. Embedded Assets for True Portability**
```go
//go:embed web/templates web/static
var embeddedFiles embed.FS
```
All HTML templates, CSS, and JavaScript are embedded in the binary. No external dependencies.

**3. Relative Path Storage**
- Database stores RELATIVE paths to .eml files (e.g., `2023/5/email.eml`)
- Paths resolved at runtime relative to configured `EmailsPath`
- Enables moving entire folder structure without breaking references

**4. Sandboxed HTML Email Rendering**
- HTML emails served from separate endpoint `/email/{id}/html`
- Rendered in iframe with `sandbox=""` attribute (blocks ALL scripts, plugins, forms)
- CSP headers enforce `frame-src 'self'` policy
- **Why not srcdoc**: Go templates HTML-escape ALL attribute values, including srcdoc, which strips HTML tags

### Critical Code Paths

**Email Display Flow:**
```
User clicks email → ViewEmail handler (email.go:36)
  ↓
Calls GetEmailWithFullContent(id)
  ↓
Reads file path from DB (relative path)
  ↓
ResolveEmailPath() → joins with EmailsPath
  ↓
ParseEMLFile() → parses .eml on-demand
  ↓
Template receives: Email (metadata) + BodyHTML + BodyText + Attachments
  ↓
Iframe loads from /email/{id}/html endpoint
  ↓
ViewEmailHTML handler (email.go:10) → serves raw HTML with security headers
```

**Search Flow:**
```
User types query → Search handler
  ↓
SearchEmailsWithFiltersAndOffset() with FTS5
  ↓
Query split into terms → each term fuzzy matched with wildcards
  ↓
Returns: EmailSearchResult with highlighted snippets (<mark> tags)
  ↓
HTMX renders results without page reload
```

### Database Schema Key Points

**emails table:**
- `file_path`: RELATIVE path (e.g., "2023/5/email.eml"), NEVER absolute
- `body_text_preview`: Only first 10KB stored for search, full text parsed on-demand
- `message_id`: Used for threading (InReplyTo, References)

**emails_fts virtual table:**
- FTS5 full-text search on subject, sender, recipients, body_text_preview
- Auto-populated via trigger on emails INSERT/UPDATE

**attachments table:**
- Stores only metadata (filename, content_type, size)
- Actual binary data parsed on-demand from .eml files

## Security Context

**Trust Model**: This is a LOCAL-ONLY application designed to run on a USB thumbdrive. It is NOT designed for network exposure.

### Implemented Security Measures

1. **Localhost-Only Binding**: Server binds to 127.0.0.1:8787 by default, enforced in config validation
2. **Sandboxed Iframes**: HTML emails rendered with `sandbox=""` - blocks ALL scripts, forms, plugins
3. **CSP Headers**: `frame-src 'self'`, `script-src 'self' 'unsafe-inline'`, `object-src 'none'`
4. **Path Traversal Protection**: ResolveEmailPath validates paths stay within EmailsPath
5. **Cycle Detection**: findConversationRoot has max hops limit to prevent infinite loops
6. **File Size Limits**: REMOVED per user request (local thumbdrive use case)

### When Making Changes

**HTML Rendering:**
- NEVER use `srcdoc` in iframes (Go templates escape it)
- Always serve HTML from separate endpoint with proper headers
- Keep `sandbox=""` attribute (empty = maximum restrictions)

**File Paths:**
- ALWAYS validate paths through ResolveEmailPath()
- NEVER accept absolute paths from user input or database
- NEVER use filepath.Join without validation

**Database Queries:**
- FTS5 queries must escape user input via escapeFTS5()
- Use parameterized queries for all SQL
- Validate input lengths before queries

**Templates:**
- Use `template.HTML` type for trusted HTML (after sanitization)
- Pipe functions won't work for attributes (they get escaped anyway)
- Pass sanitized HTML as `template.HTML` in data map, not via template function

## Common Patterns

### Adding a New Route
```go
// 1. Add handler method in internal/handlers/
func (h *Handlers) NewFeature(w http.ResponseWriter, r *http.Request) {
    // Implementation
}

// 2. Register route in main.go
r.Get("/feature", h.NewFeature)
```

### Parsing Email On-Demand
```go
// Get metadata first
email, _ := db.GetEmailByID(id)

// Parse full content when needed
emailWithContent, _ := db.GetEmailWithFullContent(id)
// Now has: BodyText, BodyHTML, CC, BCC, RawHeaders, Attachments
```

### Template Data Structure
```go
// Handlers pass data as map[string]interface{}
data := map[string]interface{}{
    "PageTitle": "Title",
    "Email":     email,              // Metadata struct
    "BodyHTML":  template.HTML(html), // Pre-sanitized HTML as template.HTML
    "BodyText":  text,
}

// Templates access: {{.Email.Subject}}, {{.BodyHTML}}, {{.BodyText}}
```

## Testing Philosophy

**Pragmatic Testing**: Focus on critical paths, not 100% coverage.

- **Parser (88% coverage)**: Thoroughly tested - handles untrusted input
- **Database (82% coverage)**: Core CRUD and search operations
- **Integration (4 tests)**: End-to-end workflows
- **Total (~40%)**: Concentrated on high-risk code

**Test Data**:
- `internal/parser/testdata/`: Real .eml files for parser tests
- Tests use SQLite `:memory:` for speed
- No external dependencies or network calls

## Dependencies

### Core
- `modernc.org/sqlite`: Pure Go SQLite (no CGO)
- `github.com/emersion/go-message`: EML/MIME parsing
- `github.com/go-chi/chi/v5`: HTTP router

### Static Assets (Embedded)
- HTMX 1.9.10: `/web/static/js/htmx.min.js`
- Tailwind CSS 3.4.1: `/web/static/js/tailwind.min.js`

**Why Embedded**: Originally used CDN, but CSP blocked external scripts. Now bundled locally for true offline operation.

## File Organization Conventions

```
eml-viewer/
├── main.go                    # Entry point, server setup, routes
├── internal/
│   ├── config/               # Configuration (defaults, validation)
│   ├── db/                   # Database layer (SQLite, FTS5, search)
│   ├── parser/               # EML parsing (MIME, charsets, attachments)
│   ├── scanner/              # File tree walking, .eml discovery
│   ├── indexer/              # Batch indexing coordinator
│   └── handlers/             # HTTP handlers (MVC controllers)
├── web/
│   ├── templates/            # Go HTML templates (embedded)
│   ├── static/               # CSS, JS (embedded)
│   └── assets.go             # Embed declarations
├── tests/integration/        # End-to-end tests
└── db/                       # Runtime: SQLite database (created automatically)
```

## Troubleshooting Development Issues

**Problem: Templates not updating**
- Templates are embedded via `go:embed`, must rebuild binary
- Solution: `go build -o eml-viewer && ./eml-viewer`

**Problem: "Port already in use"**
- Another instance running or port conflict
- Solution: `pkill eml-viewer` or change port in config

**Problem: HTML emails showing as plain text**
- Check iframe src points to `/email/{id}/html`
- Verify CSP allows `frame-src 'self'`
- Never use `srcdoc` attribute (Go escapes it)

**Problem: Path traversal errors in tests**
- Paths must be relative, not absolute
- Use `filepath.Rel()` or store relative to EmailsPath

## Known Limitations

1. **Size Limits Removed**: No file size restrictions (user requested for thumbdrive use)
2. **Single-User**: Not designed for concurrent access, no locking
3. **In-Process Scanning**: Blocks server during initial index (acceptable for local use)
4. **No Email Threading UI**: Threading logic exists but not exposed in UI yet

## Release Process

**Automated via GitHub Actions:**
```bash
git tag v1.2.0
git push origin v1.2.0
```
Builds all platforms, creates GitHub Release with binaries.

## When in Doubt

- **Portability first**: Must work on USB thumbdrive, no external dependencies
- **Security second**: Localhost-only, but still protect against malicious .eml files
- **Simplicity third**: Single binary, auto-opens browser, zero config

Refer to README.md for user-facing documentation.
