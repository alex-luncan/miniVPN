package vpn

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// TunnelState represents the current state of a tunnel
type TunnelState int

const (
	TunnelStateDisconnected TunnelState = iota
	TunnelStateConnecting
	TunnelStateHandshaking
	TunnelStateConnected
	TunnelStateDisconnecting
)

func (s TunnelState) String() string {
	switch s {
	case TunnelStateDisconnected:
		return "disconnected"
	case TunnelStateConnecting:
		return "connecting"
	case TunnelStateHandshaking:
		return "handshaking"
	case TunnelStateConnected:
		return "connected"
	case TunnelStateDisconnecting:
		return "disconnecting"
	default:
		return "unknown"
	}
}

// TunnelStats holds tunnel statistics
type TunnelStats struct {
	BytesSent     uint64
	BytesReceived uint64
	PacketsSent   uint64
	PacketsRecv   uint64
	ConnectedAt   time.Time
	LastActivity  time.Time
}

// Tunnel represents an encrypted VPN tunnel
type Tunnel struct {
	conn      net.Conn
	cipher    *Cipher
	sessionID [16]byte
	state     TunnelState
	stats     TunnelStats
	mu        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// Callbacks
	onStateChange func(TunnelState)
	onData        func([]byte)
	onError       func(error)
}

// TunnelConfig holds tunnel configuration
type TunnelConfig struct {
	OnStateChange func(TunnelState)
	OnData        func([]byte)
	OnError       func(error)
}

// NewTunnel creates a new tunnel from an established connection
func NewTunnel(conn net.Conn, cipher *Cipher, sessionID [16]byte, config *TunnelConfig) *Tunnel {
	ctx, cancel := context.WithCancel(context.Background())

	t := &Tunnel{
		conn:      conn,
		cipher:    cipher,
		sessionID: sessionID,
		state:     TunnelStateConnected,
		ctx:       ctx,
		cancel:    cancel,
		stats: TunnelStats{
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
		},
	}

	if config != nil {
		t.onStateChange = config.OnStateChange
		t.onData = config.OnData
		t.onError = config.OnError
	}

	return t
}

// Start begins processing tunnel traffic
func (t *Tunnel) Start() {
	go t.readLoop()
	go t.keepAliveLoop()
}

// readLoop continuously reads from the connection
func (t *Tunnel) readLoop() {
	defer t.Close()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		msg, err := ReadMessage(t.conn)
		if err != nil {
			if t.onError != nil && t.ctx.Err() == nil {
				t.onError(fmt.Errorf("read error: %w", err))
			}
			return
		}

		t.mu.Lock()
		t.stats.LastActivity = time.Now()
		t.mu.Unlock()

		switch msg.Type {
		case MsgTypeData:
			t.handleData(msg.Payload)
		case MsgTypeKeepAlive:
			// Just update last activity
		case MsgTypeDisconnect:
			return
		}
	}
}

// keepAliveLoop sends periodic keep-alive messages
func (t *Tunnel) keepAliveLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.sendKeepAlive()
		}
	}
}

// handleData processes incoming encrypted data
func (t *Tunnel) handleData(encrypted []byte) {
	plaintext, err := t.cipher.Decrypt(encrypted)
	if err != nil {
		if t.onError != nil {
			t.onError(fmt.Errorf("decrypt error: %w", err))
		}
		return
	}

	t.mu.Lock()
	t.stats.BytesReceived += uint64(len(plaintext))
	t.stats.PacketsRecv++
	t.mu.Unlock()

	if t.onData != nil {
		t.onData(plaintext)
	}
}

// Send encrypts and sends data through the tunnel
func (t *Tunnel) Send(data []byte) error {
	t.mu.RLock()
	if t.state != TunnelStateConnected {
		t.mu.RUnlock()
		return fmt.Errorf("tunnel not connected")
	}
	t.mu.RUnlock()

	encrypted, err := t.cipher.Encrypt(data)
	if err != nil {
		return fmt.Errorf("encrypt error: %w", err)
	}

	msg := &Message{
		Type:    MsgTypeData,
		Payload: encrypted,
	}

	if err := WriteMessage(t.conn, msg); err != nil {
		return fmt.Errorf("send error: %w", err)
	}

	t.mu.Lock()
	t.stats.BytesSent += uint64(len(data))
	t.stats.PacketsSent++
	t.stats.LastActivity = time.Now()
	t.mu.Unlock()

	return nil
}

// sendKeepAlive sends a keep-alive message
func (t *Tunnel) sendKeepAlive() error {
	msg := &Message{
		Type:    MsgTypeKeepAlive,
		Payload: nil,
	}
	return WriteMessage(t.conn, msg)
}

// Close closes the tunnel
func (t *Tunnel) Close() error {
	t.mu.Lock()
	if t.state == TunnelStateDisconnected {
		t.mu.Unlock()
		return nil
	}
	t.state = TunnelStateDisconnecting
	t.mu.Unlock()

	t.cancel()

	// Send disconnect message
	msg := &Message{
		Type:    MsgTypeDisconnect,
		Payload: nil,
	}
	WriteMessage(t.conn, msg)

	t.conn.Close()

	t.mu.Lock()
	t.state = TunnelStateDisconnected
	t.mu.Unlock()

	if t.onStateChange != nil {
		t.onStateChange(TunnelStateDisconnected)
	}

	return nil
}

// State returns the current tunnel state
func (t *Tunnel) State() TunnelState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// Stats returns tunnel statistics
func (t *Tunnel) Stats() TunnelStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats
}

// SessionID returns the tunnel session ID
func (t *Tunnel) SessionID() [16]byte {
	return t.sessionID
}

// SetOnData sets the callback for incoming data packets
func (t *Tunnel) SetOnData(handler func([]byte)) {
	t.mu.Lock()
	t.onData = handler
	t.mu.Unlock()
}

// CopyTo copies data from the tunnel to a writer
func (t *Tunnel) CopyTo(w io.Writer) error {
	t.onData = func(data []byte) {
		w.Write(data)
	}
	<-t.ctx.Done()
	return t.ctx.Err()
}

// CopyFrom copies data from a reader to the tunnel
func (t *Tunnel) CopyFrom(r io.Reader) error {
	buf := make([]byte, 1500) // MTU size
	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		default:
		}

		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if err := t.Send(buf[:n]); err != nil {
			return err
		}
	}
}
