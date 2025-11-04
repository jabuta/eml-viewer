package handlers

import (
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// sanitizeFilename removes dangerous characters from attachment filenames
func sanitizeFilename(filename string) string {
	// Remove path separators
	filename = filepath.Base(filename)

	// Remove any control characters and quotes
	cleaned := strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\'' {
			return -1 // Remove character
		}
		return r
	}, filename)

	// Limit length
	if len(cleaned) > 255 {
		cleaned = cleaned[:255]
	}

	// Fallback if empty
	if cleaned == "" {
		cleaned = "download.bin"
	}

	return cleaned
}

// DownloadAttachment handles attachment downloads
func (h *Handlers) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	// Get attachment ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid attachment ID", http.StatusBadRequest)
		return
	}

	// Get attachment metadata from database
	att, err := h.db.GetAttachmentByID(id)
	if err != nil {
		http.Error(w, "Failed to load attachment", http.StatusInternalServerError)
		return
	}
	if att == nil {
		http.Error(w, "Attachment not found", http.StatusNotFound)
		return
	}

	// Get attachment data by parsing .eml file
	data, err := h.db.GetAttachmentData(id)
	if err != nil {
		log.Printf("Error getting attachment data: %v", err)
		http.Error(w, "Failed to load attachment data", http.StatusInternalServerError)
		return
	}

	// Sanitize filename for security
	safeFilename := sanitizeFilename(att.Filename)

	// Set headers for download using proper encoding
	w.Header().Set("Content-Disposition",
		mime.FormatMediaType("attachment", map[string]string{
			"filename": safeFilename,
		}))
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Write attachment data
	w.Write(data)
}
