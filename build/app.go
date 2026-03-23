package main

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net"
	"strings"
	"sync"
)

// App struct holds the application state
type App struct {
	ctx        context.Context
	mode       string // "server" or "client"
	secretCode string
	serverIP   string
	connected  bool
	mu         sync.RWMutex

	// Split tunnel configuration
	tunneledPorts []int
	tunnelMode    string // "include" or "exclude"

	// Server state
	listener net.Listener
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		tunneledPorts: []int{},
		tunnelMode:    "include",
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

	if a.listener != nil {
		a.listener.Close()
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
		a.secretCode = generateSecretCode()
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
	a.secretCode = generateSecretCode()
	return a.secretCode
}

// ConnectToServer attempts to connect to a VPN server (client mode)
func (a *App) ConnectToServer(serverIP string, secretCode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.mode != "client" {
		return fmt.Errorf("not in client mode")
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
	a.secretCode = secretCode

	// TODO: Implement actual VPN connection using WireGuard
	// For now, this is a placeholder
	a.connected = true

	return nil
}

// Disconnect disconnects from the VPN
func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// TODO: Implement actual VPN disconnection
	a.connected = false
	return nil
}

// IsConnected returns the connection status
func (a *App) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.connected
}

// StartServer starts the VPN server (server mode only)
func (a *App) StartServer(port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.mode != "server" {
		return fmt.Errorf("not in server mode")
	}

	// TODO: Implement actual VPN server using WireGuard
	// For now, this is a placeholder that creates a TCP listener

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}

	a.listener = listener
	a.connected = true

	return nil
}

// StopServer stops the VPN server
func (a *App) StopServer() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.listener != nil {
		a.listener.Close()
		a.listener = nil
	}
	a.connected = false

	return nil
}

// SetTunneledPorts sets the ports to be tunneled through VPN
func (a *App) SetTunneledPorts(ports []int, mode string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if mode != "include" && mode != "exclude" {
		return fmt.Errorf("invalid tunnel mode: %s", mode)
	}

	// Validate ports
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("invalid port: %d", port)
		}
	}

	a.tunneledPorts = ports
	a.tunnelMode = mode

	// TODO: Apply split tunnel rules via WFP

	return nil
}

// GetTunneledPorts returns the current tunneled ports configuration
func (a *App) GetTunneledPorts() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"ports": a.tunneledPorts,
		"mode":  a.tunnelMode,
	}
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

// generateSecretCode creates a random 20-character base32 secret code
func generateSecretCode() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	code := base32.StdEncoding.EncodeToString(bytes)
	// Format as XXXX-XXXX-XXXX-XXXX-XXXX for readability
	code = strings.ToUpper(code[:20])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		code[0:4], code[4:8], code[8:12], code[12:16], code[16:20])
}
