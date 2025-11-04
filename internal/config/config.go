package config

import "errors"

// Config holds application configuration
type Config struct {
	// Server settings
	Host string
	Port string

	// Database settings
	DBPath string

	// Email folder settings
	EmailsPath string

	// Authentication settings
	RequireAuth bool
	AuthToken   string
}

// Default returns default configuration
func Default() *Config {
	return &Config{
		Host:        "localhost",
		Port:        "8787",
		DBPath:      "./db/emails.db", // Database in ./db folder
		EmailsPath:  "./emails",       // Emails in ./emails folder
		RequireAuth: false,            // Authentication disabled by default
		AuthToken:   "",               // No token by default
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

// Validate validates the configuration for security
func (c *Config) Validate() error {
	// Enforce localhost-only binding for security
	if c.Host != "localhost" && c.Host != "127.0.0.1" {
		return errors.New("host must be localhost or 127.0.0.1 for security reasons")
	}

	// If auth is required, token must be set
	if c.RequireAuth && c.AuthToken == "" {
		return errors.New("auth token must be set when authentication is required")
	}

	return nil
}
