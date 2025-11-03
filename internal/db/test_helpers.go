package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

// NewNullTime creates a sql.NullTime from a time.Time
func NewNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{Time: t, Valid: true}
}

// SetupTestDB creates an in-memory SQLite database for testing
func SetupTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db
}

// CleanupTestDB closes the test database
func CleanupTestDB(t *testing.T, db *DB) {
	t.Helper()

	if err := db.Close(); err != nil {
		t.Errorf("Failed to close test database: %v", err)
	}
}

// CreateTestEmail creates a test email with default values
func CreateTestEmail(subject, sender, body string) *Email {
	return &Email{
		FilePath:        fmt.Sprintf("/test/%s.eml", subject),
		MessageID:       fmt.Sprintf("<%s@test.com>", subject),
		Subject:         subject,
		Sender:          sender,
		SenderName:      "Test Sender",
		Recipients:      "recipient@test.com",
		CC:              "",
		BCC:             "",
		Date:            sql.NullTime{Time: time.Now(), Valid: true},
		BodyText:        body,
		BodyHTML:        "",
		HasAttachments:  false,
		AttachmentCount: 0,
		RawHeaders:      fmt.Sprintf("From: %s\nSubject: %s\n", sender, subject),
		FileSize:        int64(len(body)),
	}
}

// InsertTestEmails inserts multiple test emails and returns them
func InsertTestEmails(t *testing.T, db *DB, emails []*Email) []*Email {
	t.Helper()

	for i, email := range emails {
		id, err := db.InsertEmail(email)
		if err != nil {
			t.Fatalf("Failed to insert test email %d: %v", i, err)
		}
		emails[i].ID = id
	}

	return emails
}

// CreateTestEmailWithDate creates a test email with a specific date
func CreateTestEmailWithDate(subject, sender, body string, date time.Time) *Email {
	email := CreateTestEmail(subject, sender, body)
	email.Date = sql.NullTime{Time: date, Valid: true}
	return email
}

// CreateTestEmailWithAttachments creates a test email with attachments
func CreateTestEmailWithAttachments(subject, sender, body string, attachmentCount int) *Email {
	email := CreateTestEmail(subject, sender, body)
	email.HasAttachments = attachmentCount > 0
	email.AttachmentCount = attachmentCount
	return email
}
