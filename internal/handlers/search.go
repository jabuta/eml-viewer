package handlers

import (
	"fmt"
	"html"
	"log"
	"net/http"
)

// Search handles search requests
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	// Perform search
	results, err := h.db.SearchEmails(query, 50)
	if err != nil {
		log.Printf("Search error: %v", err)
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Return HTML fragment for HTMX
	if len(results) == 0 {
		fmt.Fprintf(w, `
			<div class="text-center py-8 text-gray-500">
				<p>No emails found</p>
			</div>`)
		return
	}

	for _, result := range results {
		subject := result.Subject
		if subject == "" {
			subject = "(No Subject)"
		}

		snippet := result.Snippet
		if snippet == "" {
			snippet = truncate(result.BodyText, 150)
		}

		fmt.Fprintf(w, `
			<div class="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
				<a href="/emails/%d" class="block">
					<div class="flex justify-between items-start mb-2">
						<h3 class="text-lg font-semibold text-gray-900">%s</h3>
						<span class="text-sm text-gray-500">%s</span>
					</div>
					<p class="text-sm text-gray-600 mb-2">From: %s</p>
					<p class="text-sm text-gray-500 line-clamp-2">%s</p>
				</a>
			</div>`,
			result.ID,
			html.EscapeString(subject),
			result.GetDate().Format("Jan 2, 2006 15:04"),
			html.EscapeString(result.Sender),
			snippet, // Already contains HTML marks for highlighting
		)
	}
}
