package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// AutocompleteSenders handles autocomplete requests for sender email addresses
func (h *Handlers) AutocompleteSenders(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter (default 100)
	limitParam := r.URL.Query().Get("limit")
	limit := 100
	if limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	senders, err := h.db.GetUniqueSenders(limit)
	if err != nil {
		log.Printf("Failed to get unique senders: %v", err)
		http.Error(w, "Failed to load senders", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(senders); err != nil {
		log.Printf("Failed to encode senders: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// AutocompleteRecipients handles autocomplete requests for recipient email addresses
func (h *Handlers) AutocompleteRecipients(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter (default 100)
	limitParam := r.URL.Query().Get("limit")
	limit := 100
	if limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	recipients, err := h.db.GetUniqueRecipients(limit)
	if err != nil {
		log.Printf("Failed to get unique recipients: %v", err)
		http.Error(w, "Failed to load recipients", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(recipients); err != nil {
		log.Printf("Failed to encode recipients: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
