package vpn

import (
	"fmt"
	"net"
	"sync"
)

// IPPool manages VPN IP address allocation for clients
type IPPool struct {
	network   *net.IPNet
	allocated map[string]bool // IP string -> allocated
	mu        sync.Mutex

	// Reserved addresses
	serverIP  net.IP // Usually .1 (e.g., 10.0.0.1)
	networkIP net.IP // Network address (e.g., 10.0.0.0)
	broadcast net.IP // Broadcast address (e.g., 10.0.0.255)

	// First and last allocatable addresses
	firstIP net.IP // Usually .2
	lastIP  net.IP // Varies based on subnet
}

// NewIPPool creates a new IP pool from a CIDR string (e.g., "10.0.0.0/24")
func NewIPPool(cidr string) (*IPPool, error) {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Get the network address
	networkIP := network.IP.To4()
	if networkIP == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}

	// Calculate broadcast address
	broadcast := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		broadcast[i] = networkIP[i] | ^network.Mask[i]
	}

	// Server IP is .1
	serverIP := make(net.IP, 4)
	copy(serverIP, networkIP)
	serverIP[3] = 1

	// First allocatable IP is .2
	firstIP := make(net.IP, 4)
	copy(firstIP, networkIP)
	firstIP[3] = 2

	// Last allocatable IP is broadcast - 1
	lastIP := make(net.IP, 4)
	copy(lastIP, broadcast)
	lastIP[3]--

	return &IPPool{
		network:   network,
		allocated: make(map[string]bool),
		serverIP:  serverIP,
		networkIP: networkIP,
		broadcast: broadcast,
		firstIP:   firstIP,
		lastIP:    lastIP,
	}, nil
}

// NewDefaultIPPool creates an IP pool with the default 10.0.0.0/24 network
func NewDefaultIPPool() (*IPPool, error) {
	return NewIPPool("10.0.0.0/24")
}

// Allocate allocates and returns the next available IP address
func (p *IPPool) Allocate() (net.IP, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Iterate through the range to find an available IP
	ip := make(net.IP, 4)
	copy(ip, p.firstIP)

	for {
		ipStr := ip.String()
		if !p.allocated[ipStr] {
			// Found an available IP
			p.allocated[ipStr] = true
			result := make(net.IP, 4)
			copy(result, ip)
			return result, nil
		}

		// Move to next IP
		ip = incrementIP(ip)

		// Check if we've exceeded the range
		if compareIP(ip, p.lastIP) > 0 {
			return nil, fmt.Errorf("no available IP addresses")
		}
	}
}

// AllocateSpecific allocates a specific IP address if available
func (p *IPPool) AllocateSpecific(ip net.IP) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ip = ip.To4()
	if ip == nil {
		return fmt.Errorf("invalid IPv4 address")
	}

	if !p.network.Contains(ip) {
		return fmt.Errorf("IP not in pool range")
	}

	if ip.Equal(p.serverIP) || ip.Equal(p.networkIP) || ip.Equal(p.broadcast) {
		return fmt.Errorf("IP is reserved")
	}

	ipStr := ip.String()
	if p.allocated[ipStr] {
		return fmt.Errorf("IP already allocated")
	}

	p.allocated[ipStr] = true
	return nil
}

// Release releases an allocated IP address back to the pool
func (p *IPPool) Release(ip net.IP) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ip = ip.To4()
	if ip == nil {
		return
	}

	delete(p.allocated, ip.String())
}

// ServerIP returns the server's IP address (always .1)
func (p *IPPool) ServerIP() net.IP {
	result := make(net.IP, 4)
	copy(result, p.serverIP)
	return result
}

// Network returns the network IP and mask
func (p *IPPool) Network() *net.IPNet {
	return p.network
}

// SubnetMask returns the subnet mask as a net.IP
func (p *IPPool) SubnetMask() net.IP {
	mask := make(net.IP, 4)
	copy(mask, p.network.Mask)
	return mask
}

// IsAllocated checks if an IP is currently allocated
func (p *IPPool) IsAllocated(ip net.IP) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	ip = ip.To4()
	if ip == nil {
		return false
	}

	return p.allocated[ip.String()]
}

// AllocatedCount returns the number of currently allocated IPs
func (p *IPPool) AllocatedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.allocated)
}

// AvailableCount returns the number of available IPs
func (p *IPPool) AvailableCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Total IPs in range minus allocated
	total := ipRange(p.firstIP, p.lastIP)
	return total - len(p.allocated)
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) net.IP {
	result := make(net.IP, 4)
	copy(result, ip)

	for i := 3; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}

	return result
}

// compareIP compares two IP addresses
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func compareIP(a, b net.IP) int {
	for i := 0; i < 4; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// ipRange calculates the number of IPs between two addresses (inclusive)
func ipRange(start, end net.IP) int {
	count := 0
	ip := make(net.IP, 4)
	copy(ip, start)

	for compareIP(ip, end) <= 0 {
		count++
		ip = incrementIP(ip)
	}

	return count
}
