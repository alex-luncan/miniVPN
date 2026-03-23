package holepunch

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client is a signaling client for NAT traversal
type Client struct {
	conn          *net.UDPConn
	signalingAddr *net.UDPAddr
	publicAddr    *net.UDPAddr
	peerAddr      *net.UDPAddr
}

// NewClient creates a new signaling client
func NewClient(signalingServer string) (*Client, error) {
	signalingAddr, err := net.ResolveUDPAddr("udp4", signalingServer)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve signaling server: %w", err)
	}

	// Bind to a random UDP port
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %w", err)
	}

	return &Client{
		conn:          conn,
		signalingAddr: signalingAddr,
	}, nil
}

// DiscoverPublicAddr discovers this client's public address
func (c *Client) DiscoverPublicAddr() (*net.UDPAddr, error) {
	msg := SignalingMessage{Type: MsgTypeDiscover}
	if err := c.sendMessage(&msg); err != nil {
		return nil, err
	}

	response, err := c.receiveMessage(5 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive discovery response: %w", err)
	}

	if response.Type != MsgTypeDiscovered {
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}

	addr, err := net.ResolveUDPAddr("udp4", response.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public address: %w", err)
	}

	c.publicAddr = addr
	return addr, nil
}

// RegisterAsServer registers as a server with the given secret code
func (c *Client) RegisterAsServer(secretCode string, sessionID string) error {
	msg := SignalingMessage{
		Type:       MsgTypeRegister,
		SecretCode: secretCode,
		SessionID:  sessionID,
	}
	if err := c.sendMessage(&msg); err != nil {
		return err
	}

	response, err := c.receiveMessage(5 * time.Second)
	if err != nil {
		return fmt.Errorf("failed to receive register response: %w", err)
	}

	if response.Type == MsgTypeError {
		return fmt.Errorf("registration failed: %s", response.Error)
	}

	if response.Type != MsgTypeRegistered {
		return fmt.Errorf("unexpected response type: %s", response.Type)
	}

	return nil
}

// ConnectToPeer connects to a peer using the secret code
func (c *Client) ConnectToPeer(secretCode string) (*net.UDPAddr, error) {
	msg := SignalingMessage{
		Type:       MsgTypeConnect,
		SecretCode: secretCode,
	}
	if err := c.sendMessage(&msg); err != nil {
		return nil, err
	}

	response, err := c.receiveMessage(10 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive connect response: %w", err)
	}

	if response.Type == MsgTypeError {
		return nil, fmt.Errorf("connection failed: %s", response.Error)
	}

	if response.Type != MsgTypePeerInfo {
		return nil, fmt.Errorf("unexpected response type: %s", response.Type)
	}

	peerAddr, err := net.ResolveUDPAddr("udp4", response.PeerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse peer address: %w", err)
	}

	c.peerAddr = peerAddr
	return peerAddr, nil
}

// WaitForPeer waits for a peer to connect (server side)
func (c *Client) WaitForPeer(ctx context.Context) (*net.UDPAddr, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		response, err := c.receiveMessage(1 * time.Second)
		if err != nil {
			continue
		}

		if response.Type == MsgTypePeerInfo {
			peerAddr, err := net.ResolveUDPAddr("udp4", response.PeerAddr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse peer address: %w", err)
			}
			c.peerAddr = peerAddr
			return peerAddr, nil
		}
	}
}

// PunchHole performs UDP hole punching to the peer
func (c *Client) PunchHole(ctx context.Context, peerAddr *net.UDPAddr) error {
	// Send punch packets rapidly
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(15 * time.Second)
	punchPacket := []byte("PUNCH")
	receivedPunch := false

	// Start receiving in parallel
	punchReceived := make(chan bool, 1)
	go func() {
		buf := make([]byte, 1024)
		for {
			c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, addr, err := c.conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}

			// Check if it's from the peer (not signaling server)
			if n > 0 && addr.IP.Equal(peerAddr.IP) && addr.Port == peerAddr.Port {
				select {
				case punchReceived <- true:
				default:
				}
				return
			}

			// Also accept from same IP different port (NAT may remap)
			if n > 0 && addr.IP.Equal(peerAddr.IP) && string(buf[:n]) == "PUNCH" {
				// Update peer address with actual port
				c.peerAddr = addr
				select {
				case punchReceived <- true:
				default:
				}
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			if receivedPunch {
				return nil
			}
			return fmt.Errorf("hole punch timeout - NAT may be symmetric")
		case <-punchReceived:
			// Keep sending for a bit more to ensure bidirectional hole
			time.Sleep(500 * time.Millisecond)
			for i := 0; i < 5; i++ {
				c.conn.WriteToUDP(punchPacket, peerAddr)
				time.Sleep(50 * time.Millisecond)
			}
			return nil
		case <-ticker.C:
			_, err := c.conn.WriteToUDP(punchPacket, peerAddr)
			if err != nil {
				continue
			}
		}
	}
}

// SendKeepAlive sends a keepalive to the signaling server
func (c *Client) SendKeepAlive() error {
	msg := SignalingMessage{Type: MsgTypePing}
	return c.sendMessage(&msg)
}

// GetConn returns the underlying UDP connection for the VPN tunnel
func (c *Client) GetConn() *net.UDPConn {
	return c.conn
}

// GetPeerAddr returns the peer's address
func (c *Client) GetPeerAddr() *net.UDPAddr {
	return c.peerAddr
}

// GetPublicAddr returns this client's public address
func (c *Client) GetPublicAddr() *net.UDPAddr {
	return c.publicAddr
}

// Close closes the client
func (c *Client) Close() error {
	return c.conn.Close()
}

// sendMessage sends a message to the signaling server
func (c *Client) sendMessage(msg *SignalingMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = c.conn.WriteToUDP(data, c.signalingAddr)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// receiveMessage receives a message with timeout
func (c *Client) receiveMessage(timeout time.Duration) (*SignalingMessage, error) {
	c.conn.SetReadDeadline(time.Now().Add(timeout))

	buf := make([]byte, 4096)
	n, _, err := c.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}

	var msg SignalingMessage
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}
