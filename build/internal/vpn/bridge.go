package vpn

import (
	"context"
	"fmt"
	"sync"

	"minivpn/internal/tun"
)

// Bridge connects a TUN adapter to a VPN tunnel, handling bidirectional
// traffic flow between local applications and the remote VPN server.
type Bridge struct {
	adapter *tun.Adapter
	tunnel  *Tunnel
	mtu     int

	ctx    context.Context
	cancel context.CancelFunc

	running bool
	mu      sync.RWMutex

	// Statistics
	packetsFromTun    uint64
	packetsToTun      uint64
	bytesFromTun      uint64
	bytesToTun        uint64
	errorCount        uint64
}

// BridgeConfig holds configuration for the bridge
type BridgeConfig struct {
	Adapter *tun.Adapter
	Tunnel  *Tunnel
	MTU     int
}

// NewBridge creates a new traffic bridge
func NewBridge(config BridgeConfig) (*Bridge, error) {
	if config.Adapter == nil {
		return nil, fmt.Errorf("adapter is required")
	}
	if config.Tunnel == nil {
		return nil, fmt.Errorf("tunnel is required")
	}

	mtu := config.MTU
	if mtu == 0 {
		mtu = 1420
	}

	return &Bridge{
		adapter: config.Adapter,
		tunnel:  config.Tunnel,
		mtu:     mtu,
	}, nil
}

// Start begins bidirectional traffic forwarding
func (b *Bridge) Start() error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return fmt.Errorf("bridge already running")
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.running = true
	b.mu.Unlock()

	// Start the TUN to tunnel goroutine
	go b.tunToTunnel()

	// Set up the tunnel's OnData callback to write to TUN
	b.tunnel.SetOnData(func(data []byte) {
		b.tunnelToTun(data)
	})

	return nil
}

// Stop stops the traffic bridge
func (b *Bridge) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.running {
		return nil
	}

	if b.cancel != nil {
		b.cancel()
	}

	b.running = false
	return nil
}

// tunToTunnel reads packets from the TUN adapter and sends them through the tunnel
func (b *Bridge) tunToTunnel() {
	buf := make([]byte, b.mtu+100) // Extra space for headers

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		// Read from TUN adapter
		n, err := b.adapter.Read(buf)
		if err != nil {
			b.mu.Lock()
			b.errorCount++
			b.mu.Unlock()

			// Check if we should stop
			select {
			case <-b.ctx.Done():
				return
			default:
				// Log error but continue
				continue
			}
		}

		if n == 0 {
			continue
		}

		// Send packet through tunnel
		packet := make([]byte, n)
		copy(packet, buf[:n])

		if err := b.tunnel.Send(packet); err != nil {
			b.mu.Lock()
			b.errorCount++
			b.mu.Unlock()
			continue
		}

		b.mu.Lock()
		b.packetsFromTun++
		b.bytesFromTun += uint64(n)
		b.mu.Unlock()
	}
}

// tunnelToTun writes decrypted packets from the tunnel to the TUN adapter
func (b *Bridge) tunnelToTun(data []byte) {
	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()

	if !running {
		return
	}

	// Write packet to TUN adapter
	_, err := b.adapter.Write(data)
	if err != nil {
		b.mu.Lock()
		b.errorCount++
		b.mu.Unlock()
		return
	}

	b.mu.Lock()
	b.packetsToTun++
	b.bytesToTun += uint64(len(data))
	b.mu.Unlock()
}

// IsRunning returns whether the bridge is running
func (b *Bridge) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// Stats returns bridge statistics
func (b *Bridge) Stats() BridgeStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return BridgeStats{
		PacketsFromTun: b.packetsFromTun,
		PacketsToTun:   b.packetsToTun,
		BytesFromTun:   b.bytesFromTun,
		BytesToTun:     b.bytesToTun,
		ErrorCount:     b.errorCount,
	}
}

// BridgeStats holds bridge statistics
type BridgeStats struct {
	PacketsFromTun uint64
	PacketsToTun   uint64
	BytesFromTun   uint64
	BytesToTun     uint64
	ErrorCount     uint64
}
