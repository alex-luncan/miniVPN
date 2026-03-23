package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

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
