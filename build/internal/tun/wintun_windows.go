//go:build windows

package tun

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	wintunDLL     *windows.DLL
	wintunLoaded  bool
	wintunLoadMu  sync.Mutex
	wintunLoadErr error
)

// Wintun function pointers
var (
	wintunCreateAdapter        *windows.Proc
	wintunCloseAdapter         *windows.Proc
	wintunStartSession         *windows.Proc
	wintunEndSession           *windows.Proc
	wintunGetReadWaitEvent     *windows.Proc
	wintunReceivePacket        *windows.Proc
	wintunReleaseReceivePacket *windows.Proc
	wintunAllocateSendPacket   *windows.Proc
	wintunSendPacket           *windows.Proc
)

// WintunAdapter represents a Wintun adapter on Windows
type WintunAdapter struct {
	adapter uintptr
	session uintptr
	name    string
	running bool
	mu      sync.RWMutex
}

// GUID structure for Windows
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// LoadWintun loads the Wintun DLL
func LoadWintun() error {
	wintunLoadMu.Lock()
	defer wintunLoadMu.Unlock()

	if wintunLoaded {
		return wintunLoadErr
	}

	// First, extract the embedded wintun.dll
	dllPath, err := ExtractWintun()
	if err != nil {
		wintunLoadErr = fmt.Errorf("failed to extract wintun.dll: %w", err)
		return wintunLoadErr
	}

	// Load the DLL from the extracted path
	dll, err := windows.LoadDLL(dllPath)
	if err != nil {
		// Fallback: try loading from system path
		dll, err = windows.LoadDLL("wintun.dll")
		if err != nil {
			wintunLoadErr = fmt.Errorf("failed to load wintun.dll: %w", err)
			return wintunLoadErr
		}
	}
	wintunDLL = dll

	// Load function pointers
	wintunCreateAdapter, _ = dll.FindProc("WintunCreateAdapter")
	wintunCloseAdapter, _ = dll.FindProc("WintunCloseAdapter")
	wintunStartSession, _ = dll.FindProc("WintunStartSession")
	wintunEndSession, _ = dll.FindProc("WintunEndSession")
	wintunGetReadWaitEvent, _ = dll.FindProc("WintunGetReadWaitEvent")
	wintunReceivePacket, _ = dll.FindProc("WintunReceivePacket")
	wintunReleaseReceivePacket, _ = dll.FindProc("WintunReleaseReceivePacket")
	wintunAllocateSendPacket, _ = dll.FindProc("WintunAllocateSendPacket")
	wintunSendPacket, _ = dll.FindProc("WintunSendPacket")

	wintunLoaded = true
	return nil
}

// IsWintunAvailable checks if Wintun is available
func IsWintunAvailable() bool {
	err := LoadWintun()
	return err == nil
}

// NewWintunAdapter creates a new Wintun adapter
func NewWintunAdapter(name string) (*WintunAdapter, error) {
	if err := LoadWintun(); err != nil {
		return nil, err
	}

	if wintunCreateAdapter == nil {
		return nil, fmt.Errorf("WintunCreateAdapter not found")
	}

	// Generate a random GUID for the adapter
	guid := GUID{
		Data1: 0x12345678,
		Data2: 0x1234,
		Data3: 0x1234,
		Data4: [8]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0},
	}

	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("failed to convert name: %w", err)
	}

	tunnelType, err := syscall.UTF16PtrFromString("miniVPN")
	if err != nil {
		return nil, fmt.Errorf("failed to convert tunnel type: %w", err)
	}

	adapter, _, callErr := wintunCreateAdapter.Call(
		uintptr(unsafe.Pointer(namePtr)),
		uintptr(unsafe.Pointer(tunnelType)),
		uintptr(unsafe.Pointer(&guid)),
	)

	if adapter == 0 {
		return nil, fmt.Errorf("failed to create adapter: %v", callErr)
	}

	return &WintunAdapter{
		adapter: adapter,
		name:    name,
	}, nil
}

// Start starts a session on the adapter
func (w *WintunAdapter) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("adapter already running")
	}

	if wintunStartSession == nil {
		return fmt.Errorf("WintunStartSession not found")
	}

	// Start session with 8MB ring capacity
	session, _, err := wintunStartSession.Call(w.adapter, 0x800000)
	if session == 0 {
		return fmt.Errorf("failed to start session: %v", err)
	}

	w.session = session
	w.running = true
	return nil
}

// Stop ends the session
func (w *WintunAdapter) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return nil
	}

	if w.session != 0 && wintunEndSession != nil {
		wintunEndSession.Call(w.session)
		w.session = 0
	}

	w.running = false
	return nil
}

// Close closes the adapter
func (w *WintunAdapter) Close() error {
	w.Stop()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.adapter != 0 && wintunCloseAdapter != nil {
		wintunCloseAdapter.Call(w.adapter)
		w.adapter = 0
	}

	return nil
}

// ReceivePacket receives a packet from the adapter
func (w *WintunAdapter) ReceivePacket() ([]byte, error) {
	w.mu.RLock()
	if !w.running || w.session == 0 {
		w.mu.RUnlock()
		return nil, fmt.Errorf("adapter not running")
	}
	session := w.session
	w.mu.RUnlock()

	if wintunReceivePacket == nil {
		return nil, fmt.Errorf("WintunReceivePacket not found")
	}

	var packetSize uint32
	packet, _, _ := wintunReceivePacket.Call(
		session,
		uintptr(unsafe.Pointer(&packetSize)),
	)

	if packet == 0 {
		return nil, nil // No packet available
	}

	// Copy packet data
	data := make([]byte, packetSize)
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(packet)), packetSize))

	// Release the packet
	if wintunReleaseReceivePacket != nil {
		wintunReleaseReceivePacket.Call(session, packet)
	}

	return data, nil
}

// SendPacket sends a packet through the adapter
func (w *WintunAdapter) SendPacket(data []byte) error {
	w.mu.RLock()
	if !w.running || w.session == 0 {
		w.mu.RUnlock()
		return fmt.Errorf("adapter not running")
	}
	session := w.session
	w.mu.RUnlock()

	if wintunAllocateSendPacket == nil || wintunSendPacket == nil {
		return fmt.Errorf("Wintun send functions not found")
	}

	packetSize := uint32(len(data))
	packet, _, _ := wintunAllocateSendPacket.Call(session, uintptr(packetSize))

	if packet == 0 {
		return fmt.Errorf("failed to allocate send packet")
	}

	// Copy data to packet
	copy(unsafe.Slice((*byte)(unsafe.Pointer(packet)), packetSize), data)

	// Send the packet
	wintunSendPacket.Call(session, packet)

	return nil
}

// GetReadWaitEvent returns the read wait event handle
func (w *WintunAdapter) GetReadWaitEvent() (windows.Handle, error) {
	w.mu.RLock()
	if !w.running || w.session == 0 {
		w.mu.RUnlock()
		return 0, fmt.Errorf("adapter not running")
	}
	session := w.session
	w.mu.RUnlock()

	if wintunGetReadWaitEvent == nil {
		return 0, fmt.Errorf("WintunGetReadWaitEvent not found")
	}

	handle, _, _ := wintunGetReadWaitEvent.Call(session)
	return windows.Handle(handle), nil
}

// Name returns the adapter name
func (w *WintunAdapter) Name() string {
	return w.name
}

// IsRunning returns whether the adapter is running
func (w *WintunAdapter) IsRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// ConfigureIP configures the IP address on the adapter using netsh
func (a *Adapter) ConfigureIP(localIP, gateway, mask string) error {
	// Use netsh to configure the IP address
	// netsh interface ip set address "miniVPN" static 10.0.0.2 255.255.255.0 10.0.0.1
	cmd := fmt.Sprintf(`netsh interface ip set address "%s" static %s %s %s`, a.name, localIP, mask, gateway)

	// Execute the command
	if err := runNetshCommand(cmd); err != nil {
		return fmt.Errorf("failed to configure IP: %w", err)
	}

	return nil
}

// runNetshCommand executes a netsh command
func runNetshCommand(cmd string) error {
	// Use syscall to run netsh
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	createProcess := kernel32.NewProc("CreateProcessW")

	cmdLine := "cmd.exe /c " + cmd
	cmdLinePtr, _ := syscall.UTF16PtrFromString(cmdLine)

	var si windows.StartupInfo
	var pi windows.ProcessInformation

	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESHOWWINDOW
	si.ShowWindow = windows.SW_HIDE

	ret, _, err := createProcess.Call(
		0,
		uintptr(unsafe.Pointer(cmdLinePtr)),
		0,
		0,
		0,
		windows.CREATE_NO_WINDOW,
		0,
		0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)

	if ret == 0 {
		return fmt.Errorf("CreateProcess failed: %v", err)
	}

	// Wait for the process to complete
	windows.WaitForSingleObject(pi.Process, windows.INFINITE)

	// Close handles
	windows.CloseHandle(pi.Process)
	windows.CloseHandle(pi.Thread)

	return nil
}

// Windows wait result constants
const (
	waitTimeout = 0x00000102
	waitFailed  = 0xFFFFFFFF
)

// waitForEvent waits for a Windows event with a timeout in milliseconds
func waitForEvent(handle windows.Handle, timeoutMs uint32) error {
	result, err := windows.WaitForSingleObject(handle, timeoutMs)
	if result == waitTimeout {
		return fmt.Errorf("timeout")
	}
	if result == waitFailed {
		return fmt.Errorf("wait failed: %v", err)
	}
	return nil
}
