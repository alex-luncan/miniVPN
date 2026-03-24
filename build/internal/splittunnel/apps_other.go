//go:build !windows

package splittunnel

import "sync"

// RunningApp represents a running application
type RunningApp struct {
	PID     uint32 `json:"pid"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	ExeName string `json:"exeName"`
}

// GetRunningApps returns running applications (stub for non-Windows)
func GetRunningApps() ([]RunningApp, error) {
	return []RunningApp{}, nil
}

// SplitTunnelApp represents an app configured for split tunneling
type SplitTunnelApp struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	ExeName string `json:"exeName"`
}

// AppFilterMode defines how app filtering operates
type AppFilterMode string

const (
	AppModeInclude AppFilterMode = "include"
	AppModeExclude AppFilterMode = "exclude"
)

// AppFilterManager manages application-based split tunneling (stub)
type AppFilterManager struct {
	mu      sync.RWMutex
	apps    []SplitTunnelApp
	mode    AppFilterMode
	enabled bool
}

var (
	appFilterManager     *AppFilterManager
	appFilterManagerOnce sync.Once
)

// GetAppFilterManager returns the singleton app filter manager
func GetAppFilterManager() *AppFilterManager {
	appFilterManagerOnce.Do(func() {
		appFilterManager = &AppFilterManager{
			apps: make([]SplitTunnelApp, 0),
			mode: AppModeInclude,
		}
	})
	return appFilterManager
}

// SetApps sets the list of apps to filter
func (m *AppFilterManager) SetApps(apps []SplitTunnelApp, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apps = apps
	if mode == "exclude" {
		m.mode = AppModeExclude
	} else {
		m.mode = AppModeInclude
	}
	return nil
}

// GetApps returns the current app configuration
func (m *AppFilterManager) GetApps() ([]SplitTunnelApp, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.apps, string(m.mode)
}

// AddApp adds an app to the filter list
func (m *AppFilterManager) AddApp(path, name, exeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.apps = append(m.apps, SplitTunnelApp{Path: path, Name: name, ExeName: exeName})
	return nil
}

// RemoveApp removes an app from the filter list
func (m *AppFilterManager) RemoveApp(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return nil
}

// SetMode sets the filter mode
func (m *AppFilterManager) SetMode(mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mode == "exclude" {
		m.mode = AppModeExclude
	} else {
		m.mode = AppModeInclude
	}
	return nil
}

// GetMode returns the current mode
func (m *AppFilterManager) GetMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return string(m.mode)
}

// Enable enables app filtering
func (m *AppFilterManager) Enable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = true
}

// Disable disables app filtering
func (m *AppFilterManager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
}

// IsEnabled returns whether app filtering is enabled
func (m *AppFilterManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// ShouldAppUseTunnel determines if an app should use the VPN tunnel
func (m *AppFilterManager) ShouldAppUseTunnel(path string) bool {
	return true
}

// Start initializes filtering (stub)
func (m *AppFilterManager) Start(vpnIfIndex int) error {
	return nil
}

// Stop cleans up filtering (stub)
func (m *AppFilterManager) Stop() error {
	return nil
}

// GetRoutingRecommendation returns routing advice based on app config
func (m *AppFilterManager) GetRoutingRecommendation() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.enabled || len(m.apps) == 0 {
		return "full"
	}
	if m.mode == AppModeInclude {
		return "split"
	}
	return "full"
}

// AppFilter manages application filtering (stub for non-Windows)
type AppFilter struct{}

// NewAppFilter creates a new app filter (stub)
func NewAppFilter() (*AppFilter, error) {
	return &AppFilter{}, nil
}

// Close closes the app filter (stub)
func (af *AppFilter) Close() error {
	return nil
}
