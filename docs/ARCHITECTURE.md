# miniVPN Architecture

## Overview

miniVPN is a split-tunnel VPN application built with Go and Wails, featuring a Svelte frontend. It supports both server and client modes with port-based routing.

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Backend | Go 1.21+ | VPN logic, networking, system integration |
| Framework | Wails v2 | Desktop app framework, Go-JS bindings |
| Frontend | Svelte 5 | Modern reactive UI |
| Build Tool | Vite | Fast frontend bundling |
| VPN Protocol | WireGuard | Secure, fast VPN tunneling |

## Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│                   miniVPN.exe                   │
├─────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────────┐   │
│  │   Svelte UI     │  │      Wails          │   │
│  │  (WebView2)     │◄─┤    Go Bindings      │   │
│  └─────────────────┘  └─────────────────────┘   │
│           │                     │               │
│           ▼                     ▼               │
│  ┌─────────────────────────────────────────┐    │
│  │              App Layer                  │    │
│  │  - Mode Management (Server/Client)      │    │
│  │  - Secret Code Generation               │    │
│  │  - Connection State                     │    │
│  └─────────────────────────────────────────┘    │
│                       │                         │
│           ┌───────────┴───────────┐             │
│           ▼                       ▼             │
│  ┌─────────────────┐  ┌─────────────────────┐   │
│  │   VPN Engine    │  │   Split Tunnel      │   │
│  │  (WireGuard)    │  │   (WFP + Wintun)    │   │
│  └─────────────────┘  └─────────────────────┘   │
│           │                       │             │
│           └───────────┬───────────┘             │
│                       ▼                         │
│  ┌─────────────────────────────────────────┐    │
│  │           Windows Network Stack         │    │
│  └─────────────────────────────────────────┘    │
└─────────────────────────────────────────────────┘
```

## Component Details

### 1. Frontend Layer (Svelte)

- **ModeSelector**: Initial screen for choosing Server/Client mode
- **ServerDashboard**: Server configuration and status
- **ClientConnect**: Client connection form
- **SplitTunnelConfig**: Port-based routing configuration

### 2. Application Layer (Go)

The `App` struct manages:
- Application mode (server/client)
- Secret code generation and validation
- Connection state
- Split tunnel configuration

### 3. VPN Engine

Uses WireGuard-go for secure tunneling:
- ChaCha20-Poly1305 encryption
- Noise protocol for key exchange
- UDP-based transport

### 4. Split Tunneling

Implements port-based routing:
- **Wintun**: User-space TUN driver for packet capture
- **WFP**: Windows Filtering Platform for packet routing

## Data Flow

### Server Mode
```
1. User starts server
2. Generate unique secret code (20-char Base32)
3. Initialize WireGuard listener
4. Wait for client connections
5. Authenticate via secret code
6. Establish encrypted tunnel
```

### Client Mode
```
1. User enters server IP + secret
2. Connect to server
3. Authenticate with secret code
4. Establish WireGuard tunnel
5. Apply split tunnel rules (WFP)
6. Route selected ports through VPN
```

## Security Model

- **Secret Codes**: Random 20-character Base32 codes, regenerated on server restart
- **Encryption**: ChaCha20-Poly1305 (WireGuard default)
- **Key Exchange**: Noise protocol (WireGuard)
- **No Persistent Keys**: New keys generated each session

## File Structure

```
build/
├── main.go              # Entry point, Wails setup
├── app.go               # Application logic
├── internal/
│   ├── vpn/
│   │   ├── server.go    # Server mode implementation
│   │   ├── client.go    # Client mode implementation
│   │   └── tunnel.go    # WireGuard tunnel management
│   ├── splittunnel/
│   │   ├── wfp.go       # Windows Filtering Platform
│   │   └── router.go    # Port routing logic
│   └── config/
│       └── config.go    # Configuration management
└── frontend/
    ├── src/
    │   ├── App.svelte
    │   └── components/
    └── wailsjs/          # Auto-generated bindings
```

## Build Output

Single executable containing:
- Go backend (compiled)
- Svelte frontend (embedded)
- WebView2 runtime (system dependency)

Target size: ~15-20MB
