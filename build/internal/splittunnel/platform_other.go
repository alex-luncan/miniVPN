//go:build !windows

package splittunnel

import (
	"fmt"
	"net"
)

// newPlatformRouter creates a platform-specific router (stub for non-Windows)
func newPlatformRouter() PlatformRouter {
	return &stubRouter{}
}

// stubRouter is a no-op router for non-Windows platforms
type stubRouter struct{}

func (s *stubRouter) AddRule(rule Rule) error {
	return nil
}

func (s *stubRouter) RemoveRule(id uint64) error {
	return nil
}

func (s *stubRouter) ClearAll() error {
	return nil
}

func (s *stubRouter) IsAvailable() bool {
	return false
}

// GetDefaultGateway returns the default gateway (stub for non-Windows)
func GetDefaultGateway() (net.IP, string, error) {
	return nil, "", fmt.Errorf("not implemented on this platform")
}

// AddRoute adds a route (stub for non-Windows)
func AddRoute(destination, mask, gateway net.IP, metric uint32) error {
	return fmt.Errorf("not implemented on this platform")
}

// DeleteRoute removes a route (stub for non-Windows)
func DeleteRoute(destination, mask, gateway net.IP) error {
	return fmt.Errorf("not implemented on this platform")
}

// RunAsAdmin checks if running with admin privileges (stub for non-Windows)
func RunAsAdmin() bool {
	return false
}
