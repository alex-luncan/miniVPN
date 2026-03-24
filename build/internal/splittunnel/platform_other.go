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

// GetDefaultGatewayWithIndex returns the default gateway with interface index (stub for non-Windows)
func GetDefaultGatewayWithIndex() (net.IP, string, int, error) {
	return nil, "", 0, fmt.Errorf("not implemented on this platform")
}

// GetInterfaceIndexByName returns interface index by name (stub for non-Windows)
func GetInterfaceIndexByName(name string) (int, error) {
	return 0, fmt.Errorf("not implemented on this platform")
}

// AddRouteWithInterface adds a route with interface specification (stub for non-Windows)
func AddRouteWithInterface(destination, mask, gateway net.IP, metric uint32, ifIndex int) error {
	return fmt.Errorf("not implemented on this platform")
}

// GetSystemDNSServers returns system DNS servers (stub for non-Windows)
func GetSystemDNSServers() []net.IP {
	return nil
}

// isPrivateIP checks if IP is private (stub for non-Windows)
func isPrivateIP(ip net.IP) bool {
	return false
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

// SetupVPNRoutes configures VPN routes (stub for non-Windows)
func SetupVPNRoutes(serverRealIP, vpnGateway net.IP, vpnInterfaceName string) error {
	return fmt.Errorf("not implemented on this platform")
}

// TeardownVPNRoutes removes VPN routes (stub for non-Windows)
func TeardownVPNRoutes() error {
	return nil
}

// SetupVPNRoutesForSplitTunnel configures routes for split tunnel mode (stub for non-Windows)
func SetupVPNRoutesForSplitTunnel(serverRealIP, vpnGateway, vpnSubnet net.IP, vpnMask net.IPMask, vpnInterfaceName string) error {
	return fmt.Errorf("not implemented on this platform")
}

// TeardownVPNRoutesForSplitTunnel removes split tunnel routes (stub for non-Windows)
func TeardownVPNRoutesForSplitTunnel(vpnSubnet net.IP, vpnMask net.IPMask) error {
	return nil
}

// IsVPNRoutesConfigured returns whether VPN routes are configured (stub for non-Windows)
func IsVPNRoutesConfigured() bool {
	return false
}
