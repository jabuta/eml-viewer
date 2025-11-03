package handlers

import (
	"fmt"
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

	// For now, return simple HTML until we add templates
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>EML Viewer</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<script src="https://cdn.tailwindcss.com"></script>
	<script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body class="bg-gray-100">
	<div class="container mx-auto px-4 py-8">
		<header class="mb-8">
			<h1 class="text-4xl font-bold text-gray-900 mb-2">EML Viewer</h1>
			<p class="text-gray-600">%d emails indexed</p>
		</header>

		<div class="mb-6">
			<input
				type="text"
				name="q"
				hx-get="/search"
				hx-trigger="keyup changed delay:300ms"
				hx-target="#email-list"
				hx-indicator="#search-spinner"
				placeholder="Search emails..."
				class="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
			/>
			<span id="search-spinner" class="htmx-indicator text-gray-500 text-sm ml-2">
				Searching...
			</span>
		</div>

		<div id="email-list" class="space-y-4">`, count)

	// Render email list
	for _, email := range emails {
		subject := email.Subject
		if subject == "" {
			subject = "(No Subject)"
		}

		fmt.Fprintf(w, `
			<div class="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
				<a href="/emails/%d" class="block">
					<div class="flex justify-between items-start mb-2">
						<h3 class="text-lg font-semibold text-gray-900">%s</h3>
						<span class="text-sm text-gray-500">%s</span>
					</div>
					<p class="text-sm text-gray-600 mb-2">From: %s</p>
					<p class="text-sm text-gray-500 line-clamp-2">%s</p>
				</a>
			</div>`,
			email.ID,
			subject,
			email.GetDate().Format("Jan 2, 2006 15:04"),
			email.Sender,
			truncate(email.BodyText, 150),
		)
	}

	fmt.Fprintf(w, `
		</div>
	</div>
</body>
</html>`)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
