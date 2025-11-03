package handlers

import (
	"fmt"
	"html"
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

	subject := email.Subject
	if subject == "" {
		subject = "(No Subject)"
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
	<title>%s - EML Viewer</title>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100">
	<div class="container mx-auto px-4 py-8">
		<div class="mb-4">
			<a href="/" class="text-blue-600 hover:text-blue-800">&larr; Back to list</a>
		</div>

		<div class="bg-white rounded-lg shadow p-6">
			<h1 class="text-3xl font-bold text-gray-900 mb-4">%s</h1>

			<div class="border-b border-gray-200 pb-4 mb-4">
				<div class="grid grid-cols-1 gap-2">
					<div>
						<span class="font-semibold text-gray-700">From:</span>
						<span class="text-gray-900">%s</span>
					</div>
					<div>
						<span class="font-semibold text-gray-700">To:</span>
						<span class="text-gray-900">%s</span>
					</div>`,
		html.EscapeString(subject),
		html.EscapeString(subject),
		html.EscapeString(formatSender(email.SenderName, email.Sender)),
		html.EscapeString(email.Recipients),
	)

	if email.CC != "" {
		fmt.Fprintf(w, `
					<div>
						<span class="font-semibold text-gray-700">CC:</span>
						<span class="text-gray-900">%s</span>
					</div>`, html.EscapeString(email.CC))
	}

	fmt.Fprintf(w, `
					<div>
						<span class="font-semibold text-gray-700">Date:</span>
						<span class="text-gray-900">%s</span>
					</div>
				</div>
			</div>`, email.GetDate().Format("Monday, January 2, 2006 at 15:04"))

	// Attachments
	if len(attachments) > 0 {
		fmt.Fprintf(w, `
			<div class="mb-4">
				<h3 class="font-semibold text-gray-700 mb-2">Attachments (%d):</h3>
				<div class="flex flex-wrap gap-2">`, len(attachments))

		for _, att := range attachments {
			fmt.Fprintf(w, `
					<a href="/attachments/%d/download"
					   class="px-3 py-1 bg-blue-100 text-blue-800 rounded hover:bg-blue-200 text-sm">
						📎 %s (%s)
					</a>`,
				att.ID,
				html.EscapeString(att.Filename),
				formatSize(att.Size),
			)
		}

		fmt.Fprintf(w, `
				</div>
			</div>`)
	}

	// Email body
	fmt.Fprintf(w, `
			<div class="border-t border-gray-200 pt-4">
				<h3 class="font-semibold text-gray-700 mb-2">Message:</h3>`)

	if email.BodyHTML != "" {
		// Render HTML in sandboxed iframe
		fmt.Fprintf(w, `
				<iframe
					sandbox="allow-same-origin"
					srcdoc="%s"
					class="w-full border border-gray-300 rounded"
					style="min-height: 400px;"
				></iframe>`,
			html.EscapeString(email.BodyHTML),
		)
	} else if email.BodyText != "" {
		// Render plain text
		fmt.Fprintf(w, `
				<pre class="whitespace-pre-wrap text-gray-900 font-mono text-sm">%s</pre>`,
			html.EscapeString(email.BodyText),
		)
	} else {
		fmt.Fprintf(w, `
				<p class="text-gray-500 italic">No message body</p>`)
	}

	fmt.Fprintf(w, `
			</div>

			<details class="mt-6">
				<summary class="cursor-pointer font-semibold text-gray-700">Raw Headers</summary>
				<pre class="mt-2 p-4 bg-gray-50 rounded text-xs overflow-x-auto">%s</pre>
			</details>
		</div>
	</div>
</body>
</html>`, html.EscapeString(email.RawHeaders))
}

func formatSender(name, email string) string {
	if name != "" {
		return fmt.Sprintf("%s <%s>", name, email)
	}
	return email
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
