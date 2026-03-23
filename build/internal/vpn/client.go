package vpn

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// ClientConfig holds client configuration
type ClientConfig struct {
	ServerAddr    string
	ServerPort    int
	SecretCode    string
	OnStateChange func(TunnelState)
	OnData        func([]byte)
	OnError       func(error)
}

// Client represents a VPN client
type Client struct {
	config  ClientConfig
	keyPair *KeyPair
	tunnel  *Tunnel
	mu      sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new VPN client
func NewClient(config ClientConfig) (*Client, error) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &Client{
		config:  config,
		keyPair: keyPair,
	}, nil
}

// Connect establishes a connection to the VPN server
func (c *Client) Connect() error {
	c.mu.Lock()
	if c.tunnel != nil && c.tunnel.State() == TunnelStateConnected {
		c.mu.Unlock()
		return fmt.Errorf("already connected")
	}
	c.mu.Unlock()

	// Notify connecting state
	if c.config.OnStateChange != nil {
		c.config.OnStateChange(TunnelStateConnecting)
	}

	// Connect to server
	addr := fmt.Sprintf("%s:%d", c.config.ServerAddr, c.config.ServerPort)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		if c.config.OnStateChange != nil {
			c.config.OnStateChange(TunnelStateDisconnected)
		}
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Perform handshake
	tunnel, err := c.performHandshake(conn)
	if err != nil {
		conn.Close()
		if c.config.OnStateChange != nil {
			c.config.OnStateChange(TunnelStateDisconnected)
		}
		return fmt.Errorf("handshake failed: %w", err)
	}

	c.mu.Lock()
	c.tunnel = tunnel
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.mu.Unlock()

	// Start tunnel
	tunnel.Start()

	// Notify connected state
	if c.config.OnStateChange != nil {
		c.config.OnStateChange(TunnelStateConnected)
	}

	return nil
}

// performHandshake performs the VPN handshake with the server
func (c *Client) performHandshake(conn net.Conn) (*Tunnel, error) {
	// Set handshake timeout
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	if c.config.OnStateChange != nil {
		c.config.OnStateChange(TunnelStateHandshaking)
	}

	// Prepare handshake init
	init := &HandshakeInit{
		Version:   ProtocolVersion,
		Timestamp: time.Now().Unix(),
	}
	init.SecretHash = HashSecretCode(c.config.SecretCode)
	copy(init.ClientPubKey[:], c.keyPair.PublicKey[:])

	// Send handshake init
	initMsg := &Message{
		Type:    MsgTypeHandshakeInit,
		Payload: EncodeHandshakeInit(init),
	}

	if err := WriteMessage(conn, initMsg); err != nil {
		return nil, fmt.Errorf("failed to send init: %w", err)
	}

	// Read handshake response
	respMsg, err := ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if respMsg.Type != MsgTypeHandshakeResponse {
		return nil, fmt.Errorf("unexpected message type: %d", respMsg.Type)
	}

	resp, err := DecodeHandshakeResponse(respMsg.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Verify protocol version
	if resp.Version != ProtocolVersion {
		return nil, fmt.Errorf("protocol version mismatch: %d != %d", resp.Version, ProtocolVersion)
	}

	// Compute shared secret
	sharedSecret, err := ComputeSharedSecret(c.keyPair.PrivateKey, resp.ServerPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to compute shared secret: %w", err)
	}

	// Derive session key
	sessionKey := DeriveKey(sharedSecret, resp.SessionID[:])

	// Create cipher
	cipher, err := NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Encrypt confirmation
	confirmation := []byte("MINIVPN_HANDSHAKE_COMPLETE")
	encrypted, err := cipher.Encrypt(confirmation)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt confirmation: %w", err)
	}

	// Send handshake complete
	complete := &HandshakeComplete{
		SessionID: resp.SessionID,
		Encrypted: encrypted,
	}

	completeMsg := &Message{
		Type:    MsgTypeHandshakeComplete,
		Payload: EncodeHandshakeComplete(complete),
	}

	if err := WriteMessage(conn, completeMsg); err != nil {
		return nil, fmt.Errorf("failed to send complete: %w", err)
	}

	// Clear deadline for established connection
	conn.SetDeadline(time.Time{})

	// Create tunnel
	tunnel := NewTunnel(conn, cipher, resp.SessionID, &TunnelConfig{
		OnStateChange: c.config.OnStateChange,
		OnData:        c.config.OnData,
		OnError:       c.config.OnError,
	})

	return tunnel, nil
}

// Disconnect closes the VPN connection
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	if c.tunnel != nil {
		c.tunnel.Close()
		c.tunnel = nil
	}

	return nil
}

// Send sends data through the VPN tunnel
func (c *Client) Send(data []byte) error {
	c.mu.RLock()
	tunnel := c.tunnel
	c.mu.RUnlock()

	if tunnel == nil {
		return fmt.Errorf("not connected")
	}

	return tunnel.Send(data)
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.tunnel != nil && c.tunnel.State() == TunnelStateConnected
}

// State returns the current tunnel state
func (c *Client) State() TunnelState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tunnel == nil {
		return TunnelStateDisconnected
	}
	return c.tunnel.State()
}

// Stats returns tunnel statistics
func (c *Client) Stats() TunnelStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tunnel == nil {
		return TunnelStats{}
	}
	return c.tunnel.Stats()
}

// SessionID returns the current session ID
func (c *Client) SessionID() [16]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tunnel == nil {
		return [16]byte{}
	}
	return c.tunnel.SessionID()
}
