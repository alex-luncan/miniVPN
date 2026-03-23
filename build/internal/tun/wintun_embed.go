//go:build windows

package tun

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Note: To embed wintun.dll, uncomment the lines below and place wintun.dll in this directory
// import "embed"
// //go:embed wintun.dll
// var wintunDLLData embed.FS

// For now, we'll look for wintun.dll in common locations

var (
	wintunExtracted   bool
	wintunExtractPath string
	wintunExtractOnce sync.Once
	wintunExtractErr  error
)

// ExtractWintun extracts the embedded wintun.dll to a temporary location
// and returns the path to the extracted file. The DLL is only extracted once
// and the same path is returned on subsequent calls.
func ExtractWintun() (string, error) {
	wintunExtractOnce.Do(func() {
		wintunExtractPath, wintunExtractErr = extractWintunDLL()
		if wintunExtractErr == nil {
			wintunExtracted = true
		}
	})

	return wintunExtractPath, wintunExtractErr
}

// extractWintunDLL locates or extracts wintun.dll
func extractWintunDLL() (string, error) {
	// Search locations for wintun.dll
	searchPaths := []string{
		// Same directory as executable
		"wintun.dll",
		// System directories
		filepath.Join(os.Getenv("SystemRoot"), "System32", "wintun.dll"),
		// Temp directory (if previously extracted)
		filepath.Join(os.TempDir(), "miniVPN", "wintun.dll"),
		// Common installation locations
		filepath.Join(os.Getenv("ProgramFiles"), "WireGuard", "wintun.dll"),
	}

	// Get executable directory
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		searchPaths = append([]string{filepath.Join(exeDir, "wintun.dll")}, searchPaths...)
	}

	// Search for existing wintun.dll
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If embed is enabled (when wintun.dll is in the source directory),
	// the DLL would be extracted here. For now, return an error with instructions.
	return "", fmt.Errorf("wintun.dll not found. Please download from https://www.wintun.net/ and place in the application directory")
}

// IsWintunExtracted returns whether wintun.dll has been extracted
func IsWintunExtracted() bool {
	return wintunExtracted
}

// GetWintunPath returns the path to the extracted wintun.dll
// Returns empty string if not yet extracted
func GetWintunPath() string {
	return wintunExtractPath
}

// CleanupWintun removes the extracted wintun.dll
// This should be called on application shutdown if desired
func CleanupWintun() error {
	if !wintunExtracted || wintunExtractPath == "" {
		return nil
	}

	// Try to remove the file
	if err := os.Remove(wintunExtractPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove wintun.dll: %w", err)
	}

	wintunExtracted = false
	wintunExtractPath = ""
	return nil
}
