package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"minivpn/internal/firewall"
	"minivpn/internal/holepunch"
	"minivpn/internal/splittunnel"
	"minivpn/internal/tun"
	"minivpn/internal/vpn"
)

// Debug logging for app
var DebugApp = true

func appDebugLog(format string, args ...interface{}) {
	if DebugApp {
		log.Printf("[APP] "+format, args...)
	}
}

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

	// TUN adapter and bridge (for actual VPN traffic routing)
	tunAdapter *tun.Adapter
	bridge     *vpn.Bridge

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

	// Teardown VPN routes
	splittunnel.TeardownVPNRoutes()

	// Stop bridge
	if a.bridge != nil {
		a.bridge.Stop()
		a.bridge = nil
	}

	// Stop TUN adapter
	if a.tunAdapter != nil {
		a.tunAdapter.Stop()
		a.tunAdapter = nil
	}

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

	maskedSecret := secretCode
	if len(maskedSecret) > 4 {
		maskedSecret = maskedSecret[:4] + "****"
	}
	appDebugLog("ConnectToServer called: server=%s:%d secret=%s", serverIP, port, maskedSecret)

	if a.mode != "client" {
		appDebugLog("ERROR: Not in client mode (current mode: %s)", a.mode)
		return fmt.Errorf("not in client mode")
	}

	// Validate port
	if port < 1 || port > 65535 {
		appDebugLog("ERROR: Invalid port: %d", port)
		return fmt.Errorf("invalid port: %d", port)
	}

	// Resolve server address to IP
	appDebugLog("Resolving server address: %s", serverIP)
	serverRealIP := net.ParseIP(serverIP)
	if serverRealIP == nil {
		// Try to resolve hostname
		appDebugLog("Not an IP, trying DNS lookup for: %s", serverIP)
		addrs, err := net.LookupHost(serverIP)
		if err != nil || len(addrs) == 0 {
			appDebugLog("ERROR: Failed to resolve hostname %s: %v", serverIP, err)
			return fmt.Errorf("invalid server address: %s", serverIP)
		}
		serverRealIP = net.ParseIP(addrs[0])
		appDebugLog("Resolved %s to %s", serverIP, serverRealIP.String())
	}

	// Store connection info
	a.serverIP = serverIP
	a.serverPort = port
	a.secretCode = secretCode

	appDebugLog("Creating VPN client...")
	// Create VPN client
	client, err := vpn.NewClient(vpn.ClientConfig{
		ServerAddr: serverIP,
		ServerPort: port,
		SecretCode: secretCode,
		OnStateChange: func(state vpn.TunnelState) {
			appDebugLog("VPN state changed: %s", state.String())
			// State change callback
			if state == vpn.TunnelStateDisconnected {
				// Cleanup on disconnect
				a.cleanupVPNConnection()
			}
		},
		OnError: func(err error) {
			appDebugLog("VPN error: %v", err)
		},
	})
	if err != nil {
		appDebugLog("ERROR: Failed to create VPN client: %v", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Connect to server (this also receives IP assignment)
	appDebugLog("Connecting to server %s:%d via TCP...", serverIP, port)
	appDebugLog("HINT: If this times out, check that:")
	appDebugLog("  1. The server is running")
	appDebugLog("  2. Port %d TCP is open in the server's firewall/NSG", port)
	appDebugLog("  3. The server IP %s is reachable", serverIP)
	if err := client.Connect(); err != nil {
		appDebugLog("ERROR: Connection failed: %v", err)
		return fmt.Errorf("connection failed: %w", err)
	}
	appDebugLog("TCP connection established!")

	a.client = client

	// Get assigned IP info from client
	appDebugLog("Getting IP assignment from server...")
	assignedIP := client.AssignedIP()
	serverVPNIP := client.ServerVPNIP()
	subnetMask := client.SubnetMask()
	mtu := client.MTU()

	appDebugLog("  Assigned IP: %v", assignedIP)
	appDebugLog("  Server VPN IP: %v", serverVPNIP)
	appDebugLog("  Subnet mask: %v", subnetMask)
	appDebugLog("  MTU: %d", mtu)

	if assignedIP == nil || serverVPNIP == nil {
		appDebugLog("ERROR: IP assignment failed (assignedIP=%v, serverVPNIP=%v)", assignedIP, serverVPNIP)
		client.Disconnect()
		return fmt.Errorf("failed to get IP assignment from server")
	}

	// Create TUN adapter with assigned IP
	appDebugLog("Creating TUN adapter...")
	tunConfig := tun.AdapterConfig{
		Name:       "miniVPN",
		LocalIP:    assignedIP.String(),
		RemoteIP:   serverVPNIP.String(),
		SubnetMask: ipMaskToString(subnetMask),
		MTU:        mtu,
	}

	tunAdapter, err := tun.NewAdapter(tunConfig)
	if err != nil {
		appDebugLog("ERROR: Failed to create TUN adapter: %v", err)
		client.Disconnect()
		return fmt.Errorf("failed to create TUN adapter: %w", err)
	}
	appDebugLog("TUN adapter created")

	// Start TUN adapter
	appDebugLog("Starting TUN adapter...")
	if err := tunAdapter.Start(); err != nil {
		appDebugLog("ERROR: Failed to start TUN adapter: %v", err)
		client.Disconnect()
		return fmt.Errorf("failed to start TUN adapter: %w", err)
	}
	appDebugLog("TUN adapter started")

	a.tunAdapter = tunAdapter

	// Create and start bridge between TUN and tunnel
	appDebugLog("Creating bridge...")
	bridge, err := vpn.NewBridge(vpn.BridgeConfig{
		Adapter: tunAdapter,
		Tunnel:  client.Tunnel(),
		MTU:     mtu,
	})
	if err != nil {
		appDebugLog("ERROR: Failed to create bridge: %v", err)
		tunAdapter.Stop()
		client.Disconnect()
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	appDebugLog("Starting bridge...")
	if err := bridge.Start(); err != nil {
		appDebugLog("ERROR: Failed to start bridge: %v", err)
		tunAdapter.Stop()
		client.Disconnect()
		return fmt.Errorf("failed to start bridge: %w", err)
	}
	appDebugLog("Bridge started")

	a.bridge = bridge

	// Setup VPN routes (route all traffic through VPN)
	appDebugLog("Setting up VPN routes...")
	appDebugLog("  Server real IP: %s", serverRealIP.String())
	appDebugLog("  VPN gateway: %s", serverVPNIP.String())
	if err := splittunnel.SetupVPNRoutes(serverRealIP, serverVPNIP, "miniVPN"); err != nil {
		appDebugLog("ERROR: Failed to setup VPN routes: %v", err)
		bridge.Stop()
		tunAdapter.Stop()
		client.Disconnect()
		return fmt.Errorf("failed to setup VPN routes: %w", err)
	}
	appDebugLog("VPN routes configured successfully!")

	// Start split tunneling if configured
	config := a.splitTunnel.GetConfig()
	if config.Enabled && len(config.Ports) > 0 {
		appDebugLog("Starting split tunnel...")
		a.splitTunnel.SetVPNInterface(serverVPNIP, "miniVPN")
		a.splitTunnel.Start()
	}

	appDebugLog("CONNECTION COMPLETE - VPN is now active!")
	return nil
}

// cleanupVPNConnection cleans up VPN resources when disconnecting
func (a *App) cleanupVPNConnection() {
	// Stop split tunneling
	if a.splitTunnel != nil {
		a.splitTunnel.Stop()
	}

	// Teardown VPN routes
	splittunnel.TeardownVPNRoutes()

	// Stop bridge
	if a.bridge != nil {
		a.bridge.Stop()
		a.bridge = nil
	}

	// Stop TUN adapter
	if a.tunAdapter != nil {
		a.tunAdapter.Stop()
		a.tunAdapter = nil
	}
}

// ipMaskToString converts net.IPMask to dotted decimal string
func ipMaskToString(mask net.IPMask) string {
	if len(mask) == 4 {
		return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	}
	return "255.255.255.0"
}

// Disconnect disconnects from the VPN
func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Cleanup VPN connection resources
	a.cleanupVPNConnection()

	// Disconnect client
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

	appDebugLog("StartServer called on port %d", port)

	if a.mode != "server" {
		appDebugLog("ERROR: Not in server mode")
		return fmt.Errorf("not in server mode")
	}

	if a.server != nil {
		appDebugLog("ERROR: Server already running")
		return fmt.Errorf("server already running")
	}

	a.serverPort = port

	appDebugLog("Creating VPN server on TCP port %d...", port)
	// Create VPN server
	server, err := vpn.NewServer(vpn.ServerConfig{
		Port:       port,
		SecretCode: a.secretCode,
		OnClient: func(session *vpn.ClientSession) {
			appDebugLog("Client connected: %s", session.RemoteAddr)
		},
		OnError: func(err error) {
			appDebugLog("Server error: %v", err)
		},
	})
	if err != nil {
		appDebugLog("ERROR: Failed to create server: %v", err)
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server
	appDebugLog("Starting VPN server...")
	if err := server.Start(); err != nil {
		appDebugLog("ERROR: Failed to start server: %v", err)
		return fmt.Errorf("failed to start server: %w", err)
	}

	a.server = server
	appDebugLog("SUCCESS: VPN server running on port %d", port)

	// Auto-register with signaling server if it's running locally
	if a.signalingServer != nil && a.signalingAddr != "" {
		appDebugLog("Signaling server is running locally, auto-registering in 500ms...")
		go func() {
			// Small delay to ensure signaling server is fully ready
			time.Sleep(500 * time.Millisecond)
			if err := a.registerWithSignalingServer(); err != nil {
				appDebugLog("Auto-registration failed: %v", err)
			} else {
				appDebugLog("Auto-registration successful!")
			}
		}()
	} else {
		appDebugLog("Signaling server not running locally (signalingServer=%v, signalingAddr=%s)", a.signalingServer != nil, a.signalingAddr)
		appDebugLog("HINT: To enable NAT traversal, start the signaling server or configure an external one")
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

	appDebugLog("SetSignalingServer: addr=%s", addr)
	a.signalingAddr = addr
	a.useHolePunching = addr != ""
	appDebugLog("  useHolePunching=%v", a.useHolePunching)
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

	appDebugLog("Starting signaling server on UDP port %d", port)

	if a.signalingServer != nil {
		appDebugLog("ERROR: Signaling server already running")
		return fmt.Errorf("signaling server already running")
	}

	server, err := holepunch.NewSignalingServer(port)
	if err != nil {
		appDebugLog("ERROR: Failed to start signaling server: %v", err)
		return fmt.Errorf("failed to start signaling server: %w", err)
	}

	server.Start()
	a.signalingServer = server

	// Auto-set signaling address to localhost for local registration
	a.signalingAddr = fmt.Sprintf("127.0.0.1:%d", port)
	a.useHolePunching = true

	appDebugLog("SUCCESS: Signaling server started on port %d, signalingAddr=%s", port, a.signalingAddr)
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

	appDebugLog("RegisterForHolePunch called")
	appDebugLog("  signalingAddr: %s", a.signalingAddr)
	appDebugLog("  secretCode: %s****", func() string {
		if len(a.secretCode) > 4 {
			return a.secretCode[:4]
		}
		return "****"
	}())

	if a.signalingAddr == "" {
		appDebugLog("ERROR: Signaling server not configured")
		return fmt.Errorf("signaling server not configured")
	}

	if a.secretCode == "" {
		appDebugLog("ERROR: Secret code not set")
		return fmt.Errorf("secret code not set")
	}

	appDebugLog("Creating holepunch client for signaling server: %s", a.signalingAddr)
	client, err := holepunch.NewClient(a.signalingAddr)
	if err != nil {
		appDebugLog("ERROR: Failed to create holepunch client: %v", err)
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address
	appDebugLog("Discovering public address...")
	publicAddr, err := client.DiscoverPublicAddr()
	if err != nil {
		appDebugLog("ERROR: Failed to discover public address: %v", err)
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}
	appDebugLog("Discovered public address: %s", publicAddr.String())

	// Register with signaling server
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	appDebugLog("Registering with signaling server (sessionID: %s)...", sessionID)
	if err := client.RegisterAsServer(a.secretCode, sessionID); err != nil {
		appDebugLog("ERROR: Failed to register with signaling server: %v", err)
		client.Close()
		return fmt.Errorf("failed to register with signaling server: %w", err)
	}

	appDebugLog("SUCCESS: Registered with signaling server!")
	a.holePunchClient = client

	// Start keepalive goroutine
	appDebugLog("Starting keepalive goroutine (interval: 30s)")
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			a.mu.RLock()
			hpClient := a.holePunchClient
			a.mu.RUnlock()

			if hpClient == nil {
				appDebugLog("Keepalive goroutine exiting (client is nil)")
				return
			}

			select {
			case <-ticker.C:
				appDebugLog("Sending keepalive to signaling server")
				hpClient.SendKeepAlive()
			case <-a.ctx.Done():
				appDebugLog("Keepalive goroutine exiting (context done)")
				return
			}
		}
	}()

	return nil
}

// registerWithSignalingServer is a helper that registers without UI interaction
func (a *App) registerWithSignalingServer() error {
	appDebugLog("registerWithSignalingServer called (internal)")

	a.mu.Lock()
	signalingAddr := a.signalingAddr
	secretCode := a.secretCode
	a.mu.Unlock()

	maskedSecret := secretCode
	if len(maskedSecret) > 4 {
		maskedSecret = maskedSecret[:4] + "****"
	}
	appDebugLog("  signalingAddr: %s", signalingAddr)
	appDebugLog("  secretCode: %s", maskedSecret)

	if signalingAddr == "" {
		appDebugLog("ERROR: Signaling server not configured")
		return fmt.Errorf("signaling server not configured")
	}
	if secretCode == "" {
		appDebugLog("ERROR: Secret code not set")
		return fmt.Errorf("secret code not set")
	}

	appDebugLog("Creating holepunch client for: %s", signalingAddr)
	client, err := holepunch.NewClient(signalingAddr)
	if err != nil {
		appDebugLog("ERROR: Failed to create holepunch client: %v", err)
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address
	appDebugLog("Discovering public address...")
	publicAddr, err := client.DiscoverPublicAddr()
	if err != nil {
		appDebugLog("ERROR: Failed to discover public address: %v", err)
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}
	appDebugLog("Our public address: %s", publicAddr.String())

	// Register with signaling server
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	appDebugLog("Registering with signaling server (sessionID: %s)...", sessionID)
	if err := client.RegisterAsServer(secretCode, sessionID); err != nil {
		appDebugLog("ERROR: Failed to register: %v", err)
		client.Close()
		return fmt.Errorf("failed to register: %w", err)
	}

	appDebugLog("SUCCESS: Registered with signaling server!")

	a.mu.Lock()
	a.holePunchClient = client
	a.mu.Unlock()

	return nil
}

// ConnectWithHolePunch connects to a peer using hole punching
func (a *App) ConnectWithHolePunch(secretCode string) error {
	maskedSecret := secretCode
	if len(maskedSecret) > 4 {
		maskedSecret = maskedSecret[:4] + "****"
	}
	appDebugLog("ConnectWithHolePunch called with secret=%s", maskedSecret)

	a.mu.Lock()
	if a.signalingAddr == "" {
		a.mu.Unlock()
		appDebugLog("ERROR: Signaling server not configured")
		return fmt.Errorf("signaling server not configured")
	}
	signalingAddr := a.signalingAddr
	a.mu.Unlock()

	appDebugLog("Using signaling server: %s", signalingAddr)

	// Create hole punch client
	appDebugLog("Creating holepunch client...")
	client, err := holepunch.NewClient(signalingAddr)
	if err != nil {
		appDebugLog("ERROR: Failed to create holepunch client: %v", err)
		return fmt.Errorf("failed to create hole punch client: %w", err)
	}

	// Discover our public address first
	appDebugLog("Discovering public address...")
	publicAddr, err := client.DiscoverPublicAddr()
	if err != nil {
		appDebugLog("ERROR: Failed to discover public address: %v", err)
		client.Close()
		return fmt.Errorf("failed to discover public address: %w", err)
	}
	appDebugLog("Our public address: %s", publicAddr.String())

	// Request peer info from signaling server
	appDebugLog("Requesting peer info for secret=%s from signaling server...", maskedSecret)
	peerAddr, err := client.ConnectToPeer(secretCode)
	if err != nil {
		appDebugLog("ERROR: Failed to get peer info: %v", err)
		appDebugLog("TROUBLESHOOTING:")
		appDebugLog("  1. Is the VPN server running?")
		appDebugLog("  2. Has the server registered with the signaling server?")
		appDebugLog("  3. Are you using the correct secret code?")
		appDebugLog("  4. Is the signaling server reachable from both sides?")
		client.Close()
		return fmt.Errorf("failed to get peer info: %w", err)
	}
	appDebugLog("Found peer at: %s", peerAddr.String())

	// Perform hole punching
	appDebugLog("Starting UDP hole punching to %s (timeout: 20s)...", peerAddr.String())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := client.PunchHole(ctx, peerAddr); err != nil {
		appDebugLog("ERROR: Hole punching failed: %v", err)
		client.Close()
		return fmt.Errorf("hole punching failed: %w", err)
	}
	appDebugLog("SUCCESS: Hole punching completed!")

	// Store the client for the VPN tunnel
	a.mu.Lock()
	a.holePunchClient = client
	a.secretCode = secretCode
	a.serverIP = peerAddr.IP.String()
	a.mu.Unlock()

	appDebugLog("Connection established, serverIP=%s", peerAddr.IP.String())
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

// GetDebugInfo returns detailed debug information about the current state
func (a *App) GetDebugInfo() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	info := map[string]interface{}{
		"mode":               a.mode,
		"serverPort":         a.serverPort,
		"serverIP":           a.serverIP,
		"serverRunning":      a.server != nil,
		"clientConnected":    a.client != nil && a.client.IsConnected(),
		"useHolePunching":    a.useHolePunching,
		"signalingAddr":      a.signalingAddr,
		"signalingRunning":   a.signalingServer != nil,
		"holePunchConnected": a.holePunchClient != nil,
		"tunAdapterActive":   a.tunAdapter != nil,
		"bridgeActive":       a.bridge != nil,
	}

	// Mask secret code
	if len(a.secretCode) > 4 {
		info["secretCode"] = a.secretCode[:4] + "****"
	} else if a.secretCode != "" {
		info["secretCode"] = "****"
	} else {
		info["secretCode"] = "(not set)"
	}

	// Add holepunch details if available
	if a.holePunchClient != nil {
		if pubAddr := a.holePunchClient.GetPublicAddr(); pubAddr != nil {
			info["publicAddr"] = pubAddr.String()
		}
		if peerAddr := a.holePunchClient.GetPeerAddr(); peerAddr != nil {
			info["peerAddr"] = peerAddr.String()
		}
	}

	// Add client details if connected
	if a.client != nil {
		info["clientState"] = a.client.State().String()
		if assignedIP := a.client.AssignedIP(); assignedIP != nil {
			info["assignedIP"] = assignedIP.String()
		}
	}

	return info
}
