package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

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

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(data)), 10))

	// Write attachment data
	w.Write(data)
}
