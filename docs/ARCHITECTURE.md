# miniVPN Architecture

## Overview

miniVPN is a full VPN application built with Go and Wails, featuring a Svelte frontend. It provides real network traffic routing through a TUN adapter, with support for server and client modes, NAT traversal, and split tunneling.

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Backend | Go 1.21+ | VPN logic, networking, system integration |
| Framework | Wails v2 | Desktop app framework, Go-JS bindings |
| Frontend | Svelte 5 | Modern reactive UI |
| Build Tool | Vite | Fast frontend bundling |
| TUN Driver | Wintun | User-space network adapter |
| Encryption | AES-256-GCM | Traffic encryption |
| Key Exchange | Curve25519 ECDH | Secure key exchange |

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                           miniVPN.exe                               │
├─────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────┐     ┌─────────────────────────────────┐   │
│  │     Svelte UI       │     │         Wails Framework         │   │
│  │    (WebView2)       │◄───►│        Go-JS Bindings           │   │
│  └─────────────────────┘     └─────────────────────────────────┘   │
│                                           │                         │
│                                           ▼                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                      App Layer (app.go)                     │   │
│  │   - Mode Management (Server/Client)                         │   │
│  │   - TUN Adapter Lifecycle                                   │   │
│  │   - Bridge Management                                       │   │
│  │   - Route Configuration                                     │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                    │                          │                     │
│       ┌────────────┴────────────┐    ┌───────┴───────┐             │
│       ▼                         ▼    ▼               ▼             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │
│  │   VPN    │  │  Bridge  │  │   TUN    │  │  Split Tunnel    │   │
│  │  Engine  │◄─┤ (bridge) │─►│ Adapter  │  │  (routes.go)     │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘   │
│       │                           │                │               │
│       │    ┌──────────────────────┘                │               │
│       │    │                                       │               │
│       ▼    ▼                                       ▼               │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │                    Windows Network Stack                      │ │
│  │   - Wintun TUN Device (10.0.0.x)                             │ │
│  │   - Route Table (0.0.0.0/1, 128.0.0.0/1 via VPN)            │ │
│  │   - TCP/IP Stack                                             │ │
│  └──────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
                        ┌───────────────────────┐
                        │      Internet         │
                        │   (via VPN Server)    │
                        └───────────────────────┘
```

## Component Details

### 1. Frontend Layer (Svelte)

- **ModeSelector**: Initial screen for choosing Server/Client mode
- **ServerDashboard**: Server configuration, status, and connected clients
- **ClientConnect**: Client connection form with IP/secret input
- **SplitTunnelConfig**: Port-based routing configuration

### 2. Application Layer (Go - app.go)

The `App` struct manages:
- Application mode (server/client)
- TUN adapter creation and configuration
- Bridge between TUN and VPN tunnel
- VPN route setup (0.0.0.0/1 and 128.0.0.0/1)
- Split tunnel configuration

### 3. VPN Engine (internal/vpn/)

| File | Purpose |
|------|---------|
| `tunnel.go` | Encrypted tunnel over TCP, message framing |
| `protocol.go` | Wire protocol, handshake, IP assignment messages |
| `crypto.go` | AES-256-GCM encryption, Curve25519 key exchange |
| `server.go` | Server mode: accepts clients, assigns IPs, forwards packets |
| `client.go` | Client mode: connects, receives IP, manages tunnel |
| `bridge.go` | Bidirectional TUN ↔ Tunnel packet forwarding |
| `forwarder.go` | Server-side NAT, TCP/UDP connection tracking |
| `ippool.go` | VPN IP allocation (10.0.0.0/24) |

### 4. TUN Adapter (internal/tun/)

| File | Purpose |
|------|---------|
| `adapter.go` | Platform-agnostic TUN adapter interface |
| `wintun_windows.go` | Wintun DLL integration, packet send/receive |
| `wintun_embed.go` | Wintun DLL location/extraction |

### 5. Split Tunneling (internal/splittunnel/)

| File | Purpose |
|------|---------|
| `routes.go` | VPN route setup/teardown (0.0.0.0/1, 128.0.0.0/1) |
| `wfp_windows.go` | Windows Filtering Platform for port-based rules |
| `manager.go` | Split tunnel configuration and rule management |

## Data Flow

### Client Connection Flow
```
1. User clicks "Connect"
2. TCP connection to server established
3. Handshake: exchange public keys, verify secret code
4. Server sends IP assignment (10.0.0.x)
5. Client creates Wintun TUN adapter
6. Client configures IP on TUN adapter (netsh)
7. Client creates Bridge (TUN ↔ Tunnel)
8. Client sets up routes:
   - Server real IP → original gateway (bypass VPN)
   - 0.0.0.0/1 → VPN gateway (10.0.0.1)
   - 128.0.0.0/1 → VPN gateway (10.0.0.1)
9. All traffic now flows through VPN
```

### Packet Flow (Client → Internet)
```
1. App sends packet to external IP
2. Route table sends to TUN (10.0.0.1)
3. Wintun captures packet
4. Bridge reads from TUN
5. Bridge encrypts and sends through Tunnel
6. Server receives, decrypts
7. Forwarder NATs and sends to internet
8. Response received by Forwarder
9. Forwarder builds response packet
10. Server encrypts and sends to client
11. Client decrypts, Bridge writes to TUN
12. App receives response
```

### Server Forwarding
```
┌─────────────────────────────────────────────────────────────┐
│                    Forwarder (forwarder.go)                 │
├─────────────────────────────────────────────────────────────┤
│  TCP Connections:                                           │
│    - Track: srcIP:srcPort → dstIP:dstPort                  │
│    - net.Dial to destination                                │
│    - Proxy data bidirectionally                             │
│                                                             │
│  UDP NAT Table:                                             │
│    - Map: clientIP:port → external socket                  │
│    - Single UDP socket for all clients                      │
│    - Route responses back by source address                 │
│                                                             │
│  ICMP:                                                      │
│    - Generate echo replies for ping                         │
└─────────────────────────────────────────────────────────────┘
```

## Security Model

- **Authentication**: 20-character Base32 secret code, SHA-256 hashed
- **Key Exchange**: Curve25519 ECDH for forward secrecy
- **Encryption**: AES-256-GCM for all tunnel traffic
- **Session Keys**: Derived from shared secret + session ID
- **No Persistent Keys**: New keys generated each session
- **Replay Protection**: Timestamp validation in handshake

## IP Addressing

| Address | Purpose |
|---------|---------|
| 10.0.0.0 | Network address |
| 10.0.0.1 | Server VPN IP / Gateway |
| 10.0.0.2-254 | Client pool |
| 10.0.0.255 | Broadcast |

## Route Configuration

When VPN connects, the following routes are added:

| Destination | Mask | Gateway | Purpose |
|-------------|------|---------|---------|
| Server Real IP | /32 | Original GW | Bypass VPN for server traffic |
| 0.0.0.0 | /1 (128.0.0.0) | 10.0.0.1 | Route 0-127.x.x.x via VPN |
| 128.0.0.0 | /1 (128.0.0.0) | 10.0.0.1 | Route 128-255.x.x.x via VPN |

Using /1 routes instead of default route (0.0.0.0/0) ensures VPN routes take precedence without removing the original default route.

## File Structure

```
build/
├── main.go                  # Entry point, Wails setup
├── app.go                   # Application logic, TUN/Bridge integration
├── internal/
│   ├── vpn/
│   │   ├── server.go        # Server mode: accept, assign IP, forward
│   │   ├── client.go        # Client mode: connect, receive IP
│   │   ├── tunnel.go        # Encrypted tunnel management
│   │   ├── bridge.go        # TUN ↔ Tunnel traffic bridge
│   │   ├── forwarder.go     # Server-side NAT/forwarding
│   │   ├── ippool.go        # IP address allocation
│   │   ├── protocol.go      # Wire protocol, IP assignment
│   │   └── crypto.go        # Encryption, key exchange
│   ├── tun/
│   │   ├── adapter.go       # TUN adapter abstraction
│   │   ├── wintun_windows.go # Wintun integration
│   │   └── wintun_embed.go  # DLL location
│   ├── splittunnel/
│   │   ├── routes.go        # VPN route management
│   │   ├── wfp_windows.go   # Windows Filtering Platform
│   │   ├── manager.go       # Split tunnel configuration
│   │   └── router.go        # Port routing logic
│   ├── holepunch/           # NAT traversal
│   └── firewall/            # Windows Firewall
└── frontend/
    ├── src/
    │   ├── App.svelte
    │   └── components/
    └── wailsjs/             # Auto-generated bindings
```

## Dependencies

### Runtime
- **wintun.dll**: TUN driver (must be in same directory as exe)
- **WebView2**: Windows component (usually pre-installed)

### Build
- Go 1.21+
- Node.js 18+
- Wails CLI

## Build Output

Single executable + wintun.dll containing:
- Go backend (compiled)
- Svelte frontend (embedded)
- WebView2 runtime (system dependency)

Typical size: ~6-8MB (exe) + ~500KB (wintun.dll)
