//go:build windows

package splittunnel

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Debug logging
var DebugApps = true

func appsDebugLog(format string, args ...interface{}) {
	if DebugApps {
		log.Printf("[APPS] "+format, args...)
	}
}

// RunningApp represents a running application
type RunningApp struct {
	PID      uint32 `json:"pid"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	ExeName  string `json:"exeName"`
}

// GetRunningApps returns a list of running applications with network activity potential
func GetRunningApps() ([]RunningApp, error) {
	apps := make(map[string]RunningApp) // Use map to deduplicate by path

	// Get snapshot of all processes
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	// Get first process
	if err := windows.Process32First(snapshot, &pe32); err != nil {
		return nil, fmt.Errorf("Process32First failed: %w", err)
	}

	for {
		exeName := windows.UTF16ToString(pe32.ExeFile[:])

		// Skip system processes
		if !isSystemProcess(exeName) {
			// Try to get full path
			path := getProcessPath(pe32.ProcessID)
			if path != "" {
				app := RunningApp{
					PID:     pe32.ProcessID,
					Name:    getAppDisplayName(exeName, path),
					Path:    path,
					ExeName: exeName,
				}
				// Deduplicate by path (lowercase for comparison)
				key := strings.ToLower(path)
				if _, exists := apps[key]; !exists {
					apps[key] = app
				}
			}
		}

		// Get next process
		if err := windows.Process32Next(snapshot, &pe32); err != nil {
			break
		}
	}

	// Convert map to slice
	result := make([]RunningApp, 0, len(apps))
	for _, app := range apps {
		result = append(result, app)
	}

	return result, nil
}

// getProcessPath gets the full path of a process
func getProcessPath(pid uint32) string {
	// Open process with query rights
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	// Get the executable path
	var buf [windows.MAX_PATH]uint16
	size := uint32(len(buf))

	err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		return ""
	}

	return windows.UTF16ToString(buf[:size])
}

// isSystemProcess checks if a process is a system process that shouldn't be listed
func isSystemProcess(name string) bool {
	name = strings.ToLower(name)
	systemProcesses := []string{
		"system",
		"registry",
		"smss.exe",
		"csrss.exe",
		"wininit.exe",
		"services.exe",
		"lsass.exe",
		"svchost.exe",
		"fontdrvhost.exe",
		"dwm.exe",
		"ctfmon.exe",
		"conhost.exe",
		"sihost.exe",
		"taskhostw.exe",
		"explorer.exe", // Usually want to keep this but not in VPN split
		"runtimebroker.exe",
		"applicationframehost.exe",
		"systemsettings.exe",
		"searchhost.exe",
		"startmenuexperiencehost.exe",
		"shellexperiencehost.exe",
		"textinputhost.exe",
		"dllhost.exe",
		"backgroundtaskhost.exe",
		"securityhealthservice.exe",
		"securityhealthsystray.exe",
		"sgrmbroker.exe",
		"spoolsv.exe",
		"wudfhost.exe",
		"audiodg.exe",
		"searchindexer.exe",
		"msiexec.exe",
		"trustedinstaller.exe",
		"tiworker.exe",
		"wmiprvse.exe",
		"dashost.exe",
		"smartscreen.exe",
		"microsoftedgeupdate.exe",
		"gamebarpresencewriter.exe",
		"gamebarftserver.exe",
		"windowsterminal.exe",
		"openssh.exe",
		"ssh.exe",
		"wsl.exe",
		"wslhost.exe",
		"msedgewebview2.exe",
		"webview2.exe",
		"minivpn.exe", // Don't show ourselves
	}

	for _, sys := range systemProcesses {
		if name == sys {
			return true
		}
	}

	// Skip Microsoft system apps
	if strings.HasPrefix(name, "microsoft") || strings.HasPrefix(name, "windows") {
		return true
	}

	return false
}

// getAppDisplayName creates a user-friendly display name for an app
func getAppDisplayName(exeName, path string) string {
	// Try to get a nice name from common apps
	lower := strings.ToLower(exeName)

	knownApps := map[string]string{
		"chrome.exe":           "Google Chrome",
		"firefox.exe":          "Mozilla Firefox",
		"msedge.exe":           "Microsoft Edge",
		"opera.exe":            "Opera",
		"brave.exe":            "Brave Browser",
		"vivaldi.exe":          "Vivaldi",
		"iexplore.exe":         "Internet Explorer",
		"discord.exe":          "Discord",
		"slack.exe":            "Slack",
		"teams.exe":            "Microsoft Teams",
		"zoom.exe":             "Zoom",
		"skype.exe":            "Skype",
		"telegram.exe":         "Telegram",
		"whatsapp.exe":         "WhatsApp",
		"signal.exe":           "Signal",
		"spotify.exe":          "Spotify",
		"steam.exe":            "Steam",
		"epicgameslauncher.exe": "Epic Games",
		"origin.exe":           "EA Origin",
		"battle.net.exe":       "Battle.net",
		"upc.exe":              "Ubisoft Connect",
		"gog galaxy.exe":       "GOG Galaxy",
		"code.exe":             "VS Code",
		"devenv.exe":           "Visual Studio",
		"idea64.exe":           "IntelliJ IDEA",
		"pycharm64.exe":        "PyCharm",
		"webstorm64.exe":       "WebStorm",
		"rider64.exe":          "JetBrains Rider",
		"notepad++.exe":        "Notepad++",
		"sublime_text.exe":     "Sublime Text",
		"atom.exe":             "Atom",
		"postman.exe":          "Postman",
		"insomnia.exe":         "Insomnia",
		"filezilla.exe":        "FileZilla",
		"winscp.exe":           "WinSCP",
		"putty.exe":            "PuTTY",
		"git.exe":              "Git",
		"node.exe":             "Node.js",
		"python.exe":           "Python",
		"pythonw.exe":          "Python",
		"java.exe":             "Java",
		"javaw.exe":            "Java",
		"powershell.exe":       "PowerShell",
		"cmd.exe":              "Command Prompt",
		"thunderbird.exe":      "Thunderbird",
		"outlook.exe":          "Outlook",
		"winword.exe":          "Microsoft Word",
		"excel.exe":            "Microsoft Excel",
		"powerpnt.exe":         "PowerPoint",
		"onenote.exe":          "OneNote",
		"onedrive.exe":         "OneDrive",
		"dropbox.exe":          "Dropbox",
		"googledrive.exe":      "Google Drive",
		"qbittorrent.exe":      "qBittorrent",
		"utorrent.exe":         "uTorrent",
		"deluge.exe":           "Deluge",
		"transmission-qt.exe":  "Transmission",
		"vlc.exe":              "VLC Media Player",
		"mpc-hc64.exe":         "MPC-HC",
		"obs64.exe":            "OBS Studio",
		"obs32.exe":            "OBS Studio",
		"audacity.exe":         "Audacity",
		"gimp-2.10.exe":        "GIMP",
		"photoshop.exe":        "Photoshop",
		"illustrator.exe":      "Illustrator",
		"premiere.exe":         "Premiere Pro",
		"afterfx.exe":          "After Effects",
		"blender.exe":          "Blender",
		"unity.exe":            "Unity",
		"unrealengine.exe":     "Unreal Engine",
	}

	if displayName, ok := knownApps[lower]; ok {
		return displayName
	}

	// Otherwise, use the exe name without extension, capitalized
	name := strings.TrimSuffix(exeName, filepath.Ext(exeName))
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return exeName
}

// AppFilterRule represents a rule to filter traffic by application
type AppFilterRule struct {
	AppPath string
	Action  string // "tunnel" or "bypass"
}

// WFP Application Filtering using ALE layers
var (
	modFwpuclnt = windows.NewLazySystemDLL("fwpuclnt.dll")

	procFwpmEngineOpen0           = modFwpuclnt.NewProc("FwpmEngineOpen0")
	procFwpmEngineClose0          = modFwpuclnt.NewProc("FwpmEngineClose0")
	procFwpmFilterAdd0            = modFwpuclnt.NewProc("FwpmFilterAdd0")
	procFwpmFilterDeleteById0     = modFwpuclnt.NewProc("FwpmFilterDeleteById0")
	procFwpmSubLayerAdd0          = modFwpuclnt.NewProc("FwpmSubLayerAdd0")
	procFwpmSubLayerDeleteByKey0  = modFwpuclnt.NewProc("FwpmSubLayerDeleteByKey0")
	procFwpmTransactionBegin0     = modFwpuclnt.NewProc("FwpmTransactionBegin0")
	procFwpmTransactionCommit0    = modFwpuclnt.NewProc("FwpmTransactionCommit0")
	procFwpmTransactionAbort0     = modFwpuclnt.NewProc("FwpmTransactionAbort0")
)

// GUID for our WFP sublayer
var miniVPNSublayerGUID = windows.GUID{
	Data1: 0x12345678,
	Data2: 0xABCD,
	Data3: 0xEF01,
	Data4: [8]byte{0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF, 0x01},
}

// WFP layer GUIDs
var (
	// ALE Auth Connect Layer - intercepts outbound connection attempts
	FWPM_LAYER_ALE_AUTH_CONNECT_V4_GUID = windows.GUID{
		Data1: 0xc38d57d1,
		Data2: 0x05a7,
		Data3: 0x4c33,
		Data4: [8]byte{0x90, 0x4f, 0x7f, 0xbc, 0xee, 0xe6, 0x0e, 0x82},
	}

	// Condition for matching application path
	FWPM_CONDITION_ALE_APP_ID_GUID = windows.GUID{
		Data1: 0xd78e1e87,
		Data2: 0x8644,
		Data3: 0x4ea5,
		Data4: [8]byte{0x94, 0x37, 0xd8, 0x09, 0xec, 0xef, 0xc9, 0x71},
	}
)

// SplitTunnelApp represents an app configured for split tunneling
type SplitTunnelApp struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	ExeName string `json:"exeName"`
}

// AppFilterMode defines how app filtering operates
type AppFilterMode string

const (
	// AppModeInclude routes only selected apps through VPN
	AppModeInclude AppFilterMode = "include"
	// AppModeExclude routes everything except selected apps through VPN
	AppModeExclude AppFilterMode = "exclude"
)

// AppFilterManager manages application-based split tunneling
type AppFilterManager struct {
	mu            sync.RWMutex
	apps          []SplitTunnelApp
	mode          AppFilterMode
	enabled       bool
	wfpEngine     uintptr
	filterIDs     []uint64
	sublayerAdded bool
	vpnIfIndex    int
	vpnIfLuid     uint64
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

	appsDebugLog("App filter configured: mode=%s, apps=%d", m.mode, len(m.apps))
	for _, app := range m.apps {
		appsDebugLog("  - %s (%s)", app.Name, app.ExeName)
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

	// Check if already exists
	pathLower := strings.ToLower(path)
	for _, app := range m.apps {
		if strings.ToLower(app.Path) == pathLower {
			return nil // Already exists
		}
	}

	m.apps = append(m.apps, SplitTunnelApp{
		Path:    path,
		Name:    name,
		ExeName: exeName,
	})

	appsDebugLog("Added app to filter: %s (%s)", name, exeName)
	return nil
}

// RemoveApp removes an app from the filter list
func (m *AppFilterManager) RemoveApp(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pathLower := strings.ToLower(path)
	newApps := make([]SplitTunnelApp, 0, len(m.apps))
	for _, app := range m.apps {
		if strings.ToLower(app.Path) != pathLower {
			newApps = append(newApps, app)
		}
	}
	m.apps = newApps

	appsDebugLog("Removed app from filter: %s", path)
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

	appsDebugLog("App filter mode set to: %s", m.mode)
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
	appsDebugLog("App filtering enabled")
}

// Disable disables app filtering
func (m *AppFilterManager) Disable() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = false
	appsDebugLog("App filtering disabled")
}

// IsEnabled returns whether app filtering is enabled
func (m *AppFilterManager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// IsAppInList checks if an app path is in the filter list
func (m *AppFilterManager) IsAppInList(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pathLower := strings.ToLower(path)
	for _, app := range m.apps {
		if strings.ToLower(app.Path) == pathLower {
			return true
		}
	}
	return false
}

// ShouldAppUseTunnel determines if an app should use the VPN tunnel
func (m *AppFilterManager) ShouldAppUseTunnel(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled || len(m.apps) == 0 {
		return true // If not enabled or no apps configured, tunnel all
	}

	inList := false
	pathLower := strings.ToLower(path)
	for _, app := range m.apps {
		if strings.ToLower(app.Path) == pathLower {
			inList = true
			break
		}
	}

	if m.mode == AppModeInclude {
		return inList // Only tunnel if in list
	}
	return !inList // Tunnel if NOT in list (exclude mode)
}

// Start initializes WFP filtering (if needed)
func (m *AppFilterManager) Start(vpnIfIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		appsDebugLog("App filtering not enabled, skipping WFP setup")
		return nil
	}

	m.vpnIfIndex = vpnIfIndex

	// Open WFP engine
	var engineHandle uintptr
	ret, _, _ := procFwpmEngineOpen0.Call(
		0, // serverName
		0, // authnService
		0, // authIdentity
		0, // session
		uintptr(unsafe.Pointer(&engineHandle)),
	)

	if ret != 0 {
		appsDebugLog("Failed to open WFP engine: 0x%x (this is OK, app filtering will use routing only)", ret)
		return nil // Non-fatal, we'll use routing-based filtering
	}

	m.wfpEngine = engineHandle
	appsDebugLog("WFP engine opened for app filtering")

	// Add our sublayer
	if err := m.addSublayer(); err != nil {
		appsDebugLog("Failed to add sublayer: %v", err)
	}

	// Apply filters for configured apps
	if err := m.applyFilters(); err != nil {
		appsDebugLog("Failed to apply filters: %v", err)
	}

	return nil
}

// Stop cleans up WFP filtering
func (m *AppFilterManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.wfpEngine == 0 {
		return nil
	}

	// Remove all filters
	for _, filterID := range m.filterIDs {
		procFwpmFilterDeleteById0.Call(m.wfpEngine, uintptr(filterID), 0)
	}
	m.filterIDs = nil

	// Remove sublayer
	if m.sublayerAdded {
		procFwpmSubLayerDeleteByKey0.Call(
			m.wfpEngine,
			uintptr(unsafe.Pointer(&miniVPNSublayerGUID)),
		)
		m.sublayerAdded = false
	}

	// Close engine
	procFwpmEngineClose0.Call(m.wfpEngine)
	m.wfpEngine = 0

	appsDebugLog("WFP engine closed")
	return nil
}

// FWPM_SUBLAYER0 structure
type fwpmSublayer0 struct {
	subLayerKey   windows.GUID
	displayData   fwpmDisplayData0
	flags         uint32
	providerKey   *windows.GUID
	providerData  fwpByteBlob
	weight        uint16
}

type fwpmDisplayData0 struct {
	name        *uint16
	description *uint16
}

type fwpByteBlob struct {
	size uint32
	data *byte
}

func (m *AppFilterManager) addSublayer() error {
	if m.wfpEngine == 0 {
		return fmt.Errorf("WFP engine not open")
	}

	name, _ := syscall.UTF16PtrFromString("miniVPN App Filter")
	desc, _ := syscall.UTF16PtrFromString("Split tunnel application filtering")

	sublayer := fwpmSublayer0{
		subLayerKey: miniVPNSublayerGUID,
		displayData: fwpmDisplayData0{
			name:        name,
			description: desc,
		},
		weight: 0x100, // Higher weight = processed first
	}

	ret, _, _ := procFwpmSubLayerAdd0.Call(
		m.wfpEngine,
		uintptr(unsafe.Pointer(&sublayer)),
		0, // No security descriptor
	)

	// 0x80320009 = FWP_E_ALREADY_EXISTS - this is OK
	if ret != 0 && ret != 0x80320009 {
		return fmt.Errorf("FwpmSubLayerAdd0 failed: 0x%x", ret)
	}

	m.sublayerAdded = true
	appsDebugLog("WFP sublayer added/exists")
	return nil
}

func (m *AppFilterManager) applyFilters() error {
	if m.wfpEngine == 0 || len(m.apps) == 0 {
		return nil
	}

	appsDebugLog("Applying WFP filters for %d apps in %s mode", len(m.apps), m.mode)

	// For exclude mode: we would block selected apps from VPN interface
	// For include mode: we would block non-selected apps from VPN interface
	// However, WFP blocking causes drops, not rerouting
	// So we log the configuration but rely primarily on routing

	for _, app := range m.apps {
		appsDebugLog("  Configured: %s (%s) - %s mode", app.Name, app.ExeName, m.mode)
	}

	// Note: Full WFP filter implementation would go here
	// For now, we rely on routing-based split tunneling
	// True per-app requires WFP callouts (kernel driver)

	return nil
}

// GetRoutingRecommendation returns routing advice based on app config
func (m *AppFilterManager) GetRoutingRecommendation() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled || len(m.apps) == 0 {
		return "full" // Route all through VPN
	}

	if m.mode == AppModeInclude {
		return "split" // Only route VPN subnet
	}

	return "full" // Exclude mode: route all, apps list is informational
}

// AppFilter manages WFP-based application filtering (legacy interface)
type AppFilter struct {
	engineHandle  uintptr
	filterIDs     []uint64
	sublayerAdded bool
}

// NewAppFilter creates a new application filter
func NewAppFilter() (*AppFilter, error) {
	af := &AppFilter{
		filterIDs: make([]uint64, 0),
	}

	// Open WFP engine
	var engineHandle uintptr
	ret, _, _ := procFwpmEngineOpen0.Call(
		0, // serverName
		0, // authnService
		0, // authIdentity
		0, // session
		uintptr(unsafe.Pointer(&engineHandle)),
	)

	if ret != 0 {
		return nil, fmt.Errorf("FwpmEngineOpen0 failed: 0x%x", ret)
	}

	af.engineHandle = engineHandle
	appsDebugLog("WFP engine opened successfully")

	return af, nil
}

// Close closes the app filter and removes all rules
func (af *AppFilter) Close() error {
	if af.engineHandle == 0 {
		return nil
	}

	// Remove all filters
	for _, filterID := range af.filterIDs {
		procFwpmFilterDeleteById0.Call(af.engineHandle, uintptr(filterID), 0)
	}
	af.filterIDs = nil

	// Close engine
	procFwpmEngineClose0.Call(af.engineHandle)
	af.engineHandle = 0

	appsDebugLog("WFP engine closed")
	return nil
}

// Ensure syscall is imported
var _ = syscall.EINVAL
