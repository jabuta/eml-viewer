package db

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// NullTime is a custom type that handles both string and time.Time from SQLite
type NullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements sql.Scanner for NullTime
func (nt *NullTime) Scan(value interface{}) error {
	if value == nil {
		nt.Time, nt.Valid = time.Time{}, false
		return nil
	}

	switch v := value.(type) {
	case time.Time:
		nt.Time, nt.Valid = v, true
		return nil
	case string:
		// Try multiple time formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			// SQLite timestamp formats including Go's time.String() format
			"2006-01-02 15:04:05.999999999 -0700 -0700", // Go's time.String() format with duplicate timezone
			"2006-01-02 15:04:05 -0700 -0700",
			"2006-01-02 15:04:05.999999999 -0700 MST",
			"2006-01-02 15:04:05 -0700 MST",
			"2006-01-02 15:04:05.999999999 -0700",
			"2006-01-02 15:04:05 -0700",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			time.RFC1123Z,
			time.RFC1123,
		}

		var t time.Time
		var err error
		for _, format := range formats {
			t, err = time.Parse(format, v)
			if err == nil {
				nt.Time, nt.Valid = t, true
				return nil
			}
		}

		return fmt.Errorf("failed to parse time string %q: %w", v, err)
	default:
		return fmt.Errorf("unsupported Scan type for NullTime: %T", value)
	}
}

// Value implements driver.Valuer for NullTime
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

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
	Date            NullTime
	BodyText        string
	BodyHTML        string
	HasAttachments  bool
	AttachmentCount int
	RawHeaders      string
	FileSize        int64
	IndexedAt       NullTime
	UpdatedAt       NullTime
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

// InsertEmailsBatch inserts multiple emails in a single transaction
// Returns the inserted email IDs in the same order as the input
func (db *DB) InsertEmailsBatch(emails []*Email) ([]int64, error) {
	if len(emails) == 0 {
		return []int64{}, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO emails (
			file_path, message_id, subject, sender, sender_name,
			recipients, cc, bcc, date, body_text, body_html,
			has_attachments, attachment_count, raw_headers, file_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	ids := make([]int64, 0, len(emails))
	for _, email := range emails {
		result, err := stmt.Exec(
			email.FilePath, email.MessageID, email.Subject, email.Sender, email.SenderName,
			email.Recipients, email.CC, email.BCC, email.Date, email.BodyText, email.BodyHTML,
			email.HasAttachments, email.AttachmentCount, email.RawHeaders, email.FileSize,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert email %s: %w", email.FilePath, err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert id: %w", err)
		}
		ids = append(ids, id)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return ids, nil
}

// InsertAttachmentsBatch inserts multiple attachments in a single transaction
func (db *DB) InsertAttachmentsBatch(attachments []*Attachment) error {
	if len(attachments) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO attachments (email_id, filename, content_type, size, data)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, att := range attachments {
		_, err := stmt.Exec(att.EmailID, att.Filename, att.ContentType, att.Size, att.Data)
		if err != nil {
			return fmt.Errorf("failed to insert attachment %s: %w", att.Filename, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// EmailsExistBatch checks which emails already exist in the database
// Returns a map of file paths to their existence status
func (db *DB) EmailsExistBatch(filePaths []string) (map[string]bool, error) {
	if len(filePaths) == 0 {
		return map[string]bool{}, nil
	}

	result := make(map[string]bool, len(filePaths))

	// SQLite has a limit on the number of variables in a query (default 999)
	// Process in chunks if necessary
	chunkSize := 500
	for i := 0; i < len(filePaths); i += chunkSize {
		end := i + chunkSize
		if end > len(filePaths) {
			end = len(filePaths)
		}
		chunk := filePaths[i:end]

		if err := db.checkExistenceChunk(chunk, result); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// checkExistenceChunk checks a chunk of file paths for existence
func (db *DB) checkExistenceChunk(filePaths []string, result map[string]bool) error {
	if len(filePaths) == 0 {
		return nil
	}

	// Build query with placeholders
	query := "SELECT file_path FROM emails WHERE file_path IN (?" +
		strings.Repeat(",?", len(filePaths)-1) + ")"

	// Convert file paths to interface slice
	args := make([]interface{}, len(filePaths))
	for i, fp := range filePaths {
		args[i] = fp
		result[fp] = false // Initialize all as false
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to check email existence: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var filePath string
		if err := rows.Scan(&filePath); err != nil {
			return fmt.Errorf("failed to scan file path: %w", err)
		}
		result[filePath] = true
	}

	return rows.Err()
}

// GetUniqueSenders retrieves a list of unique sender email addresses
// ordered by frequency (most emails sent first)
func (db *DB) GetUniqueSenders(limit int) ([]string, error) {
	rows, err := db.Query(`
		SELECT sender, COUNT(*) as email_count
		FROM emails
		WHERE sender != ''
		GROUP BY sender
		ORDER BY email_count DESC, sender ASC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique senders: %w", err)
	}
	defer rows.Close()

	var senders []string
	for rows.Next() {
		var sender string
		var count int
		if err := rows.Scan(&sender, &count); err != nil {
			return nil, fmt.Errorf("failed to scan sender: %w", err)
		}
		senders = append(senders, sender)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating senders: %w", err)
	}

	return senders, nil
}
