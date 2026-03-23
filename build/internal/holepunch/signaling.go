package holepunch

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// Message types
const (
	MsgTypeDiscover   = "DISCOVER"
	MsgTypeDiscovered = "DISCOVERED"
	MsgTypeRegister   = "REGISTER"
	MsgTypeRegistered = "REGISTERED"
	MsgTypeConnect    = "CONNECT"
	MsgTypePeerInfo   = "PEER_INFO"
	MsgTypeError      = "ERROR"
	MsgTypePing       = "PING"
	MsgTypePong       = "PONG"
)

// SignalingMessage is the message format for signaling
type SignalingMessage struct {
	Type       string `json:"type"`
	SessionID  string `json:"sessionId,omitempty"`
	SecretCode string `json:"secretCode,omitempty"`
	Addr       string `json:"addr,omitempty"`
	PeerAddr   string `json:"peerAddr,omitempty"`
	Error      string `json:"error,omitempty"`
}

// RegisteredPeer represents a peer registered with the signaling server
type RegisteredPeer struct {
	Addr       *net.UDPAddr
	SecretCode string
	SessionID  string
	LastSeen   time.Time
}

// SignalingServer handles peer discovery and coordination
type SignalingServer struct {
	conn  *net.UDPConn
	peers map[string]*RegisteredPeer // keyed by secret code
	mu    sync.RWMutex
	done  chan struct{}
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer(port int) (*SignalingServer, error) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on UDP port %d: %w", port, err)
	}

	return &SignalingServer{
		conn:  conn,
		peers: make(map[string]*RegisteredPeer),
		done:  make(chan struct{}),
	}, nil
}

// Start starts the signaling server
func (s *SignalingServer) Start() {
	go s.cleanupLoop()
	go s.handleMessages()
}

// Stop stops the signaling server
func (s *SignalingServer) Stop() {
	close(s.done)
	s.conn.Close()
}

// handleMessages processes incoming messages
func (s *SignalingServer) handleMessages() {
	buf := make([]byte, 4096)

	for {
		select {
		case <-s.done:
			return
		default:
		}

		s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}

		// Handle raw DISCOVER for backward compatibility
		if string(buf[:n]) == "DISCOVER" {
			s.handleDiscover(addr)
			continue
		}

		// Parse JSON message
		var msg SignalingMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			continue
		}

		switch msg.Type {
		case MsgTypeDiscover:
			s.handleDiscover(addr)
		case MsgTypeRegister:
			s.handleRegister(addr, &msg)
		case MsgTypeConnect:
			s.handleConnect(addr, &msg)
		case MsgTypePing:
			s.handlePing(addr, &msg)
		}
	}
}

// handleDiscover responds with the peer's public address
func (s *SignalingServer) handleDiscover(addr *net.UDPAddr) {
	response := SignalingMessage{
		Type: MsgTypeDiscovered,
		Addr: addr.String(),
	}
	s.sendMessage(addr, &response)
}

// handleRegister registers a peer (server) with a secret code
func (s *SignalingServer) handleRegister(addr *net.UDPAddr, msg *SignalingMessage) {
	s.mu.Lock()
	s.peers[msg.SecretCode] = &RegisteredPeer{
		Addr:       addr,
		SecretCode: msg.SecretCode,
		SessionID:  msg.SessionID,
		LastSeen:   time.Now(),
	}
	s.mu.Unlock()

	response := SignalingMessage{
		Type:      MsgTypeRegistered,
		SessionID: msg.SessionID,
		Addr:      addr.String(),
	}
	s.sendMessage(addr, &response)
}

// handleConnect handles a client trying to connect to a registered server
func (s *SignalingServer) handleConnect(addr *net.UDPAddr, msg *SignalingMessage) {
	s.mu.RLock()
	peer, exists := s.peers[msg.SecretCode]
	s.mu.RUnlock()

	if !exists {
		response := SignalingMessage{
			Type:  MsgTypeError,
			Error: "peer not found",
		}
		s.sendMessage(addr, &response)
		return
	}

	// Send peer info to the connecting client
	clientResponse := SignalingMessage{
		Type:     MsgTypePeerInfo,
		PeerAddr: peer.Addr.String(),
	}
	s.sendMessage(addr, &clientResponse)

	// Notify the server about the incoming client
	serverResponse := SignalingMessage{
		Type:     MsgTypePeerInfo,
		PeerAddr: addr.String(),
	}
	s.sendMessage(peer.Addr, &serverResponse)
}

// handlePing updates the last seen time for a peer
func (s *SignalingServer) handlePing(addr *net.UDPAddr, msg *SignalingMessage) {
	s.mu.Lock()
	for _, peer := range s.peers {
		if peer.Addr.String() == addr.String() {
			peer.LastSeen = time.Now()
			break
		}
	}
	s.mu.Unlock()

	response := SignalingMessage{
		Type: MsgTypePong,
	}
	s.sendMessage(addr, &response)
}

// sendMessage sends a message to an address
func (s *SignalingServer) sendMessage(addr *net.UDPAddr, msg *SignalingMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	s.conn.WriteToUDP(data, addr)
}

// cleanupLoop removes stale peers
func (s *SignalingServer) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for code, peer := range s.peers {
				if now.Sub(peer.LastSeen) > 2*time.Minute {
					delete(s.peers, code)
				}
			}
			s.mu.Unlock()
		}
	}
}

// GetAddr returns the server's listening address
func (s *SignalingServer) GetAddr() net.Addr {
	return s.conn.LocalAddr()
}
