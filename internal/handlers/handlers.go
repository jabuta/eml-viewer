package handlers

import (
	"embed"
	"html/template"

	"github.com/felo/eml-viewer/internal/config"
	"github.com/felo/eml-viewer/internal/db"
)

// Handlers holds all HTTP handlers and their dependencies
type Handlers struct {
	db        *db.DB
	cfg       *config.Config
	templates *template.Template
}

// New creates a new Handlers instance
func New(database *db.DB, cfg *config.Config) *Handlers {
	return &Handlers{
		db:  database,
		cfg: cfg,
	}
}

// LoadTemplates loads HTML templates from embedded filesystem
func (h *Handlers) LoadTemplates(embeddedFiles embed.FS) error {
	// Create template with custom functions
	tmpl := template.New("").Funcs(template.FuncMap{
		"html": func(s string) template.HTML {
			return template.HTML(s)
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
