package handlers

import (
	"strings"
	"testing"
	
	"github.com/microcosm-cc/bluemonday"
	"html/template"
)

// TestHTMLSanitization tests that malicious HTML is properly sanitized
func TestHTMLSanitization(t *testing.T) {
	// Create sanitization policy
	p := bluemonday.UGCPolicy()
	
	tests := []struct {
		name           string
		input          string
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name: "Script tag removal",
			input: "<p>Hello</p><script>alert('XSS')</script>",
			shouldContain: []string{"<p>Hello</p>"},
			shouldNotContain: []string{"<script>", "alert"},
		},
		{
			name: "Event handler removal",
			input: `<img src="x" onerror="alert('XSS')">`,
			shouldContain: []string{},
			shouldNotContain: []string{"onerror", "alert"},
		},
		{
			name: "JavaScript protocol removal",
			input: `<a href="javascript:alert('XSS')">Click</a>`,
			shouldContain: []string{"Click"},
			shouldNotContain: []string{"javascript:"},
		},
		{
			name: "Iframe removal",
			input: `<iframe src="evil.com"></iframe>`,
			shouldContain: []string{},
			shouldNotContain: []string{"<iframe>", "evil.com"},
		},
		{
			name: "SVG onload removal",
			input: `<svg onload="alert('XSS')"></svg>`,
			shouldContain: []string{},
			shouldNotContain: []string{"onload", "alert"},
		},
		{
			name: "Safe content preservation",
			input: `<p>Safe text</p><a href="https://example.com">Link</a>`,
			shouldContain: []string{"<p>Safe text</p>", "https://example.com", "Link"},
			shouldNotContain: []string{},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized := p.Sanitize(tt.input)
			
			for _, expected := range tt.shouldContain {
				if !strings.Contains(sanitized, expected) {
					t.Errorf("Expected sanitized output to contain %q, got: %s", expected, sanitized)
				}
			}
			
			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(sanitized, notExpected) {
					t.Errorf("Expected sanitized output NOT to contain %q, got: %s", notExpected, sanitized)
				}
			}
		})
	}
}

// TestSanitizeHTMLTemplateFunction tests the template function integration
func TestSanitizeHTMLTemplateFunction(t *testing.T) {
	p := bluemonday.UGCPolicy()
	
	// Create template with sanitizeHTML function
	tmpl := template.New("test").Funcs(template.FuncMap{
		"sanitizeHTML": func(s string) template.HTML {
			return template.HTML(p.Sanitize(s))
		},
	})
	
	tmpl, err := tmpl.Parse(`{{.Content | sanitizeHTML}}`)
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}
	
	// Test with malicious content
	var result strings.Builder
	data := map[string]string{
		"Content": `<p>Safe</p><script>alert('XSS')</script>`,
	}
	
	err = tmpl.Execute(&result, data)
	if err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}
	
	output := result.String()
	
	if !strings.Contains(output, "<p>Safe</p>") {
		t.Errorf("Expected output to contain safe content")
	}
	
	if strings.Contains(output, "<script>") || strings.Contains(output, "alert") {
		t.Errorf("Expected output to NOT contain script tags, got: %s", output)
	}
}
