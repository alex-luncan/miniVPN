//go:build !windows

package tun

import (
	"fmt"
)

// ExtractWintun extracts the embedded wintun.dll (stub for non-Windows)
func ExtractWintun() (string, error) {
	return "", fmt.Errorf("Wintun is only available on Windows")
}

// IsWintunExtracted returns whether wintun.dll has been extracted (stub for non-Windows)
func IsWintunExtracted() bool {
	return false
}

// GetWintunPath returns the path to the extracted wintun.dll (stub for non-Windows)
func GetWintunPath() string {
	return ""
}

// CleanupWintun removes the extracted wintun.dll (stub for non-Windows)
func CleanupWintun() error {
	return nil
}
