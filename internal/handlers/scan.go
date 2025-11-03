package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/felo/eml-viewer/internal/indexer"
)

// ScanPage displays the scan page
func (h *Handlers) ScanPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"EmailsPath": h.cfg.EmailsPath,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "base.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// Scan handles manual re-scanning of emails
func (h *Handlers) Scan(w http.ResponseWriter, r *http.Request) {
	// Create indexer
	idx := indexer.NewIndexer(h.db, h.cfg.EmailsPath, false)

	// Run indexing
	result, err := idx.IndexAll()
	if err != nil {
		http.Error(w, fmt.Sprintf("Indexing failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return result
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
		<div class="bg-green-100 border border-green-400 text-green-700 px-4 py-3 rounded">
			<p class="font-bold">Scan Complete!</p>
			<p>Found: %d | New: %d | Skipped: %d | Failed: %d</p>
			<a href="/" class="underline mt-2 inline-block">Back to email list</a>
		</div>`,
		result.TotalFound,
		result.NewIndexed,
		result.Skipped,
		result.Failed,
	)
}
