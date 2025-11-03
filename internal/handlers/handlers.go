package handlers

import (
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
		// templates will be loaded when we add template support
	}
}
