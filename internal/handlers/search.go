package handlers

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/felo/eml-viewer/internal/db"
)

// Search handles search requests with filters
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	sender := r.URL.Query().Get("sender")
	recipient := r.URL.Query().Get("recipient")
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

	// Fetch one more than limit to check if there are more results
	limit := 50

	var results []*db.EmailSearchResult
	var err error

	// If no search query and no filters, get recent emails
	if query == "" && sender == "" && recipient == "" && !hasAttachments && dateFrom == "" && dateTo == "" {
		emails, err := h.db.ListEmails(limit+1, offset)
		if err != nil {
			log.Printf("Failed to list emails: %v", err)
			http.Error(w, "Failed to load emails", http.StatusInternalServerError)
			return
		}

		// Convert to EmailSearchResult for consistent rendering
		results = make([]*db.EmailSearchResult, len(emails))
		for i, email := range emails {
			results[i] = &db.EmailSearchResult{
				Email:   *email,
				Snippet: "",
			}
		}
	} else {
		results, err = h.db.SearchEmailsWithFiltersAndOffset(query, sender, recipient, hasAttachments, dateFrom, dateTo, limit+1, offset)
	}
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

	// Calculate total count for the counter
	// If filters are applied, count only filtered results
	var totalCount int
	if query != "" || sender != "" || recipient != "" || hasAttachments || dateFrom != "" || dateTo != "" {
		totalCount, err = h.db.CountFilteredEmails(query, sender, recipient, hasAttachments, dateFrom, dateTo)
		if err != nil {
			log.Printf("Failed to get filtered count: %v", err)
			totalCount = 0
		}
	} else {
		// No filters, use total email count
		totalCount, err = h.db.CountEmails()
		if err != nil {
			log.Printf("Failed to get total count: %v", err)
			totalCount = 0
		}
	}

	// Current display count (offset + results shown)
	displayCount := offset + len(results)

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

		// Build the Load More URL - use /search endpoint to avoid full page reload
		var loadMoreURL string
		if query == "" && sender == "" && recipient == "" && !hasAttachments && dateFrom == "" && dateTo == "" {
			// For no filters, still use /search but with empty params to get email-row fragments
			loadMoreURL = fmt.Sprintf("/search?offset=%d", nextOffset)
		} else {
			loadMoreURL = fmt.Sprintf("/search?q=%s&sender=%s&recipient=%s&has_attachments=%s&date_from=%s&date_to=%s&offset=%d",
				query, sender, recipient, hasAttachmentsParam, dateFrom, dateTo, nextOffset)
		}

		loadMoreBtn := fmt.Sprintf(`
			<div class="flex justify-center mt-6" id="load-more-container" hx-swap-oob="true">
				<button
					hx-get="%s"
					hx-target="#email-list"
					hx-swap="beforeend"
					hx-indicator="#search-spinner"
					class="px-6 py-3 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors shadow-sm"
					data-current-count="%d"
				>
					Load More
				</button>
			</div>
		`, loadMoreURL, displayCount)
		buf.WriteString(loadMoreBtn)
	} else {
		// No more results - remove the Load More button
		buf.WriteString(`<div id="load-more-container" hx-swap-oob="true"></div>`)
	}

	// Add email counter update using out-of-band swap
	// displayCount already includes offset + current batch, so it's cumulative
	counterHTML := fmt.Sprintf(`
		<div id="email-counter" class="mt-8 text-center text-sm text-gray-500" hx-swap-oob="true">
			Showing %d of %d emails
		</div>
	`, displayCount, totalCount)
	buf.WriteString(counterHTML)

	w.Write(buf.Bytes())
}
