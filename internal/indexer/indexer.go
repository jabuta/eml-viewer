package indexer

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/parser"
	"github.com/felo/eml-viewer/internal/scanner"
)

// Indexer handles email indexing operations
type Indexer struct {
	db      *db.DB
	scanner *scanner.Scanner
	verbose bool
}

// NewIndexer creates a new indexer
func NewIndexer(database *db.DB, emailsPath string, verbose bool) *Indexer {
	return &Indexer{
		db:      database,
		scanner: scanner.NewScanner(emailsPath),
		verbose: verbose,
	}
}

// IndexResult contains statistics about an indexing operation
type IndexResult struct {
	TotalFound  int
	NewIndexed  int
	Skipped     int
	Failed      int
	FailedFiles []string
}

// IndexAll scans and indexes all .eml files
func (idx *Indexer) IndexAll() (*IndexResult, error) {
	result := &IndexResult{}

	// Get all .eml files
	files, err := idx.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for files: %w", err)
	}

	result.TotalFound = len(files)

	if idx.verbose {
		log.Printf("Found %d .eml files to process\n", result.TotalFound)
	}

	// Process each file
	for i, filePath := range files {
		if idx.verbose && (i+1)%10 == 0 {
			log.Printf("Processing file %d/%d...\n", i+1, result.TotalFound)
		}

		// Check if already indexed
		exists, err := idx.db.EmailExists(filePath)
		if err != nil {
			log.Printf("Error checking if email exists: %v\n", err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		if exists {
			result.Skipped++
			continue
		}

		// Parse the email
		parsed, err := parser.ParseEMLFile(filePath)
		if err != nil {
			log.Printf("Error parsing %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Get file size
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Create email record
		email := &db.Email{
			FilePath:        filePath,
			MessageID:       parsed.MessageID,
			Subject:         parsed.Subject,
			Sender:          parsed.Sender,
			SenderName:      parsed.SenderName,
			Recipients:      strings.Join(parsed.Recipients, ", "),
			CC:              strings.Join(parsed.CC, ", "),
			BCC:             strings.Join(parsed.BCC, ", "),
			Date:            sql.NullTime{Time: parsed.Date, Valid: !parsed.Date.IsZero()},
			BodyText:        parsed.BodyText,
			BodyHTML:        parsed.BodyHTML,
			HasAttachments:  len(parsed.Attachments) > 0,
			AttachmentCount: len(parsed.Attachments),
			RawHeaders:      parsed.RawHeaders,
			FileSize:        fileInfo.Size(),
		}

		// Insert email
		emailID, err := idx.db.InsertEmail(email)
		if err != nil {
			log.Printf("Error inserting email %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Insert attachments
		for _, att := range parsed.Attachments {
			attachment := &db.Attachment{
				EmailID:     emailID,
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Size:        att.Size,
				Data:        att.Data,
			}

			_, err := idx.db.InsertAttachment(attachment)
			if err != nil {
				log.Printf("Error inserting attachment for email %s: %v\n", filePath, err)
				// Continue even if attachment insertion fails
			}
		}

		result.NewIndexed++
	}

	if idx.verbose {
		log.Printf("Indexing complete: %d new, %d skipped, %d failed\n",
			result.NewIndexed, result.Skipped, result.Failed)
	}

	return result, nil
}

// IndexWithProgress indexes all files and reports progress via a callback
func (idx *Indexer) IndexWithProgress(progress func(current, total int, filePath string)) (*IndexResult, error) {
	result := &IndexResult{}

	// Get all .eml files
	files, err := idx.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for files: %w", err)
	}

	result.TotalFound = len(files)

	// Process each file
	for i, filePath := range files {
		if progress != nil {
			progress(i+1, result.TotalFound, filePath)
		}

		// Check if already indexed
		exists, err := idx.db.EmailExists(filePath)
		if err != nil {
			log.Printf("Error checking if email exists: %v\n", err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		if exists {
			result.Skipped++
			continue
		}

		// Parse the email
		parsed, err := parser.ParseEMLFile(filePath)
		if err != nil {
			log.Printf("Error parsing %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Get file size
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Create email record
		email := &db.Email{
			FilePath:        filePath,
			MessageID:       parsed.MessageID,
			Subject:         parsed.Subject,
			Sender:          parsed.Sender,
			SenderName:      parsed.SenderName,
			Recipients:      strings.Join(parsed.Recipients, ", "),
			CC:              strings.Join(parsed.CC, ", "),
			BCC:             strings.Join(parsed.BCC, ", "),
			Date:            sql.NullTime{Time: parsed.Date, Valid: !parsed.Date.IsZero()},
			BodyText:        parsed.BodyText,
			BodyHTML:        parsed.BodyHTML,
			HasAttachments:  len(parsed.Attachments) > 0,
			AttachmentCount: len(parsed.Attachments),
			RawHeaders:      parsed.RawHeaders,
			FileSize:        fileInfo.Size(),
		}

		// Insert email
		emailID, err := idx.db.InsertEmail(email)
		if err != nil {
			log.Printf("Error inserting email %s: %v\n", filePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Insert attachments
		for _, att := range parsed.Attachments {
			attachment := &db.Attachment{
				EmailID:     emailID,
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Size:        att.Size,
				Data:        att.Data,
			}

			_, err := idx.db.InsertAttachment(attachment)
			if err != nil {
				log.Printf("Error inserting attachment for email %s: %v\n", filePath, err)
			}
		}

		result.NewIndexed++
	}

	return result, nil
}
