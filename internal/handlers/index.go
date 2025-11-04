package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/felo/eml-viewer/internal/db"
)

// Index handles the home page
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	// Parse offset parameter
	offsetParam := r.URL.Query().Get("offset")
	offset := 0
	if offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil {
			offset = parsed
		}
	}

	// Get email count
	count, err := h.db.CountEmails()
	if err != nil {
		http.Error(w, "Failed to get email count", http.StatusInternalServerError)
		return
	}

	// Fetch one more than limit to check if there are more results
	limit := 50
	emailList, err := h.db.ListEmails(limit+1, offset)
	if err != nil {
		log.Printf("Failed to load emails: %v", err)
		http.Error(w, "Failed to load emails", http.StatusInternalServerError)
		return
	}

	// Check if there are more results
	hasMore := len(emailList) > limit
	log.Printf("Index handler: offset=%d, fetched=%d emails, hasMore=%v", offset, len(emailList), hasMore)
	if hasMore {
		emailList = emailList[:limit] // Trim to actual limit
	}

	// Convert to EmailSearchResult for consistent template rendering
	emails := make([]*db.EmailSearchResult, len(emailList))
	for i, email := range emailList {
		emails[i] = &db.EmailSearchResult{
			Email:   *email,
			Snippet: "", // No snippet for non-search results
		}
	}

	// Prepare template data
	// Note: Sender/recipient autocomplete data is now loaded lazily via API endpoints
	// This eliminates expensive full-table scans on every page load
	data := map[string]interface{}{
		"PageTitle": "Email List - EML Viewer",
		"Stats": map[string]interface{}{
			"TotalEmails": count,
		},
		"Emails":     emails,
		"Senders":    []string{}, // Populated lazily via /api/autocomplete/senders
		"Recipients": []string{}, // Populated lazily via /api/autocomplete/recipients
		"HasMore":    hasMore,
		"NextOffset": offset + limit,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check if this is an HTMX request (pagination)
	isHTMX := r.Header.Get("HX-Request") == "true"

	if isHTMX && offset > 0 {
		// For pagination, return only email rows and the new Load More button
		for _, email := range emails {
			if err := h.templates.ExecuteTemplate(w, "email-row", email); err != nil {
				log.Printf("Template error: %v", err)
				continue
			}
		}

		// Calculate display count (cumulative)
		displayCount := offset + len(emails)

		// Replace the old Load More button with a new one (or remove if no more)
		if hasMore {
			loadMoreHTML := `<div class="flex justify-center mt-6" id="load-more-container" hx-swap-oob="true">
				<button hx-get="/?offset=` + strconv.Itoa(offset+limit) + `" hx-target="#email-list" hx-swap="beforeend" class="px-6 py-3 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors">Load More</button>
			</div>`
			w.Write([]byte(loadMoreHTML))
		} else {
			// Remove the Load More button if no more results
			w.Write([]byte(`<div id="load-more-container" hx-swap-oob="true"></div>`))
		}

		// Update the email counter using out-of-band swap
		counterHTML := `<div id="email-counter" class="mt-8 text-center text-sm text-gray-500" hx-swap-oob="true">
			Showing ` + strconv.Itoa(displayCount) + ` of ` + strconv.Itoa(count) + ` emails
		</div>`
		w.Write([]byte(counterHTML))

		return
	}

	// Render full page template
	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}
