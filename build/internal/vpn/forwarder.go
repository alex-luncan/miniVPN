package vpn

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
)

// Forwarder handles server-side packet forwarding for VPN clients.
// It acts as a NAT gateway, forwarding packets from VPN clients to
// the internet and routing responses back.
type Forwarder struct {
	// TCP connection tracking
	tcpConns   map[string]*tcpForwardConn
	tcpConnsMu sync.RWMutex

	// UDP socket for all UDP traffic (shared)
	udpConn   *net.UDPConn
	udpNAT    map[string]*udpNATEntry
	udpNATMu  sync.RWMutex

	// Running state
	running bool
	mu      sync.RWMutex
}

// tcpForwardConn tracks a TCP connection being forwarded
type tcpForwardConn struct {
	conn       net.Conn
	clientIP   net.IP
	clientPort uint16
	destIP     net.IP
	destPort   uint16
	sessionID  [16]byte
	lastActive time.Time
	onResponse func([]byte)
}

// udpNATEntry tracks UDP NAT mappings
type udpNATEntry struct {
	clientIP   net.IP
	clientPort uint16
	destAddr   *net.UDPAddr
	sessionID  [16]byte
	lastActive time.Time
	onResponse func([]byte)
}

// NewForwarder creates a new packet forwarder
func NewForwarder() *Forwarder {
	f := &Forwarder{
		tcpConns: make(map[string]*tcpForwardConn),
		udpNAT:   make(map[string]*udpNATEntry),
		running:  true,
	}

	// Start UDP listener
	go f.startUDPListener()

	// Start cleanup goroutine
	go f.cleanupLoop()

	return f
}

// ForwardPacket forwards an IP packet from a VPN client
func (f *Forwarder) ForwardPacket(packet []byte, sessionID [16]byte, onResponse func([]byte)) {
	f.mu.RLock()
	if !f.running {
		f.mu.RUnlock()
		return
	}
	f.mu.RUnlock()

	// Parse IP packet
	if len(packet) < 20 {
		return // Too short for IP header
	}

	version := packet[0] >> 4
	if version != 4 {
		return // Only IPv4 supported
	}

	headerLen := int(packet[0]&0x0f) * 4
	if headerLen < 20 || len(packet) < headerLen {
		return // Invalid header
	}

	protocol := packet[9]
	srcIP := net.IP(packet[12:16])
	dstIP := net.IP(packet[16:20])

	switch protocol {
	case 6: // TCP
		f.forwardTCP(packet, headerLen, srcIP, dstIP, sessionID, onResponse)
	case 17: // UDP
		f.forwardUDP(packet, headerLen, srcIP, dstIP, sessionID, onResponse)
	case 1: // ICMP
		f.forwardICMP(packet, srcIP, dstIP, sessionID, onResponse)
	}
}

// forwardTCP handles TCP packet forwarding
func (f *Forwarder) forwardTCP(packet []byte, ipHeaderLen int, srcIP, dstIP net.IP, sessionID [16]byte, onResponse func([]byte)) {
	if len(packet) < ipHeaderLen+20 {
		return // Too short for TCP header
	}

	tcpHeader := packet[ipHeaderLen:]
	srcPort := binary.BigEndian.Uint16(tcpHeader[0:2])
	dstPort := binary.BigEndian.Uint16(tcpHeader[2:4])

	connKey := fmt.Sprintf("%s:%d->%s:%d", srcIP, srcPort, dstIP, dstPort)

	f.tcpConnsMu.RLock()
	existing := f.tcpConns[connKey]
	f.tcpConnsMu.RUnlock()

	// Check TCP flags
	flags := tcpHeader[13]
	isSYN := (flags & 0x02) != 0
	isFIN := (flags & 0x01) != 0
	isRST := (flags & 0x04) != 0

	if isSYN && existing == nil {
		// New connection - establish outbound TCP connection
		go f.establishTCPConn(connKey, srcIP, srcPort, dstIP, dstPort, sessionID, onResponse)
		return
	}

	if existing != nil {
		// Forward data to existing connection
		existing.lastActive = time.Now()

		// Extract TCP payload
		tcpHeaderLen := int((tcpHeader[12] >> 4)) * 4
		if len(packet) > ipHeaderLen+tcpHeaderLen {
			payload := packet[ipHeaderLen+tcpHeaderLen:]
			if len(payload) > 0 {
				existing.conn.Write(payload)
			}
		}

		if isFIN || isRST {
			// Connection closing
			f.tcpConnsMu.Lock()
			delete(f.tcpConns, connKey)
			f.tcpConnsMu.Unlock()
			existing.conn.Close()
		}
	}
}

// establishTCPConn establishes a new TCP connection for forwarding
func (f *Forwarder) establishTCPConn(connKey string, srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, sessionID [16]byte, onResponse func([]byte)) {
	// Connect to destination
	addr := fmt.Sprintf("%s:%d", dstIP, dstPort)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		// Send RST back to client
		f.sendTCPReset(srcIP, srcPort, dstIP, dstPort, onResponse)
		return
	}

	fwdConn := &tcpForwardConn{
		conn:       conn,
		clientIP:   srcIP,
		clientPort: srcPort,
		destIP:     dstIP,
		destPort:   dstPort,
		sessionID:  sessionID,
		lastActive: time.Now(),
		onResponse: onResponse,
	}

	f.tcpConnsMu.Lock()
	f.tcpConns[connKey] = fwdConn
	f.tcpConnsMu.Unlock()

	// Start reading responses
	go f.readTCPResponses(fwdConn, connKey)
}

// readTCPResponses reads responses from the forwarded TCP connection
func (f *Forwarder) readTCPResponses(fwdConn *tcpForwardConn, connKey string) {
	buf := make([]byte, 65535)
	for {
		n, err := fwdConn.conn.Read(buf)
		if err != nil {
			break
		}

		if n > 0 {
			// Build IP packet with TCP payload
			packet := f.buildTCPPacket(fwdConn.destIP, fwdConn.destPort, fwdConn.clientIP, fwdConn.clientPort, buf[:n])
			fwdConn.onResponse(packet)
			fwdConn.lastActive = time.Now()
		}
	}

	// Cleanup
	f.tcpConnsMu.Lock()
	delete(f.tcpConns, connKey)
	f.tcpConnsMu.Unlock()
	fwdConn.conn.Close()
}

// sendTCPReset sends a TCP RST packet
func (f *Forwarder) sendTCPReset(srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, onResponse func([]byte)) {
	// Build minimal RST packet
	packet := make([]byte, 40) // IP header (20) + TCP header (20)

	// IP header
	packet[0] = 0x45 // Version 4, header length 5 words
	packet[8] = 64   // TTL
	packet[9] = 6    // TCP
	copy(packet[12:16], dstIP.To4())
	copy(packet[16:20], srcIP.To4())
	// Calculate IP checksum
	binary.BigEndian.PutUint16(packet[2:4], 40) // Total length

	// TCP header
	binary.BigEndian.PutUint16(packet[20:22], dstPort)
	binary.BigEndian.PutUint16(packet[22:24], srcPort)
	packet[33] = 0x04 // RST flag
	packet[32] = 0x50 // Data offset = 5 words

	onResponse(packet)
}

// forwardUDP handles UDP packet forwarding
func (f *Forwarder) forwardUDP(packet []byte, ipHeaderLen int, srcIP, dstIP net.IP, sessionID [16]byte, onResponse func([]byte)) {
	if len(packet) < ipHeaderLen+8 {
		return // Too short for UDP header
	}

	udpHeader := packet[ipHeaderLen:]
	srcPort := binary.BigEndian.Uint16(udpHeader[0:2])
	dstPort := binary.BigEndian.Uint16(udpHeader[2:4])
	udpLen := binary.BigEndian.Uint16(udpHeader[4:6])

	if int(udpLen) < 8 || len(packet) < ipHeaderLen+int(udpLen) {
		return
	}

	payload := udpHeader[8:udpLen]

	// Create NAT entry key
	natKey := fmt.Sprintf("%s:%d", srcIP, srcPort)

	f.udpNATMu.Lock()
	entry, exists := f.udpNAT[natKey]
	if !exists {
		entry = &udpNATEntry{
			clientIP:   srcIP,
			clientPort: srcPort,
			destAddr:   &net.UDPAddr{IP: dstIP, Port: int(dstPort)},
			sessionID:  sessionID,
			lastActive: time.Now(),
			onResponse: onResponse,
		}
		f.udpNAT[natKey] = entry
	} else {
		entry.lastActive = time.Now()
		entry.destAddr = &net.UDPAddr{IP: dstIP, Port: int(dstPort)}
		entry.onResponse = onResponse
	}
	f.udpNATMu.Unlock()

	// Forward the packet
	if f.udpConn != nil {
		f.udpConn.WriteToUDP(payload, entry.destAddr)
	}
}

// startUDPListener starts the UDP listener for responses
func (f *Forwarder) startUDPListener() {
	var err error
	f.udpConn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return
	}

	buf := make([]byte, 65535)
	for {
		f.mu.RLock()
		running := f.running
		f.mu.RUnlock()
		if !running {
			break
		}

		f.udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := f.udpConn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		if n > 0 {
			// Find the NAT entry for this response
			f.udpNATMu.RLock()
			for _, entry := range f.udpNAT {
				if entry.destAddr.IP.Equal(remoteAddr.IP) && entry.destAddr.Port == remoteAddr.Port {
					// Build response packet
					packet := f.buildUDPPacket(remoteAddr.IP, uint16(remoteAddr.Port), entry.clientIP, entry.clientPort, buf[:n])
					entry.onResponse(packet)
					entry.lastActive = time.Now()
					break
				}
			}
			f.udpNATMu.RUnlock()
		}
	}
}

// forwardICMP handles ICMP packet forwarding (ping)
func (f *Forwarder) forwardICMP(packet []byte, srcIP, dstIP net.IP, sessionID [16]byte, onResponse func([]byte)) {
	// Simple ICMP echo request/reply forwarding
	if len(packet) < 28 {
		return
	}

	// Forward ICMP using raw socket would require elevated privileges
	// For simplicity, we'll generate a fake echo reply for ping requests
	icmpType := packet[20]
	if icmpType == 8 { // Echo request
		// Create echo reply
		reply := make([]byte, len(packet))
		copy(reply, packet)

		// Swap src/dst IP
		copy(reply[12:16], dstIP.To4())
		copy(reply[16:20], srcIP.To4())

		// Change ICMP type to echo reply
		reply[20] = 0

		// Recalculate ICMP checksum
		reply[22] = 0
		reply[23] = 0
		checksum := calculateChecksum(reply[20:])
		binary.BigEndian.PutUint16(reply[22:24], checksum)

		onResponse(reply)
	}
}

// buildTCPPacket builds an IP packet with TCP payload
func (f *Forwarder) buildTCPPacket(srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, payload []byte) []byte {
	// Simplified TCP packet construction
	totalLen := 40 + len(payload) // IP(20) + TCP(20) + payload
	packet := make([]byte, totalLen)

	// IP header
	packet[0] = 0x45 // Version 4, IHL 5
	binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
	packet[8] = 64 // TTL
	packet[9] = 6  // TCP
	copy(packet[12:16], srcIP.To4())
	copy(packet[16:20], dstIP.To4())

	// IP checksum
	ipChecksum := calculateChecksum(packet[:20])
	binary.BigEndian.PutUint16(packet[10:12], ipChecksum)

	// TCP header
	binary.BigEndian.PutUint16(packet[20:22], srcPort)
	binary.BigEndian.PutUint16(packet[22:24], dstPort)
	packet[32] = 0x50   // Data offset = 5 words
	packet[33] = 0x18   // PSH, ACK
	packet[34] = 0xff   // Window size high
	packet[35] = 0xff   // Window size low

	// Copy payload
	copy(packet[40:], payload)

	return packet
}

// buildUDPPacket builds an IP packet with UDP payload
func (f *Forwarder) buildUDPPacket(srcIP net.IP, srcPort uint16, dstIP net.IP, dstPort uint16, payload []byte) []byte {
	udpLen := 8 + len(payload)
	totalLen := 20 + udpLen
	packet := make([]byte, totalLen)

	// IP header
	packet[0] = 0x45 // Version 4, IHL 5
	binary.BigEndian.PutUint16(packet[2:4], uint16(totalLen))
	packet[8] = 64  // TTL
	packet[9] = 17  // UDP
	copy(packet[12:16], srcIP.To4())
	copy(packet[16:20], dstIP.To4())

	// IP checksum
	ipChecksum := calculateChecksum(packet[:20])
	binary.BigEndian.PutUint16(packet[10:12], ipChecksum)

	// UDP header
	binary.BigEndian.PutUint16(packet[20:22], srcPort)
	binary.BigEndian.PutUint16(packet[22:24], dstPort)
	binary.BigEndian.PutUint16(packet[24:26], uint16(udpLen))

	// Copy payload
	copy(packet[28:], payload)

	return packet
}

// calculateChecksum calculates IP/TCP/UDP checksum
func calculateChecksum(data []byte) uint16 {
	sum := uint32(0)
	length := len(data)

	for i := 0; i < length-1; i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}

	if length%2 == 1 {
		sum += uint32(data[length-1]) << 8
	}

	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}

	return ^uint16(sum)
}

// cleanupLoop periodically cleans up stale connections
func (f *Forwarder) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		f.mu.RLock()
		running := f.running
		f.mu.RUnlock()
		if !running {
			return
		}

		<-ticker.C

		now := time.Now()
		timeout := 5 * time.Minute

		// Cleanup TCP connections
		f.tcpConnsMu.Lock()
		for key, conn := range f.tcpConns {
			if now.Sub(conn.lastActive) > timeout {
				conn.conn.Close()
				delete(f.tcpConns, key)
			}
		}
		f.tcpConnsMu.Unlock()

		// Cleanup UDP NAT entries
		f.udpNATMu.Lock()
		for key, entry := range f.udpNAT {
			if now.Sub(entry.lastActive) > timeout {
				delete(f.udpNAT, key)
			}
		}
		f.udpNATMu.Unlock()
	}
}

// Close closes the forwarder
func (f *Forwarder) Close() {
	f.mu.Lock()
	f.running = false
	f.mu.Unlock()

	// Close all TCP connections
	f.tcpConnsMu.Lock()
	for _, conn := range f.tcpConns {
		conn.conn.Close()
	}
	f.tcpConns = make(map[string]*tcpForwardConn)
	f.tcpConnsMu.Unlock()

	// Close UDP socket
	if f.udpConn != nil {
		f.udpConn.Close()
	}

	// Clear UDP NAT table
	f.udpNATMu.Lock()
	f.udpNAT = make(map[string]*udpNATEntry)
	f.udpNATMu.Unlock()
}
