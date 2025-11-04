package db

import (
	"fmt"
	"strings"
)

// ConversationEmail represents an email with conversation metadata
type ConversationEmail struct {
	*Email
	Children    []*ConversationEmail // Child emails (replies)
	ReplyCount  int                  // Total number of replies in thread
	IsRootEmail bool                 // True if this is the start of a conversation
	ThreadDepth int                  // Depth in conversation tree (0 = root)
}

// GetRootEmails retrieves only emails that are not replies (root emails)
// These are emails where in_reply_to is empty or points to non-existent message
func (db *DB) GetRootEmails(limit, offset int) ([]*Email, error) {
	rows, err := db.Query(`
		SELECT id, file_path, message_id, in_reply_to, thread_references,
		       subject, sender, sender_name, recipients, date,
		       body_text_preview, has_attachments, attachment_count, file_size,
		       indexed_at, updated_at
		FROM emails
		WHERE in_reply_to IS NULL OR in_reply_to = ''
		   OR in_reply_to NOT IN (SELECT message_id FROM emails WHERE message_id IS NOT NULL AND message_id != '')
		ORDER BY date DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get root emails: %w", err)
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

// GetEmailsByMessageID retrieves an email by its Message-ID header
func (db *DB) GetEmailsByMessageID(messageID string) (*Email, error) {
	if messageID == "" {
		return nil, nil
	}

	email := &Email{}
	err := db.QueryRow(`
		SELECT id, file_path, message_id, in_reply_to, thread_references,
		       subject, sender, sender_name, recipients, date,
		       body_text_preview, has_attachments, attachment_count, file_size,
		       indexed_at, updated_at
		FROM emails
		WHERE message_id = ?
		LIMIT 1
	`, messageID).Scan(
		&email.ID, &email.FilePath, &email.MessageID, &email.InReplyTo, &email.ThreadReferences,
		&email.Subject, &email.Sender, &email.SenderName, &email.Recipients, &email.Date,
		&email.BodyTextPreview, &email.HasAttachments, &email.AttachmentCount, &email.FileSize,
		&email.IndexedAt, &email.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get email by message_id: %w", err)
	}

	return email, nil
}

// GetDirectReplies retrieves all emails that directly reply to the given message ID
func (db *DB) GetDirectReplies(messageID string) ([]*Email, error) {
	if messageID == "" {
		return []*Email{}, nil
	}

	rows, err := db.Query(`
		SELECT id, file_path, message_id, in_reply_to, thread_references,
		       subject, sender, sender_name, recipients, date,
		       body_text_preview, has_attachments, attachment_count, file_size,
		       indexed_at, updated_at
		FROM emails
		WHERE in_reply_to = ?
		ORDER BY date ASC
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get direct replies: %w", err)
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
			return nil, fmt.Errorf("failed to scan reply: %w", err)
		}
		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating replies: %w", err)
	}

	return emails, nil
}

// BuildConversationTree builds a nested conversation tree starting from a root email
// It recursively fetches all replies and organizes them in a tree structure
func (db *DB) BuildConversationTree(rootEmail *Email) (*ConversationEmail, error) {
	conv := &ConversationEmail{
		Email:       rootEmail,
		Children:    make([]*ConversationEmail, 0),
		IsRootEmail: true,
		ThreadDepth: 0,
		ReplyCount:  0,
	}

	// Recursively build the tree with circular reference protection and max depth
	visited := make(map[string]bool)
	maxDepth := 50
	if err := db.buildConversationTreeRecursive(conv, 1, visited, maxDepth); err != nil {
		return nil, err
	}

	return conv, nil
}

// buildConversationTreeRecursive recursively builds the conversation tree
func (db *DB) buildConversationTreeRecursive(parent *ConversationEmail, depth int, visited map[string]bool, maxDepth int) error {
	if parent.Email.MessageID == "" {
		return nil // Can't find replies without a message ID
	}

	// Prevent infinite recursion from circular references
	if visited[parent.Email.MessageID] {
		return nil // Already processed, skip to prevent cycles
	}

	// Prevent excessive depth
	if depth > maxDepth {
		return nil // Max depth reached
	}

	// Mark as visited
	visited[parent.Email.MessageID] = true

	// Get direct replies to this email
	replies, err := db.GetDirectReplies(parent.Email.MessageID)
	if err != nil {
		return err
	}

	// Process each reply
	for _, reply := range replies {
		childConv := &ConversationEmail{
			Email:       reply,
			Children:    make([]*ConversationEmail, 0),
			IsRootEmail: false,
			ThreadDepth: depth,
			ReplyCount:  0,
		}

		// Recursively build children for this reply
		if err := db.buildConversationTreeRecursive(childConv, depth+1, visited, maxDepth); err != nil {
			return err
		}

		// Add to parent's children
		parent.Children = append(parent.Children, childConv)

		// Update reply counts (this reply + all its descendants)
		parent.ReplyCount += 1 + childConv.ReplyCount
	}

	return nil
}

// GetConversationEmails gets all emails in a conversation (flat list)
// Starting from any email in the conversation, finds the root and returns all related emails
func (db *DB) GetConversationEmails(emailID int64) ([]*Email, error) {
	// Get the starting email
	email, err := db.GetEmailByID(emailID)
	if err != nil {
		return nil, err
	}
	if email == nil {
		return nil, fmt.Errorf("email not found")
	}

	// Find the root of the conversation
	root, err := db.findConversationRoot(email)
	if err != nil {
		return nil, err
	}

	// Get all emails in this conversation thread
	return db.getConversationEmailsRecursive(root.MessageID, make(map[string]bool))
}

// findConversationRoot finds the root email of a conversation
func (db *DB) findConversationRoot(email *Email) (*Email, error) {
	current := email

	// Keep following in_reply_to until we find the root
	for current.InReplyTo != "" {
		parent, err := db.GetEmailsByMessageID(current.InReplyTo)
		if err != nil || parent == nil {
			// Parent not found in database, current is the root we know about
			break
		}
		current = parent
	}

	return current, nil
}

// getConversationEmailsRecursive recursively collects all emails in a conversation
func (db *DB) getConversationEmailsRecursive(messageID string, visited map[string]bool) ([]*Email, error) {
	if messageID == "" || visited[messageID] {
		return []*Email{}, nil
	}

	visited[messageID] = true
	var result []*Email

	// Get this email
	email, err := db.GetEmailsByMessageID(messageID)
	if err != nil || email == nil {
		return []*Email{}, nil
	}
	result = append(result, email)

	// Get all direct replies
	replies, err := db.GetDirectReplies(messageID)
	if err != nil {
		return nil, err
	}

	// Recursively get replies to replies
	for _, reply := range replies {
		descendants, err := db.getConversationEmailsRecursive(reply.MessageID, visited)
		if err != nil {
			return nil, err
		}
		result = append(result, descendants...)
	}

	return result, nil
}

// CountReplies counts the number of direct and indirect replies to an email
func (db *DB) CountReplies(messageID string) (int, error) {
	if messageID == "" {
		return 0, nil
	}

	var count int
	err := db.QueryRow(`
		WITH RECURSIVE replies AS (
			-- Base case: direct replies
			SELECT id, message_id, in_reply_to
			FROM emails
			WHERE in_reply_to = ?

			UNION ALL

			-- Recursive case: replies to replies
			SELECT e.id, e.message_id, e.in_reply_to
			FROM emails e
			INNER JOIN replies r ON e.in_reply_to = r.message_id
		)
		SELECT COUNT(*) FROM replies
	`, messageID).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count replies: %w", err)
	}

	return count, nil
}

// GetRootEmailsWithReplyCounts retrieves root emails with their reply counts
func (db *DB) GetRootEmailsWithReplyCounts(limit, offset int) ([]*ConversationEmail, error) {
	rootEmails, err := db.GetRootEmails(limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]*ConversationEmail, 0, len(rootEmails))
	for _, email := range rootEmails {
		replyCount, err := db.CountReplies(email.MessageID)
		if err != nil {
			// Log error but don't fail, just set count to 0
			replyCount = 0
		}

		result = append(result, &ConversationEmail{
			Email:       email,
			Children:    nil, // Not building full tree, just counts
			ReplyCount:  replyCount,
			IsRootEmail: true,
			ThreadDepth: 0,
		})
	}

	return result, nil
}

// GetReferencesList parses the References header into a slice
func (e *Email) GetReferencesList() []string {
	if e.ThreadReferences == "" {
		return []string{}
	}
	refs := strings.Split(e.ThreadReferences, ",")
	result := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			result = append(result, ref)
		}
	}
	return result
}
