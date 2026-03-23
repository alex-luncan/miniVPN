package holepunch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// PeerInfo contains information about a peer's endpoint
type PeerInfo struct {
	PublicAddr  string `json:"publicAddr"`
	PrivateAddr string `json:"privateAddr"`
	SessionID   string `json:"sessionId"`
}

// HolePuncher handles UDP hole punching
type HolePuncher struct {
	conn       *net.UDPConn
	localAddr  *net.UDPAddr
	publicAddr *net.UDPAddr
	mu         sync.RWMutex
}

// NewHolePuncher creates a new hole puncher
func NewHolePuncher() (*HolePuncher, error) {
	// Bind to a random UDP port
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return &HolePuncher{
		conn:      conn,
		localAddr: localAddr,
	}, nil
}

// DiscoverPublicAddr discovers the public address using a STUN-like server
func (hp *HolePuncher) DiscoverPublicAddr(stunServer string) (*net.UDPAddr, error) {
	stunAddr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve STUN server: %w", err)
	}

	// Send discovery request
	request := []byte("DISCOVER")
	_, err = hp.conn.WriteToUDP(request, stunAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to send discovery request: %w", err)
	}

	// Wait for response
	hp.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, _, err := hp.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to receive discovery response: %w", err)
	}

	// Parse response (format: "DISCOVERED:ip:port")
	var response struct {
		Type string `json:"type"`
		Addr string `json:"addr"`
	}
	if err := json.Unmarshal(buf[:n], &response); err != nil {
		return nil, fmt.Errorf("failed to parse discovery response: %w", err)
	}

	publicAddr, err := net.ResolveUDPAddr("udp4", response.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public address: %w", err)
	}

	hp.mu.Lock()
	hp.publicAddr = publicAddr
	hp.mu.Unlock()

	return publicAddr, nil
}

// PunchHole attempts to punch a hole to the peer
func (hp *HolePuncher) PunchHole(ctx context.Context, peerAddr *net.UDPAddr) error {
	// Send punch packets for several seconds
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(10 * time.Second)
	punchPacket := []byte("PUNCH")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("hole punch timeout")
		case <-ticker.C:
			_, err := hp.conn.WriteToUDP(punchPacket, peerAddr)
			if err != nil {
				// Continue trying
				continue
			}

			// Check for incoming packets
			hp.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			buf := make([]byte, 1024)
			n, addr, err := hp.conn.ReadFromUDP(buf)
			if err == nil && n > 0 {
				// Received a packet from peer
				if addr.IP.Equal(peerAddr.IP) {
					// Hole punched successfully
					return nil
				}
			}
		}
	}
}

// GetConn returns the underlying UDP connection
func (hp *HolePuncher) GetConn() *net.UDPConn {
	return hp.conn
}

// GetLocalAddr returns the local address
func (hp *HolePuncher) GetLocalAddr() *net.UDPAddr {
	return hp.localAddr
}

// GetPublicAddr returns the discovered public address
func (hp *HolePuncher) GetPublicAddr() *net.UDPAddr {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	return hp.publicAddr
}

// Close closes the hole puncher
func (hp *HolePuncher) Close() error {
	return hp.conn.Close()
}
