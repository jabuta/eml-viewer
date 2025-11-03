package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ViewEmail handles displaying a single email
func (h *Handlers) ViewEmail(w http.ResponseWriter, r *http.Request) {
	// Get email ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid email ID", http.StatusBadRequest)
		return
	}

	// Get email with full content (parses from .eml file)
	emailWithContent, err := h.db.GetEmailWithFullContent(id)
	if err != nil {
		log.Printf("Error loading email with full content: %v", err)
		http.Error(w, "Failed to load email", http.StatusInternalServerError)
		return
	}
	if emailWithContent == nil {
		http.Error(w, "Email not found", http.StatusNotFound)
		return
	}

	// Prepare template data
	pageTitle := "Email - EML Viewer"
	if emailWithContent.Subject != "" {
		pageTitle = emailWithContent.Subject + " - EML Viewer"
	}

	data := map[string]interface{}{
		"PageTitle":   pageTitle,
		"Email":       emailWithContent,
		"Attachments": emailWithContent.Attachments,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "email.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}
