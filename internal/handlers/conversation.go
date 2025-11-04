package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/felo/eml-viewer/internal/db"
	"github.com/go-chi/chi/v5"
)

// ViewConversationThread handles loading the full conversation thread for an email
// This is called via HTMX when a user expands a conversation
func (h *Handlers) ViewConversationThread(w http.ResponseWriter, r *http.Request) {
	// Get email ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid email ID", http.StatusBadRequest)
		return
	}

	// Get the root email
	rootEmail, err := h.db.GetEmailByID(id)
	if err != nil {
		log.Printf("Error loading email: %v", err)
		http.Error(w, "Failed to load email", http.StatusInternalServerError)
		return
	}
	if rootEmail == nil {
		http.Error(w, "Email not found", http.StatusNotFound)
		return
	}

	// Build the conversation tree
	conversation, err := h.db.BuildConversationTree(rootEmail)
	if err != nil {
		log.Printf("Error building conversation tree: %v", err)
		http.Error(w, "Failed to build conversation", http.StatusInternalServerError)
		return
	}

	// Render only the children (root is already shown)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if len(conversation.Children) == 0 {
		w.Write([]byte(`<div class="text-center text-sm text-gray-500">No replies yet</div>`))
		return
	}

	// Render each child conversation
	for _, child := range conversation.Children {
		if err := h.templates.ExecuteTemplate(w, "conversation-thread", child); err != nil {
			log.Printf("Template error: %v", err)
			continue
		}
	}
}

// ViewFullConversation shows a full conversation view in a dedicated page
func (h *Handlers) ViewFullConversation(w http.ResponseWriter, r *http.Request) {
	// Get email ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid email ID", http.StatusBadRequest)
		return
	}

	// Get the email
	email, err := h.db.GetEmailByID(id)
	if err != nil {
		log.Printf("Error loading email: %v", err)
		http.Error(w, "Failed to load email", http.StatusInternalServerError)
		return
	}
	if email == nil {
		http.Error(w, "Email not found", http.StatusNotFound)
		return
	}

	// Find the root of this conversation
	var rootEmail *db.Email
	if email.InReplyTo == "" {
		// This is already the root
		rootEmail = email
	} else {
		// Walk up to find the root
		current := email
		for current.InReplyTo != "" {
			parent, err := h.db.GetEmailsByMessageID(current.InReplyTo)
			if err != nil || parent == nil {
				// Parent not found, use current as root
				break
			}
			current = parent
		}
		rootEmail = current
	}

	// Build the conversation tree from the root
	conversation, err := h.db.BuildConversationTree(rootEmail)
	if err != nil {
		log.Printf("Error building conversation tree: %v", err)
		http.Error(w, "Failed to build conversation", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	pageTitle := "Conversation - EML Viewer"
	if rootEmail.Subject != "" {
		pageTitle = rootEmail.Subject + " - Conversation - EML Viewer"
	}

	data := map[string]interface{}{
		"PageTitle":    pageTitle,
		"Conversation": conversation,
		"RootEmail":    rootEmail,
	}

	// Render template
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, "conversation.html", data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// ListThreaded returns emails organized by conversation threads
func (h *Handlers) ListThreaded(w http.ResponseWriter, r *http.Request) {
	// Parse offset parameter
	offsetParam := r.URL.Query().Get("offset")
	offset := 0
	if offsetParam != "" {
		if parsed, err := strconv.Atoi(offsetParam); err == nil {
			offset = parsed
		}
	}

	// Fetch one more than limit to check if there are more results
	limit := 50
	conversations, err := h.db.GetRootEmailsWithReplyCounts(limit+1, offset)
	if err != nil {
		log.Printf("Failed to load conversations: %v", err)
		http.Error(w, "Failed to load conversations", http.StatusInternalServerError)
		return
	}

	// Check if there are more results
	hasMore := len(conversations) > limit
	if hasMore {
		conversations = conversations[:limit] // Trim to actual limit
	}

	// Get total count
	count, err := h.db.CountEmails()
	if err != nil {
		log.Printf("Failed to get email count: %v", err)
		count = 0
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Check if this is an HTMX request (pagination)
	isHTMX := r.Header.Get("HX-Request") == "true"

	if isHTMX && offset > 0 {
		// For pagination, return only conversation rows
		for _, conv := range conversations {
			if err := h.templates.ExecuteTemplate(w, "conversation-row", conv); err != nil {
				log.Printf("Template error: %v", err)
				continue
			}
		}

		// Calculate display count
		displayCount := offset + len(conversations)

		// Replace the old Load More button
		if hasMore {
			loadMoreHTML := `<div class="flex justify-center mt-6" id="load-more-container" hx-swap-oob="true">
				<button hx-get="/threaded?offset=` + strconv.Itoa(offset+limit) + `" hx-target="#email-list" hx-swap="beforeend" class="px-6 py-3 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors">Load More</button>
			</div>`
			w.Write([]byte(loadMoreHTML))
		} else {
			w.Write([]byte(`<div id="load-more-container" hx-swap-oob="true"></div>`))
		}

		// Update the email counter
		counterHTML := `<div id="email-counter" class="mt-8 text-center text-sm text-gray-500" hx-swap-oob="true">
			Showing ` + strconv.Itoa(displayCount) + ` conversations of ` + strconv.Itoa(count) + ` emails
		</div>`
		w.Write([]byte(counterHTML))

		return
	}

	// Return email rows for replacement in the email list
	for _, conv := range conversations {
		if err := h.templates.ExecuteTemplate(w, "conversation-row", conv); err != nil {
			log.Printf("Template error: %v", err)
			continue
		}
	}
}
