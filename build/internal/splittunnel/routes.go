//go:build windows

package splittunnel

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// Debug logging for routes
var DebugRoutes = true

func routeDebugLog(format string, args ...interface{}) {
	if DebugRoutes {
		log.Printf("[ROUTES] "+format, args...)
	}
}

// VPNRouteManager handles VPN route setup and teardown
type VPNRouteManager struct {
	originalGateway   net.IP
	originalInterface string
	originalIfIndex   int
	serverRealIP      net.IP
	vpnGateway        net.IP
	vpnIfIndex        int
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

	routeDebugLog("SetupVPNRoutes called:")
	routeDebugLog("  serverRealIP: %s", serverRealIP)
	routeDebugLog("  vpnGateway: %s", vpnGateway)
	routeDebugLog("  vpnInterfaceName: %s", vpnInterfaceName)

	if rm.configured {
		return fmt.Errorf("VPN routes already configured")
	}

	// Save the original default gateway
	originalGateway, originalIface, originalIfIndex, err := GetDefaultGatewayWithIndex()
	if err != nil {
		return fmt.Errorf("failed to get original gateway: %w", err)
	}
	routeDebugLog("Original gateway: %s via %s (IF %d)", originalGateway, originalIface, originalIfIndex)

	// Get the VPN interface index - this is critical for routing
	vpnIfIndex, err := GetInterfaceIndexByName(vpnInterfaceName)
	if err != nil {
		return fmt.Errorf("failed to get VPN interface index: %w", err)
	}
	routeDebugLog("VPN interface index: %d", vpnIfIndex)

	rm.originalGateway = originalGateway
	rm.originalInterface = originalIface
	rm.originalIfIndex = originalIfIndex
	rm.serverRealIP = serverRealIP
	rm.vpnGateway = vpnGateway
	rm.vpnIfIndex = vpnIfIndex

	// Step 1: Add route for VPN server via original gateway (bypass VPN)
	// This ensures we can still reach the VPN server after changing default route
	// IMPORTANT: We must specify the interface explicitly to prevent routing loops
	// when using custom ports, as Windows may otherwise route through the wrong interface
	routeDebugLog("Adding server bypass route: %s -> %s IF %d", serverRealIP, originalGateway, originalIfIndex)
	if err := addRouteViaGatewayWithInterface(serverRealIP, originalGateway, originalIfIndex); err != nil {
		return fmt.Errorf("failed to add server bypass route: %w", err)
	}
	routeDebugLog("Server bypass route added successfully")

	// Step 2: Add bypass routes for DNS servers to ensure DNS resolution works
	// Without this, DNS queries would go through VPN but the DNS server
	// (often your router's IP) may not be reachable from the VPN server
	routeDebugLog("Adding DNS bypass routes...")
	dnsServers := GetSystemDNSServers()
	for _, dns := range dnsServers {
		if dns != nil && !dns.Equal(vpnGateway) {
			routeDebugLog("  Adding DNS bypass for %s via %s IF %d", dns, originalGateway, originalIfIndex)
			// Ignore errors for DNS bypass - not critical
			addRouteViaGatewayWithInterface(dns, originalGateway, originalIfIndex)
		}
	}
	// Also add common public DNS servers as bypass
	publicDNS := []net.IP{
		net.ParseIP("8.8.8.8"),
		net.ParseIP("8.8.4.4"),
		net.ParseIP("1.1.1.1"),
		net.ParseIP("1.0.0.1"),
	}
	for _, dns := range publicDNS {
		routeDebugLog("  Adding public DNS bypass for %s via %s IF %d", dns, originalGateway, originalIfIndex)
		addRouteViaGatewayWithInterface(dns, originalGateway, originalIfIndex)
	}

	// Step 3: Add default route via VPN gateway with explicit interface
	// We add two routes: 0.0.0.0/1 and 128.0.0.0/1 to override 0.0.0.0/0
	// This is more specific than the default route and takes precedence
	routeDebugLog("Adding VPN default routes via %s IF %d", vpnGateway, vpnIfIndex)
	if err := addVPNDefaultRoutesWithInterface(vpnGateway, vpnIfIndex); err != nil {
		// Rollback
		deleteRouteViaGateway(serverRealIP, originalGateway)
		return fmt.Errorf("failed to add VPN default routes: %w", err)
	}
	routeDebugLog("VPN default routes added successfully")

	rm.configured = true
	routeDebugLog("VPN routes setup complete!")
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
	rm.originalIfIndex = 0
	rm.serverRealIP = nil
	rm.vpnGateway = nil
	rm.vpnIfIndex = 0

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

// addRouteViaGatewayWithInterface adds a host route via a specific gateway and interface
// This is more reliable than addRouteViaGateway as it prevents routing ambiguity
func addRouteViaGatewayWithInterface(dest, gateway net.IP, ifIndex int) error {
	// Host route: /32 mask (255.255.255.255)
	mask := net.IPv4Mask(255, 255, 255, 255)
	return AddRouteWithInterface(dest, net.IP(mask), gateway, 1, ifIndex)
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

// addVPNDefaultRoutesWithInterface adds routes with explicit interface specification
// This is needed when the VPN gateway IP conflicts with local network
func addVPNDefaultRoutesWithInterface(vpnGateway net.IP, ifIndex int) error {
	// Mask for /1 subnet (128.0.0.0)
	mask1 := net.IPv4(128, 0, 0, 0)

	// Route for 0.0.0.0/1 (covers 0.0.0.0 - 127.255.255.255)
	if err := AddRouteWithInterface(
		net.IPv4(0, 0, 0, 0),
		mask1,
		vpnGateway,
		1,
		ifIndex,
	); err != nil {
		return fmt.Errorf("failed to add 0.0.0.0/1 route: %w", err)
	}

	// Route for 128.0.0.0/1 (covers 128.0.0.0 - 255.255.255.255)
	if err := AddRouteWithInterface(
		net.IPv4(128, 0, 0, 0),
		mask1,
		vpnGateway,
		1,
		ifIndex,
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

// SetupVPNRoutesForSplitTunnel configures routes for split tunnel "include" mode
// In this mode, only VPN subnet traffic goes through VPN, everything else uses normal network
// serverRealIP: The actual IP address of the VPN server
// vpnGateway: The VPN gateway IP (e.g., 10.0.0.1)
// vpnSubnet: The VPN subnet (e.g., 10.0.0.0)
// vpnMask: The VPN subnet mask (e.g., 255.255.255.0)
// vpnInterfaceName: The name of the TUN interface
func SetupVPNRoutesForSplitTunnel(serverRealIP, vpnGateway, vpnSubnet net.IP, vpnMask net.IPMask, vpnInterfaceName string) error {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()

	routeDebugLog("SetupVPNRoutesForSplitTunnel called (include mode - partial routing):")
	routeDebugLog("  serverRealIP: %s", serverRealIP)
	routeDebugLog("  vpnGateway: %s", vpnGateway)
	routeDebugLog("  vpnSubnet: %s/%s", vpnSubnet, net.IP(vpnMask))
	routeDebugLog("  vpnInterfaceName: %s", vpnInterfaceName)

	if rm.configured {
		return fmt.Errorf("VPN routes already configured")
	}

	// Save the original default gateway (for reference, not for routing changes)
	originalGateway, originalIface, originalIfIndex, err := GetDefaultGatewayWithIndex()
	if err != nil {
		return fmt.Errorf("failed to get original gateway: %w", err)
	}
	routeDebugLog("Original gateway: %s via %s (IF %d) - will be preserved", originalGateway, originalIface, originalIfIndex)

	// Get the VPN interface index
	vpnIfIndex, err := GetInterfaceIndexByName(vpnInterfaceName)
	if err != nil {
		return fmt.Errorf("failed to get VPN interface index: %w", err)
	}
	routeDebugLog("VPN interface index: %d", vpnIfIndex)

	rm.originalGateway = originalGateway
	rm.originalInterface = originalIface
	rm.originalIfIndex = originalIfIndex
	rm.serverRealIP = serverRealIP
	rm.vpnGateway = vpnGateway
	rm.vpnIfIndex = vpnIfIndex

	// Only add route for VPN subnet - NOT the catch-all routes
	// This means only traffic to VPN network (e.g., 10.0.0.0/24) goes through VPN
	// All other traffic uses the normal default route
	routeDebugLog("Adding VPN subnet route: %s mask %s via %s IF %d", vpnSubnet, net.IP(vpnMask), vpnGateway, vpnIfIndex)
	if err := AddRouteWithInterface(vpnSubnet, net.IP(vpnMask), vpnGateway, 1, vpnIfIndex); err != nil {
		return fmt.Errorf("failed to add VPN subnet route: %w", err)
	}

	// Add additional routes for networks accessible through VPN server
	additionalNetworks := []struct {
		subnet net.IP
		mask   net.IPMask
	}{
		{net.IPv4(10, 101, 4, 0), net.IPv4Mask(255, 255, 255, 0)},   // 10.101.4.0/24
		{net.IPv4(10, 101, 0, 0), net.IPv4Mask(255, 255, 0, 0)},     // 10.101.0.0/16 (broader access)
	}

	for _, network := range additionalNetworks {
		routeDebugLog("Adding additional VPN route: %s mask %s via %s IF %d", network.subnet, net.IP(network.mask), vpnGateway, vpnIfIndex)
		if err := AddRouteWithInterface(network.subnet, net.IP(network.mask), vpnGateway, 1, vpnIfIndex); err != nil {
			routeDebugLog("Warning: failed to add route for %s: %v", network.subnet, err)
		}
	}

	// IMPORTANT: Delete the automatic default route that Windows creates when
	// setting up the TUN adapter with a gateway. This route would override our
	// split tunnel configuration and send all traffic through VPN.
	routeDebugLog("Removing automatic VPN default route to enable split tunneling...")
	if err := DeleteRoute(net.IPv4(0, 0, 0, 0), net.IPv4(0, 0, 0, 0), vpnGateway); err != nil {
		routeDebugLog("Note: Could not remove automatic default route (may not exist): %v", err)
	} else {
		routeDebugLog("Automatic default route removed successfully")
	}

	rm.configured = true
	routeDebugLog("Split tunnel routes setup complete - normal traffic will use original gateway")
	return nil
}

// TeardownVPNRoutesForSplitTunnel removes split tunnel routes
func TeardownVPNRoutesForSplitTunnel(vpnSubnet net.IP, vpnMask net.IPMask) error {
	rm := GetRouteManager()
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.configured {
		return nil
	}

	routeDebugLog("Tearing down split tunnel routes...")

	// Remove VPN subnet route
	if err := DeleteRoute(vpnSubnet, net.IP(vpnMask), rm.vpnGateway); err != nil {
		routeDebugLog("Warning: failed to remove VPN subnet route: %v", err)
	}

	rm.configured = false
	rm.originalGateway = nil
	rm.originalInterface = ""
	rm.originalIfIndex = 0
	rm.serverRealIP = nil
	rm.vpnGateway = nil
	rm.vpnIfIndex = 0

	return nil
}
