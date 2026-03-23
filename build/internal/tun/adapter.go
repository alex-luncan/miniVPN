package tun

import (
	"fmt"
	"net"
	"sync"
)

// Adapter represents a virtual network adapter
type Adapter struct {
	name       string
	mtu        int
	localIP    net.IP
	remoteIP   net.IP
	subnetMask net.IPMask
	running    bool
	mu         sync.RWMutex

	// Packet handlers
	onPacket func([]byte)
}

// AdapterConfig holds adapter configuration
type AdapterConfig struct {
	Name       string
	LocalIP    string
	RemoteIP   string
	SubnetMask string
	MTU        int
}

// NewAdapter creates a new virtual adapter
func NewAdapter(config AdapterConfig) (*Adapter, error) {
	localIP := net.ParseIP(config.LocalIP)
	if localIP == nil {
		return nil, fmt.Errorf("invalid local IP: %s", config.LocalIP)
	}

	remoteIP := net.ParseIP(config.RemoteIP)
	if remoteIP == nil {
		return nil, fmt.Errorf("invalid remote IP: %s", config.RemoteIP)
	}

	mask := net.ParseIP(config.SubnetMask)
	if mask == nil {
		return nil, fmt.Errorf("invalid subnet mask: %s", config.SubnetMask)
	}

	mtu := config.MTU
	if mtu == 0 {
		mtu = 1420 // Default MTU for VPN
	}

	return &Adapter{
		name:       config.Name,
		mtu:        mtu,
		localIP:    localIP.To4(),
		remoteIP:   remoteIP.To4(),
		subnetMask: net.IPMask(mask.To4()),
	}, nil
}

// Start activates the virtual adapter
func (a *Adapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("adapter already running")
	}

	// On Windows, we would use Wintun here
	// For now, this is a placeholder that will be implemented
	// when we have the actual Wintun DLL

	a.running = true
	return nil
}

// Stop deactivates the virtual adapter
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	a.running = false
	return nil
}

// Write sends a packet through the adapter
func (a *Adapter) Write(packet []byte) (int, error) {
	a.mu.RLock()
	if !a.running {
		a.mu.RUnlock()
		return 0, fmt.Errorf("adapter not running")
	}
	a.mu.RUnlock()

	// Process outgoing packet
	// In a real implementation, this would write to the TUN device
	return len(packet), nil
}

// Read receives a packet from the adapter
func (a *Adapter) Read(buf []byte) (int, error) {
	a.mu.RLock()
	if !a.running {
		a.mu.RUnlock()
		return 0, fmt.Errorf("adapter not running")
	}
	a.mu.RUnlock()

	// In a real implementation, this would read from the TUN device
	// For now, block until stopped
	select {}
}

// SetPacketHandler sets the callback for incoming packets
func (a *Adapter) SetPacketHandler(handler func([]byte)) {
	a.mu.Lock()
	a.onPacket = handler
	a.mu.Unlock()
}

// Name returns the adapter name
func (a *Adapter) Name() string {
	return a.name
}

// MTU returns the adapter MTU
func (a *Adapter) MTU() int {
	return a.mtu
}

// LocalIP returns the local IP address
func (a *Adapter) LocalIP() net.IP {
	return a.localIP
}

// IsRunning returns whether the adapter is running
func (a *Adapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
