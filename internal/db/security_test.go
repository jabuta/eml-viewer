package db

import (
	"fmt"
	"strings"
	"testing"
)

// TestPathTraversal tests the path traversal protection
func TestPathTraversal(t *testing.T) {
	db := &DB{
		emailsPath: "/home/user/emails",
	}

	tests := []struct {
		name        string
		path        string
		shouldError bool
	}{
		{
			name:        "Valid relative path",
			path:        "inbox/test.eml",
			shouldError: false,
		},
		{
			name:        "Path traversal with ../",
			path:        "../../../etc/passwd",
			shouldError: true,
		},
		{
			name:        "Path traversal hidden in path",
			path:        "inbox/../../etc/shadow",
			shouldError: true,
		},
		{
			name:        "Absolute path",
			path:        "/etc/passwd",
			shouldError: true,
		},
		// Note: On Unix systems, backslashes are valid filename characters
		// This test would work on Windows where C: is detected as absolute
		{
			name:        "Valid file starting with dots",
			path:        "inbox/.hidden",
			shouldError: false, // Hidden files are valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := db.ResolveEmailPath(tt.path)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for path %q, got nil (resolved to %q)", tt.path, resolved)
				} else if err != ErrPathTraversal && !strings.Contains(err.Error(), "path traversal") {
					t.Errorf("Expected path traversal error, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for path %q, got: %v", tt.path, err)
				}
				// Verify the resolved path is within emailsPath
				if resolved != "" && !strings.HasPrefix(resolved, db.emailsPath) {
					t.Errorf("Resolved path %q is not within emails path %q", resolved, db.emailsPath)
				}
			}
		})
	}
}

// TestCircularReferenceDetection tests the conversation threading protection
func TestCircularReferenceDetection(t *testing.T) {
	// Create test database
	testDB := SetupTestDB(t)
	defer CleanupTestDB(t, testDB)

	// Insert emails with circular references
	email1 := &Email{
		FilePath:  "email1.eml",
		MessageID: "<msg1@example.com>",
		InReplyTo: "<msg2@example.com>", // Points to email2
		Subject:   "Email 1",
		Sender:    "test1@example.com",
	}

	email2 := &Email{
		FilePath:  "email2.eml",
		MessageID: "<msg2@example.com>",
		InReplyTo: "<msg1@example.com>", // Points back to email1 - circular!
		Subject:   "Email 2",
		Sender:    "test2@example.com",
	}

	_, err := testDB.InsertEmail(email1)
	if err != nil {
		t.Fatalf("Failed to insert email1: %v", err)
	}

	_, err = testDB.InsertEmail(email2)
	if err != nil {
		t.Fatalf("Failed to insert email2: %v", err)
	}

	// Try to find conversation root - should detect circular reference
	root, err := testDB.findConversationRoot(email1)

	if err == nil {
		t.Errorf("Expected circular reference error, got nil (root: %v)", root)
	}

	if err != nil && !strings.Contains(err.Error(), "circular reference") {
		t.Errorf("Expected 'circular reference' error, got: %v", err)
	}
}

// TestMaxHopsProtection tests that the max hops limit prevents infinite loops
func TestMaxHopsProtection(t *testing.T) {
	testDB := SetupTestDB(t)
	defer CleanupTestDB(t, testDB)

	// Create a very long chain (more than maxHops)
	prevMessageID := ""
	for i := 0; i < 150; i++ {
		email := &Email{
			FilePath:  fmt.Sprintf("email%d.eml", i),
			MessageID: fmt.Sprintf("<msg%d@example.com>", i),
			InReplyTo: prevMessageID,
			Subject:   "Email in chain",
			Sender:    "test@example.com",
		}

		_, err := testDB.InsertEmail(email)
		if err != nil {
			t.Fatalf("Failed to insert email %d: %v", i, err)
		}

		prevMessageID = email.MessageID
	}

	// Get the last email and try to find root
	lastEmail, err := testDB.GetEmailsByMessageID(prevMessageID)
	if err != nil || lastEmail == nil {
		t.Fatalf("Failed to get last email: %v", err)
	}

	// Should complete without hanging (protected by maxHops)
	root, err := testDB.findConversationRoot(lastEmail)

	// Should succeed and return some email (limited by maxHops)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if root == nil {
		t.Errorf("Expected root email, got nil")
	}
}
