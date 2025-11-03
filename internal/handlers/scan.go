package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/indexer"
)

// ScanProgress holds the current scan progress state
type ScanProgress struct {
	mu              sync.RWMutex
	isScanning      bool
	current         int
	total           int
	currentFile     string
	totalFound      int
	newIndexed      int
	skipped         int
	failed          int
	completed       bool
	err             error
	lastUpdate      time.Time
	progressClients []chan ProgressEvent
}

// ProgressEvent represents a progress update event
type ProgressEvent struct {
	Type string      `json:"type"` // "progress", "complete", "error"
	Data interface{} `json:"data"`
}

var (
	scanProgress = &ScanProgress{
		progressClients: make([]chan ProgressEvent, 0),
	}
)

// ScanPage displays the scan page
func (h *Handlers) ScanPage(w http.ResponseWriter, r *http.Request) {
	// Get current stats
	stats, err := h.db.GetStats()
	if err != nil {
		log.Printf("Error getting stats: %v", err)
		stats = &db.Stats{} // Use empty stats on error
	}

	// Format last indexed time
	var lastIndexed string
	if !stats.LastIndexed.IsZero() {
		lastIndexed = stats.LastIndexed.Format("Jan 2, 2006 3:04 PM")
	} else {
		lastIndexed = "Never"
	}

	data := map[string]interface{}{
		"EmailsPath": h.cfg.EmailsPath,
		"Stats": map[string]interface{}{
			"TotalEmails":     stats.TotalEmails,
			"WithAttachments": stats.WithAttachments,
			"LastIndexed":     lastIndexed,
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// Scan handles manual re-scanning of emails
func (h *Handlers) Scan(w http.ResponseWriter, r *http.Request) {
	scanProgress.mu.Lock()
	if scanProgress.isScanning {
		scanProgress.mu.Unlock()
		http.Error(w, "Scan already in progress", http.StatusConflict)
		return
	}

	// Reset progress state
	scanProgress.isScanning = true
	scanProgress.current = 0
	scanProgress.total = 0
	scanProgress.currentFile = ""
	scanProgress.totalFound = 0
	scanProgress.newIndexed = 0
	scanProgress.skipped = 0
	scanProgress.failed = 0
	scanProgress.completed = false
	scanProgress.err = nil
	scanProgress.lastUpdate = time.Now()
	scanProgress.mu.Unlock()

	// Run scan in background
	go func() {
		defer func() {
			scanProgress.mu.Lock()
			scanProgress.isScanning = false
			scanProgress.completed = true
			scanProgress.mu.Unlock()
		}()

		// Create indexer
		idx := indexer.NewIndexer(h.db, h.cfg.EmailsPath, false)

		// Run indexing with progress callback
		result, err := idx.IndexWithProgress(func(current, total int, filePath string) {
			scanProgress.mu.Lock()
			scanProgress.current = current
			scanProgress.total = total
			scanProgress.currentFile = filePath
			scanProgress.lastUpdate = time.Now()
			scanProgress.mu.Unlock()

			// Broadcast to all SSE clients
			scanProgress.broadcastProgress()
		})

		scanProgress.mu.Lock()
		if err != nil {
			scanProgress.err = err
			scanProgress.mu.Unlock()
			scanProgress.broadcastError(err)
			return
		}

		// Update final stats
		scanProgress.totalFound = result.TotalFound
		scanProgress.newIndexed = result.NewIndexed
		scanProgress.skipped = result.Skipped
		scanProgress.failed = result.Failed
		scanProgress.mu.Unlock()

		// Broadcast completion
		scanProgress.broadcastComplete(result)
	}()

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, "Scan started")
}

// ScanProgressSSE handles Server-Sent Events for scan progress
func (h *Handlers) ScanProgressSSE(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create a channel for this client
	clientChan := make(chan ProgressEvent, 10)

	// Register client
	scanProgress.mu.Lock()
	scanProgress.progressClients = append(scanProgress.progressClients, clientChan)

	// Send initial state if scan is in progress
	if scanProgress.isScanning {
		initialData := map[string]interface{}{
			"current": scanProgress.current,
			"total":   scanProgress.total,
			"file":    scanProgress.currentFile,
			"stats": map[string]int{
				"found":   scanProgress.totalFound,
				"new":     scanProgress.newIndexed,
				"skipped": scanProgress.skipped,
				"failed":  scanProgress.failed,
			},
		}
		sendSSE(w, flusher, "progress", initialData)
	}
	scanProgress.mu.Unlock()

	// Listen for updates or client disconnect
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected, clean up
			scanProgress.mu.Lock()
			for i, ch := range scanProgress.progressClients {
				if ch == clientChan {
					scanProgress.progressClients = append(scanProgress.progressClients[:i], scanProgress.progressClients[i+1:]...)
					break
				}
			}
			scanProgress.mu.Unlock()
			close(clientChan)
			return

		case event := <-clientChan:
			sendSSE(w, flusher, event.Type, event.Data)

			// Close connection after complete or error
			if event.Type == "complete" || event.Type == "error" {
				return
			}
		}
	}
}

// broadcastProgress sends progress update to all connected clients
func (sp *ScanProgress) broadcastProgress() {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	data := map[string]interface{}{
		"current": sp.current,
		"total":   sp.total,
		"file":    sp.currentFile,
		"stats": map[string]int{
			"found":   sp.totalFound,
			"new":     sp.newIndexed,
			"skipped": sp.skipped,
			"failed":  sp.failed,
		},
	}

	event := ProgressEvent{
		Type: "progress",
		Data: data,
	}

	for _, client := range sp.progressClients {
		select {
		case client <- event:
		default:
			// Client channel full, skip
		}
	}
}

// broadcastComplete sends completion event to all connected clients
func (sp *ScanProgress) broadcastComplete(result *indexer.IndexResult) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	data := map[string]interface{}{
		"found":   result.TotalFound,
		"new":     result.NewIndexed,
		"skipped": result.Skipped,
		"failed":  result.Failed,
	}

	event := ProgressEvent{
		Type: "complete",
		Data: data,
	}

	for _, client := range sp.progressClients {
		select {
		case client <- event:
		default:
		}
	}
}

// broadcastError sends error event to all connected clients
func (sp *ScanProgress) broadcastError(err error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	data := map[string]interface{}{
		"error": err.Error(),
	}

	event := ProgressEvent{
		Type: "error",
		Data: data,
	}

	for _, client := range sp.progressClients {
		select {
		case client <- event:
		default:
		}
	}
}

// sendSSE sends an SSE message to the client
func sendSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling SSE data: %v", err)
		return
	}

	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}
