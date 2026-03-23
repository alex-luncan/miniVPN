//go:build !windows

package firewall

// EnsureAppAllowed is a no-op on non-Windows platforms
func EnsureAppAllowed() error {
	return nil
}

// RemoveAppRules is a no-op on non-Windows platforms
func RemoveAppRules() error {
	return nil
}
