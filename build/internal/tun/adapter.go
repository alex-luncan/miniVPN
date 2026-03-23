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

	// Platform-specific adapter (Wintun on Windows)
	wintun *WintunAdapter

	// Packet handlers
	onPacket func([]byte)

	// Stop signal for read loop
	stopCh chan struct{}
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
		stopCh:     make(chan struct{}),
	}, nil
}

// Start activates the virtual adapter
func (a *Adapter) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("adapter already running")
	}

	// Create the Wintun adapter
	wintun, err := NewWintunAdapter(a.name)
	if err != nil {
		return fmt.Errorf("failed to create Wintun adapter: %w", err)
	}

	// Start the Wintun session
	if err := wintun.Start(); err != nil {
		wintun.Close()
		return fmt.Errorf("failed to start Wintun session: %w", err)
	}

	a.wintun = wintun
	a.stopCh = make(chan struct{})
	a.running = true

	// Configure the IP address
	if err := a.ConfigureIP(a.localIP.String(), a.remoteIP.String(), ipMaskToString(a.subnetMask)); err != nil {
		a.wintun.Close()
		a.wintun = nil
		a.running = false
		return fmt.Errorf("failed to configure IP: %w", err)
	}

	return nil
}

// Stop deactivates the virtual adapter
func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	// Signal stop to any read loops
	close(a.stopCh)

	// Close Wintun
	if a.wintun != nil {
		a.wintun.Close()
		a.wintun = nil
	}

	a.running = false
	return nil
}

// Write sends a packet through the adapter (to the TUN interface)
func (a *Adapter) Write(packet []byte) (int, error) {
	a.mu.RLock()
	if !a.running || a.wintun == nil {
		a.mu.RUnlock()
		return 0, fmt.Errorf("adapter not running")
	}
	wintun := a.wintun
	a.mu.RUnlock()

	// Send packet to the TUN interface
	if err := wintun.SendPacket(packet); err != nil {
		return 0, fmt.Errorf("failed to send packet: %w", err)
	}

	return len(packet), nil
}

// Read receives a packet from the adapter (from the TUN interface)
func (a *Adapter) Read(buf []byte) (int, error) {
	a.mu.RLock()
	if !a.running || a.wintun == nil {
		a.mu.RUnlock()
		return 0, fmt.Errorf("adapter not running")
	}
	wintun := a.wintun
	stopCh := a.stopCh
	a.mu.RUnlock()

	// Get the read wait event
	event, err := wintun.GetReadWaitEvent()
	if err != nil {
		return 0, fmt.Errorf("failed to get read event: %w", err)
	}

	for {
		// Try to receive a packet
		packet, err := wintun.ReceivePacket()
		if err != nil {
			return 0, fmt.Errorf("failed to receive packet: %w", err)
		}

		if packet != nil {
			// Copy packet to buffer
			n := copy(buf, packet)
			return n, nil
		}

		// No packet available, wait for event
		select {
		case <-stopCh:
			return 0, fmt.Errorf("adapter stopped")
		default:
			// Wait for read event with timeout
			if err := waitForEvent(event, 100); err != nil {
				// Timeout, loop again
				continue
			}
		}
	}
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

// ipMaskToString converts an IP mask to dotted decimal string
func ipMaskToString(mask net.IPMask) string {
	if len(mask) == 4 {
		return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	}
	return "255.255.255.0"
}
