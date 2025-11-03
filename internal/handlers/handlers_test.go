package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felo/eml-viewer/internal/config"
	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/web"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestHandlers creates a handlers instance with a test database and loaded templates
func setupTestHandlers(t *testing.T) (*Handlers, *db.DB) {
	t.Helper()

	database := db.SetupTestDB(t)
	cfg := config.Default()
	h := New(database, cfg)

	// Load templates from embedded files
	err := h.LoadTemplates(web.Assets)
	require.NoError(t, err, "Failed to load templates for testing")

	return h, database
}

// setupTestHandlersWithTempDir creates handlers with a temp directory for .eml files
func setupTestHandlersWithTempDir(t *testing.T) (*Handlers, *db.DB, string) {
	t.Helper()

	// Create temp directory for .eml files
	tempDir, err := os.MkdirTemp("", "handler-test-*")
	require.NoError(t, err)

	database := db.SetupTestDB(t)
	database.SetEmailsPath(tempDir) // Configure for relative path resolution

	cfg := config.Default()
	cfg.EmailsPath = tempDir
	h := New(database, cfg)

	// Load templates from embedded files
	err = h.LoadTemplates(web.Assets)
	require.NoError(t, err, "Failed to load templates for testing")

	return h, database, tempDir
}

// createTestEMLFile creates a test .eml file in the given directory
func createTestEMLFile(t *testing.T, dir, filename, from, to, subject, body string) string {
	t.Helper()

	content := fmt.Sprintf(`From: %s
To: %s
Subject: %s
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

%s
`, from, to, subject, body)

	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return filename // Return relative path
}

// createTestEMLFileWithAttachments creates a test .eml file with attachments
func createTestEMLFileWithAttachments(t *testing.T, dir, filename, from, to, subject, body string) string {
	t.Helper()

	// Create a simple multipart email with attachments
	content := `From: ` + from + `
To: ` + to + `
Subject: ` + subject + `
Date: Mon, 1 Jan 2024 10:00:00 +0000
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="boundary123"

--boundary123
Content-Type: text/plain; charset=utf-8

` + body + `

--boundary123
Content-Type: application/pdf; name="document.pdf"
Content-Disposition: attachment; filename="document.pdf"
Content-Transfer-Encoding: base64

JVBERi0xLjQKJeLjz9MKMSAwIG9iago8PC9UeXBlL0NhdGFsb2cvUGFnZXMgMiAwIFI+PgplbmRv
YmoKMiAwIG9iago8PC9UeXBlL1BhZ2VzL0NvdW50IDEvS2lkc1szIDAgUl0+PgplbmRvYmoKMyAw
IG9iago8PC9UeXBlL1BhZ2UvTWVkaWFCb3hbMCAwIDMgM10+PgplbmRvYmoKeHJlZgowIDQKMDAw
MDAwMDAwMCA2NTUzNSBmIAowMDAwMDAwMDEwIDAwMDAwIG4gCjAwMDAwMDAwNTMgMDAwMDAgbiAK
MDAwMDAwMDEwMiAwMDAwMCBuIAp0cmFpbGVyCjw8L1NpemUgNC9Sb290IDEgMCBSPj4Kc3RhcnR4
cmVmCjE0OQolRU9GCg==

--boundary123
Content-Type: image/png; name="image.png"
Content-Disposition: attachment; filename="image.png"
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9
awAAAABJRU5ErkJggg==

--boundary123--
`

	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return filename // Return relative path
}

// Test that templates load without errors
func TestTemplatesLoadWithoutErrors(t *testing.T) {
	cfg := config.Default()
	h := New(nil, cfg)

	err := h.LoadTemplates(web.Assets)

	require.NoError(t, err, "Templates must load successfully")
	require.NotNil(t, h.templates, "Templates should be initialized")
}

// Test that all required templates exist
func TestAllRequiredTemplatesExist(t *testing.T) {
	h, _ := setupTestHandlers(t)

	templates := []string{"index.html", "email.html", "header", "footer", "email-row"}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			assert.NotNil(t, h.templates.Lookup(tmpl), "Template %s must exist", tmpl)
		})
	}
}

// Test that index template renders with data
func TestIndexTemplateRendersWithData(t *testing.T) {
	h, _ := setupTestHandlers(t)

	data := map[string]interface{}{
		"PageTitle": "Test",
		"Stats": map[string]interface{}{
			"TotalEmails": 10,
		},
		"Emails": []*db.EmailSearchResult{
			{
				Email:   db.Email{ID: 1, Subject: "Test Email", Sender: "test@example.com"},
				Snippet: "",
			},
		},
		"Senders": []string{},
	}

	var buf bytes.Buffer
	err := h.templates.ExecuteTemplate(&buf, "index.html", data)

	require.NoError(t, err, "Template should render without errors")
	output := buf.String()

	assert.Contains(t, output, "Test Email")
	assert.Contains(t, output, "test@example.com")
	assert.Contains(t, output, "10", "Should show total email count")
	assert.Greater(t, len(output), 1000, "Should render substantial HTML")
}

// Test that email template renders with data
func TestEmailTemplateRendersWithData(t *testing.T) {
	h, _ := setupTestHandlers(t)

	now := time.Now()
	data := map[string]interface{}{
		"PageTitle": "Test Email - EML Viewer",
		"Email": &db.EmailWithContent{
			Email: &db.Email{
				ID:              1,
				Subject:         "Test Subject",
				Sender:          "sender@test.com",
				SenderName:      "Test Sender",
				Recipients:      "recipient@test.com",
				BodyTextPreview: "Test email body",
				Date:            db.NewNullTime(now),
			},
			BodyText: "Test email body",
			BodyHTML: "<p>Test email body</p>",
		},
		"Attachments": []db.Attachment{},
	}

	var buf bytes.Buffer
	err := h.templates.ExecuteTemplate(&buf, "email.html", data)

	require.NoError(t, err, "Template should render without errors")
	output := buf.String()

	assert.Contains(t, output, "Test Subject")
	assert.Contains(t, output, "sender@test.com")
	assert.Contains(t, output, "Test Sender")
	assert.Contains(t, output, "Back to email list")
	assert.Contains(t, output, "From:")
	assert.Greater(t, len(output), 1000, "Should render substantial HTML")
}

// Test Index handler with no emails
func TestIndexHandlerNoEmails(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.Index(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "EML Viewer")
	assert.Contains(t, body, "No emails found")
	assert.Contains(t, body, "0 emails indexed")
}

// Test Index handler with emails
func TestIndexHandlerWithEmails(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	// Insert test emails
	email1 := db.CreateTestEmail("First Email", "sender1@test.com", "Body 1")
	email2 := db.CreateTestEmail("Second Email", "sender2@test.com", "Body 2")

	_, err := database.InsertEmail(email1)
	require.NoError(t, err)
	_, err = database.InsertEmail(email2)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	h.Index(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	// Critical checks
	assert.Contains(t, body, "EML Viewer")
	assert.Contains(t, body, "email-list", "Should contain email list container")
	assert.Contains(t, body, "First Email")
	assert.Contains(t, body, "Second Email")
	assert.Contains(t, body, "sender1@test.com")
	assert.Contains(t, body, "2 emails indexed")
	assert.Greater(t, len(body), 5000, "Response should contain substantial HTML")
}

// Test Email detail handler
func TestEmailDetailHandler(t *testing.T) {
	h, database, tempDir := setupTestHandlersWithTempDir(t)
	defer db.CleanupTestDB(t, database)
	defer os.RemoveAll(tempDir)

	// Create actual .eml file
	filename := createTestEMLFile(t, tempDir, "test.eml",
		"test@example.com", "recipient@example.com",
		"Test Email Subject", "This is the test email body")

	// Insert test email with reference to the .eml file
	email := db.CreateTestEmail("Test Email Subject", "test@example.com", "This is the test email body")
	email.FilePath = filename // Use relative path
	id, err := database.InsertEmail(email)
	require.NoError(t, err)

	// Create request with URL parameter
	req := httptest.NewRequest("GET", fmt.Sprintf("/email/%d", id), nil)
	w := httptest.NewRecorder()

	// Set URL param using chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", id))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.ViewEmail(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	// Critical checks
	assert.Contains(t, body, "Back to email list", "Should have back button")
	assert.Contains(t, body, "Test Email Subject")
	assert.Contains(t, body, "test@example.com")
	assert.Contains(t, body, "This is the test email body")
	assert.Contains(t, body, "From:")
	assert.Contains(t, body, "To:")
	assert.Greater(t, len(body), 3000, "Response should contain substantial HTML")
}

// Test Email detail handler with attachments
func TestEmailDetailHandlerWithAttachments(t *testing.T) {
	h, database, tempDir := setupTestHandlersWithTempDir(t)
	defer db.CleanupTestDB(t, database)
	defer os.RemoveAll(tempDir)

	// Create actual .eml file with attachments
	filename := createTestEMLFileWithAttachments(t, tempDir, "with-attachments.eml",
		"sender@test.com", "recipient@test.com",
		"Email With Attachments", "This email has attachments")

	// Insert email with attachment metadata
	email := db.CreateTestEmail("Email With Attachments", "sender@test.com", "This email has attachments")
	email.FilePath = filename
	email.HasAttachments = true
	email.AttachmentCount = 2
	id, err := database.InsertEmail(email)
	require.NoError(t, err)

	// Insert attachment metadata (data will be parsed from .eml)
	att1 := &db.Attachment{
		EmailID:     id,
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Size:        149, // Actual size of the base64 decoded data
	}
	att2 := &db.Attachment{
		EmailID:     id,
		Filename:    "image.png",
		ContentType: "image/png",
		Size:        68,
	}
	_, err = database.InsertAttachment(att1)
	require.NoError(t, err)
	_, err = database.InsertAttachment(att2)
	require.NoError(t, err)

	// Create request
	req := httptest.NewRequest("GET", fmt.Sprintf("/email/%d", id), nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", id))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.ViewEmail(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "Attachments (2)")
	assert.Contains(t, body, "document.pdf")
	assert.Contains(t, body, "image.png")
	assert.Contains(t, body, "Download")
}

// Test Email detail handler with invalid ID
func TestEmailDetailHandlerInvalidID(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	req := httptest.NewRequest("GET", "/email/invalid", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "invalid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.ViewEmail(w, req)

	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid email ID")
}

// Test Email detail handler with non-existent email
func TestEmailDetailHandlerNotFound(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	req := httptest.NewRequest("GET", "/email/99999", nil)
	w := httptest.NewRecorder()

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "99999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	h.ViewEmail(w, req)

	assert.Equal(t, 404, w.Code)
	assert.Contains(t, w.Body.String(), "Email not found")
}

// Test Search handler with results
func TestSearchHandlerWithResults(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	// Insert searchable emails
	email1 := db.CreateTestEmail("Meeting Notes", "john@test.com", "Discussion about project")
	email2 := db.CreateTestEmail("Invoice", "billing@test.com", "Payment details")
	email3 := db.CreateTestEmail("Meeting Reminder", "admin@test.com", "Don't forget the meeting")

	_, err := database.InsertEmail(email1)
	require.NoError(t, err)
	_, err = database.InsertEmail(email2)
	require.NoError(t, err)
	_, err = database.InsertEmail(email3)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/search?q=meeting", nil)
	w := httptest.NewRecorder()

	h.Search(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "Meeting Notes")
	assert.Contains(t, body, "Meeting Reminder")
	assert.NotContains(t, body, "Invoice")
	assert.Contains(t, body, "href=\"/email/")
}

// Test Search handler with no results
func TestSearchHandlerNoResults(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	email := db.CreateTestEmail("Test Email", "test@test.com", "Test body")
	_, err := database.InsertEmail(email)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/search?q=nonexistent", nil)
	w := httptest.NewRecorder()

	h.Search(w, req)

	assert.Equal(t, 200, w.Code)
	body := w.Body.String()

	assert.Contains(t, body, "No emails found")
	assert.NotContains(t, body, "Test Email")
}

// Test Search handler with empty query
func TestSearchHandlerEmptyQuery(t *testing.T) {
	h, database := setupTestHandlers(t)
	defer db.CleanupTestDB(t, database)

	// Insert test email
	email := db.CreateTestEmail("Test Email", "test@test.com", "Body")
	_, err := database.InsertEmail(email)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/search?q=", nil)
	w := httptest.NewRecorder()

	h.Search(w, req)

	assert.Equal(t, 200, w.Code)
	// Empty query should show all emails (calls Index)
	assert.Contains(t, w.Body.String(), "Test Email")
}
