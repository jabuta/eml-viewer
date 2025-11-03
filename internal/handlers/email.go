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

	// Get email from database
	email, err := h.db.GetEmailByID(id)
	if err != nil {
		http.Error(w, "Failed to load email", http.StatusInternalServerError)
		return
	}
	if email == nil {
		http.Error(w, "Email not found", http.StatusNotFound)
		return
	}

	// Get attachments
	attachments, err := h.db.GetAttachmentsByEmailID(id)
	if err != nil {
		http.Error(w, "Failed to load attachments", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	pageTitle := "Email - EML Viewer"
	if email.Subject != "" {
		pageTitle = email.Subject + " - EML Viewer"
	}

	data := map[string]interface{}{
		"PageTitle":   pageTitle,
		"Email":       email,
		"Attachments": attachments,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "email.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}
