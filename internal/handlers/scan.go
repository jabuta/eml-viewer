package handlers

import (
	"fmt"
	"net/http"

	"github.com/felo/eml-viewer/internal/indexer"
)

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
		</div>`,
		result.TotalFound,
		result.NewIndexed,
		result.Skipped,
		result.Failed,
	)
}
