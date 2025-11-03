package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInsertEmail tests inserting an email into the database
func TestInsertEmail(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	email := CreateTestEmail("Test Subject", "sender@test.com", "Test body content")

	id, err := db.InsertEmail(email)

	require.NoError(t, err, "Should insert email without error")
	assert.Greater(t, id, int64(0), "Should return valid ID")

	// Verify it was inserted
	retrieved, err := db.GetEmailByID(id)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, email.Subject, retrieved.Subject)
	assert.Equal(t, email.Sender, retrieved.Sender)
	assert.Equal(t, email.BodyText, retrieved.BodyText)
}

// TestEmailExists tests checking if an email exists by file path
func TestEmailExists(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	email := CreateTestEmail("Test Subject", "sender@test.com", "Test body")
	email.FilePath = "/unique/path/test.eml"

	// Should not exist initially
	exists, err := db.EmailExists(email.FilePath)
	require.NoError(t, err)
	assert.False(t, exists, "Email should not exist before insertion")

	// Insert email
	_, err = db.InsertEmail(email)
	require.NoError(t, err)

	// Should exist now
	exists, err = db.EmailExists(email.FilePath)
	require.NoError(t, err)
	assert.True(t, exists, "Email should exist after insertion")

	// Different path should not exist
	exists, err = db.EmailExists("/different/path.eml")
	require.NoError(t, err)
	assert.False(t, exists, "Different path should not exist")
}

// TestGetEmailByID tests retrieving an email by its ID
func TestGetEmailByID(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Insert test email
	email := CreateTestEmail("Test Subject", "sender@test.com", "Test body")
	id, err := db.InsertEmail(email)
	require.NoError(t, err)

	// Retrieve by ID
	retrieved, err := db.GetEmailByID(id)

	require.NoError(t, err)
	require.NotNil(t, retrieved, "Should retrieve email")
	assert.Equal(t, id, retrieved.ID)
	assert.Equal(t, "Test Subject", retrieved.Subject)
	assert.Equal(t, "sender@test.com", retrieved.Sender)
	assert.Equal(t, "Test body", retrieved.BodyText)

	// Non-existent ID should return nil
	retrieved, err = db.GetEmailByID(99999)
	require.NoError(t, err)
	assert.Nil(t, retrieved, "Non-existent ID should return nil")
}

// TestListEmails tests listing emails with pagination
func TestListEmails(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Insert multiple test emails
	emails := []*Email{
		CreateTestEmailWithDate("Email 1", "sender1@test.com", "Body 1", time.Now().Add(-3*time.Hour)),
		CreateTestEmailWithDate("Email 2", "sender2@test.com", "Body 2", time.Now().Add(-2*time.Hour)),
		CreateTestEmailWithDate("Email 3", "sender3@test.com", "Body 3", time.Now().Add(-1*time.Hour)),
		CreateTestEmailWithDate("Email 4", "sender4@test.com", "Body 4", time.Now()),
	}

	InsertTestEmails(t, db, emails)

	// Test listing with limit
	list, err := db.ListEmails(2, 0)
	require.NoError(t, err)
	assert.Len(t, list, 2, "Should return 2 emails with limit=2")

	// Should be ordered by date DESC (most recent first)
	assert.Equal(t, "Email 4", list[0].Subject, "Most recent email should be first")
	assert.Equal(t, "Email 3", list[1].Subject, "Second most recent should be second")

	// Test pagination with offset
	list, err = db.ListEmails(2, 2)
	require.NoError(t, err)
	assert.Len(t, list, 2, "Should return 2 emails with offset=2")
	assert.Equal(t, "Email 2", list[0].Subject)
	assert.Equal(t, "Email 1", list[1].Subject)

	// Test listing all
	list, err = db.ListEmails(100, 0)
	require.NoError(t, err)
	assert.Len(t, list, 4, "Should return all 4 emails")
}

// TestCountEmails tests counting total emails
func TestCountEmails(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Initially should be 0
	count, err := db.CountEmails()
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Should start with 0 emails")

	// Insert emails
	emails := []*Email{
		CreateTestEmail("Email 1", "sender1@test.com", "Body 1"),
		CreateTestEmail("Email 2", "sender2@test.com", "Body 2"),
		CreateTestEmail("Email 3", "sender3@test.com", "Body 3"),
	}
	InsertTestEmails(t, db, emails)

	// Count should be 3
	count, err = db.CountEmails()
	require.NoError(t, err)
	assert.Equal(t, 3, count, "Should have 3 emails")
}

// TestAttachmentOperations tests inserting and retrieving attachments
func TestAttachmentOperations(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Insert email first
	email := CreateTestEmail("Email with Attachment", "sender@test.com", "Body")
	emailID, err := db.InsertEmail(email)
	require.NoError(t, err)

	// Create and insert attachment
	att := &Attachment{
		EmailID:     emailID,
		Filename:    "test.pdf",
		ContentType: "application/pdf",
		Size:        1024,
		Data:        []byte("fake pdf data"),
	}

	attID, err := db.InsertAttachment(att)
	require.NoError(t, err)
	assert.Greater(t, attID, int64(0), "Should return valid attachment ID")

	// Retrieve attachments by email ID
	attachments, err := db.GetAttachmentsByEmailID(emailID)
	require.NoError(t, err)
	require.Len(t, attachments, 1, "Should have 1 attachment")

	retrieved := attachments[0]
	assert.Equal(t, "test.pdf", retrieved.Filename)
	assert.Equal(t, "application/pdf", retrieved.ContentType)
	assert.Equal(t, int64(1024), retrieved.Size)
	assert.Equal(t, []byte("fake pdf data"), retrieved.Data)

	// Retrieve attachment by ID
	retrievedByID, err := db.GetAttachmentByID(attID)
	require.NoError(t, err)
	require.NotNil(t, retrievedByID)
	assert.Equal(t, "test.pdf", retrievedByID.Filename)

	// Multiple attachments
	att2 := &Attachment{
		EmailID:     emailID,
		Filename:    "test2.jpg",
		ContentType: "image/jpeg",
		Size:        2048,
		Data:        []byte("fake jpg data"),
	}
	_, err = db.InsertAttachment(att2)
	require.NoError(t, err)

	attachments, err = db.GetAttachmentsByEmailID(emailID)
	require.NoError(t, err)
	assert.Len(t, attachments, 2, "Should have 2 attachments")
}

// TestNullDateHandling tests that NULL dates are handled correctly
func TestNullDateHandling(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Create email with NULL date
	email := CreateTestEmail("Test Subject", "sender@test.com", "Body")
	email.Date = NullTime{Valid: false} // NULL date

	id, err := db.InsertEmail(email)
	require.NoError(t, err)

	// Retrieve and check
	retrieved, err := db.GetEmailByID(id)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.False(t, retrieved.Date.Valid, "Date should be NULL/invalid")
	assert.True(t, retrieved.GetDate().IsZero(), "GetDate() should return zero time for NULL")

	// Create email with valid date
	email2 := CreateTestEmail("Test Subject 2", "sender2@test.com", "Body 2")
	testDate := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	email2.Date = NullTime{Time: testDate, Valid: true}

	id2, err := db.InsertEmail(email2)
	require.NoError(t, err)

	retrieved2, err := db.GetEmailByID(id2)
	require.NoError(t, err)
	require.NotNil(t, retrieved2)

	assert.True(t, retrieved2.Date.Valid, "Date should be valid")
	assert.Equal(t, testDate.Unix(), retrieved2.Date.Time.Unix(), "Date should match")
}

// TestFTS5TriggerBehavior tests that FTS5 triggers work correctly
func TestFTS5TriggerBehavior(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Insert email
	email := CreateTestEmail("Searchable Subject", "sender@test.com", "Searchable body content")
	id, err := db.InsertEmail(email)
	require.NoError(t, err)

	// Should be searchable immediately via FTS5
	results, err := db.SearchEmails("Searchable", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should find 1 result")
	assert.Equal(t, id, results[0].ID)

	// Insert another email
	email2 := CreateTestEmail("Another Email", "sender2@test.com", "Different content")
	id2, err := db.InsertEmail(email2)
	require.NoError(t, err)

	// Both should be searchable
	results, err = db.SearchEmails("content", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2, "Should find 2 results with 'content'")

	// Specific search should find one
	results, err = db.SearchEmails("Different", 10)
	require.NoError(t, err)
	assert.Len(t, results, 1, "Should find 1 result")
	assert.Equal(t, id2, results[0].ID)
}

// TestSettings tests setting and getting application settings
func TestSettings(t *testing.T) {
	db := SetupTestDB(t)
	defer CleanupTestDB(t, db)

	// Get non-existent setting
	value, err := db.GetSetting("test_key")
	require.NoError(t, err)
	assert.Empty(t, value, "Non-existent setting should return empty string")

	// Set a setting
	err = db.SetSetting("test_key", "test_value")
	require.NoError(t, err)

	// Get the setting
	value, err = db.GetSetting("test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", value)

	// Update the setting
	err = db.SetSetting("test_key", "updated_value")
	require.NoError(t, err)

	value, err = db.GetSetting("test_key")
	require.NoError(t, err)
	assert.Equal(t, "updated_value", value, "Setting should be updated")
}
