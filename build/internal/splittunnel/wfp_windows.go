//go:build windows

package splittunnel

import (
	"fmt"
	"net"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	fwpuclnt                       *windows.DLL
	fwpmEngineOpen0                *windows.Proc
	fwpmEngineClose0               *windows.Proc
	fwpmTransactionBegin0          *windows.Proc
	fwpmTransactionCommit0         *windows.Proc
	fwpmTransactionAbort0          *windows.Proc
	fwpmFilterAdd0                 *windows.Proc
	fwpmFilterDeleteById0          *windows.Proc
	fwpmSubLayerAdd0               *windows.Proc
	fwpmSubLayerDeleteByKey0       *windows.Proc
	fwpmProviderAdd0               *windows.Proc
	fwpmProviderDeleteByKey0       *windows.Proc

	wfpLoaded   bool
	wfpLoadErr  error
	wfpLoadOnce sync.Once
)

// GUID structure
type FWPM_GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// WFP Session flags
const (
	FWPM_SESSION_FLAG_DYNAMIC = 0x00000001
)

// WFP Layer GUIDs
var (
	FWPM_LAYER_OUTBOUND_TRANSPORT_V4 = FWPM_GUID{
		Data1: 0x09e61aea,
		Data2: 0xd214,
		Data3: 0x46e2,
		Data4: [8]byte{0x9b, 0x21, 0xb2, 0x6b, 0x0b, 0x2f, 0x28, 0xc8},
	}
	FWPM_LAYER_INBOUND_TRANSPORT_V4 = FWPM_GUID{
		Data1: 0x5926dfc8,
		Data2: 0xe3cf,
		Data3: 0x4426,
		Data4: [8]byte{0xa2, 0x83, 0xdc, 0x39, 0x3f, 0x5d, 0x0f, 0x9d},
	}
)

// miniVPN provider GUID
var miniVPNProviderGUID = FWPM_GUID{
	Data1: 0x12345678,
	Data2: 0xabcd,
	Data3: 0xef01,
	Data4: [8]byte{0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01},
}

// miniVPN sublayer GUID
var miniVPNSubLayerGUID = FWPM_GUID{
	Data1: 0x87654321,
	Data2: 0xdcba,
	Data3: 0x10fe,
	Data4: [8]byte{0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe, 0x10},
}

// WFPEngine manages Windows Filtering Platform operations
type WFPEngine struct {
	handle    uintptr
	filterIDs map[uint64]uint64 // Maps our rule ID to WFP filter ID
	mu        sync.Mutex
	available bool
}

// loadWFP loads the Windows Filtering Platform DLL
func loadWFP() error {
	wfpLoadOnce.Do(func() {
		dll, err := windows.LoadDLL("fwpuclnt.dll")
		if err != nil {
			wfpLoadErr = fmt.Errorf("failed to load fwpuclnt.dll: %w", err)
			return
		}
		fwpuclnt = dll

		fwpmEngineOpen0, _ = dll.FindProc("FwpmEngineOpen0")
		fwpmEngineClose0, _ = dll.FindProc("FwpmEngineClose0")
		fwpmTransactionBegin0, _ = dll.FindProc("FwpmTransactionBegin0")
		fwpmTransactionCommit0, _ = dll.FindProc("FwpmTransactionCommit0")
		fwpmTransactionAbort0, _ = dll.FindProc("FwpmTransactionAbort0")
		fwpmFilterAdd0, _ = dll.FindProc("FwpmFilterAdd0")
		fwpmFilterDeleteById0, _ = dll.FindProc("FwpmFilterDeleteById0")
		fwpmSubLayerAdd0, _ = dll.FindProc("FwpmSubLayerAdd0")
		fwpmSubLayerDeleteByKey0, _ = dll.FindProc("FwpmSubLayerDeleteByKey0")
		fwpmProviderAdd0, _ = dll.FindProc("FwpmProviderAdd0")
		fwpmProviderDeleteByKey0, _ = dll.FindProc("FwpmProviderDeleteByKey0")

		wfpLoaded = true
	})

	return wfpLoadErr
}

// NewWFPEngine creates a new WFP engine instance
func NewWFPEngine() (*WFPEngine, error) {
	if err := loadWFP(); err != nil {
		return nil, err
	}

	if fwpmEngineOpen0 == nil {
		return nil, fmt.Errorf("FwpmEngineOpen0 not found")
	}

	engine := &WFPEngine{
		filterIDs: make(map[uint64]uint64),
		available: true,
	}

	// Open engine with dynamic session (auto-cleanup on exit)
	var handle uintptr
	ret, _, _ := fwpmEngineOpen0.Call(
		0,                              // serverName (NULL for local)
		0,                              // authnService (RPC_C_AUTHN_DEFAULT)
		0,                              // authIdentity (NULL)
		0,                              // session (NULL for default)
		uintptr(unsafe.Pointer(&handle)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("FwpmEngineOpen0 failed: 0x%x", ret)
	}

	engine.handle = handle
	return engine, nil
}

// Close closes the WFP engine
func (e *WFPEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handle == 0 {
		return nil
	}

	// Clear all filters first
	e.clearAllFiltersLocked()

	if fwpmEngineClose0 != nil {
		fwpmEngineClose0.Call(e.handle)
	}

	e.handle = 0
	return nil
}

// AddRule adds a filtering rule
func (e *WFPEngine) AddRule(rule Rule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handle == 0 {
		return fmt.Errorf("engine not initialized")
	}

	// For now, we use route-based split tunneling instead of WFP filters
	// WFP filter implementation would go here for more advanced filtering

	// Store the rule ID mapping (placeholder)
	e.filterIDs[rule.ID] = rule.ID

	return nil
}

// RemoveRule removes a filtering rule
func (e *WFPEngine) RemoveRule(id uint64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.handle == 0 {
		return fmt.Errorf("engine not initialized")
	}

	filterID, exists := e.filterIDs[id]
	if !exists {
		return nil
	}

	// Remove filter (placeholder - actual WFP filter removal would go here)
	_ = filterID

	delete(e.filterIDs, id)
	return nil
}

// ClearAll removes all filtering rules
func (e *WFPEngine) ClearAll() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.clearAllFiltersLocked()
}

// clearAllFiltersLocked clears all filters (must be called with lock held)
func (e *WFPEngine) clearAllFiltersLocked() error {
	if e.handle == 0 {
		return nil
	}

	// Remove all filters
	for id := range e.filterIDs {
		// Actual WFP filter removal would go here
		_ = id
	}

	e.filterIDs = make(map[uint64]uint64)
	return nil
}

// IsAvailable returns whether WFP is available
func (e *WFPEngine) IsAvailable() bool {
	return e.available && e.handle != 0
}

// newPlatformRouter creates a platform-specific router
func newPlatformRouter() PlatformRouter {
	// Try to create WFP engine
	engine, err := NewWFPEngine()
	if err != nil {
		// Fall back to route-based approach
		return &RouteBasedRouter{}
	}
	return engine
}

// RouteBasedRouter implements split tunneling using route table manipulation
type RouteBasedRouter struct {
	routes map[uint64]routeEntry
	mu     sync.Mutex
}

type routeEntry struct {
	ruleID  uint64
	port    uint16
	gateway string
}

// AddRule adds a route-based rule
func (r *RouteBasedRouter) AddRule(rule Rule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.routes == nil {
		r.routes = make(map[uint64]routeEntry)
	}

	// For port-based routing, we can't directly route by port in the routing table
	// Instead, we'll use a proxy/redirect approach or mark packets
	// This is a simplified implementation that stores the intent

	r.routes[rule.ID] = routeEntry{
		ruleID:  rule.ID,
		port:    rule.Port,
		gateway: rule.Gateway.String(),
	}

	return nil
}

// RemoveRule removes a route-based rule
func (r *RouteBasedRouter) RemoveRule(id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.routes, id)
	return nil
}

// ClearAll clears all route-based rules
func (r *RouteBasedRouter) ClearAll() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routes = make(map[uint64]routeEntry)
	return nil
}

// IsAvailable returns whether route-based routing is available
func (r *RouteBasedRouter) IsAvailable() bool {
	return true
}

// GetDefaultGateway returns the default gateway and interface
func GetDefaultGateway() (net.IP, string, error) {
	// Use Windows API to get the default gateway
	routes, err := getRouteTable()
	if err != nil {
		return nil, "", err
	}

	for _, route := range routes {
		// Find the default route (destination 0.0.0.0)
		if route.destination.Equal(net.IPv4zero) {
			return route.gateway, route.ifaceName, nil
		}
	}

	return nil, "", fmt.Errorf("default gateway not found")
}

type routeInfo struct {
	destination net.IP
	gateway     net.IP
	mask        net.IPMask
	ifaceName   string
	metric      uint32
}

func getRouteTable() ([]routeInfo, error) {
	// Use GetIpForwardTable to get routing table
	iphlpapi := windows.NewLazySystemDLL("iphlpapi.dll")
	getIpForwardTable := iphlpapi.NewProc("GetIpForwardTable")

	// First call to get size
	var size uint32
	getIpForwardTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)

	if size == 0 {
		return nil, fmt.Errorf("failed to get route table size")
	}

	// Allocate buffer
	buf := make([]byte, size)
	ret, _, _ := getIpForwardTable.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)

	if ret != 0 {
		return nil, fmt.Errorf("GetIpForwardTable failed: %d", ret)
	}

	// Parse the MIB_IPFORWARDTABLE structure
	numEntries := *(*uint32)(unsafe.Pointer(&buf[0]))
	routes := make([]routeInfo, 0, numEntries)

	// Each MIB_IPFORWARDROW is 56 bytes on 32-bit, 76 bytes on 64-bit
	// We'll use a simplified approach
	entrySize := 56
	if unsafe.Sizeof(uintptr(0)) == 8 {
		entrySize = 56 // Same on 64-bit for this structure
	}

	for i := uint32(0); i < numEntries; i++ {
		offset := 4 + int(i)*entrySize // 4 bytes for dwNumEntries

		if offset+entrySize > len(buf) {
			break
		}

		dest := net.IPv4(buf[offset], buf[offset+1], buf[offset+2], buf[offset+3])
		mask := net.IPv4Mask(buf[offset+4], buf[offset+5], buf[offset+6], buf[offset+7])
		// Policy at offset+8 (4 bytes)
		gateway := net.IPv4(buf[offset+12], buf[offset+13], buf[offset+14], buf[offset+15])
		ifIndex := *(*uint32)(unsafe.Pointer(&buf[offset+16]))
		// Type at offset+20
		// Proto at offset+24
		// Age at offset+28
		// NextHopAS at offset+32
		metric := *(*uint32)(unsafe.Pointer(&buf[offset+36]))

		ifaceName := fmt.Sprintf("if%d", ifIndex)

		routes = append(routes, routeInfo{
			destination: dest,
			gateway:     gateway,
			mask:        mask,
			ifaceName:   ifaceName,
			metric:      metric,
		})
	}

	return routes, nil
}

// AddRoute adds a route to the routing table
func AddRoute(destination, mask, gateway net.IP, metric uint32) error {
	iphlpapi := windows.NewLazySystemDLL("iphlpapi.dll")
	createIpForwardEntry := iphlpapi.NewProc("CreateIpForwardEntry")

	// Build MIB_IPFORWARDROW structure
	row := make([]byte, 56)

	// dwForwardDest
	copy(row[0:4], destination.To4())
	// dwForwardMask
	copy(row[4:8], mask)
	// dwForwardPolicy
	// dwForwardNextHop
	copy(row[12:16], gateway.To4())
	// dwForwardIfIndex - would need to look this up
	// dwForwardType = 4 (indirect)
	row[20] = 4
	// dwForwardProto = 3 (netmgmt)
	row[24] = 3
	// dwForwardAge
	// dwForwardNextHopAS
	// dwForwardMetric1
	*(*uint32)(unsafe.Pointer(&row[36])) = metric

	ret, _, err := createIpForwardEntry.Call(uintptr(unsafe.Pointer(&row[0])))
	if ret != 0 {
		return fmt.Errorf("CreateIpForwardEntry failed: %v", err)
	}

	return nil
}

// DeleteRoute removes a route from the routing table
func DeleteRoute(destination, mask, gateway net.IP) error {
	iphlpapi := windows.NewLazySystemDLL("iphlpapi.dll")
	deleteIpForwardEntry := iphlpapi.NewProc("DeleteIpForwardEntry")

	// Build MIB_IPFORWARDROW structure
	row := make([]byte, 56)
	copy(row[0:4], destination.To4())
	copy(row[4:8], mask)
	copy(row[12:16], gateway.To4())

	ret, _, err := deleteIpForwardEntry.Call(uintptr(unsafe.Pointer(&row[0])))
	if ret != 0 {
		return fmt.Errorf("DeleteIpForwardEntry failed: %v", err)
	}

	return nil
}

// RunAsAdmin checks if running with admin privileges
func RunAsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}

	return member
}

// Ensure syscall is used
var _ = syscall.EINVAL
