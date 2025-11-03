package config

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
	return &Config{
		Host:       "localhost",
		Port:       "8080",
		DBPath:     "./db/emails.db", // Database in ./db folder
		EmailsPath: "./emails",       // Emails in ./emails folder
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
