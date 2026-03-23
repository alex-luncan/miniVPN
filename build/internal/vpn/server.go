package vpn

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	Port       int
	SecretCode string
	OnClient   func(*ClientSession)
	OnError    func(error)
}

// ClientSession represents a connected client
type ClientSession struct {
	ID          [16]byte
	Tunnel      *Tunnel
	ConnectedAt time.Time
	RemoteAddr  string
	VPNAddress  net.IP // Assigned VPN IP address
}

// Server represents a VPN server
type Server struct {
	config     ServerConfig
	listener   net.Listener
	keyPair    *KeyPair
	secretHash [32]byte
	clients    map[[16]byte]*ClientSession
	mu         sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc

	// IP address management
	ipPool    *IPPool
	forwarder *Forwarder
}

// NewServer creates a new VPN server
func NewServer(config ServerConfig) (*Server, error) {
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Create IP pool
	ipPool, err := NewDefaultIPPool()
	if err != nil {
		return nil, fmt.Errorf("failed to create IP pool: %w", err)
	}

	// Create forwarder
	forwarder := NewForwarder()

	return &Server{
		config:     config,
		keyPair:    keyPair,
		secretHash: HashSecretCode(config.SecretCode),
		clients:    make(map[[16]byte]*ClientSession),
		ipPool:     ipPool,
		forwarder:  forwarder,
	}, nil
}

// Start starts the VPN server
func (s *Server) Start() error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	s.listener = listener
	s.ctx, s.cancel = context.WithCancel(context.Background())

	go s.acceptLoop()

	return nil
}

// Stop stops the VPN server
func (s *Server) Stop() error {
	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	// Close the forwarder
	if s.forwarder != nil {
		s.forwarder.Close()
	}

	// Close all client tunnels and release IPs
	s.mu.Lock()
	for _, client := range s.clients {
		if client.VPNAddress != nil && s.ipPool != nil {
			s.ipPool.Release(client.VPNAddress)
		}
		client.Tunnel.Close()
	}
	s.clients = make(map[[16]byte]*ClientSession)
	s.mu.Unlock()

	return nil
}

// acceptLoop accepts incoming connections
func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			if s.config.OnError != nil {
				s.config.OnError(fmt.Errorf("accept error: %w", err))
			}
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles a new client connection
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			conn.Close()
		}
	}()

	// Set initial timeout for handshake
	conn.SetDeadline(time.Now().Add(30 * time.Second))

	// Read handshake init
	msg, err := ReadMessage(conn)
	if err != nil {
		conn.Close()
		return
	}

	if msg.Type != MsgTypeHandshakeInit {
		conn.Close()
		return
	}

	init, err := DecodeHandshakeInit(msg.Payload)
	if err != nil {
		conn.Close()
		return
	}

	// Verify protocol version
	if init.Version != ProtocolVersion {
		conn.Close()
		return
	}

	// Verify secret code hash
	if init.SecretHash != s.secretHash {
		conn.Close()
		return
	}

	// Check timestamp (prevent replay attacks, allow 5 minute window)
	now := time.Now().Unix()
	if init.Timestamp < now-300 || init.Timestamp > now+300 {
		conn.Close()
		return
	}

	// Generate session ID
	var sessionID [16]byte
	if _, err := io.ReadFull(rand.Reader, sessionID[:]); err != nil {
		conn.Close()
		return
	}

	// Send handshake response
	resp := &HandshakeResponse{
		Version:   ProtocolVersion,
		SessionID: sessionID,
		Timestamp: time.Now().Unix(),
	}
	copy(resp.ServerPubKey[:], s.keyPair.PublicKey[:])

	respMsg := &Message{
		Type:    MsgTypeHandshakeResponse,
		Payload: EncodeHandshakeResponse(resp),
	}

	if err := WriteMessage(conn, respMsg); err != nil {
		conn.Close()
		return
	}

	// Compute shared secret
	sharedSecret, err := ComputeSharedSecret(s.keyPair.PrivateKey, init.ClientPubKey)
	if err != nil {
		conn.Close()
		return
	}

	// Derive session key
	sessionKey := DeriveKey(sharedSecret, sessionID[:])

	// Create cipher
	cipher, err := NewCipher(sessionKey)
	if err != nil {
		conn.Close()
		return
	}

	// Wait for handshake complete
	completeMsg, err := ReadMessage(conn)
	if err != nil {
		conn.Close()
		return
	}

	if completeMsg.Type != MsgTypeHandshakeComplete {
		conn.Close()
		return
	}

	complete, err := DecodeHandshakeComplete(completeMsg.Payload)
	if err != nil {
		conn.Close()
		return
	}

	// Verify session ID
	if complete.SessionID != sessionID {
		conn.Close()
		return
	}

	// Decrypt and verify confirmation
	confirmation, err := cipher.Decrypt(complete.Encrypted)
	if err != nil {
		conn.Close()
		return
	}

	expectedConfirmation := []byte("MINIVPN_HANDSHAKE_COMPLETE")
	if string(confirmation) != string(expectedConfirmation) {
		conn.Close()
		return
	}

	// Clear deadline for established connection
	conn.SetDeadline(time.Time{})

	// Allocate VPN IP for this client
	clientIP, err := s.ipPool.Allocate()
	if err != nil {
		conn.Close()
		return
	}

	// Send IP assignment
	serverIP := s.ipPool.ServerIP()
	subnetMask := s.ipPool.SubnetMask()

	ipAssign := &IPAssignment{
		MTU: 1420,
	}
	copy(ipAssign.ClientIP[:], clientIP.To4())
	copy(ipAssign.ServerIP[:], serverIP.To4())
	copy(ipAssign.SubnetMask[:], subnetMask.To4())

	ipAssignMsg := &Message{
		Type:    MsgTypeIPAssign,
		Payload: EncodeIPAssignment(ipAssign),
	}

	if err := WriteMessage(conn, ipAssignMsg); err != nil {
		s.ipPool.Release(clientIP)
		conn.Close()
		return
	}

	// Create tunnel first without OnData (we'll set it after)
	tunnel := NewTunnel(conn, cipher, sessionID, &TunnelConfig{
		OnStateChange: func(state TunnelState) {
			if state == TunnelStateDisconnected {
				s.removeClient(sessionID)
			}
		},
		OnError: func(err error) {
			if s.config.OnError != nil {
				s.config.OnError(fmt.Errorf("client %x: %w", sessionID[:4], err))
			}
		},
	})

	// Set OnData callback now that tunnel variable is assigned
	tunnel.SetOnData(func(packet []byte) {
		// Forward packets from client to internet
		s.forwarder.ForwardPacket(packet, sessionID, func(response []byte) {
			// Send response back to client through tunnel
			tunnel.Send(response)
		})
	})

	// Create client session
	session := &ClientSession{
		ID:          sessionID,
		Tunnel:      tunnel,
		ConnectedAt: time.Now(),
		RemoteAddr:  conn.RemoteAddr().String(),
		VPNAddress:  clientIP,
	}

	// Add to clients map
	s.mu.Lock()
	s.clients[sessionID] = session
	s.mu.Unlock()

	// Start tunnel
	tunnel.Start()

	// Notify callback
	if s.config.OnClient != nil {
		s.config.OnClient(session)
	}
}

// removeClient removes a client from the server
func (s *Server) removeClient(sessionID [16]byte) {
	s.mu.Lock()
	if client, exists := s.clients[sessionID]; exists {
		// Release the client's VPN IP
		if client.VPNAddress != nil && s.ipPool != nil {
			s.ipPool.Release(client.VPNAddress)
		}
		delete(s.clients, sessionID)
	}
	s.mu.Unlock()
}

// ClientCount returns the number of connected clients
func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// GetClients returns a list of connected clients
func (s *Server) GetClients() []*ClientSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*ClientSession, 0, len(s.clients))
	for _, c := range s.clients {
		clients = append(clients, c)
	}
	return clients
}

// Broadcast sends data to all connected clients
func (s *Server) Broadcast(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, client := range s.clients {
		client.Tunnel.Send(data)
	}
}

// Address returns the server's listening address
func (s *Server) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// PublicKey returns the server's public key
func (s *Server) PublicKey() [32]byte {
	return s.keyPair.PublicKey
}
