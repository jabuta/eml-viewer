package db

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/felo/eml-viewer/internal/parser"
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

// Email represents an email record in the database (metadata only)
// Full content (body_html, raw_headers, cc, bcc) is parsed from .eml file on-demand
type Email struct {
	ID               int64
	FilePath         string
	MessageID        string
	InReplyTo        string // Message-ID of parent email (for threading)
	ThreadReferences string // Comma-separated Message-IDs (conversation ancestry)
	Subject          string
	Sender           string
	SenderName       string
	Recipients       string
	Date             NullTime
	BodyTextPreview  string // First 10KB for FTS5 search only
	HasAttachments   bool
	AttachmentCount  int
	FileSize         int64
	IndexedAt        NullTime
	UpdatedAt        NullTime
}

// GetDate returns the date as time.Time, or zero time if NULL
func (e *Email) GetDate() time.Time {
	if e.Date.Valid {
		return e.Date.Time
	}
	return time.Time{}
}

// EmailWithContent represents a full email with all content parsed from .eml file
type EmailWithContent struct {
	*Email                            // Embedded metadata from database
	BodyText    string                // Full body text (parsed from .eml)
	BodyHTML    string                // Full body HTML (parsed from .eml)
	CC          []string              // CC recipients (parsed from .eml)
	BCC         []string              // BCC recipients (parsed from .eml)
	RawHeaders  string                // Raw headers (parsed from .eml)
	Attachments []*AttachmentWithData // Attachments with data
}

// AttachmentWithData represents an attachment with its binary data
type AttachmentWithData struct {
	*Attachment        // Embedded metadata from database
	Data        []byte // Actual attachment data (parsed from .eml)
}

// Attachment represents an email attachment (metadata only)
// Actual attachment data is extracted from .eml file on-demand
type Attachment struct {
	ID          int64
	EmailID     int64
	Filename    string
	ContentType string
	Size        int64
}

// InsertEmail inserts a new email into the database (metadata only)
func (db *DB) InsertEmail(email *Email) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO emails (
			file_path, message_id, in_reply_to, thread_references,
			subject, sender, sender_name, recipients, date,
			body_text_preview, has_attachments, attachment_count, file_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		email.FilePath, email.MessageID, email.InReplyTo, email.ThreadReferences,
		email.Subject, email.Sender, email.SenderName, email.Recipients, email.Date,
		email.BodyTextPreview, email.HasAttachments, email.AttachmentCount, email.FileSize,
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

// GetEmailByID retrieves an email by its ID (metadata only)
func (db *DB) GetEmailByID(id int64) (*Email, error) {
	email := &Email{}
	err := db.QueryRow(`
		SELECT id, file_path, message_id, in_reply_to, thread_references,
		       subject, sender, sender_name, recipients, date,
		       body_text_preview, has_attachments, attachment_count, file_size,
		       indexed_at, updated_at
		FROM emails WHERE id = ?
	`, id).Scan(
		&email.ID, &email.FilePath, &email.MessageID, &email.InReplyTo, &email.ThreadReferences,
		&email.Subject, &email.Sender, &email.SenderName, &email.Recipients, &email.Date,
		&email.BodyTextPreview, &email.HasAttachments, &email.AttachmentCount, &email.FileSize,
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

// ListEmails retrieves the most recent emails with pagination (metadata only)
func (db *DB) ListEmails(limit, offset int) ([]*Email, error) {
	rows, err := db.Query(`
		SELECT id, file_path, message_id, in_reply_to, thread_references,
		       subject, sender, sender_name, recipients, date,
		       body_text_preview, has_attachments, attachment_count, file_size,
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
			&email.ID, &email.FilePath, &email.MessageID, &email.InReplyTo, &email.ThreadReferences,
			&email.Subject, &email.Sender, &email.SenderName, &email.Recipients, &email.Date,
			&email.BodyTextPreview, &email.HasAttachments, &email.AttachmentCount, &email.FileSize,
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

// InsertAttachment inserts an attachment into the database (metadata only)
func (db *DB) InsertAttachment(att *Attachment) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO attachments (email_id, filename, content_type, size)
		VALUES (?, ?, ?, ?)
	`, att.EmailID, att.Filename, att.ContentType, att.Size)
	if err != nil {
		return 0, fmt.Errorf("failed to insert attachment: %w", err)
	}

	return result.LastInsertId()
}

// GetAttachmentsByEmailID retrieves all attachments for an email (metadata only)
func (db *DB) GetAttachmentsByEmailID(emailID int64) ([]*Attachment, error) {
	rows, err := db.Query(`
		SELECT id, email_id, filename, content_type, size
		FROM attachments WHERE email_id = ?
	`, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	defer rows.Close()

	var attachments []*Attachment
	for rows.Next() {
		att := &Attachment{}
		err := rows.Scan(&att.ID, &att.EmailID, &att.Filename, &att.ContentType, &att.Size)
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

// GetAttachmentByID retrieves a single attachment by ID (metadata only)
func (db *DB) GetAttachmentByID(id int64) (*Attachment, error) {
	att := &Attachment{}
	err := db.QueryRow(`
		SELECT id, email_id, filename, content_type, size
		FROM attachments WHERE id = ?
	`, id).Scan(&att.ID, &att.EmailID, &att.Filename, &att.ContentType, &att.Size)
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
			file_path, message_id, in_reply_to, thread_references,
			subject, sender, sender_name, recipients, date,
			body_text_preview, has_attachments, attachment_count, file_size
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	ids := make([]int64, 0, len(emails))
	for _, email := range emails {
		result, err := stmt.Exec(
			email.FilePath, email.MessageID, email.InReplyTo, email.ThreadReferences,
			email.Subject, email.Sender, email.SenderName, email.Recipients, email.Date,
			email.BodyTextPreview, email.HasAttachments, email.AttachmentCount, email.FileSize,
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
		INSERT INTO attachments (email_id, filename, content_type, size)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, att := range attachments {
		_, err := stmt.Exec(att.EmailID, att.Filename, att.ContentType, att.Size)
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

// GetUniqueRecipients retrieves a list of unique recipient email addresses
// Recipients are stored as comma-separated values, so this function splits them
// and returns unique addresses ordered by frequency
func (db *DB) GetUniqueRecipients(limit int) ([]string, error) {
	rows, err := db.Query(`
		SELECT recipients
		FROM emails
		WHERE recipients != ''
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get recipients: %w", err)
	}
	defer rows.Close()

	// Count occurrences of each recipient
	recipientCount := make(map[string]int)
	for rows.Next() {
		var recipients string
		if err := rows.Scan(&recipients); err != nil {
			return nil, fmt.Errorf("failed to scan recipients: %w", err)
		}

		// Split by comma and trim whitespace
		parts := strings.Split(recipients, ",")
		for _, part := range parts {
			recipient := strings.TrimSpace(part)
			if recipient != "" {
				recipientCount[recipient]++
			}
		}
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recipients: %w", err)
	}

	// Convert map to sorted slice
	type recipientFreq struct {
		email string
		count int
	}
	var freqs []recipientFreq
	for email, count := range recipientCount {
		freqs = append(freqs, recipientFreq{email, count})
	}

	// Sort by count (descending) then by email (ascending)
	for i := 0; i < len(freqs); i++ {
		for j := i + 1; j < len(freqs); j++ {
			if freqs[j].count > freqs[i].count ||
				(freqs[j].count == freqs[i].count && freqs[j].email < freqs[i].email) {
				freqs[i], freqs[j] = freqs[j], freqs[i]
			}
		}
	}

	// Take top N
	result := make([]string, 0, limit)
	for i := 0; i < len(freqs) && i < limit; i++ {
		result = append(result, freqs[i].email)
	}

	return result, nil
}

// Stats holds database statistics
type Stats struct {
	TotalEmails     int
	WithAttachments int
	LastIndexed     time.Time
}

// GetStats returns current database statistics
func (db *DB) GetStats() (*Stats, error) {
	stats := &Stats{}

	// Get total emails
	err := db.QueryRow("SELECT COUNT(*) FROM emails").Scan(&stats.TotalEmails)
	if err != nil {
		return nil, fmt.Errorf("failed to count emails: %w", err)
	}

	// Get count with attachments
	err = db.QueryRow("SELECT COUNT(*) FROM emails WHERE has_attachments = 1").Scan(&stats.WithAttachments)
	if err != nil {
		return nil, fmt.Errorf("failed to count emails with attachments: %w", err)
	}

	// Get last indexed time
	var lastIndexed sql.NullString
	err = db.QueryRow("SELECT MAX(indexed_at) FROM emails").Scan(&lastIndexed)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last indexed time: %w", err)
	}

	if lastIndexed.Valid {
		// Try to parse the timestamp
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02 15:04:05.999999999 -0700 -0700",
			"2006-01-02 15:04:05 -0700 -0700",
			"2006-01-02 15:04:05.999999999 -0700",
			"2006-01-02 15:04:05 -0700",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
		}

		var t time.Time
		var parseErr error
		for _, format := range formats {
			t, parseErr = time.Parse(format, lastIndexed.String)
			if parseErr == nil {
				stats.LastIndexed = t
				break
			}
		}
		// If all formats fail, leave LastIndexed as zero time
	}

	return stats, nil
}

// GetEmailWithFullContent retrieves an email and parses full content from .eml file
// This is used when viewing an email to get body_html, raw_headers, cc, bcc, etc.
func (db *DB) GetEmailWithFullContent(id int64) (*EmailWithContent, error) {
	// First get metadata from database
	email, err := db.GetEmailByID(id)
	if err != nil {
		return nil, err
	}
	if email == nil {
		return nil, nil
	}

	// Resolve relative path to absolute path
	absolutePath := db.ResolveEmailPath(email.FilePath)

	// Parse full content from .eml file
	parsed, err := parser.ParseEMLFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .eml file %s: %w", absolutePath, err)
	}

	// Get attachment metadata from database
	attachmentMeta, err := db.GetAttachmentsByEmailID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}

	// Match parsed attachments with database metadata
	attachmentsWithData := make([]*AttachmentWithData, 0, len(attachmentMeta))
	for i, meta := range attachmentMeta {
		// Find matching parsed attachment by index
		var data []byte
		if i < len(parsed.Attachments) {
			data = parsed.Attachments[i].Data
		}

		attachmentsWithData = append(attachmentsWithData, &AttachmentWithData{
			Attachment: meta,
			Data:       data,
		})
	}

	return &EmailWithContent{
		Email:       email,
		BodyText:    parsed.BodyText,
		BodyHTML:    parsed.BodyHTML,
		CC:          parsed.CC,
		BCC:         parsed.BCC,
		RawHeaders:  parsed.RawHeaders,
		Attachments: attachmentsWithData,
	}, nil
}

// GetAttachmentData retrieves attachment data by parsing the .eml file
// Returns the attachment data for the given attachment ID
func (db *DB) GetAttachmentData(attachmentID int64) ([]byte, error) {
	// Get attachment metadata
	att, err := db.GetAttachmentByID(attachmentID)
	if err != nil {
		return nil, err
	}
	if att == nil {
		return nil, fmt.Errorf("attachment not found")
	}

	// Get email to find the .eml file path
	email, err := db.GetEmailByID(att.EmailID)
	if err != nil {
		return nil, err
	}
	if email == nil {
		return nil, fmt.Errorf("email not found for attachment")
	}

	// Resolve relative path to absolute path
	absolutePath := db.ResolveEmailPath(email.FilePath)

	// Parse the .eml file
	parsed, err := parser.ParseEMLFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .eml file %s: %w", absolutePath, err)
	}

	// Find the matching attachment by filename
	for _, parsedAtt := range parsed.Attachments {
		if parsedAtt.Filename == att.Filename {
			return parsedAtt.Data, nil
		}
	}

	return nil, fmt.Errorf("attachment %s not found in .eml file", att.Filename)
}

// DeleteEmail deletes an email and its attachments from the database
// The .eml file is NOT deleted from disk
func (db *DB) DeleteEmail(id int64) error {
	result, err := db.Exec("DELETE FROM emails WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete email: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("email not found")
	}

	return nil
}

// DeleteEmailsBatch deletes multiple emails in a single transaction
func (db *DB) DeleteEmailsBatch(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("DELETE FROM emails WHERE id = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, id := range ids {
		_, err := stmt.Exec(id)
		if err != nil {
			return fmt.Errorf("failed to delete email %d: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
