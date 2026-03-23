//go:build windows

package splittunnel

import (
	"fmt"
	"net"
	"sync"
)

// VPNRouteManager handles VPN route setup and teardown
type VPNRouteManager struct {
	originalGateway   net.IP
	originalInterface string
	serverRealIP      net.IP
	vpnGateway        net.IP
	configured        bool
	mu                sync.Mutex
}

var (
	routeManager     *VPNRouteManager
	routeManagerOnce sync.Once
)

// GetRouteManager returns the singleton route manager
func GetRouteManager() *VPNRouteManager {
	routeManagerOnce.Do(func() {
		routeManager = &VPNRouteManager{}
	})
	return routeManager
}

// SetupVPNRoutes configures routes to send all traffic through the VPN
// serverRealIP: The actual IP address of the VPN server (to bypass VPN)
// vpnGateway: The VPN gateway IP (e.g., 10.0.0.1)
// vpnInterfaceName: The name of the TUN interface
func SetupVPNRoutes(serverRealIP, vpnGateway net.IP, vpnInterfaceName string) error {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.configured {
		return fmt.Errorf("VPN routes already configured")
	}

	// Save the original default gateway
	originalGateway, originalIface, err := GetDefaultGateway()
	if err != nil {
		return fmt.Errorf("failed to get original gateway: %w", err)
	}

	rm.originalGateway = originalGateway
	rm.originalInterface = originalIface
	rm.serverRealIP = serverRealIP
	rm.vpnGateway = vpnGateway

	// Step 1: Add route for VPN server via original gateway (bypass VPN)
	// This ensures we can still reach the VPN server after changing default route
	if err := addRouteViaGateway(serverRealIP, originalGateway); err != nil {
		return fmt.Errorf("failed to add server bypass route: %w", err)
	}

	// Step 2: Add default route via VPN gateway
	// We add two routes: 0.0.0.0/1 and 128.0.0.0/1 to override 0.0.0.0/0
	// This is more specific than the default route and takes precedence
	if err := addVPNDefaultRoutes(vpnGateway); err != nil {
		// Rollback
		deleteRouteViaGateway(serverRealIP, originalGateway)
		return fmt.Errorf("failed to add VPN default routes: %w", err)
	}

	rm.configured = true
	return nil
}

// TeardownVPNRoutes removes VPN routes and restores original routing
func TeardownVPNRoutes() error {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.configured {
		return nil
	}

	var errs []error

	// Remove VPN default routes
	if err := removeVPNDefaultRoutes(rm.vpnGateway); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove VPN default routes: %w", err))
	}

	// Remove server bypass route
	if rm.serverRealIP != nil && rm.originalGateway != nil {
		if err := deleteRouteViaGateway(rm.serverRealIP, rm.originalGateway); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove server bypass route: %w", err))
		}
	}

	rm.configured = false
	rm.originalGateway = nil
	rm.originalInterface = ""
	rm.serverRealIP = nil
	rm.vpnGateway = nil

	if len(errs) > 0 {
		return fmt.Errorf("errors during route teardown: %v", errs)
	}
	return nil
}

// addRouteViaGateway adds a host route via a specific gateway
func addRouteViaGateway(dest, gateway net.IP) error {
	// Host route: /32 mask (255.255.255.255)
	mask := net.IPv4Mask(255, 255, 255, 255)
	return AddRoute(dest, net.IP(mask), gateway, 1)
}

// deleteRouteViaGateway removes a host route
func deleteRouteViaGateway(dest, gateway net.IP) error {
	mask := net.IPv4Mask(255, 255, 255, 255)
	return DeleteRoute(dest, net.IP(mask), gateway)
}

// addVPNDefaultRoutes adds routes that override the default gateway
// Using 0.0.0.0/1 and 128.0.0.0/1 instead of 0.0.0.0/0 to be more specific
func addVPNDefaultRoutes(vpnGateway net.IP) error {
	// Mask for /1 subnet (128.0.0.0)
	mask1 := net.IPv4(128, 0, 0, 0)

	// Route for 0.0.0.0/1 (covers 0.0.0.0 - 127.255.255.255)
	if err := AddRoute(
		net.IPv4(0, 0, 0, 0),
		mask1,
		vpnGateway,
		1,
	); err != nil {
		return fmt.Errorf("failed to add 0.0.0.0/1 route: %w", err)
	}

	// Route for 128.0.0.0/1 (covers 128.0.0.0 - 255.255.255.255)
	if err := AddRoute(
		net.IPv4(128, 0, 0, 0),
		mask1,
		vpnGateway,
		1,
	); err != nil {
		// Rollback first route
		DeleteRoute(net.IPv4(0, 0, 0, 0), mask1, vpnGateway)
		return fmt.Errorf("failed to add 128.0.0.0/1 route: %w", err)
	}

	return nil
}

// removeVPNDefaultRoutes removes the VPN override routes
func removeVPNDefaultRoutes(vpnGateway net.IP) error {
	var errs []error

	// Mask for /1 subnet (128.0.0.0)
	mask1 := net.IPv4(128, 0, 0, 0)

	// Remove 0.0.0.0/1 route
	if err := DeleteRoute(
		net.IPv4(0, 0, 0, 0),
		mask1,
		vpnGateway,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove 0.0.0.0/1: %w", err))
	}

	// Remove 128.0.0.0/1 route
	if err := DeleteRoute(
		net.IPv4(128, 0, 0, 0),
		mask1,
		vpnGateway,
	); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove 128.0.0.0/1: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("route removal errors: %v", errs)
	}
	return nil
}

// IsConfigured returns whether VPN routes are currently configured
func IsVPNRoutesConfigured() bool {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.configured
}

// GetOriginalGateway returns the original default gateway (before VPN)
func GetOriginalGateway() (net.IP, string) {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.originalGateway, rm.originalInterface
}
