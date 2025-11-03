package db

import (
	"fmt"
	"strings"
)

// EmailSearchResult represents a search result with snippet
type EmailSearchResult struct {
	Email
	Snippet string
}

// SearchEmails performs a full-text search on emails using FTS5
func (db *DB) SearchEmails(query string, limit int) ([]*EmailSearchResult, error) {
	if query == "" {
		// If no query, just return recent emails
		emails, err := db.ListEmails(limit, 0)
		if err != nil {
			return nil, err
		}

		results := make([]*EmailSearchResult, len(emails))
		for i, email := range emails {
			results[i] = &EmailSearchResult{
				Email:   *email,
				Snippet: truncateText(email.BodyText, 200),
			}
		}
		return results, nil
	}

	// Build FTS5 MATCH query with fuzzy matching
	// Add wildcards to each term for fuzzy matching: "john doe" -> "john* doe*"
	terms := strings.Fields(query)
	fuzzyTerms := make([]string, len(terms))
	for i, term := range terms {
		// Escape special FTS5 characters
		term = strings.ReplaceAll(term, `"`, `""`)
		fuzzyTerms[i] = term + "*"
	}
	fuzzyQuery := strings.Join(fuzzyTerms, " ")

	sql := `
		SELECT
			e.id, e.file_path, e.message_id, e.subject, e.sender, e.sender_name,
			e.recipients, e.cc, e.bcc, e.date, e.body_text, e.body_html,
			e.has_attachments, e.attachment_count, e.raw_headers, e.file_size,
			e.indexed_at, e.updated_at,
			snippet(emails_fts, 4, '<mark>', '</mark>', '...', 32) as snippet
		FROM emails e
		JOIN emails_fts ON e.id = emails_fts.rowid
		WHERE emails_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`

	rows, err := db.Query(sql, fuzzyQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search emails: %w", err)
	}
	defer rows.Close()

	var results []*EmailSearchResult
	for rows.Next() {
		result := &EmailSearchResult{}
		err := rows.Scan(
			&result.ID, &result.FilePath, &result.MessageID, &result.Subject, &result.Sender, &result.SenderName,
			&result.Recipients, &result.CC, &result.BCC, &result.Date, &result.BodyText, &result.BodyHTML,
			&result.HasAttachments, &result.AttachmentCount, &result.RawHeaders, &result.FileSize,
			&result.IndexedAt, &result.UpdatedAt,
			&result.Snippet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// SearchEmailsWithFilters performs a search with additional filters
func (db *DB) SearchEmailsWithFilters(query, sender string, hasAttachments bool, dateFrom, dateTo string, limit int) ([]*EmailSearchResult, error) {
	return db.SearchEmailsWithFiltersAndOffset(query, sender, hasAttachments, dateFrom, dateTo, limit, 0)
}

// SearchEmailsWithFiltersAndOffset performs a search with additional filters and pagination
func (db *DB) SearchEmailsWithFiltersAndOffset(query, sender string, hasAttachments bool, dateFrom, dateTo string, limit, offset int) ([]*EmailSearchResult, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}

	// FTS5 search
	if query != "" {
		terms := strings.Fields(query)
		fuzzyTerms := make([]string, len(terms))
		for i, term := range terms {
			term = strings.ReplaceAll(term, `"`, `""`)
			fuzzyTerms[i] = term + "*"
		}
		fuzzyQuery := strings.Join(fuzzyTerms, " ")
		conditions = append(conditions, "emails_fts MATCH ?")
		args = append(args, fuzzyQuery)
	}

	// Sender filter
	if sender != "" {
		conditions = append(conditions, "e.sender LIKE ?")
		args = append(args, "%"+sender+"%")
	}

	// Attachments filter
	if hasAttachments {
		conditions = append(conditions, "e.has_attachments = 1")
	}

	// Date range filters
	if dateFrom != "" {
		conditions = append(conditions, "e.date >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conditions = append(conditions, "e.date <= ?")
		args = append(args, dateTo)
	}

	// Build SQL query
	sqlQuery := `
		SELECT
			e.id, e.file_path, e.message_id, e.subject, e.sender, e.sender_name,
			e.recipients, e.cc, e.bcc, e.date, e.body_text, e.body_html,
			e.has_attachments, e.attachment_count, e.raw_headers, e.file_size,
			e.indexed_at, e.updated_at
	`

	var snippet string
	if query != "" {
		snippet = `, snippet(emails_fts, 4, '<mark>', '</mark>', '...', 32) as snippet`
		sqlQuery += snippet + `
		FROM emails e
		JOIN emails_fts ON e.id = emails_fts.rowid
		`
	} else {
		snippet = `, '' as snippet`
		sqlQuery += snippet + `
		FROM emails e
		`
	}

	if len(conditions) > 0 {
		sqlQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	if query != "" {
		sqlQuery += " ORDER BY rank"
	} else {
		sqlQuery += " ORDER BY e.date DESC"
	}

	sqlQuery += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search with filters: %w", err)
	}
	defer rows.Close()

	var results []*EmailSearchResult
	for rows.Next() {
		result := &EmailSearchResult{}
		err := rows.Scan(
			&result.ID, &result.FilePath, &result.MessageID, &result.Subject, &result.Sender, &result.SenderName,
			&result.Recipients, &result.CC, &result.BCC, &result.Date, &result.BodyText, &result.BodyHTML,
			&result.HasAttachments, &result.AttachmentCount, &result.RawHeaders, &result.FileSize,
			&result.IndexedAt, &result.UpdatedAt,
			&result.Snippet,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan filtered result: %w", err)
		}

		// Generate snippet if not from FTS5
		if result.Snippet == "" {
			result.Snippet = truncateText(result.BodyText, 200)
		}

		results = append(results, result)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating filtered results: %w", err)
	}

	return results, nil
}

// truncateText truncates text to maxLen characters
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
