package main

import (
	"context"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/felo/eml-viewer/internal/config"
	"github.com/felo/eml-viewer/internal/db"
	"github.com/felo/eml-viewer/internal/handlers"
	"github.com/felo/eml-viewer/internal/indexer"
	"github.com/felo/eml-viewer/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Load configuration
	cfg := config.Default()

	// Ensure database directory exists
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	// Open database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Set emails path for resolving relative .eml file paths
	database.SetEmailsPath(cfg.EmailsPath)

	log.Printf("Database opened at: %s", cfg.DBPath)
	log.Printf("Emails path configured: %s", cfg.EmailsPath)

	// Check if emails directory exists
	if _, err := os.Stat(cfg.EmailsPath); os.IsNotExist(err) {
		log.Printf("Emails directory not found: %s", cfg.EmailsPath)
		log.Printf("Creating directory...")
		if err := os.MkdirAll(cfg.EmailsPath, 0755); err != nil {
			log.Fatalf("Failed to create emails directory: %v", err)
		}
		log.Printf("Created emails directory at: %s", cfg.EmailsPath)
		log.Printf("Please place your .eml files in this directory and restart the application")
	} else {
		// Index emails on startup
		log.Printf("Indexing emails from: %s", cfg.EmailsPath)
		idx := indexer.NewIndexer(database, cfg.EmailsPath, true)
		result, err := idx.IndexAll()
		if err != nil {
			log.Printf("Warning: Indexing failed: %v", err)
		} else {
			log.Printf("Indexing complete: %d new, %d skipped, %d failed",
				result.NewIndexed, result.Skipped, result.Failed)
		}
	}

	// Create shutdown signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Initialize handlers with embedded templates
	h := handlers.New(database, cfg)
	h.SetShutdownChannel(sigChan)
	if err := h.LoadTemplates(web.Assets); err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Set up router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Routes
	r.Get("/", h.Index)
	r.Get("/email/{id}", h.ViewEmail)
	r.Get("/search", h.Search)
	r.Get("/attachments/{id}/download", h.DownloadAttachment)
	r.Post("/scan", h.Scan)
	r.Get("/scan", h.ScanPage)
	r.Get("/scan/progress", h.ScanProgressSSE)
	r.Post("/shutdown", h.Shutdown)

	// Conversation/threading routes
	r.Get("/threaded", h.ListThreaded)
	r.Get("/conversation/{id}", h.ViewFullConversation)
	r.Get("/conversation/{id}/thread", h.ViewConversationThread)

	// Static files from embedded assets
	staticFS, err := fs.Sub(web.Assets, "static")
	if err != nil {
		log.Fatalf("Failed to get static files: %v", err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Create server
	srv := &http.Server{
		Addr:         cfg.Address(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 5 * time.Minute, // Increased for SSE connections
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on %s", cfg.URL())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Auto-open browser
	time.Sleep(500 * time.Millisecond) // Give server time to start
	if err := openBrowser(cfg.URL()); err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please open your browser and navigate to: %s", cfg.URL())
	} else {
		log.Printf("Browser opened at: %s", cfg.URL())
	}

	// Wait for interrupt signal
	<-sigChan
	log.Println("\nShutting down gracefully...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}
