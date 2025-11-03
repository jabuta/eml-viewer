package indexer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/parser"
	"github.com/felo/eml-viewer/internal/scanner"
)

// Indexer handles email indexing operations
type Indexer struct {
	db          *db.DB
	scanner     *scanner.Scanner
	verbose     bool
	concurrency int           // Number of concurrent workers
	batchSize   int           // Number of emails to batch before writing
	flushTime   time.Duration // Maximum time to wait before flushing batch
}

// parsedEmail holds a parsed email with its attachments ready for batching
type parsedEmail struct {
	email       *db.Email
	attachments []parser.ParsedAttachment
	filePath    string
}

// batchWriteResult holds the result of processing a batch write
type batchWriteResult struct {
	indexed     int
	failed      int
	failedFiles []string
}

// NewIndexer creates a new indexer
func NewIndexer(database *db.DB, emailsPath string, verbose bool) *Indexer {
	return &Indexer{
		db:          database,
		scanner:     scanner.NewScanner(emailsPath),
		verbose:     verbose,
		concurrency: runtime.NumCPU() * 2,   // 2x CPUs for optimal I/O parallelism
		batchSize:   50,                     // Batch 50 emails at a time
		flushTime:   500 * time.Millisecond, // Flush every 500ms if batch not full
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

// indexAllConcurrent indexes files using a worker pool with batch writes
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

	// Check which files already exist in the database (batch check)
	existingFiles, err := idx.db.EmailsExistBatch(files)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing emails: %w", err)
	}

	// Filter out existing files
	filesToProcess := make([]string, 0, len(files))
	for _, file := range files {
		if existingFiles[file] {
			result.Skipped++
		} else {
			filesToProcess = append(filesToProcess, file)
		}
	}

	if idx.verbose && result.Skipped > 0 {
		log.Printf("Skipping %d already indexed files\n", result.Skipped)
	}

	if len(filesToProcess) == 0 {
		if idx.verbose {
			log.Printf("No new files to index\n")
		}
		return result, nil
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(filesToProcess))
	parsedChan := make(chan indexResult, idx.concurrency)
	batchChan := make(chan *parsedEmail, idx.concurrency*2)

	// Start worker pool for parsing
	var parseWg sync.WaitGroup
	for i := 0; i < idx.concurrency; i++ {
		parseWg.Add(1)
		go idx.parseWorker(&parseWg, fileChan, parsedChan, batchChan)
	}

	// Start batch writer
	var batchWg sync.WaitGroup
	batchWg.Add(1)
	batchResultChan := make(chan batchWriteResult, 10)
	go idx.batchWriter(&batchWg, batchChan, batchResultChan)

	// Send files to workers
	for _, file := range filesToProcess {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all parsers to finish, then close batch channel
	go func() {
		parseWg.Wait()
		close(batchChan)
	}()

	// Wait for batch writer to finish
	go func() {
		batchWg.Wait()
		close(batchResultChan)
		close(parsedChan)
	}()

	// Collect parse results (for skipped/failed tracking)
	processedCount := 0
	go func() {
		for res := range parsedChan {
			processedCount++
			if idx.verbose && processedCount%10 == 0 {
				log.Printf("Processing file %d/%d...\n", processedCount, len(filesToProcess))
			}

			switch res.status {
			case statusSkipped:
				result.Skipped++
			case statusFailed:
				result.Failed++
				result.FailedFiles = append(result.FailedFiles, res.filePath)
			}
		}
	}()

	// Collect batch write results
	for batchRes := range batchResultChan {
		result.NewIndexed += batchRes.indexed
		result.Failed += batchRes.failed
		result.FailedFiles = append(result.FailedFiles, batchRes.failedFiles...)
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

// parseWorker reads and parses EML files, sending parsed data to batch writer
func (idx *Indexer) parseWorker(wg *sync.WaitGroup, fileChan <-chan string, resultChan chan<- indexResult, batchChan chan<- *parsedEmail) {
	defer wg.Done()

	for filePath := range fileChan {
		// Resolve relative path to absolute path (scanner returns relative paths)
		absolutePath := filepath.Join(idx.scanner.GetRootPath(), filePath)

		// Parse the email
		parsed, err := parser.ParseEMLFile(absolutePath)
		if err != nil {
			log.Printf("Error parsing %s: %v\n", absolutePath, err)
			resultChan <- indexResult{
				filePath: filePath,
				status:   statusFailed,
			}
			continue
		}

		// Get file size
		fileInfo, err := os.Stat(absolutePath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v\n", absolutePath, err)
			resultChan <- indexResult{
				filePath: filePath,
				status:   statusFailed,
			}
			continue
		}

		// Create email record (metadata only, truncate body text to 10KB for FTS5)
		bodyTextPreview := parsed.BodyText
		if len(bodyTextPreview) > 10240 {
			bodyTextPreview = bodyTextPreview[:10240]
		}

		email := &db.Email{
			FilePath:        filePath,
			MessageID:       parsed.MessageID,
			Subject:         parsed.Subject,
			Sender:          parsed.Sender,
			SenderName:      parsed.SenderName,
			Recipients:      strings.Join(parsed.Recipients, ", "),
			Date:            db.NullTime{Time: parsed.Date, Valid: !parsed.Date.IsZero()},
			BodyTextPreview: bodyTextPreview,
			HasAttachments:  len(parsed.Attachments) > 0,
			AttachmentCount: len(parsed.Attachments),
			FileSize:        fileInfo.Size(),
		}

		// Send to batch writer
		batchChan <- &parsedEmail{
			email:       email,
			attachments: parsed.Attachments,
			filePath:    filePath,
		}

		// Signal successful parse (for progress tracking)
		resultChan <- indexResult{
			filePath: filePath,
			status:   statusIndexed,
		}
	}
}

// batchWriter collects parsed emails and writes them in batches
func (idx *Indexer) batchWriter(wg *sync.WaitGroup, batchChan <-chan *parsedEmail, resultChan chan<- batchWriteResult) {
	defer wg.Done()

	batch := make([]*parsedEmail, 0, idx.batchSize)
	ticker := time.NewTicker(idx.flushTime)
	defer ticker.Stop()

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		result := idx.writeBatch(batch)
		resultChan <- result

		// Reset batch
		batch = make([]*parsedEmail, 0, idx.batchSize)
	}

	for {
		select {
		case parsed, ok := <-batchChan:
			if !ok {
				// Channel closed, flush remaining and exit
				flushBatch()
				return
			}

			batch = append(batch, parsed)

			// Flush if batch is full
			if len(batch) >= idx.batchSize {
				flushBatch()
			}

		case <-ticker.C:
			// Periodic flush
			flushBatch()
		}
	}
}

// writeBatch writes a batch of emails to the database
func (idx *Indexer) writeBatch(batch []*parsedEmail) batchWriteResult {
	result := batchWriteResult{
		failedFiles: make([]string, 0),
	}

	if len(batch) == 0 {
		return result
	}

	// Extract emails for batch insert
	emails := make([]*db.Email, len(batch))
	for i, p := range batch {
		emails[i] = p.email
	}

	// Batch insert emails
	emailIDs, err := idx.db.InsertEmailsBatch(emails)
	if err != nil {
		log.Printf("Error batch inserting emails: %v\n", err)
		// Mark all as failed
		result.failed = len(batch)
		for _, p := range batch {
			result.failedFiles = append(result.failedFiles, p.filePath)
		}
		return result
	}

	result.indexed = len(emailIDs)

	// Collect all attachments for batch insert (metadata only, no BLOB data)
	var allAttachments []*db.Attachment
	for i, p := range batch {
		if len(p.attachments) > 0 {
			emailID := emailIDs[i]
			for _, att := range p.attachments {
				allAttachments = append(allAttachments, &db.Attachment{
					EmailID:     emailID,
					Filename:    att.Filename,
					ContentType: att.ContentType,
					Size:        att.Size,
				})
			}
		}
	}

	// Batch insert attachments if any
	if len(allAttachments) > 0 {
		if err := idx.db.InsertAttachmentsBatch(allAttachments); err != nil {
			log.Printf("Error batch inserting attachments: %v\n", err)
			// Don't fail the whole batch, just log the error
		}
	}

	if idx.verbose {
		log.Printf("Batch wrote %d emails with %d attachments\n", len(batch), len(allAttachments))
	}

	return result
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

		// Resolve relative path to absolute path (scanner returns relative paths)
		absolutePath := filepath.Join(idx.scanner.GetRootPath(), filePath)

		// Parse the email
		parsed, err := parser.ParseEMLFile(absolutePath)
		if err != nil {
			log.Printf("Error parsing %s: %v\n", absolutePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Get file size
		fileInfo, err := os.Stat(absolutePath)
		if err != nil {
			log.Printf("Error getting file info for %s: %v\n", absolutePath, err)
			result.Failed++
			result.FailedFiles = append(result.FailedFiles, filePath)
			continue
		}

		// Create email record (metadata only, truncate body text to 10KB for FTS5)
		bodyTextPreview := parsed.BodyText
		if len(bodyTextPreview) > 10240 {
			bodyTextPreview = bodyTextPreview[:10240]
		}

		email := &db.Email{
			FilePath:        filePath,
			MessageID:       parsed.MessageID,
			Subject:         parsed.Subject,
			Sender:          parsed.Sender,
			SenderName:      parsed.SenderName,
			Recipients:      strings.Join(parsed.Recipients, ", "),
			Date:            db.NullTime{Time: parsed.Date, Valid: !parsed.Date.IsZero()},
			BodyTextPreview: bodyTextPreview,
			HasAttachments:  len(parsed.Attachments) > 0,
			AttachmentCount: len(parsed.Attachments),
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

		// Insert attachments (metadata only, no BLOB data)
		for _, att := range parsed.Attachments {
			attachment := &db.Attachment{
				EmailID:     emailID,
				Filename:    att.Filename,
				ContentType: att.ContentType,
				Size:        att.Size,
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

	// Check which files already exist in the database (batch check)
	existingFiles, err := idx.db.EmailsExistBatch(files)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing emails: %w", err)
	}

	// Filter out existing files
	filesToProcess := make([]string, 0, len(files))
	for _, file := range files {
		if existingFiles[file] {
			result.Skipped++
		} else {
			filesToProcess = append(filesToProcess, file)
		}
	}

	if len(filesToProcess) == 0 {
		return result, nil
	}

	// Create channels for work distribution
	fileChan := make(chan string, len(filesToProcess))
	parsedChan := make(chan indexResult, idx.concurrency)
	batchChan := make(chan *parsedEmail, idx.concurrency*2)

	// Start worker pool for parsing
	var parseWg sync.WaitGroup
	for i := 0; i < idx.concurrency; i++ {
		parseWg.Add(1)
		go idx.parseWorker(&parseWg, fileChan, parsedChan, batchChan)
	}

	// Start batch writer
	var batchWg sync.WaitGroup
	batchWg.Add(1)
	batchResultChan := make(chan batchWriteResult, 10)
	go idx.batchWriter(&batchWg, batchChan, batchResultChan)

	// Send files to workers
	for _, file := range filesToProcess {
		fileChan <- file
	}
	close(fileChan)

	// Wait for all parsers to finish, then close channels
	go func() {
		parseWg.Wait()
		close(batchChan)
		close(parsedChan) // Close parsedChan after all workers are done
	}()

	// Wait for batch writer to finish, then close result channel
	go func() {
		batchWg.Wait()
		close(batchResultChan)
	}()

	// Collect parse results with progress reporting (in separate goroutine)
	processedCount := 0
	parseDone := make(chan struct{})
	go func() {
		for res := range parsedChan {
			processedCount++
			if progress != nil {
				progress(processedCount, len(filesToProcess), res.filePath)
			}

			switch res.status {
			case statusSkipped:
				result.Skipped++
			case statusFailed:
				result.Failed++
				result.FailedFiles = append(result.FailedFiles, res.filePath)
			}
		}
		close(parseDone)
	}()

	// Collect batch write results
	for batchRes := range batchResultChan {
		result.NewIndexed += batchRes.indexed
		result.Failed += batchRes.failed
		result.FailedFiles = append(result.FailedFiles, batchRes.failedFiles...)
	}

	// Wait for parse result collection to finish
	<-parseDone

	return result, nil
}
