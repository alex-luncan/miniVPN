//go:build !windows

package tun

import (
	"fmt"
)

// WintunAdapter represents a Wintun adapter (stub for non-Windows)
type WintunAdapter struct {
	name    string
	running bool
}

// LoadWintun loads the Wintun DLL (stub for non-Windows)
func LoadWintun() error {
	return fmt.Errorf("Wintun is only available on Windows")
}

// IsWintunAvailable checks if Wintun is available (stub for non-Windows)
func IsWintunAvailable() bool {
	return false
}

// NewWintunAdapter creates a new Wintun adapter (stub for non-Windows)
func NewWintunAdapter(name string) (*WintunAdapter, error) {
	return nil, fmt.Errorf("Wintun is only available on Windows")
}

// Start starts a session on the adapter (stub for non-Windows)
func (w *WintunAdapter) Start() error {
	return fmt.Errorf("Wintun is only available on Windows")
}

// Stop ends the session (stub for non-Windows)
func (w *WintunAdapter) Stop() error {
	return nil
}

// Close closes the adapter (stub for non-Windows)
func (w *WintunAdapter) Close() error {
	return nil
}

// ReceivePacket receives a packet from the adapter (stub for non-Windows)
func (w *WintunAdapter) ReceivePacket() ([]byte, error) {
	return nil, fmt.Errorf("Wintun is only available on Windows")
}

// SendPacket sends a packet through the adapter (stub for non-Windows)
func (w *WintunAdapter) SendPacket(data []byte) error {
	return fmt.Errorf("Wintun is only available on Windows")
}

// GetReadWaitEvent returns the read wait event handle (stub for non-Windows)
func (w *WintunAdapter) GetReadWaitEvent() (uintptr, error) {
	return 0, fmt.Errorf("Wintun is only available on Windows")
}

// Name returns the adapter name
func (w *WintunAdapter) Name() string {
	return w.name
}

// IsRunning returns whether the adapter is running
func (w *WintunAdapter) IsRunning() bool {
	return w.running
}

// ConfigureIP configures the IP address (stub for non-Windows)
func (a *Adapter) ConfigureIP(localIP, gateway, mask string) error {
	return fmt.Errorf("not implemented on this platform")
}

// waitForEvent waits for an event (stub for non-Windows)
func waitForEvent(handle uintptr, timeoutMs uint32) error {
	return fmt.Errorf("not implemented on this platform")
}
