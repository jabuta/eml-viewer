package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScanResult contains information about a scan operation
type ScanResult struct {
	TotalFiles     int
	ProcessedFiles int
	SkippedFiles   int
	ErrorFiles     int
	Errors         []error
}

// Scanner scans directories for .eml files
type Scanner struct {
	rootPath string
}

// NewScanner creates a new scanner for the given root path
func NewScanner(rootPath string) *Scanner {
	return &Scanner{
		rootPath: rootPath,
	}
}

// GetRootPath returns the root path for resolving relative paths
func (s *Scanner) GetRootPath() string {
	return s.rootPath
}

// Scan recursively scans for .eml files and returns paths relative to rootPath
// This ensures portability across different systems and drive mappings
func (s *Scanner) Scan() ([]string, error) {
	var emlFiles []string

	// Get absolute path of root for reliable relative path calculation
	absRoot, err := filepath.Abs(s.rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute root path: %w", err)
	}

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file has .eml extension
		if strings.ToLower(filepath.Ext(path)) == ".eml" {
			// Store relative path from root for portability
			relPath, err := filepath.Rel(absRoot, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %s: %w", path, err)
			}
			// Normalize to forward slashes for cross-platform compatibility
			// This ensures paths work when moving USB drive between Linux/Windows
			relPath = filepath.ToSlash(relPath)
			emlFiles = append(emlFiles, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	return emlFiles, nil
}

// ScanWithCallback scans for .eml files and calls the callback for each file found
func (s *Scanner) ScanWithCallback(callback func(path string, index, total int) error) error {
	// First, get all files
	files, err := s.Scan()
	if err != nil {
		return err
	}

	total := len(files)

	// Process each file
	for i, file := range files {
		if err := callback(file, i+1, total); err != nil {
			return fmt.Errorf("callback error for file %s: %w", file, err)
		}
	}

	return nil
}

// CountEMLFiles counts the number of .eml files without scanning them all
func (s *Scanner) CountEMLFiles() (int, error) {
	count := 0

	err := filepath.Walk(s.rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".eml" {
			count++
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to count files: %w", err)
	}

	return count, nil
}
