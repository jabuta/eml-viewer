package integration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/indexer"
	"github.com/felo/eml-viewer/internal/parser"
	"github.com/felo/eml-viewer/internal/scanner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndWorkflow tests the complete workflow from scanning to retrieval
func TestEndToEndWorkflow(t *testing.T) {
	// Step 1: Set up temporary directory with test .eml files
	tempDir, err := os.MkdirTemp("", "eml-viewer-test-*")
	require.NoError(t, err, "Should create temp directory")
	defer os.RemoveAll(tempDir)

	// Copy test .eml files to temp directory
	testFiles := []string{"sample.eml"}
	for _, filename := range testFiles {
		srcPath := filepath.Join("testdata", filename)
		dstPath := filepath.Join(tempDir, filename)

		err := copyFile(srcPath, dstPath)
		require.NoError(t, err, "Should copy test file %s", filename)
	}

	// Step 2: Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err, "Should open test database")
	defer testDB.Close()

	// Verify database schema is initialized
	count, err := testDB.CountEmails()
	require.NoError(t, err, "Should query empty database")
	assert.Equal(t, 0, count, "Database should start empty")

	// Step 3: Scan for .eml files
	scan := scanner.NewScanner(tempDir)
	files, err := scan.Scan()
	require.NoError(t, err, "Should scan directory")
	assert.Len(t, files, len(testFiles), "Should find all test files")

	// Step 4: Index emails
	idx := indexer.NewIndexer(testDB, tempDir, false)
	result, err := idx.IndexAll()
	require.NoError(t, err, "Should index all emails")

	assert.Equal(t, len(testFiles), result.TotalFound, "Should find all files")
	assert.Equal(t, len(testFiles), result.NewIndexed, "Should index all files")
	assert.Equal(t, 0, result.Failed, "Should have no failures")
	assert.Equal(t, 0, result.Skipped, "Should skip no files (first run)")

	// Step 5: Verify emails are in database
	count, err = testDB.CountEmails()
	require.NoError(t, err, "Should count emails")
	assert.Equal(t, len(testFiles), count, "Database should contain indexed emails")

	// Step 6: Retrieve email by querying the list
	emails, err := testDB.ListEmails(10, 0)
	require.NoError(t, err, "Should list emails")
	require.Len(t, emails, len(testFiles), "Should retrieve all emails")

	email := emails[0]
	assert.Equal(t, "Integration Test Email", email.Subject)
	assert.Equal(t, "john.doe@example.com", email.Sender)
	assert.Contains(t, email.BodyText, "integration test email")

	// Step 7: Test search functionality
	searchResults, err := testDB.SearchEmails("integration", 10)
	require.NoError(t, err, "Should search emails")
	assert.Len(t, searchResults, 1, "Should find 1 email with 'integration'")

	searchResult := searchResults[0]
	assert.Equal(t, email.ID, searchResult.ID, "Search result should match email")
	assert.Contains(t, searchResult.Snippet, "<mark>", "Search result should have highlighting")

	// Step 8: Retrieve email by ID
	retrievedEmail, err := testDB.GetEmailByID(email.ID)
	require.NoError(t, err, "Should retrieve email by ID")
	require.NotNil(t, retrievedEmail, "Email should exist")
	assert.Equal(t, email.Subject, retrievedEmail.Subject)
	assert.Equal(t, email.Sender, retrievedEmail.Sender)

	// Step 9: Test attachment handling
	assert.True(t, email.HasAttachments, "Email should have attachments")
	assert.Equal(t, 1, email.AttachmentCount, "Email should have 1 attachment")

	attachments, err := testDB.GetAttachmentsByEmailID(email.ID)
	require.NoError(t, err, "Should retrieve attachments")
	require.Len(t, attachments, 1, "Should have 1 attachment")

	att := attachments[0]
	assert.Equal(t, "readme.txt", att.Filename)
	assert.Greater(t, att.Size, int64(0), "Attachment should have size")
	assert.NotEmpty(t, att.Data, "Attachment should have data")

	// Step 10: Test re-indexing (should skip existing emails)
	result2, err := idx.IndexAll()
	require.NoError(t, err, "Should re-index without error")
	assert.Equal(t, 0, result2.NewIndexed, "Should not index duplicates")
	assert.Equal(t, len(testFiles), result2.Skipped, "Should skip all existing emails")

	// Step 11: Verify count hasn't changed
	finalCount, err := testDB.CountEmails()
	require.NoError(t, err, "Should count emails again")
	assert.Equal(t, len(testFiles), finalCount, "Count should remain same after re-index")
}

// TestWorkflow_MultipleEmails tests workflow with multiple emails
func TestWorkflow_MultipleEmails(t *testing.T) {
	// Set up temp directory
	tempDir, err := os.MkdirTemp("", "eml-viewer-multi-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create multiple test emails
	emailContents := []struct {
		filename string
		content  string
	}{
		{
			filename: "email1.eml",
			content: `From: sender1@test.com
To: recipient@test.com
Subject: First Email
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

This is the first test email.
`,
		},
		{
			filename: "email2.eml",
			content: `From: sender2@test.com
To: recipient@test.com
Subject: Second Email
Date: Mon, 1 Jan 2024 11:00:00 +0000
Content-Type: text/plain; charset=utf-8

This is the second test email.
`,
		},
		{
			filename: "email3.eml",
			content: `From: sender3@test.com
To: recipient@test.com
Subject: Third Email
Date: Mon, 1 Jan 2024 12:00:00 +0000
Content-Type: text/plain; charset=utf-8

This is the third test email.
`,
		},
	}

	// Write test files
	for _, ec := range emailContents {
		path := filepath.Join(tempDir, ec.filename)
		err := os.WriteFile(path, []byte(ec.content), 0644)
		require.NoError(t, err)
	}

	// Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Index all emails
	idx := indexer.NewIndexer(testDB, tempDir, false)
	result, err := idx.IndexAll()
	require.NoError(t, err)

	assert.Equal(t, 3, result.NewIndexed, "Should index 3 emails")
	assert.Equal(t, 0, result.Failed, "Should have no failures")

	// Test listing with pagination
	page1, err := testDB.ListEmails(2, 0)
	require.NoError(t, err)
	assert.Len(t, page1, 2, "First page should have 2 emails")

	page2, err := testDB.ListEmails(2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1, "Second page should have 1 email")

	// Test search across multiple emails
	results, err := testDB.SearchEmails("test email", 10)
	require.NoError(t, err)
	assert.Len(t, results, 3, "Should find all 3 emails with 'test email'")

	// Test specific search
	results, err = testDB.SearchEmails("first", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should find only first email")
	assert.Equal(t, "First Email", results[0].Subject)
}

// TestWorkflow_ParserIntegration tests the parser separately
func TestWorkflow_ParserIntegration(t *testing.T) {
	// Parse the sample test file directly
	parsed, err := parser.ParseEMLFile("testdata/sample.eml")
	require.NoError(t, err, "Should parse sample.eml")

	// Verify parsed content
	assert.Equal(t, "Integration Test Email", parsed.Subject)
	assert.Equal(t, "john.doe@example.com", parsed.Sender)
	assert.Equal(t, []string{"jane.smith@example.com"}, parsed.Recipients)
	assert.Contains(t, parsed.BodyText, "integration test email")

	// Verify attachment
	require.Len(t, parsed.Attachments, 1, "Should have 1 attachment")
	att := parsed.Attachments[0]
	assert.Equal(t, "readme.txt", att.Filename)
	assert.Contains(t, string(att.Data), "test attachment file")
}

// TestWorkflow_ErrorRecovery tests that the system handles errors gracefully
func TestWorkflow_ErrorRecovery(t *testing.T) {
	// Set up temp directory
	tempDir, err := os.MkdirTemp("", "eml-viewer-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a valid email
	validEmail := `From: sender@test.com
To: recipient@test.com
Subject: Valid Email
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

This is a valid email.
`
	err = os.WriteFile(filepath.Join(tempDir, "valid.eml"), []byte(validEmail), 0644)
	require.NoError(t, err)

	// Create a corrupted file (not valid EML)
	err = os.WriteFile(filepath.Join(tempDir, "corrupted.eml"), []byte("not a valid email"), 0644)
	require.NoError(t, err)

	// Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Index should handle the error gracefully
	idx := indexer.NewIndexer(testDB, tempDir, false)
	result, err := idx.IndexAll()

	// The indexer should complete without fatal error
	require.NoError(t, err, "Indexer should handle errors gracefully")

	// Should have indexed the valid email
	assert.Equal(t, 1, result.NewIndexed, "Should index valid email")

	// Should have failed on corrupted email
	assert.Equal(t, 1, result.Failed, "Should fail on corrupted email")

	// Database should contain only the valid email
	count, err := testDB.CountEmails()
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Database should contain only valid email")
}

// TestConcurrentIndexing tests that concurrent indexing works correctly
func TestConcurrentIndexing(t *testing.T) {
	// Set up temp directory with multiple test emails
	tempDir, err := os.MkdirTemp("", "eml-viewer-concurrent-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create 20 test emails to process concurrently
	numEmails := 20
	for i := 1; i <= numEmails; i++ {
		content := fmt.Sprintf(`From: sender%d@test.com
To: recipient@test.com
Subject: Test Email %d
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

This is test email number %d.
`, i, i, i)

		path := filepath.Join(tempDir, fmt.Sprintf("email%d.eml", i))
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Index with concurrent workers
	idx := indexer.NewIndexer(testDB, tempDir, false)
	idx.WithConcurrency(4) // Use 4 workers

	result, err := idx.IndexAll()
	require.NoError(t, err, "Concurrent indexing should succeed")

	assert.Equal(t, numEmails, result.TotalFound, "Should find all emails")
	assert.Equal(t, numEmails, result.NewIndexed, "Should index all emails")
	assert.Equal(t, 0, result.Failed, "Should have no failures")
	assert.Equal(t, 0, result.Skipped, "Should skip no emails")

	// Verify all emails are in database
	count, err := testDB.CountEmails()
	require.NoError(t, err)
	assert.Equal(t, numEmails, count, "All emails should be in database")

	// Verify no duplicates (common race condition issue)
	emails, err := testDB.ListEmails(100, 0)
	require.NoError(t, err)
	assert.Len(t, emails, numEmails, "Should have exactly the right number of emails")

	// Verify unique subjects
	subjectSet := make(map[string]bool)
	for _, email := range emails {
		assert.False(t, subjectSet[email.Subject], "Should not have duplicate subjects")
		subjectSet[email.Subject] = true
	}
}

// TestConcurrentIndexingWithProgress tests progress reporting with concurrent indexing
func TestConcurrentIndexingWithProgress(t *testing.T) {
	// Set up temp directory
	tempDir, err := os.MkdirTemp("", "eml-viewer-progress-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create 10 test emails
	numEmails := 10
	for i := 1; i <= numEmails; i++ {
		content := fmt.Sprintf(`From: sender%d@test.com
To: recipient@test.com
Subject: Progress Test %d
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

Progress test email %d.
`, i, i, i)

		path := filepath.Join(tempDir, fmt.Sprintf("progress%d.eml", i))
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Track progress
	progressCalls := 0
	var lastCurrent int

	idx := indexer.NewIndexer(testDB, tempDir, false)
	idx.WithConcurrency(2)

	result, err := idx.IndexWithProgress(func(current, total int, filePath string) {
		progressCalls++
		lastCurrent = current
		assert.LessOrEqual(t, current, total, "Current should not exceed total")
		assert.NotEmpty(t, filePath, "File path should not be empty")
	})

	require.NoError(t, err)
	assert.Equal(t, numEmails, result.NewIndexed, "Should index all emails")
	assert.Equal(t, numEmails, progressCalls, "Should call progress callback for each file")
	assert.Equal(t, numEmails, lastCurrent, "Last current should equal total")
}

// TestConcurrentIndexingRaceConditions tests for race conditions
func TestConcurrentIndexingRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	// Set up temp directory with many test emails
	tempDir, err := os.MkdirTemp("", "eml-viewer-race-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create 50 test emails to increase chance of catching race conditions
	numEmails := 50
	for i := 1; i <= numEmails; i++ {
		content := fmt.Sprintf(`From: sender%d@test.com
To: recipient@test.com
Subject: Race Test %d
Date: Mon, 1 Jan 2024 10:00:00 +0000
Content-Type: text/plain; charset=utf-8

Race condition test email %d.
`, i, i, i)

		path := filepath.Join(tempDir, fmt.Sprintf("race%d.eml", i))
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize database
	testDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Index with high concurrency
	idx := indexer.NewIndexer(testDB, tempDir, false)
	idx.WithConcurrency(8) // High concurrency to stress test

	result, err := idx.IndexAll()
	require.NoError(t, err, "High concurrency indexing should succeed")

	assert.Equal(t, numEmails, result.NewIndexed, "Should index all emails")
	assert.Equal(t, 0, result.Failed, "Should have no failures")

	// Critical: Verify no duplicate insertions (race condition)
	count, err := testDB.CountEmails()
	require.NoError(t, err)
	assert.Equal(t, numEmails, count, "Database count should match indexed count")

	// Verify data integrity
	emails, err := testDB.ListEmails(100, 0)
	require.NoError(t, err)
	assert.Len(t, emails, numEmails, "Should retrieve exact number of emails")
}

// copyFile is a helper to copy files for testing
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
