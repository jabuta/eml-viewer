package handlers

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ViewEmailHTML serves the raw HTML content of an email for iframe display
func (h *Handlers) ViewEmailHTML(w http.ResponseWriter, r *http.Request) {
	// Get email ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid email ID", http.StatusBadRequest)
		return
	}

	// Get email with full content
	emailWithContent, err := h.db.GetEmailWithFullContent(id)
	if err != nil {
		log.Printf("Error loading email HTML: %v", err)
		http.Error(w, "Failed to load email", http.StatusInternalServerError)
		return
	}
	if emailWithContent == nil {
		http.Error(w, "Email not found", http.StatusNotFound)
		return
	}

	// Return raw HTML content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Write([]byte(emailWithContent.BodyHTML))
}

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

	// Debug logging
	log.Printf("Email %d: BodyHTML length=%d, BodyText length=%d", id, len(emailWithContent.BodyHTML), len(emailWithContent.BodyText))
	if len(emailWithContent.BodyHTML) > 100 {
		log.Printf("First 100 chars of HTML: %s", emailWithContent.BodyHTML[:100])
	}

	// Prepare template data
	pageTitle := "Email - EML Viewer"
	if emailWithContent.Subject != "" {
		pageTitle = emailWithContent.Subject + " - EML Viewer"
	}

	// Create a map that includes both the email metadata and full content
	// Convert BodyHTML to template.HTML to prevent escaping
	data := map[string]interface{}{
		"PageTitle":   pageTitle,
		"Email":       emailWithContent.Email, // Just the metadata
		"BodyHTML":    template.HTML(emailWithContent.BodyHTML),
		"BodyText":    emailWithContent.BodyText,
		"CC":          emailWithContent.CC,
		"BCC":         emailWithContent.BCC,
		"RawHeaders":  emailWithContent.RawHeaders,
		"Attachments": emailWithContent.Attachments,
	}

	// Debug: verify data before template
	log.Printf("Template data: BodyHTML length=%d", len(emailWithContent.BodyHTML))

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "email.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}
