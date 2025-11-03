package indexer

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/parser"
	"github.com/felo/eml-viewer/internal/scanner"
)

// Indexer handles email indexing operations
type Indexer struct {
	db          *db.DB
	scanner     *scanner.Scanner
	verbose     bool
	concurrency int // Number of concurrent workers
}

// NewIndexer creates a new indexer
func NewIndexer(database *db.DB, emailsPath string, verbose bool) *Indexer {
	return &Indexer{
		db:          database,
		scanner:     scanner.NewScanner(emailsPath),
		verbose:     verbose,
		concurrency: runtime.NumCPU() * 2, // 2x CPUs for optimal I/O parallelism
	}
}

// WithConcurrency sets the number of concurrent workers
func (idx *Indexer) WithConcurrency(workers int) *Indexer {
	if workers < 1 {
		workers = 1
	}
	idx.concurrency = workers
	return idx
}

// IndexResult contains statistics about an indexing operation
type IndexResult struct {
	TotalFound  int
	NewIndexed  int
	Skipped     int
	Failed      int
	FailedFiles []string
}

// IndexAll scans and indexes all .eml files using concurrent workers
func (idx *Indexer) IndexAll() (*IndexResult, error) {
	return idx.indexAllConcurrent()
}

// indexAllConcurrent indexes files using a worker pool
func (idx *Indexer) indexAllConcurrent() (*IndexResult, error) {
	// Get all .eml files
	files, err := idx.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for files: %w", err)
	}

	result := &IndexResult{
		TotalFound:  len(files),
		FailedFiles: make([]string, 0),
	}

	if idx.verbose {
		log.Printf("Found %d .eml files to process with %d workers\n", result.TotalFound, idx.concurrency)
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(files))
	resultChan := make(chan indexResult, len(files))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < idx.concurrency; i++ {
		wg.Add(1)
		go idx.indexWorker(&wg, fileChan, resultChan)
	}

	// Send files to workers
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	processedCount := 0
	for res := range resultChan {
		processedCount++
		if idx.verbose && processedCount%10 == 0 {
			log.Printf("Processing file %d/%d...\n", processedCount, result.TotalFound)
		}

		switch res.status {
		case statusIndexed:
			result.NewIndexed++
		case statusSkipped:
			result.Skipped++
		case statusFailed:
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, res.filePath)
		}
	}

	if idx.verbose {
		log.Printf("Indexing complete: %d new, %d skipped, %d failed\n",
			result.NewIndexed, result.Skipped, result.Failed)
	}

	return result, nil
}

type indexStatus int

const (
	statusIndexed indexStatus = iota
	statusSkipped
	statusFailed
)

type indexResult struct {
	filePath string
	status   indexStatus
}

// indexWorker processes files from the file channel
func (idx *Indexer) indexWorker(wg *sync.WaitGroup, fileChan <-chan string, resultChan chan<- indexResult) {
	defer wg.Done()

	for filePath := range fileChan {
		status := idx.processFile(filePath)
		resultChan <- indexResult{
			filePath: filePath,
			status:   status,
		}
	}
}

// processFile processes a single file and returns its status
func (idx *Indexer) processFile(filePath string) indexStatus {
	// Check if already indexed
	exists, err := idx.db.EmailExists(filePath)
	if err != nil {
		log.Printf("Error checking if email exists: %v\n", err)
		return statusFailed
	}

	if exists {
		return statusSkipped
	}

	// Parse the email
	parsed, err := parser.ParseEMLFile(filePath)
	if err != nil {
		log.Printf("Error parsing %s: %v\n", filePath, err)
		return statusFailed
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Error getting file info for %s: %v\n", filePath, err)
		return statusFailed
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
		return statusFailed
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

	return statusIndexed
}

// indexAllSequential scans and indexes all .eml files sequentially (old implementation)
func (idx *Indexer) indexAllSequential() (*IndexResult, error) {
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
	// Get all .eml files
	files, err := idx.scanner.Scan()
	if err != nil {
		return nil, fmt.Errorf("failed to scan for files: %w", err)
	}

	result := &IndexResult{
		TotalFound:  len(files),
		FailedFiles: make([]string, 0),
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(files))
	resultChan := make(chan indexResult, len(files))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < idx.concurrency; i++ {
		wg.Add(1)
		go idx.indexWorker(&wg, fileChan, resultChan)
	}

	// Send files to workers
	for _, file := range files {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results with progress reporting
	processedCount := 0
	for res := range resultChan {
		processedCount++
		if progress != nil {
			progress(processedCount, result.TotalFound, res.filePath)
		}

		switch res.status {
		case statusIndexed:
			result.NewIndexed++
		case statusSkipped:
			result.Skipped++
		case statusFailed:
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, res.filePath)
		}
	}

	return result, nil
}
