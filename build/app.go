package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"minivpn/internal/firewall"
	"minivpn/internal/holepunch"
	"minivpn/internal/splittunnel"
	"minivpn/internal/vpn"
)

// ConnectionInfo holds information about the current connection
type ConnectionInfo struct {
	State       string `json:"state"`
	ServerIP    string `json:"serverIP"`
	ConnectedAt string `json:"connectedAt"`
	BytesSent   uint64 `json:"bytesSent"`
	BytesRecv   uint64 `json:"bytesRecv"`
}

// ClientInfo holds information about a connected client (server mode)
type ClientInfo struct {
	SessionID   string `json:"sessionId"`
	RemoteAddr  string `json:"remoteAddr"`
	ConnectedAt string `json:"connectedAt"`
}

// SplitTunnelStatus holds split tunnel status information
type SplitTunnelStatus struct {
	Enabled    bool     `json:"enabled"`
	Active     bool     `json:"active"`
	Mode       string   `json:"mode"`
	Ports      []int    `json:"ports"`
	RuleCount  int      `json:"ruleCount"`
	IsAdmin    bool     `json:"isAdmin"`
}

// App struct holds the application state
type App struct {
	ctx context.Context
	mu  sync.RWMutex

	// Mode
	mode       string // "server" or "client"
	secretCode string

	// Server state
	server     *vpn.Server
	serverPort int

	// Client state
	client   *vpn.Client
	serverIP string

	// Split tunnel
	splitTunnel *splittunnel.Manager

	// NAT Traversal / Hole Punching
	signalingServer  *holepunch.SignalingServer
	holePunchClient  *holepunch.Client
	signalingAddr    string // Address of signaling server (e.g., "203.0.113.50:51821")
	useHolePunching  bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		serverPort:  51820,
		splitTunnel: splittunnel.NewManager(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Ensure Windows Firewall allows this app
	firewall.EnsureAppAllowed()
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop split tunneling
	if a.splitTunnel != nil {
		a.splitTunnel.Stop()
	}

	if a.server != nil {
		a.server.Stop()
	}

	if a.client != nil {
		a.client.Disconnect()
	}
}

// SetMode sets the application mode (server or client)
func (a *App) SetMode(mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if mode != "server" && mode != "client" {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	a.mode = mode

	if mode == "server" {
		// Generate new secret code for server mode
		a.secretCode = vpn.GenerateSecretCode()
	}

	return nil
}

// GetMode returns the current mode
func (a *App) GetMode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mode
}

// GetSecretCode returns the current secret code (server mode only)
func (a *App) GetSecretCode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.secretCode
}

// RegenerateSecretCode generates a new secret code (server mode only)
func (a *App) RegenerateSecretCode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.secretCode = vpn.GenerateSecretCode()
	return a.secretCode
}

// ConnectToServer attempts to connect to a VPN server (client mode)
func (a *App) ConnectToServer(serverIP string, port int, secretCode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.mode != "client" {
		return fmt.Errorf("not in client mode")
	}

	// Validate port
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	// Validate IP address
	if net.ParseIP(serverIP) == nil {
		// Try to resolve hostname
		_, err := net.LookupHost(serverIP)
		if err != nil {
			return fmt.Errorf("invalid server address: %s", serverIP)
		}
	}

	// Store connection info
	a.serverIP = serverIP
	a.serverPort = port
	a.secretCode = secretCode

	// Create VPN client
	client, err := vpn.NewClient(vpn.ClientConfig{
		ServerAddr: serverIP,
		ServerPort: port,
		SecretCode: secretCode,
		OnStateChange: func(state vpn.TunnelState) {
			// State change callback
			if state == vpn.TunnelStateConnected {
				// Start split tunneling when connected
				config := a.splitTunnel.GetConfig()
				if config.Enabled && len(config.Ports) > 0 {
					a.splitTunnel.Start()
				}
			} else if state == vpn.TunnelStateDisconnected {
				// Stop split tunneling when disconnected
				a.splitTunnel.Stop()
			}
		},
		OnError: func(err error) {
			// Error callback
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to server
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	a.client = client

	// Start split tunneling if configured
	config := a.splitTunnel.GetConfig()
	if config.Enabled && len(config.Ports) > 0 {
		// Set VPN interface info (placeholder - would use actual VPN interface)
		a.splitTunnel.SetVPNInterface(net.ParseIP(serverIP), "miniVPN")
		a.splitTunnel.Start()
	}

	return nil
}

// Disconnect disconnects from the VPN
func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Stop split tunneling first
	if a.splitTunnel != nil {
		a.splitTunnel.Stop()
	}

	if a.client != nil {
		a.client.Disconnect()
		a.client = nil
	}

	return nil
}

// IsConnected returns the connection status
func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.mode == "server" {
		return a.server != nil
	}

	return a.client != nil && a.client.IsConnected()
}

// GetConnectionInfo returns detailed connection information
func (a *App) GetConnectionInfo() ConnectionInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	info := ConnectionInfo{
		State: "disconnected",
	}

	if a.mode == "client" && a.client != nil {
		state := a.client.State()
		info.State = state.String()
		info.ServerIP = a.serverIP

		if state == vpn.TunnelStateConnected {
			stats := a.client.Stats()
			info.ConnectedAt = stats.ConnectedAt.Format(time.RFC3339)
			info.BytesSent = stats.BytesSent
			info.BytesRecv = stats.BytesReceived
		}
	} else if a.mode == "server" && a.server != nil {
		info.State = "running"
	}

	return info
}

// StartServer starts the VPN server (server mode only)
func (a *App) StartServer(port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.mode != "server" {
		return fmt.Errorf("not in server mode")
	}

	if a.server != nil {
		return fmt.Errorf("server already running")
	}

	a.serverPort = port

	// Create VPN server
	server, err := vpn.NewServer(vpn.ServerConfig{
		Port:       port,
		SecretCode: a.secretCode,
		OnClient: func(session *vpn.ClientSession) {
			// Client connected callback
		},
		OnError: func(err error) {
			// Error callback
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	a.server = server

	// Auto-register with signaling server if it's running locally
	if a.signalingServer != nil && a.signalingAddr != "" {
		go func() {
			// Small delay to ensure signaling server is fully ready
			time.Sleep(500 * time.Millisecond)
			a.registerWithSignalingServer()
		}()
	}

	return nil
}

// StopServer stops the VPN server
func (a *App) StopServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
		a.server.Stop()
		a.server = nil
	}

	return nil
}

// GetConnectedClients returns list of connected clients (server mode)
func (a *App) GetConnectedClients() []ClientInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.server == nil {
		return []ClientInfo{}
	}

	clients := a.server.GetClients()
	result := make([]ClientInfo, len(clients))

	for i, c := range clients {
		result[i] = ClientInfo{
			SessionID:   fmt.Sprintf("%x", c.ID[:8]),
			RemoteAddr:  c.RemoteAddr,
			ConnectedAt: c.ConnectedAt.Format(time.RFC3339),
		}
	}

	return result
}

// GetClientCount returns number of connected clients (server mode)
func (a *App) GetClientCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.server == nil {
		return 0
	}

	return a.server.ClientCount()
}

// SetTunneledPorts sets the ports to be tunneled through VPN
func (a *App) SetTunneledPorts(ports []int, mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if mode != "include" && mode != "exclude" {
		return fmt.Errorf("invalid tunnel mode: %s", mode)
	}

	// Convert to uint16 and validate
	portList := make([]uint16, 0, len(ports))
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port: %d", port)
		}
		portList = append(portList, uint16(port))
	}

	// Configure split tunnel manager
	config := splittunnel.Config{
		Enabled: len(portList) > 0,
		Mode:    splittunnel.Mode(mode),
		Ports:   portList,
	}

	if err := a.splitTunnel.Configure(config); err != nil {
		return fmt.Errorf("failed to configure split tunnel: %w", err)
	}

	// If connected, apply rules immediately
	if a.client != nil && a.client.IsConnected() {
		if config.Enabled {
			a.splitTunnel.Start()
		} else {
			a.splitTunnel.Stop()
		}
	}

	return nil
}

// GetTunneledPorts returns the current tunneled ports configuration
func (a *App) GetTunneledPorts() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	config := a.splitTunnel.GetConfig()

	// Convert uint16 to int for JSON
	ports := make([]int, len(config.Ports))
	for i, p := range config.Ports {
		ports[i] = int(p)
	}

	return map[string]interface{}{
		"ports": ports,
		"mode":  string(config.Mode),
	}
}

// GetSplitTunnelStatus returns the split tunnel status
func (a *App) GetSplitTunnelStatus() SplitTunnelStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	config := a.splitTunnel.GetConfig()

	// Convert uint16 to int for JSON
	ports := make([]int, len(config.Ports))
	for i, p := range config.Ports {
		ports[i] = int(p)
	}

	return SplitTunnelStatus{
		Enabled:   config.Enabled,
		Active:    a.splitTunnel.IsActive(),
		Mode:      string(config.Mode),
		Ports:     ports,
		RuleCount: len(config.Ports),
		IsAdmin:   splittunnel.RunAsAdmin(),
	}
}

// EnableSplitTunnel enables split tunneling
func (a *App) EnableSplitTunnel() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.splitTunnel.Enable()

	// If connected, start split tunneling
	if a.client != nil && a.client.IsConnected() {
		return a.splitTunnel.Start()
	}

	return nil
}

// DisableSplitTunnel disables split tunneling
func (a *App) DisableSplitTunnel() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.splitTunnel.Disable()
	return a.splitTunnel.Stop()
}

// GetLocalIP returns the local IP address
func (a *App) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "Unknown"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "Unknown"
}

// GetPublicIP returns the public/external IP address
func (a *App) GetPublicIP() string {
	// Try multiple services in case one is down
	services := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me/ip",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			ip := strings.TrimSpace(string(body))
			// Validate it looks like an IP
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	return "Unable to determine"
}

// GetServerPort returns the configured server port
func (a *App) GetServerPort() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.serverPort
}

// SetServerPort sets the server port (before starting)
func (a *App) SetServerPort(port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.server != nil {
		return fmt.Errorf("cannot change port while server is running")
	}

	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	a.serverPort = port
	return nil
}

// CheckAdminPrivileges checks if running with admin privileges
func (a *App) CheckAdminPrivileges() bool {
	return splittunnel.RunAsAdmin()
}

// ============== NAT Traversal / Hole Punching ==============

// SetSignalingServer sets the signaling server address for NAT traversal
func (a *App) SetSignalingServer(addr string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.signalingAddr = addr
	a.useHolePunching = addr != ""
	return nil
}

// GetSignalingServer returns the current signaling server address
func (a *App) GetSignalingServer() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.signalingAddr
}

// IsHolePunchingEnabled returns whether hole punching is enabled
func (a *App) IsHolePunchingEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.useHolePunching
}

// StartSignalingServer starts the signaling server (run on machine with public IP)
func (a *App) StartSignalingServer(port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.signalingServer != nil {
		return fmt.Errorf("signaling server already running")
	}

	server, err := holepunch.NewSignalingServer(port)
	if err != nil {
		return fmt.Errorf("failed to start signaling server: %w", err)
	}

	server.Start()
	a.signalingServer = server

	// Auto-set signaling address to localhost for local registration
	a.signalingAddr = fmt.Sprintf("127.0.0.1:%d", port)
	a.useHolePunching = true

	return nil
}

// StopSignalingServer stops the signaling server
func (a *App) StopSignalingServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.signalingServer != nil {
		a.signalingServer.Stop()
		a.signalingServer = nil
	}
	return nil
}

// IsSignalingServerRunning returns whether the signaling server is running
func (a *App) IsSignalingServerRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.signalingServer != nil
}

// RegisterForHolePunch registers this server with the signaling server for hole punching
func (a *App) RegisterForHolePunch() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.signalingAddr == "" {
		return fmt.Errorf("signaling server not configured")
	}

	if a.secretCode == "" {
		return fmt.Errorf("secret code not set")
	}

	client, err := holepunch.NewClient(a.signalingAddr)
	if err != nil {
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address
	publicAddr, err := client.DiscoverPublicAddr()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}

	// Register with signaling server
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.RegisterAsServer(a.secretCode, sessionID); err != nil {
		client.Close()
		return fmt.Errorf("failed to register with signaling server: %w", err)
	}

	a.holePunchClient = client
	_ = publicAddr // We have our public address now

	// Start keepalive goroutine
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			a.mu.RLock()
			hpClient := a.holePunchClient
			a.mu.RUnlock()

			if hpClient == nil {
				return
			}

			select {
			case <-ticker.C:
				hpClient.SendKeepAlive()
			case <-a.ctx.Done():
				return
			}
		}
	}()

	return nil
}

// registerWithSignalingServer is a helper that registers without UI interaction
func (a *App) registerWithSignalingServer() error {
	a.mu.Lock()
	signalingAddr := a.signalingAddr
	secretCode := a.secretCode
	a.mu.Unlock()

	if signalingAddr == "" {
		return fmt.Errorf("signaling server not configured")
	}
	if secretCode == "" {
		return fmt.Errorf("secret code not set")
	}

	client, err := holepunch.NewClient(signalingAddr)
	if err != nil {
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address
	_, err = client.DiscoverPublicAddr()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}

	// Register with signaling server
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	if err := client.RegisterAsServer(secretCode, sessionID); err != nil {
		client.Close()
		return fmt.Errorf("failed to register: %w", err)
	}

	a.mu.Lock()
	a.holePunchClient = client
	a.mu.Unlock()

	return nil
}

// ConnectWithHolePunch connects to a peer using hole punching
func (a *App) ConnectWithHolePunch(secretCode string) error {
	a.mu.Lock()
	if a.signalingAddr == "" {
		a.mu.Unlock()
		return fmt.Errorf("signaling server not configured")
	}
	signalingAddr := a.signalingAddr
	a.mu.Unlock()

	// Create hole punch client
	client, err := holepunch.NewClient(signalingAddr)
	if err != nil {
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address first
	_, err = client.DiscoverPublicAddr()
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}

	// Request peer info from signaling server
	peerAddr, err := client.ConnectToPeer(secretCode)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	// Perform hole punching
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := client.PunchHole(ctx, peerAddr); err != nil {
		client.Close()
		return fmt.Errorf("hole punching failed: %w", err)
	}

	// Store the client for the VPN tunnel
	a.mu.Lock()
	a.holePunchClient = client
	a.secretCode = secretCode
	a.serverIP = peerAddr.IP.String()
	a.mu.Unlock()

	return nil
}

// GetHolePunchStatus returns the current hole punch connection status
func (a *App) GetHolePunchStatus() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := map[string]interface{}{
		"enabled":          a.useHolePunching,
		"signalingServer":  a.signalingAddr,
		"signalingRunning": a.signalingServer != nil,
		"connected":        a.holePunchClient != nil,
	}

	if a.holePunchClient != nil {
		if pubAddr := a.holePunchClient.GetPublicAddr(); pubAddr != nil {
			status["publicAddr"] = pubAddr.String()
		}
		if peerAddr := a.holePunchClient.GetPeerAddr(); peerAddr != nil {
			status["peerAddr"] = peerAddr.String()
		}
	}

	return status
}
