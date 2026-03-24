package splittunnel

import (
	"fmt"
	"net"
	"sync"
)

// Mode defines how split tunneling operates
type Mode string

const (
	// ModeInclude routes only specified ports through VPN
	ModeInclude Mode = "include"
	// ModeExclude routes everything except specified ports through VPN
	ModeExclude Mode = "exclude"
)

// Config holds split tunnel configuration
type Config struct {
	Enabled bool
	Mode    Mode
	Ports   []uint16
}

// Manager handles split tunneling operations
type Manager struct {
	config      Config
	vpnGateway  net.IP
	vpnIface    string
	origGateway net.IP
	origIface   string
	active      bool
	mu          sync.RWMutex

	// Platform-specific components
	router *Router
}

// NewManager creates a new split tunnel manager
func NewManager() *Manager {
	return &Manager{
		config: Config{
			Mode:  ModeInclude,
			Ports: []uint16{},
		},
		router: NewRouter(),
	}
}

// Configure sets the split tunnel configuration
func (m *Manager) Configure(config Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate ports
	for _, port := range config.Ports {
		if port == 0 {
			return fmt.Errorf("invalid port: 0")
		}
	}

	m.config = config
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetVPNInterface sets the VPN interface details
func (m *Manager) SetVPNInterface(gateway net.IP, ifaceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vpnGateway = gateway
	m.vpnIface = ifaceName
}

// Start activates split tunneling
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return fmt.Errorf("split tunneling already active")
	}

	if !m.config.Enabled {
		return nil
	}

	// Get original default gateway
	gateway, iface, err := GetDefaultGateway()
	if err != nil {
		return fmt.Errorf("failed to get default gateway: %w", err)
	}
	m.origGateway = gateway
	m.origIface = iface

	// Apply routing rules based on mode
	if err := m.applyRules(); err != nil {
		return fmt.Errorf("failed to apply rules: %w", err)
	}

	m.active = true
	return nil
}

// Stop deactivates split tunneling
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return nil
	}

	// Remove all rules
	if err := m.removeRules(); err != nil {
		return fmt.Errorf("failed to remove rules: %w", err)
	}

	m.active = false
	return nil
}

// applyRules applies the routing rules based on configuration
func (m *Manager) applyRules() error {
	rules := make([]Rule, 0, len(m.config.Ports))

	for _, port := range m.config.Ports {
		var action RuleAction
		var gateway net.IP
		var iface string

		if m.config.Mode == ModeInclude {
			// Include mode: specified ports go through VPN
			action = ActionRoute
			gateway = m.vpnGateway
			iface = m.vpnIface
		} else {
			// Exclude mode: specified ports bypass VPN
			action = ActionBypass
			gateway = m.origGateway
			iface = m.origIface
		}

		rules = append(rules, Rule{
			Port:      port,
			Protocol:  ProtocolBoth,
			Direction: DirectionOutbound,
			Action:    action,
			Gateway:   gateway,
			Interface: iface,
		})
	}

	return m.router.ApplyRules(rules)
}

// removeRules removes all routing rules
func (m *Manager) removeRules() error {
	return m.router.ClearRules()
}

// IsActive returns whether split tunneling is active
func (m *Manager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// IsEnabled returns whether split tunneling is enabled (for SplitTunnelChecker interface)
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// GetMode returns the current mode as string (for SplitTunnelChecker interface)
func (m *Manager) GetMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return string(m.config.Mode)
}

// ShouldTunnel determines if traffic to a port should go through VPN
func (m *Manager) ShouldTunnel(port uint16) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return true // If split tunnel disabled, tunnel everything
	}

	portInList := false
	for _, p := range m.config.Ports {
		if p == port {
			portInList = true
			break
		}
	}

	if m.config.Mode == ModeInclude {
		return portInList // Only tunnel if port is in list
	}
	return !portInList // Tunnel if port is NOT in list
}

// AddPort adds a port to the configuration
func (m *Manager) AddPort(port uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if port == 0 {
		return fmt.Errorf("invalid port: 0")
	}

	// Check if port already exists
	for _, p := range m.config.Ports {
		if p == port {
			return nil // Already exists
		}
	}

	m.config.Ports = append(m.config.Ports, port)

	// If active, apply new rule
	if m.active {
		return m.applyRules()
	}

	return nil
}

// RemovePort removes a port from the configuration
func (m *Manager) RemovePort(port uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newPorts := make([]uint16, 0, len(m.config.Ports))
	for _, p := range m.config.Ports {
		if p != port {
			newPorts = append(newPorts, p)
		}
	}
	m.config.Ports = newPorts

	// If active, reapply rules
	if m.active {
		return m.applyRules()
	}

	return nil
}

// SetMode sets the split tunnel mode
func (m *Manager) SetMode(mode Mode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mode != ModeInclude && mode != ModeExclude {
		return fmt.Errorf("invalid mode: %s", mode)
	}

	m.config.Mode = mode

	// If active, reapply rules
	if m.active {
		return m.applyRules()
	}

	return nil
}

// Enable enables split tunneling
func (m *Manager) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = true
}

// Disable disables split tunneling
func (m *Manager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Enabled = false
}
