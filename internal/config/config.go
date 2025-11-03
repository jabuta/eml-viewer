package config

import (
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	// Server settings
	Host string
	Port string

	// Database settings
	DBPath string

	// Email folder settings
	EmailsPath string
}

// Default returns default configuration
func Default() *Config {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	// Use ~/.eml-viewer for data directory
	dataDir := filepath.Join(homeDir, ".eml-viewer")

	return &Config{
		Host:       "localhost",
		Port:       "8080",
		DBPath:     filepath.Join(dataDir, "emails.db"),
		EmailsPath: "./emails", // Default to ./emails directory
	}
}

// Address returns the full server address
func (c *Config) Address() string {
	return c.Host + ":" + c.Port
}

// URL returns the full server URL
func (c *Config) URL() string {
	return "http://" + c.Address()
}
