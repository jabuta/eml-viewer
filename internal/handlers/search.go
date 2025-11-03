package handlers

import (
	"bytes"
	"log"
	"net/http"
)

// Search handles search requests
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	// If empty query, show recent emails
	if query == "" {
		h.Index(w, r)
		return
	}

	// Perform search
	results, err := h.db.SearchEmails(query, 50)
	if err != nil {
		log.Printf("Search error: %v", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Return HTML fragment for HTMX (just the email rows)
	if len(results) == 0 {
		w.Write([]byte(`
			<div class="bg-white rounded-lg shadow-sm border border-gray-200 p-12 text-center">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>
				<h3 class="mt-4 text-lg font-medium text-gray-900">No emails found</h3>
				<p class="mt-2 text-sm text-gray-500">Try different search terms</p>
			</div>
		`))
		return
	}

	// Render each result using email-row template
	var buf bytes.Buffer
	for _, result := range results {
		if err := h.templates.ExecuteTemplate(&buf, "email-row", result); err != nil {
			log.Printf("Template error: %v", err)
			continue
		}
	}

	w.Write(buf.Bytes())
}
