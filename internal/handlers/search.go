package handlers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// Search handles search requests with filters
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sender := r.URL.Query().Get("sender")
	hasAttachmentsParam := r.URL.Query().Get("has_attachments")
	dateFrom := r.URL.Query().Get("date_from")
	dateTo := r.URL.Query().Get("date_to")
	offsetParam := r.URL.Query().Get("offset")

	// Convert has_attachments to boolean
	hasAttachments := hasAttachmentsParam == "true" || hasAttachmentsParam == "1"

	// Parse offset
	offset := 0
	if offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil {
			offset = parsed
		}
	}

	// If no search query and no filters, show recent emails
	if query == "" && sender == "" && !hasAttachments && dateFrom == "" && dateTo == "" {
		h.Index(w, r)
		return
	}

	// Fetch one more than limit to check if there are more results
	limit := 50
	results, err := h.db.SearchEmailsWithFiltersAndOffset(query, sender, hasAttachments, dateFrom, dateTo, limit+1, offset)
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
				<p class="mt-2 text-sm text-gray-500">Try different search terms or adjust your filters</p>
			</div>
		`))
		return
	}

	// Check if there are more results
	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit] // Trim to actual limit
	}

	// Render each result using email-row template
	var buf bytes.Buffer
	for _, result := range results {
		if err := h.templates.ExecuteTemplate(&buf, "email-row", result); err != nil {
			log.Printf("Template error: %v", err)
			continue
		}
	}

	// Add "Load More" button if there are more results
	if hasMore {
		nextOffset := offset + limit
		loadMoreBtn := fmt.Sprintf(`
			<div class="flex justify-center mt-6" id="load-more-container">
				<button
					hx-get="/search?q=%s&sender=%s&has_attachments=%s&date_from=%s&date_to=%s&offset=%d"
					hx-target="#email-list"
					hx-swap="beforeend"
					hx-indicator="#search-spinner"
					class="px-6 py-3 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors shadow-sm"
				>
					Load More
				</button>
			</div>
		`, query, sender, hasAttachmentsParam, dateFrom, dateTo, nextOffset)
		buf.WriteString(loadMoreBtn)
	}

	w.Write(buf.Bytes())
}
