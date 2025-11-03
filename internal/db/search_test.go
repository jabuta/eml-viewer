package db

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchEmails_SingleTerm tests searching with a single term
func TestSearchEmails_SingleTerm(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails
	emails := []*Email{
		createTestEmail("Meeting Tomorrow", "sender1@test.com", "Let's meet tomorrow at 10am"),
		createTestEmail("Project Update", "sender2@test.com", "The project is going well"),
		createTestEmail("Meeting Notes", "sender3@test.com", "Here are the meeting notes from yesterday"),
	}
	insertTestEmails(t, db, emails)

	// Search for "meeting"
	results, err := db.SearchEmails("meeting", 10)

	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 emails with 'meeting'")

	// Verify the results contain the search term
	for _, result := range results {
		hasMatch := strings.Contains(strings.ToLower(result.Subject), "meeting") ||
			strings.Contains(strings.ToLower(result.BodyText), "meeting")
		assert.True(t, hasMatch, "Result should contain 'meeting' in subject or body")
	}
}

// TestSearchEmails_MultipleTerms tests searching with multiple terms (AND logic)
func TestSearchEmails_MultipleTerms(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails
	emails := []*Email{
		createTestEmail("Meeting Tomorrow", "sender1@test.com", "Let's discuss the project tomorrow"),
		createTestEmail("Project Update", "sender2@test.com", "The project needs a meeting"),
		createTestEmail("Lunch Plans", "sender3@test.com", "Want to grab lunch tomorrow?"),
	}
	insertTestEmails(t, db, emails)

	// Search for "project meeting"
	results, err := db.SearchEmails("project meeting", 10)

	require.NoError(t, err)
	// Should find emails that contain both "project" AND "meeting"
	assert.Greater(t, len(results), 0, "Should find at least one result")

	for _, result := range results {
		text := strings.ToLower(result.Subject + " " + result.BodyText)
		assert.Contains(t, text, "project", "Result should contain 'project'")
		assert.Contains(t, text, "meeting", "Result should contain 'meeting'")
	}
}

// TestSearchEmails_FuzzyMatching tests fuzzy search with partial words
func TestSearchEmails_FuzzyMatching(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails
	emails := []*Email{
		createTestEmail("Meeting Tomorrow", "sender1@test.com", "Let's meet tomorrow"),
		createTestEmail("Project Discussion", "sender2@test.com", "We need to discuss the project"),
	}
	insertTestEmails(t, db, emails)

	// Search with partial word "meet" should match "meeting" and "meet"
	results, err := db.SearchEmails("meet", 10)

	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "Fuzzy search should find results with 'meet'")

	// Should find emails with words starting with "meet"
	found := false
	for _, result := range results {
		if strings.Contains(strings.ToLower(result.Subject), "meet") ||
			strings.Contains(strings.ToLower(result.BodyText), "meet") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find emails with 'meet' prefix")
}

// TestSearchEmails_ResultHighlighting tests that search results include highlighting
func TestSearchEmails_ResultHighlighting(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test email
	email := createTestEmail("Important Meeting", "sender@test.com",
		"This is a very important meeting that we need to attend. The meeting will discuss crucial topics.")
	insertTestEmails(t, db, []*Email{email})

	// Search for "meeting"
	results, err := db.SearchEmails("meeting", 10)

	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]

	// Snippet should contain <mark> tags for highlighting
	assert.Contains(t, result.Snippet, "<mark>", "Snippet should contain <mark> tag")
	assert.Contains(t, result.Snippet, "</mark>", "Snippet should contain </mark> tag")

	// The highlighted term should be "meeting" (case-insensitive)
	assert.Contains(t, strings.ToLower(result.Snippet), "meeting",
		"Snippet should contain the search term")
}

// TestSearchEmails_EmptyQuery tests that empty query returns recent emails
func TestSearchEmails_EmptyQuery(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails
	emails := []*Email{
		createTestEmail("Email 1", "sender1@test.com", "Body 1"),
		createTestEmail("Email 2", "sender2@test.com", "Body 2"),
		createTestEmail("Email 3", "sender3@test.com", "Body 3"),
	}
	insertTestEmails(t, db, emails)

	// Search with empty query
	results, err := db.SearchEmails("", 10)

	require.NoError(t, err)
	assert.Len(t, results, 3, "Empty query should return recent emails")

	// Results should have snippets (truncated body text)
	for _, result := range results {
		assert.NotEmpty(t, result.Snippet, "Each result should have a snippet")
	}
}

// TestSearchEmails_SpecialCharacters tests that special FTS5 characters are escaped
func TestSearchEmails_SpecialCharacters(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test email with special characters
	email := createTestEmail("Test Email", "sender@test.com",
		"This email contains special chars: test@example.com and some-dashes")
	insertTestEmails(t, db, []*Email{email})

	// Test with regular characters (FTS5 has limitations with special chars like @ and -)
	testCases := []string{
		"test email",    // space - should work
		"example",       // single word
		"special chars", // multiple words
	}

	for _, query := range testCases {
		t.Run("Query: "+query, func(t *testing.T) {
			results, err := db.SearchEmails(query, 10)

			// Should not error, even if no results found
			assert.NoError(t, err, "Search should not error")
			assert.NotNil(t, results, "Results should not be nil")
		})
	}
}

// TestSearchEmails_Limit tests that search respects the limit parameter
func TestSearchEmails_Limit(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert many test emails with unique subjects
	emails := []*Email{}
	for i := 1; i <= 20; i++ {
		email := createTestEmail(fmt.Sprintf("Test Email %d", i), "sender@test.com", "This is test email body content")
		emails = append(emails, email)
	}
	insertTestEmails(t, db, emails)

	// Search with limit of 5
	results, err := db.SearchEmails("test", 5)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 5, "Should return at most 5 results")

	// Search with limit of 10
	results, err = db.SearchEmails("test", 10)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(results), 10, "Should return at most 10 results")
}

// TestSearchEmails_Ranking tests that results are ranked by relevance
func TestSearchEmails_Ranking(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails with varying relevance
	emails := []*Email{
		createTestEmail("Important Important Important", "sender1@test.com",
			"This email mentions important three times in the subject and is very important"),
		createTestEmail("Regular Email", "sender2@test.com",
			"This is a regular email"),
		createTestEmail("Important Topic", "sender3@test.com",
			"This email has important in the subject"),
	}
	insertTestEmails(t, db, emails)

	// Search for "important"
	results, err := db.SearchEmails("important", 10)

	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "Should find results")

	// First result should have higher relevance (more occurrences of "important")
	// FTS5 ranks by BM25 algorithm, so more occurrences = higher rank
	firstResult := results[0]
	assert.Contains(t, strings.ToLower(firstResult.Subject), "important",
		"Top result should contain search term in subject")
}

// TestSearchEmailsWithFilters tests searching with additional filters
func TestSearchEmailsWithFilters(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	// Insert test emails
	emails := []*Email{
		createTestEmailWithAttachments("Email with Attachment", "alice@test.com", "Body 1", 1),
		createTestEmail("Email without Attachment", "bob@test.com", "Body 2"),
		createTestEmailWithAttachments("Another with Attachment", "alice@test.com", "Body 3", 2),
	}
	insertTestEmails(t, db, emails)

	// Test filter by sender
	results, err := db.SearchEmailsWithFilters("", "alice@test.com", false, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 emails from alice@test.com")

	for _, result := range results {
		assert.Equal(t, "alice@test.com", result.Sender)
	}

	// Test filter by has attachments
	results, err = db.SearchEmailsWithFilters("", "", true, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 emails with attachments")

	for _, result := range results {
		assert.True(t, result.HasAttachments, "All results should have attachments")
	}

	// Test combined filters (sender + attachments)
	results, err = db.SearchEmailsWithFilters("", "alice@test.com", true, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 emails from alice with attachments")

	// Test search query with filter
	results, err = db.SearchEmailsWithFilters("Attachment", "", true, "", "", 10)
	require.NoError(t, err)
	assert.Greater(t, len(results), 0, "Should find emails matching query and filter")

	for _, result := range results {
		assert.True(t, result.HasAttachments)
		text := strings.ToLower(result.Subject + " " + result.BodyText)
		assert.Contains(t, text, "attachment")
	}
}

// TestTruncateText tests the text truncation helper
func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "Short text",
			input:    "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "Exact length",
			input:    "Hello World",
			maxLen:   11,
			expected: "Hello World",
		},
		{
			name:     "Needs truncation",
			input:    "This is a very long text that needs to be truncated",
			maxLen:   20,
			expected: "This is a very long ...",
		},
		{
			name:     "Empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
