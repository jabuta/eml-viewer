# Handler & Template Testing Recommendations

## Issue Found
Template rendering broke during refactoring, but wasn't caught because:
- No handler-level tests existed
- No template rendering validation
- Manual testing only happened at the end

## Recommended Tests

### 1. Handler Tests (High Priority)

Create `internal/handlers/handlers_test.go`:

```go
package handlers_test

import (
    "net/http/httptest"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestIndexHandler(t *testing.T) {
    h, db := setupTestHandlers(t)
    defer cleanupTestDB(t, db)
    
    // Insert test email
    email := createTestEmail("Subject", "sender@test.com", "Body")
    db.InsertEmail(email)
    
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    
    h.Index(w, req)
    
    assert.Equal(t, 200, w.Code)
    body := w.Body.String()
    
    // Critical checks
    assert.Contains(t, body, "EML Viewer")
    assert.Contains(t, body, "email-list")
    assert.Contains(t, body, "Subject")
    assert.Greater(t, len(body), 5000, "Response should contain substantial HTML")
}

func TestEmailDetailHandler(t *testing.T) {
    h, db := setupTestHandlers(t)
    defer cleanupTestDB(t, db)
    
    email := createTestEmail("Test Email", "test@example.com", "Test body")
    id, _ := db.InsertEmail(email)
    
    req := httptest.NewRequest("GET", fmt.Sprintf("/email/%d", id), nil)
    w := httptest.NewRecorder()
    
    // Set URL param
    rctx := chi.NewRouteContext()
    rctx.URLParams.Add("id", fmt.Sprintf("%d", id))
    req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
    
    h.ViewEmail(w, req)
    
    assert.Equal(t, 200, w.Code)
    body := w.Body.String()
    
    // Critical checks
    assert.Contains(t, body, "Back to email list")
    assert.Contains(t, body, "Test Email")
    assert.Contains(t, body, "test@example.com")
    assert.Contains(t, body, "Test body")
}

func TestSearchHandler(t *testing.T) {
    h, db := setupTestHandlers(t)
    defer cleanupTestDB(t, db)
    
    // Insert searchable emails
    db.InsertEmail(createTestEmail("Meeting Notes", "john@test.com", "Discussion about project"))
    db.InsertEmail(createTestEmail("Invoice", "billing@test.com", "Payment details"))
    
    req := httptest.NewRequest("GET", "/search?q=meeting", nil)
    w := httptest.NewRecorder()
    
    h.Search(w, req)
    
    assert.Equal(t, 200, w.Code)
    body := w.Body.String()
    
    assert.Contains(t, body, "Meeting Notes")
    assert.NotContains(t, body, "Invoice")
}
```

### 2. Template Loading Tests

```go
func TestTemplatesLoadWithoutErrors(t *testing.T) {
    h := handlers.New(nil, config.Default())
    
    err := h.LoadTemplates(embeddedFiles)
    
    require.NoError(t, err, "Templates must load successfully")
}

func TestAllRequiredTemplatesExist(t *testing.T) {
    h := handlers.New(nil, config.Default())
    h.LoadTemplates(embeddedFiles)
    
    templates := []string{"index.html", "email.html", "header", "footer", "email-row"}
    
    for _, tmpl := range templates {
        t.Run(tmpl, func(t *testing.T) {
            assert.NotNil(t, h.templates.Lookup(tmpl), "Template %s must exist", tmpl)
        })
    }
}
```

### 3. Template Rendering Tests

```go
func TestIndexTemplateRendersWithData(t *testing.T) {
    h := handlers.New(nil, config.Default())
    h.LoadTemplates(embeddedFiles)
    
    data := map[string]interface{}{
        "PageTitle": "Test",
        "Stats": map[string]interface{}{
            "TotalEmails": 10,
        },
        "Emails": []db.Email{
            {ID: 1, Subject: "Test", Sender: "test@example.com"},
        },
    }
    
    var buf bytes.Buffer
    err := h.templates.ExecuteTemplate(&buf, "index.html", data)
    
    require.NoError(t, err)
    output := buf.String()
    
    assert.Contains(t, output, "Test")
    assert.Contains(t, output, "10")
    assert.Contains(t, output, "test@example.com")
}
```

## Test Coverage Goals

- ✅ Parser: 88% (already good)
- ✅ Database: 82% (already good)  
- ❌ **Handlers: 0% → Target: 80%+**
- ❌ **Template rendering: 0% → Target: 100%**

## Benefits

1. **Prevents regressions**: Template changes immediately tested
2. **Faster feedback**: Catch issues in seconds, not after manual testing
3. **Confidence in refactoring**: Can restructure templates safely
4. **Documentation**: Tests show how templates should behave
5. **CI/CD ready**: Automated validation before deployment

## Implementation Priority

1. **High**: Template loading tests (catches syntax errors)
2. **High**: Index and email detail handler tests (core functionality)
3. **Medium**: Search handler tests
4. **Low**: Visual/DOM structure tests
