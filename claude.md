# EML Viewer Project Plan

## Project Overview

**Goal:** Build a cross-platform EML email viewer with search capabilities that works for non-technical users on Windows and macOS.

**Stack:** Go + HTMX + Single Binary

### Architecture
- **Backend:** Go 1.21+ with embedded SQLite
- **Frontend:** HTMX + TailwindCSS (minimal JavaScript)
- **Database:** SQLite with FTS5 full-text search
- **Distribution:** Single binary (~10-20MB) per platform
- **Deployment:** User downloads → double-click → browser opens

### Key Features
- ✅ Recursive .eml file scanning from `./emails/*`
- ✅ SQLite database with full-text search (FTS5)
- ✅ Fuzzy search by sender, subject, recipients, date, body
- ✅ Email visualization (HTML + text rendering)
- ✅ Attachment extraction and download
- ✅ Single binary distribution (no dependencies)
- ✅ Auto-opens browser on startup
- ✅ Cross-platform (Windows, macOS, Linux)

---

## Technology Stack

### Backend
```
- Go 1.21+
- modernc.org/sqlite (pure Go SQLite, no CGo)
- emersion/go-message (EML parsing)
- net/http or github.com/go-chi/chi/v5 (routing)
- html/template (templating)
- embed.FS (embed frontend into binary)
```

### Frontend
```
- HTMX 1.9+ (dynamic HTML updates)
- TailwindCSS 3.x (styling)
- Alpine.js (optional, for small interactions)
- DaisyUI or custom components
```

### Build Tools
```
- go build (single command compilation)
- Cross-compilation: GOOS/GOARCH
- Optional: UPX for compression
```

---

## Database Schema

### Main Tables

```sql
-- Main emails table
CREATE TABLE emails (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT UNIQUE NOT NULL,
    message_id TEXT,
    subject TEXT,
    sender TEXT NOT NULL,
    sender_name TEXT,
    recipients TEXT,
    cc TEXT,
    bcc TEXT,
    date DATETIME,
    body_text TEXT,
    body_html TEXT,
    has_attachments BOOLEAN DEFAULT 0,
    attachment_count INTEGER DEFAULT 0,
    raw_headers TEXT,
    file_size INTEGER,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search virtual table
CREATE VIRTUAL TABLE emails_fts USING fts5(
    subject,
    sender,
    sender_name,
    recipients,
    body_text,
    content='emails',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER emails_ai AFTER INSERT ON emails BEGIN
    INSERT INTO emails_fts(rowid, subject, sender, sender_name, recipients, body_text)
    VALUES (new.id, new.subject, new.sender, new.sender_name, new.recipients, new.body_text);
END;

CREATE TRIGGER emails_ad AFTER DELETE ON emails BEGIN
    DELETE FROM emails_fts WHERE rowid = old.id;
END;

CREATE TRIGGER emails_au AFTER UPDATE ON emails BEGIN
    UPDATE emails_fts 
    SET subject = new.subject,
        sender = new.sender,
        sender_name = new.sender_name,
        recipients = new.recipients,
        body_text = new.body_text
    WHERE rowid = new.id;
END;

-- Attachments table
CREATE TABLE attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_id INTEGER NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT,
    size INTEGER,
    data BLOB,
    FOREIGN KEY(email_id) REFERENCES emails(id) ON DELETE CASCADE
);

-- Settings table (for storing email folder path, preferences)
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_emails_date ON emails(date DESC);
CREATE INDEX idx_emails_sender ON emails(sender);
CREATE INDEX idx_emails_file_path ON emails(file_path);
CREATE INDEX idx_attachments_email_id ON attachments(email_id);
```

---

## Project Structure

```
eml-viewer/
├── main.go                      # Entry point, HTTP server setup
├── go.mod                       # Go module dependencies
├── go.sum                       # Dependency checksums
├── claude.md                    # This file - project plan & progress
├── README.md                    # User-facing documentation
├── .gitignore
│
├── internal/
│   ├── config/
│   │   └── config.go           # App configuration
│   │
│   ├── db/
│   │   ├── db.go               # SQLite connection & initialization
│   │   ├── schema.go           # Database schema/migrations
│   │   ├── emails.go           # Email CRUD operations
│   │   └── search.go           # Search queries (FTS5)
│   │
│   ├── parser/
│   │   ├── eml.go              # EML file parsing logic
│   │   └── types.go            # Email data structures
│   │
│   ├── scanner/
│   │   ├── scanner.go          # Recursive directory scanning
│   │   └── watcher.go          # File system watching (optional)
│   │
│   └── handlers/
│       ├── index.go            # Home page handler
│       ├── search.go           # Search handler
│       ├── email.go            # Email detail handler
│       ├── settings.go         # Settings handler
│       ├── scan.go             # Scan/index handler
│       └── attachments.go      # Attachment download handler
│
├── web/
│   ├── templates/
│   │   ├── base.html           # Base layout (header, footer)
│   │   ├── index.html          # Email list page
│   │   ├── email.html          # Email detail page
│   │   ├── settings.html       # Settings page
│   │   ├── scan.html           # Scan progress page
│   │   └── components/
│   │       ├── email-row.html      # Email list item component
│   │       ├── search-bar.html     # Search component
│   │       ├── filters.html        # Filter components
│   │       └── pagination.html     # Pagination component
│   │
│   └── static/
│       ├── css/
│       │   └── styles.css      # TailwindCSS + custom styles
│       ├── js/
│       │   └── app.js          # Minimal JS (HTMX extensions, Alpine)
│       └── img/
│           └── logo.svg        # App logo/icon
│
└── build/
    ├── build.sh                # Build script for all platforms
    └── icons/                  # App icons for different platforms
```

---

## Implementation Phases

### Phase 1: Core Backend (MVP) ⏳
**Goal:** Get basic email indexing working with SQLite

**Tasks:**
- [ ] Initialize Go module (`go mod init`)
- [ ] Set up dependencies (modernc.org/sqlite, emersion/go-message)
- [ ] Create database package (`internal/db/`)
  - [ ] SQLite connection handling
  - [ ] Schema creation (emails, attachments, settings tables)
  - [ ] FTS5 virtual table setup
  - [ ] Triggers for FTS sync
- [ ] Create EML parser (`internal/parser/`)
  - [ ] Parse email headers (from, to, subject, date)
  - [ ] Extract plain text body
  - [ ] Extract HTML body
  - [ ] Parse attachments metadata (name, type, size)
- [ ] Create directory scanner (`internal/scanner/`)
  - [ ] Recursive .eml file discovery
  - [ ] Progress tracking
  - [ ] Handle errors gracefully
- [ ] Implement indexing logic
  - [ ] Read .eml files
  - [ ] Parse and insert into database
  - [ ] Skip already-indexed files
  - [ ] Log progress
- [ ] Basic HTTP server (`main.go`)
  - [ ] Serve on localhost:8080
  - [ ] Auto-open browser
  - [ ] Graceful shutdown (Ctrl+C)

**Deliverable:** CLI that scans folder, indexes emails, starts server

---

### Phase 2: Basic Frontend ⏳
**Goal:** Display emails in a simple web interface

**Tasks:**
- [ ] Set up HTML templates (`web/templates/`)
  - [ ] Base layout (header, navigation, footer)
  - [ ] Email list page
  - [ ] Email detail page
- [ ] Create handlers (`internal/handlers/`)
  - [ ] Index handler: show email list
  - [ ] Email detail handler: show single email
- [ ] Embed static files
  - [ ] Use `//go:embed` for templates and static files
  - [ ] Serve static CSS/JS
- [ ] Add TailwindCSS
  - [ ] Include via CDN (for MVP)
  - [ ] Basic responsive layout
- [ ] Implement email list rendering
  - [ ] Query 50 most recent emails
  - [ ] Display: subject, sender, date, snippet
  - [ ] Click → navigate to detail page
- [ ] Implement email detail rendering
  - [ ] Show full headers
  - [ ] Display text body
  - [ ] Display HTML body (in iframe, sandboxed)
  - [ ] List attachments

**Deliverable:** Working web interface to browse indexed emails

---

### Phase 3: Search & Filters ⏳
**Goal:** Implement fuzzy search with SQLite FTS5

**Tasks:**
- [ ] Implement FTS5 search queries (`internal/db/search.go`)
  - [ ] Full-text search with MATCH
  - [ ] Fuzzy matching with wildcards
  - [ ] Result ranking
  - [ ] Limit to 50 results
- [ ] Create search handler (`internal/handlers/search.go`)
  - [ ] Parse query parameters
  - [ ] Execute FTS5 search
  - [ ] Return HTML fragment (for HTMX)
- [ ] Add HTMX search component
  - [ ] Search input with debouncing
  - [ ] hx-get to /search endpoint
  - [ ] hx-target to update results
  - [ ] Show "searching..." indicator
- [ ] Implement filters
  - [ ] Date range filter (from/to dates)
  - [ ] Sender filter (dropdown or autocomplete)
  - [ ] "Has attachments" checkbox
  - [ ] Combine filters with search
- [ ] Add search result highlighting
  - [ ] Highlight matched terms in results
  - [ ] Snippet generation with context
- [ ] Pagination
  - [ ] Limit to 50 results per page
  - [ ] "Load more" button (HTMX)

**Deliverable:** Fast, fuzzy search with filters

---

### Phase 4: Email Rendering & Attachments ⏳
**Goal:** Safely render HTML emails and handle attachments

**Tasks:**
- [ ] HTML email rendering
  - [ ] Render in sandboxed iframe
  - [ ] CSP headers to block external resources
  - [ ] Fallback to text if no HTML
- [ ] Text email display
  - [ ] Preserve formatting (whitespace, line breaks)
  - [ ] Make URLs clickable
- [ ] Attachment handling
  - [ ] Extract attachments during parsing
  - [ ] Store in database (BLOB) or filesystem
  - [ ] Create download endpoint (`/attachments/:id/download`)
  - [ ] Stream files to browser
- [ ] Attachment preview
  - [ ] Image thumbnails (jpg, png, gif)
  - [ ] PDF preview (optional, via PDF.js)
  - [ ] Text file preview
- [ ] Email headers display
  - [ ] Show all headers (collapsible)
  - [ ] Copy header values
  - [ ] "View raw .eml" link

**Deliverable:** Full email visualization with attachments

---

### Phase 5: Polish & UX ⏳
**Goal:** Make the app feel professional and easy to use

**Tasks:**
- [ ] Loading states
  - [ ] Spinner during search
  - [ ] Progress bar during indexing
  - [ ] Skeleton loaders for email list
- [ ] Error handling
  - [ ] User-friendly error messages
  - [ ] Toast notifications (Alpine.js)
  - [ ] Retry logic for failed operations
- [ ] Empty states
  - [ ] "No emails found" message
  - [ ] "No search results" with suggestions
  - [ ] "Select a folder to get started"
- [ ] Keyboard shortcuts
  - [ ] `j/k` - Navigate email list
  - [ ] `/` - Focus search
  - [ ] `Esc` - Clear search
  - [ ] `Enter` - Open selected email
- [ ] Responsive design
  - [ ] Mobile-friendly layout
  - [ ] Touch-friendly buttons
  - [ ] Hamburger menu for mobile
- [ ] Dark mode
  - [ ] Toggle button
  - [ ] Save preference to database
  - [ ] Use CSS variables for theming
- [ ] Settings page
  - [ ] Change email folder path
  - [ ] Re-scan folder
  - [ ] Clear database
  - [ ] About/version info
- [ ] Performance optimization
  - [ ] Database query optimization
  - [ ] Index usage verification
  - [ ] Lazy loading for large email lists

**Deliverable:** Polished, professional-feeling app

---

### Phase 6: Distribution & Documentation ⏳
**Goal:** Package for non-technical users

**Tasks:**
- [ ] Build scripts (`build/build.sh`)
  - [ ] Cross-compile for Windows (amd64)
  - [ ] Cross-compile for macOS (Intel + Apple Silicon)
  - [ ] Cross-compile for Linux (amd64)
  - [ ] Strip debug symbols (`-ldflags="-s -w"`)
  - [ ] Optional: UPX compression
- [ ] Create README.md
  - [ ] Installation instructions
  - [ ] Screenshots
  - [ ] Features list
  - [ ] Troubleshooting
  - [ ] Build from source instructions
- [ ] User documentation
  - [ ] How to use search
  - [ ] Keyboard shortcuts
  - [ ] FAQ
- [ ] Optional: System tray icon
  - [ ] "Quit" option
  - [ ] "Open in browser" option
  - [ ] Status indicator
- [ ] Optional: Auto-updater
  - [ ] Check for updates on startup
  - [ ] Download and apply updates
  - [ ] Notify user of new version
- [ ] Testing on multiple platforms
  - [ ] Test on Windows 10/11
  - [ ] Test on macOS (Intel + Apple Silicon)
  - [ ] Test with various .eml files
  - [ ] Test with large datasets (1000+ emails)

**Deliverable:** Production-ready binaries with documentation

---

## Technical Implementation Details

### 1. Embedded Assets (Single Binary)

```go
package main

import (
    "embed"
    "html/template"
    "net/http"
)

//go:embed web/templates/* web/static/*
var embeddedFiles embed.FS

func main() {
    // Parse templates
    tmpl := template.Must(template.ParseFS(embeddedFiles, "web/templates/*.html"))
    
    // Serve static files
    http.Handle("/static/", http.FileServer(http.FS(embeddedFiles)))
    
    // ... rest of server setup
}
```

### 2. Auto-Open Browser

```go
import (
    "os/exec"
    "runtime"
)

func openBrowser(url string) error {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", url)
    case "windows":
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
    default:
        cmd = exec.Command("xdg-open", url)
    }
    return cmd.Start()
}
```

### 3. HTMX Search Pattern

```html
<!-- Search input -->
<input 
    type="text" 
    name="q"
    hx-get="/search"
    hx-trigger="keyup changed delay:300ms"
    hx-target="#email-list"
    hx-indicator="#search-spinner"
    placeholder="Search emails..."
    class="input input-bordered w-full"
/>

<span id="search-spinner" class="htmx-indicator">
    Searching...
</span>

<!-- Results container -->
<div id="email-list">
    <!-- HTMX will swap results here -->
</div>
```

### 4. FTS5 Fuzzy Search Query

```go
func SearchEmails(db *sql.DB, query string, limit int) ([]Email, error) {
    // Build FTS5 MATCH query
    sql := `
        SELECT 
            e.id, e.subject, e.sender, e.sender_name, 
            e.date, e.body_text,
            snippet(emails_fts, -1, '<mark>', '</mark>', '...', 32) as snippet
        FROM emails e
        JOIN emails_fts ON e.id = emails_fts.rowid
        WHERE emails_fts MATCH ?
        ORDER BY rank
        LIMIT ?
    `
    
    // Add wildcards for fuzzy matching
    // "john doe" -> "john* doe*"
    terms := strings.Fields(query)
    fuzzyTerms := make([]string, len(terms))
    for i, term := range terms {
        fuzzyTerms[i] = term + "*"
    }
    fuzzyQuery := strings.Join(fuzzyTerms, " ")
    
    rows, err := db.Query(sql, fuzzyQuery, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var emails []Email
    for rows.Next() {
        var e Email
        err := rows.Scan(
            &e.ID, &e.Subject, &e.Sender, &e.SenderName,
            &e.Date, &e.BodyText, &e.Snippet,
        )
        if err != nil {
            return nil, err
        }
        emails = append(emails, e)
    }
    
    return emails, nil
}
```

### 5. Safe HTML Email Rendering

```html
<!-- Email detail template -->
<div class="email-body">
    {{if .BodyHTML}}
        <!-- Sandboxed iframe for HTML emails -->
        <iframe 
            sandbox="allow-same-origin"
            srcdoc="{{.BodyHTML}}"
            class="w-full h-96 border"
        ></iframe>
    {{else}}
        <!-- Plain text fallback -->
        <pre class="whitespace-pre-wrap">{{.BodyText}}</pre>
    {{end}}
</div>
```

### 6. Graceful Shutdown

```go
func main() {
    // ... setup server
    
    // Catch interrupt signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        fmt.Println("\nShutting down gracefully...")
        db.Close()
        os.Exit(0)
    }()
    
    // Start server
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

## Build & Distribution

### Build Commands

```bash
# Install dependencies
go mod download

# Run in development mode
go run main.go

# Build for current OS
go build -o eml-viewer

# Build with smaller binary size
go build -ldflags="-s -w" -o eml-viewer

# Cross-compile for all platforms
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/eml-viewer-windows-amd64.exe
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/eml-viewer-macos-intel
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/eml-viewer-macos-apple
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/eml-viewer-linux-amd64

# Optional: Compress with UPX
upx --best --lzma dist/eml-viewer-*
```

### Build Script (build/build.sh)

```bash
#!/bin/bash
set -e

VERSION="v1.0.0"
PLATFORMS=("windows/amd64" "darwin/amd64" "darwin/arm64" "linux/amd64")
OUTPUT_DIR="dist"

mkdir -p $OUTPUT_DIR

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT_NAME="eml-viewer-${GOOS}-${GOARCH}"
    
    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$OUTPUT_DIR/$OUTPUT_NAME"
    
    echo "✓ Built $OUTPUT_NAME"
done

echo "Build complete! Binaries in $OUTPUT_DIR/"
```

---

## Dependencies (go.mod)

```go
module github.com/yourusername/eml-viewer

go 1.21

require (
    modernc.org/sqlite v1.27.0              // Pure Go SQLite (no CGo)
    github.com/emersion/go-message v0.17.0  // EML parsing
    github.com/go-chi/chi/v5 v5.0.10        // HTTP routing (optional)
)
```

---

## User Experience Flow

### First Run
1. User downloads `eml-viewer.exe` (Windows) or `eml-viewer` (macOS)
2. Double-click to run
3. Binary starts HTTP server on `localhost:8080`
4. Browser auto-opens to welcome page
5. User clicks "Select Folder" → native folder picker appears
6. User selects folder containing .eml files (e.g., `./emails/`)
7. Indexing starts with progress bar
8. When complete, redirects to email list

### Daily Usage
1. Double-click `eml-viewer` → browser opens
2. If folder already configured, shows email list immediately
3. User can:
   - Search emails (fuzzy search)
   - Filter by date, sender, attachments
   - Click email to view full content
   - Download attachments
   - Re-scan folder for new emails

### Search Example
1. User types "invoice" in search bar
2. HTMX sends request after 300ms delay
3. Server searches using FTS5: `SELECT * WHERE MATCH 'invoice*'`
4. Results update instantly (no page reload)
5. Matched terms highlighted in subject/body

---

## Estimated Timeline

| Phase | Tasks | Estimated Time |
|-------|-------|----------------|
| Phase 1: Core Backend | Database setup, EML parsing, scanning | 2-3 days |
| Phase 2: Basic Frontend | Templates, handlers, email list/detail | 1-2 days |
| Phase 3: Search & Filters | FTS5 search, filters, HTMX integration | 1-2 days |
| Phase 4: Email Rendering | HTML rendering, attachments, preview | 1-2 days |
| Phase 5: Polish & UX | Loading states, errors, keyboard shortcuts, dark mode | 2-3 days |
| Phase 6: Distribution | Build scripts, docs, testing | 1 day |
| **Total** | | **8-13 days** |

---

## Progress Tracking

### Overall Progress: 0% Complete

- [ ] **Phase 1:** Core Backend (0/6 tasks)
- [ ] **Phase 2:** Basic Frontend (0/7 tasks)
- [ ] **Phase 3:** Search & Filters (0/6 tasks)
- [ ] **Phase 4:** Email Rendering (0/5 tasks)
- [ ] **Phase 5:** Polish & UX (0/8 tasks)
- [ ] **Phase 6:** Distribution (0/6 tasks)

### Next Steps
1. Initialize Go module
2. Set up project structure
3. Create database schema
4. Implement EML parser
5. Build basic HTTP server

---

## Notes & Decisions

### Why Go + HTMX?
- **Single binary:** Easiest distribution for non-technical users
- **No dependencies:** Works on any OS without installing runtimes
- **Fast:** Go's performance + minimal JavaScript
- **Simple:** HTMX removes frontend complexity
- **Small:** 10-20MB total binary size

### Why SQLite FTS5?
- Built-in full-text search (no external engine needed)
- Fast fuzzy matching with wildcards
- Result ranking and snippets
- Perfect for < 100k emails
- Zero configuration

### Why Embedded Assets?
- Single file distribution
- No "missing files" errors
- Simpler deployment
- Users can't accidentally delete CSS/JS files

### Future Enhancements (Post-MVP)
- [ ] Email tagging/labels
- [ ] Export to PDF/CSV
- [ ] Email threading/conversations
- [ ] Multiple folder support
- [ ] Cloud backup integration
- [ ] Email composition (send replies)
- [ ] System tray icon
- [ ] Auto-updater
- [ ] Plugins/extensions

---

## Troubleshooting

### Common Issues

**Issue:** Binary won't start on macOS
- **Solution:** Right-click → Open (bypass Gatekeeper), or sign the binary

**Issue:** Windows SmartScreen warning
- **Solution:** Click "More info" → "Run anyway", or sign the binary

**Issue:** Browser doesn't auto-open
- **Solution:** Manually navigate to `http://localhost:8080`

**Issue:** "Port already in use"
- **Solution:** Change port in config, or kill process using port 8080

**Issue:** Emails not parsing correctly
- **Solution:** Check .eml file encoding, ensure files are valid RFC822 format

---

## Resources

### Documentation
- [Go Documentation](https://go.dev/doc/)
- [HTMX Documentation](https://htmx.org/docs/)
- [SQLite FTS5 Extension](https://www.sqlite.org/fts5.html)
- [emersion/go-message](https://github.com/emersion/go-message)
- [modernc.org/sqlite](https://gitlab.com/cznic/sqlite)

### Tools
- [Go Playground](https://go.dev/play/)
- [TailwindCSS CDN](https://tailwindcss.com/docs/installation/play-cdn)
- [HTMX Examples](https://htmx.org/examples/)

---

**Last Updated:** 2025-11-02
**Version:** 1.0.0 (Planning Phase)
