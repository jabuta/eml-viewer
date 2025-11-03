package parser

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseEML_SimpleEmail tests parsing a basic plain text email
func TestParseEML_SimpleEmail(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/simple.eml")

	require.NoError(t, err, "Should parse simple email without error")
	assert.Equal(t, "Simple Test Email", parsed.Subject)
	assert.Equal(t, "sender@example.com", parsed.Sender)
	assert.Equal(t, "", parsed.SenderName) // No display name in test file
	assert.Equal(t, []string{"recipient@example.com"}, parsed.Recipients)
	assert.Contains(t, parsed.BodyText, "This is a simple test email")
	assert.Empty(t, parsed.BodyHTML)
	assert.Empty(t, parsed.Attachments)
	assert.Equal(t, "<simple123@example.com>", parsed.MessageID)
	assert.False(t, parsed.Date.IsZero())
}

// TestParseEML_MIMEEncodedSubject tests parsing emails with MIME-encoded headers
func TestParseEML_MIMEEncodedSubject(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/mime-encoded.eml")

	require.NoError(t, err, "Should parse MIME-encoded email without error")

	// The subject should be decoded from =?UTF-8?Q?Invitaci=C3=B3n:_Reuni=C3=B3n_de_proyecto?=
	assert.Equal(t, "Invitación: Reunión de proyecto", parsed.Subject,
		"MIME-encoded subject should be decoded properly")
	assert.Equal(t, "sender@example.com", parsed.Sender)
	assert.Contains(t, parsed.BodyText, "MIME-encoded subject line")
}

// TestParseEML_Windows1252Charset tests parsing emails with windows-1252 charset
func TestParseEML_Windows1252Charset(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/windows-1252.eml")

	require.NoError(t, err, "Should parse windows-1252 email without error")
	assert.Equal(t, "Windows-1252 Charset Test", parsed.Subject)
	assert.Equal(t, "sender@example.com", parsed.Sender)
	assert.Contains(t, parsed.BodyText, "windows-1252 charset")
	// Just verify it parsed successfully, charset decoder is registered
	assert.NotEmpty(t, parsed.BodyText, "Should have body text")
}

// TestParseEML_ISO88591Charset tests parsing emails with iso-8859-1 charset
func TestParseEML_ISO88591Charset(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/iso-8859-1.eml")

	require.NoError(t, err, "Should parse iso-8859-1 email without error")
	assert.Equal(t, "ISO-8859-1 Charset Test", parsed.Subject)
	assert.Equal(t, "sender@example.com", parsed.Sender)
	assert.Contains(t, parsed.BodyText, "iso-8859-1 charset")
}

// TestParseEML_WithAttachment tests parsing emails with attachments
func TestParseEML_WithAttachment(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/with-attachment.eml")

	require.NoError(t, err, "Should parse email with attachment without error")
	assert.Equal(t, "Email with Attachment", parsed.Subject)
	assert.Contains(t, parsed.BodyText, "This email has an attachment")

	// Check attachment
	require.Len(t, parsed.Attachments, 1, "Should have exactly 1 attachment")

	att := parsed.Attachments[0]
	assert.Equal(t, "document.pdf", att.Filename)
	assert.Equal(t, "application/pdf", att.ContentType)
	assert.Greater(t, att.Size, int64(0), "Attachment should have size > 0")
	assert.NotEmpty(t, att.Data, "Attachment data should not be empty")
}

// TestParseEML_HTMLEmail tests parsing emails with both HTML and plain text
func TestParseEML_HTMLEmail(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/html-email.eml")

	require.NoError(t, err, "Should parse HTML email without error")
	assert.Equal(t, "HTML Email Test", parsed.Subject)

	// Should have both plain text and HTML parts
	assert.Contains(t, parsed.BodyText, "plain text version")
	assert.Contains(t, parsed.BodyHTML, "<html>")
	assert.Contains(t, parsed.BodyHTML, "<h1>This is an HTML email</h1>")
	assert.Contains(t, parsed.BodyHTML, "<strong>HTML</strong>")
}

// TestParseEML_MissingHeaders tests parsing emails with missing optional headers
func TestParseEML_MissingHeaders(t *testing.T) {
	parsed, err := ParseEMLFile("testdata/missing-headers.eml")

	require.NoError(t, err, "Should parse email with missing headers without error")
	assert.Equal(t, "Missing Headers Test", parsed.Subject)
	assert.Equal(t, "sender@example.com", parsed.Sender)

	// Message-ID is missing, should be empty
	assert.Empty(t, parsed.MessageID)

	// Date is missing - parser may set current time or zero time depending on library behavior
	// Either is acceptable, just verify no crash
	_ = parsed.Date

	// Should still parse the body
	assert.Contains(t, parsed.BodyText, "missing some headers")
}

// TestParseEML_InvalidFile tests error handling for non-existent files
func TestParseEML_InvalidFile(t *testing.T) {
	_, err := ParseEMLFile("testdata/does-not-exist.eml")

	assert.Error(t, err, "Should return error for non-existent file")
	assert.Contains(t, err.Error(), "failed to open file")
}

// TestDecodeMIMEWord tests the MIME word decoder function
func TestDecodeMIMEWord(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UTF-8 Quoted-Printable",
			input:    "=?UTF-8?Q?Invitaci=C3=B3n?=",
			expected: "Invitación",
		},
		{
			name:     "UTF-8 Base64",
			input:    "=?UTF-8?B?SW52aXRhY2nDs24=?=",
			expected: "Invitación",
		},
		{
			name:     "Multiple encoded words",
			input:    "=?UTF-8?Q?Invitaci=C3=B3n:?= =?UTF-8?Q?_Reuni=C3=B3n?=",
			expected: "Invitación: Reunión",
		},
		{
			name:     "Plain text (no encoding)",
			input:    "Simple Subject",
			expected: "Simple Subject",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeMIMEWord(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseEML_ComplexRecipients tests parsing emails with multiple recipients
func TestParseEML_ComplexRecipients(t *testing.T) {
	// Create a test email with multiple To, CC, and BCC recipients
	emlContent := `From: sender@example.com
To: recipient1@example.com, recipient2@example.com
Cc: cc1@example.com, cc2@example.com
Bcc: bcc1@example.com
Subject: Multiple Recipients Test
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

Test email with multiple recipients.
`

	// Write temporary test file
	tmpFile := "testdata/temp-multiple-recipients.eml"
	err := os.WriteFile(tmpFile, []byte(emlContent), 0644)
	require.NoError(t, err)
	defer os.Remove(tmpFile)

	parsed, err := ParseEMLFile(tmpFile)
	require.NoError(t, err)

	assert.Len(t, parsed.Recipients, 2, "Should have 2 To recipients")
	assert.Contains(t, parsed.Recipients, "recipient1@example.com")
	assert.Contains(t, parsed.Recipients, "recipient2@example.com")

	assert.Len(t, parsed.CC, 2, "Should have 2 CC recipients")
	assert.Contains(t, parsed.CC, "cc1@example.com")
	assert.Contains(t, parsed.CC, "cc2@example.com")

	assert.Len(t, parsed.BCC, 1, "Should have 1 BCC recipient")
	assert.Contains(t, parsed.BCC, "bcc1@example.com")
}

// TestParseEML_DateParsing tests various date formats
func TestParseEML_DateParsing(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		shouldOK bool
	}{
		{
			name:     "RFC 2822 format",
			dateStr:  "Mon, 1 Jan 2024 10:00:00 +0000",
			shouldOK: true,
		},
		{
			name:     "Alternative format",
			dateStr:  "1 Jan 2024 10:00:00 GMT",
			shouldOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emlContent := "From: sender@example.com\n"
			emlContent += "Subject: Date Test\n"
			emlContent += "Date: " + tt.dateStr + "\n"
			emlContent += "Content-Type: text/plain; charset=utf-8\n\n"
			emlContent += "Test body\n"

			tmpFile := "testdata/temp-date-test.eml"
			err := os.WriteFile(tmpFile, []byte(emlContent), 0644)
			require.NoError(t, err)
			defer os.Remove(tmpFile)

			parsed, err := ParseEMLFile(tmpFile)
			require.NoError(t, err)

			if tt.shouldOK {
				assert.False(t, parsed.Date.IsZero(), "Date should be parsed successfully")
				assert.Equal(t, 2024, parsed.Date.Year())
				assert.Equal(t, time.January, parsed.Date.Month())
			}
		})
	}
}
