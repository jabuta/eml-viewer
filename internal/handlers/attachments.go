package handlers

import (
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

	// Get attachment from database
	att, err := h.db.GetAttachmentByID(id)
	if err != nil {
		http.Error(w, "Failed to load attachment", http.StatusInternalServerError)
		return
	}
	if att == nil {
		http.Error(w, "Attachment not found", http.StatusNotFound)
		return
	}

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+att.Filename+"\"")
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(att.Size, 10))

	// Write attachment data
	w.Write(att.Data)
}
