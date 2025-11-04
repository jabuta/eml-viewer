package handlers

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/felo/eml-viewer/internal/config"
	"github.com/felo/eml-viewer/internal/db"
	"github.com/microcosm-cc/bluemonday"
)

// Handlers holds all HTTP handlers and their dependencies
type Handlers struct {
	db           *db.DB
	cfg          *config.Config
	templates    *template.Template
	shutdownChan chan os.Signal
}

// New creates a new Handlers instance
func New(database *db.DB, cfg *config.Config) *Handlers {
	return &Handlers{
		db:  database,
		cfg: cfg,
	}
}

// SetShutdownChannel sets the shutdown channel for the handlers
func (h *Handlers) SetShutdownChannel(ch chan os.Signal) {
	h.shutdownChan = ch
}

// LoadTemplates loads HTML templates from embedded filesystem
func (h *Handlers) LoadTemplates(embeddedFiles embed.FS) error {
	// Create HTML sanitization policy for email content
	p := bluemonday.UGCPolicy()

	// Create template with custom functions
	tmpl := template.New("").Funcs(template.FuncMap{
		"html": func(s string) template.HTML {
			return template.HTML(s)
		},
		"sanitizeHTML": func(s string) template.HTML {
			return template.HTML(p.Sanitize(s))
		},
	})

	tmpl, err := tmpl.ParseFS(embeddedFiles,
		"templates/*.html",
		"templates/components/*.html",
	)
	if err != nil {
		return err
	}
	h.templates = tmpl
	return nil
}

// AuthMiddleware implements basic authentication middleware
func (h *Handlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If authentication is not required, pass through
		if !h.cfg.RequireAuth {
			next.ServeHTTP(w, r)
			return
		}

		// Check for Authorization header
		authHeader := r.Header.Get("Authorization")
		expectedToken := "Bearer " + h.cfg.AuthToken

		if authHeader != expectedToken {
			w.Header().Set("WWW-Authenticate", `Bearer realm="EML Viewer"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Shutdown handles the shutdown request
func (h *Handlers) Shutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("Shutdown requested via web interface")

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`
		<html>
			<head><title>Shutting Down</title></head>
			<body style="font-family: sans-serif; text-align: center; padding: 50px;">
				<h1>Server Shutting Down</h1>
				<p>The application is shutting down gracefully...</p>
				<p>You can close this window.</p>
			</body>
		</html>
	`))

	// Trigger shutdown after response is sent
	if h.shutdownChan != nil {
		go func() {
			h.shutdownChan <- os.Interrupt
		}()
	}
}
