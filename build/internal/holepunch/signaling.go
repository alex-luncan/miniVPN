package holepunch

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Debug logging - set to true to enable verbose logging
var DebugSignaling = true

func debugLog(format string, args ...interface{}) {
	if DebugSignaling {
		log.Printf("[SIGNALING] "+format, args...)
	}
}

// maskSecret masks the secret code for logging (shows first 4 chars)
func maskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:4] + "****"
}

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
		debugLog("ERROR: Failed to listen on UDP port %d: %v", port, err)
		return nil, fmt.Errorf("failed to listen on UDP port %d: %w", port, err)
	}

	debugLog("Signaling server created on port %d", port)
	return &SignalingServer{
		conn:  conn,
		peers: make(map[string]*RegisteredPeer),
		done:  make(chan struct{}),
	}, nil
}

// Start starts the signaling server
func (s *SignalingServer) Start() {
	debugLog("Signaling server starting...")
	go s.cleanupLoop()
	go s.handleMessages()
	debugLog("Signaling server started and listening")
}

// Stop stops the signaling server
func (s *SignalingServer) Stop() {
	close(s.done)
	s.conn.Close()
}

// handleMessages processes incoming messages
func (s *SignalingServer) handleMessages() {
	buf := make([]byte, 4096)
	debugLog("Message handler started")

	for {
		select {
		case <-s.done:
			debugLog("Message handler stopping (done signal)")
			return
		default:
		}

		s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			debugLog("Read error from UDP: %v", err)
			continue
		}

		debugLog("Received %d bytes from %s", n, addr.String())

		// Handle raw DISCOVER for backward compatibility
		if string(buf[:n]) == "DISCOVER" {
			debugLog("Received raw DISCOVER from %s", addr.String())
			s.handleDiscover(addr)
			continue
		}

		// Parse JSON message
		var msg SignalingMessage
		if err := json.Unmarshal(buf[:n], &msg); err != nil {
			debugLog("Failed to parse JSON from %s: %v (data: %s)", addr.String(), err, string(buf[:n]))
			continue
		}

		debugLog("Received message type=%s from %s (secretCode=%s)", msg.Type, addr.String(), maskSecret(msg.SecretCode))

		switch msg.Type {
		case MsgTypeDiscover:
			s.handleDiscover(addr)
		case MsgTypeRegister:
			s.handleRegister(addr, &msg)
		case MsgTypeConnect:
			s.handleConnect(addr, &msg)
		case MsgTypePing:
			s.handlePing(addr, &msg)
		default:
			debugLog("Unknown message type: %s", msg.Type)
		}
	}
}

// handleDiscover responds with the peer's public address
func (s *SignalingServer) handleDiscover(addr *net.UDPAddr) {
	debugLog("DISCOVER: Client %s asking for their public address", addr.String())
	response := SignalingMessage{
		Type: MsgTypeDiscovered,
		Addr: addr.String(),
	}
	s.sendMessage(addr, &response)
	debugLog("DISCOVER: Replied to %s with public addr: %s", addr.String(), addr.String())
}

// handleRegister registers a peer (server) with a secret code
func (s *SignalingServer) handleRegister(addr *net.UDPAddr, msg *SignalingMessage) {
	debugLog("REGISTER: Server %s registering with secret=%s sessionID=%s", addr.String(), maskSecret(msg.SecretCode), msg.SessionID)

	s.mu.Lock()
	s.peers[msg.SecretCode] = &RegisteredPeer{
		Addr:       addr,
		SecretCode: msg.SecretCode,
		SessionID:  msg.SessionID,
		LastSeen:   time.Now(),
	}
	peerCount := len(s.peers)
	s.mu.Unlock()

	debugLog("REGISTER: Server %s registered successfully (total registered peers: %d)", addr.String(), peerCount)

	response := SignalingMessage{
		Type:      MsgTypeRegistered,
		SessionID: msg.SessionID,
		Addr:      addr.String(),
	}
	s.sendMessage(addr, &response)
}

// handleConnect handles a client trying to connect to a registered server
func (s *SignalingServer) handleConnect(addr *net.UDPAddr, msg *SignalingMessage) {
	debugLog("CONNECT: Client %s trying to connect with secret=%s", addr.String(), maskSecret(msg.SecretCode))

	s.mu.RLock()
	peer, exists := s.peers[msg.SecretCode]
	// Log all registered peers for debugging
	registeredSecrets := make([]string, 0, len(s.peers))
	for secret := range s.peers {
		registeredSecrets = append(registeredSecrets, maskSecret(secret))
	}
	s.mu.RUnlock()

	debugLog("CONNECT: Currently registered peers: %v", registeredSecrets)

	if !exists {
		debugLog("CONNECT: FAILED - No peer found for secret=%s (registered: %d peers)", maskSecret(msg.SecretCode), len(registeredSecrets))
		response := SignalingMessage{
			Type:  MsgTypeError,
			Error: "peer not found",
		}
		s.sendMessage(addr, &response)
		return
	}

	debugLog("CONNECT: SUCCESS - Found peer %s for secret=%s", peer.Addr.String(), maskSecret(msg.SecretCode))

	// Send peer info to the connecting client
	clientResponse := SignalingMessage{
		Type:     MsgTypePeerInfo,
		PeerAddr: peer.Addr.String(),
	}
	s.sendMessage(addr, &clientResponse)
	debugLog("CONNECT: Sent peer addr %s to client %s", peer.Addr.String(), addr.String())

	// Notify the server about the incoming client
	serverResponse := SignalingMessage{
		Type:     MsgTypePeerInfo,
		PeerAddr: addr.String(),
	}
	s.sendMessage(peer.Addr, &serverResponse)
	debugLog("CONNECT: Notified server %s about client %s", peer.Addr.String(), addr.String())
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
			debugLog("Cleanup loop stopping")
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			removed := 0
			for code, peer := range s.peers {
				if now.Sub(peer.LastSeen) > 2*time.Minute {
					debugLog("CLEANUP: Removing stale peer secret=%s (last seen: %v ago)", maskSecret(code), now.Sub(peer.LastSeen))
					delete(s.peers, code)
					removed++
				}
			}
			remaining := len(s.peers)
			s.mu.Unlock()
			if removed > 0 {
				debugLog("CLEANUP: Removed %d stale peers, %d remaining", removed, remaining)
			}
		}
	}
}

// GetAddr returns the server's listening address
func (s *SignalingServer) GetAddr() net.Addr {
	return s.conn.LocalAddr()
}
