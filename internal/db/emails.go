package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Email represents an email record in the database
type Email struct {
	ID              int64
	FilePath        string
	MessageID       string
	Subject         string
	Sender          string
	SenderName      string
	Recipients      string
	CC              string
	BCC             string
	Date            sql.NullTime
	BodyText        string
	BodyHTML        string
	HasAttachments  bool
	AttachmentCount int
	RawHeaders      string
	FileSize        int64
	IndexedAt       sql.NullTime
	UpdatedAt       sql.NullTime
}

// GetDate returns the date as time.Time, or zero time if NULL
func (e *Email) GetDate() time.Time {
	if e.Date.Valid {
		return e.Date.Time
	}
	return time.Time{}
}

// Attachment represents an email attachment
type Attachment struct {
	ID          int64
	EmailID     int64
	Filename    string
	ContentType string
	Size        int64
	Data        []byte
}

// InsertEmail inserts a new email into the database
func (db *DB) InsertEmail(email *Email) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO emails (
			file_path, message_id, subject, sender, sender_name,
			recipients, cc, bcc, date, body_text, body_html,
			has_attachments, attachment_count, raw_headers, file_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		email.FilePath, email.MessageID, email.Subject, email.Sender, email.SenderName,
		email.Recipients, email.CC, email.BCC, email.Date, email.BodyText, email.BodyHTML,
		email.HasAttachments, email.AttachmentCount, email.RawHeaders, email.FileSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert email: %w", err)
	}

	return result.LastInsertId()
}

// EmailExists checks if an email with the given file path already exists
func (db *DB) EmailExists(filePath string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM emails WHERE file_path = ?)", filePath).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return exists, nil
}

// GetEmailByID retrieves an email by its ID
func (db *DB) GetEmailByID(id int64) (*Email, error) {
	email := &Email{}
	err := db.QueryRow(`
		SELECT id, file_path, message_id, subject, sender, sender_name,
		       recipients, cc, bcc, date, body_text, body_html,
		       has_attachments, attachment_count, raw_headers, file_size,
		       indexed_at, updated_at
		FROM emails WHERE id = ?
	`, id).Scan(
		&email.ID, &email.FilePath, &email.MessageID, &email.Subject, &email.Sender, &email.SenderName,
		&email.Recipients, &email.CC, &email.BCC, &email.Date, &email.BodyText, &email.BodyHTML,
		&email.HasAttachments, &email.AttachmentCount, &email.RawHeaders, &email.FileSize,
		&email.IndexedAt, &email.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}
	return email, nil
}

// ListEmails retrieves the most recent emails with pagination
func (db *DB) ListEmails(limit, offset int) ([]*Email, error) {
	rows, err := db.Query(`
		SELECT id, file_path, message_id, subject, sender, sender_name,
		       recipients, cc, bcc, date, body_text, body_html,
		       has_attachments, attachment_count, raw_headers, file_size,
		       indexed_at, updated_at
		FROM emails
		ORDER BY date DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list emails: %w", err)
	}
	defer rows.Close()

	var emails []*Email
	for rows.Next() {
		email := &Email{}
		err := rows.Scan(
			&email.ID, &email.FilePath, &email.MessageID, &email.Subject, &email.Sender, &email.SenderName,
			&email.Recipients, &email.CC, &email.BCC, &email.Date, &email.BodyText, &email.BodyHTML,
			&email.HasAttachments, &email.AttachmentCount, &email.RawHeaders, &email.FileSize,
			&email.IndexedAt, &email.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan email: %w", err)
		}
		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating emails: %w", err)
	}

	return emails, nil
}

// CountEmails returns the total number of emails
func (db *DB) CountEmails() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM emails").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count emails: %w", err)
	}
	return count, nil
}

// InsertAttachment inserts an attachment into the database
func (db *DB) InsertAttachment(att *Attachment) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO attachments (email_id, filename, content_type, size, data)
		VALUES (?, ?, ?, ?, ?)
	`, att.EmailID, att.Filename, att.ContentType, att.Size, att.Data)
	if err != nil {
		return 0, fmt.Errorf("failed to insert attachment: %w", err)
	}

	return result.LastInsertId()
}

// GetAttachmentsByEmailID retrieves all attachments for an email
func (db *DB) GetAttachmentsByEmailID(emailID int64) ([]*Attachment, error) {
	rows, err := db.Query(`
		SELECT id, email_id, filename, content_type, size, data
		FROM attachments WHERE email_id = ?
	`, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*Attachment
	for rows.Next() {
		att := &Attachment{}
		err := rows.Scan(&att.ID, &att.EmailID, &att.Filename, &att.ContentType, &att.Size, &att.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to scan attachment: %w", err)
		}
		attachments = append(attachments, att)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating attachments: %w", err)
	}

	return attachments, nil
}

// GetAttachmentByID retrieves a single attachment by ID
func (db *DB) GetAttachmentByID(id int64) (*Attachment, error) {
	att := &Attachment{}
	err := db.QueryRow(`
		SELECT id, email_id, filename, content_type, size, data
		FROM attachments WHERE id = ?
	`, id).Scan(&att.ID, &att.EmailID, &att.Filename, &att.ContentType, &att.Size, &att.Data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}
	return att, nil
}
