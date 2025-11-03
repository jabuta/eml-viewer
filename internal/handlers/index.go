package handlers

import (
	"log"
	"net/http"
)

// Index handles the home page
func (h *Handlers) Index(w http.ResponseWriter, r *http.Request) {
	// Get email count
	count, err := h.db.CountEmails()
	if err != nil {
		http.Error(w, "Failed to get email count", http.StatusInternalServerError)
		return
	}

	// Get recent emails
	emails, err := h.db.ListEmails(50, 0)
	if err != nil {
		http.Error(w, "Failed to load emails", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]interface{}{
		"PageTitle": "Email List - EML Viewer",
		"Stats": map[string]interface{}{
			"TotalEmails": count,
		},
		"Emails": emails,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
		return
	}
}
