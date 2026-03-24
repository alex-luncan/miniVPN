# miniVPN

A lightweight, modern VPN application with real traffic routing, split tunneling, and NAT traversal support for Windows.

[![Download](https://img.shields.io/badge/Download-v2.2.0-blue?style=for-the-badge&logo=windows)](https://github.com/alex-luncan/miniVPN/releases/download/v2.2.0/miniVPN.zip)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](https://github.com/alex-luncan/miniVPN/blob/main/license)

![miniVPN Screenshot](docs/images/screenshot.png)

## Features

- **Real VPN Traffic Routing**: All network traffic routed through VPN tunnel using Wintun TUN adapter
- **Server Mode**: Host a VPN server with auto-generated secret codes and NAT/forwarding
- **Client Mode**: Connect to VPN servers with IP and secret code
- **NAT Traversal**: UDP hole punching for connecting through firewalls and routers
- **Split Tunneling**: Choose between full VPN (all traffic) or split tunnel (only VPN network traffic)
- **Auto Firewall Rules**: Automatically configures Windows Firewall on startup
- **Automatic IP Assignment**: Server assigns VPN IPs (10.0.0.x) to clients automatically
- **Modern UI**: Clean, dark-themed interface built with Svelte

## Requirements

- Windows 10, Windows 11, or Windows Server 2019+
- Administrator privileges (required for TUN adapter, routing, and firewall)
- **wintun.dll** - Must be placed in the same directory as miniVPN.exe (included in release zip)

## Quick Start

### Scenario 1: Server has Public IP (Cloud VM, VPS)

This is the simplest setup when your server has a public IP address.

**On the Server (e.g., Azure VM, AWS EC2):**
1. Launch miniVPN and select **Server Mode**
2. Note the generated **Secret Code**
3. Set the **Port** (default: 51820)
4. Click **Start Server**
5. Ensure port 51820 (TCP) is open in your cloud firewall/NSG

**On the Client:**
1. Launch miniVPN and select **Client Mode**
2. Enter the server's **Public IP** and **Port**
3. Enter the **Secret Code**
4. Click **Connect to VPN**

### Scenario 2: Both Peers Behind NAT (Home Networks)

When both server and client are behind routers/NAT, use hole punching.

**Prerequisites:** You need a machine with a public IP to run the signaling server (e.g., a cloud VM).

**Step 1: Start Signaling Server (on machine with public IP)**
```bash
.\signaling-server.exe -port 51821
```
Or use miniVPN and click "Start Signaling" in Server Mode.

**Step 2: On the VPN Server (behind NAT)**
1. Launch miniVPN → **Server Mode**
2. Click **Start Server**
3. In "Register with External Signaling Server", enter: `<signaling-ip>:51821`
4. Click **Register**
5. Note the **Secret Code**

**Step 3: On the Client (behind NAT)**
1. Launch miniVPN → **Client Mode**
2. Enable **NAT Traversal (UDP Hole Punching)**
3. Enter Signaling Server: `<signaling-ip>:51821`
4. Enter the **Secret Code**
5. Click **Connect to VPN**

## Connection Modes

| Server Location | Client Location | Mode to Use |
|----------------|-----------------|-------------|
| Public IP (Cloud) | Anywhere | Direct Connection |
| Behind NAT | Behind NAT | NAT Traversal (Hole Punching) |
| Behind NAT | Public IP | NAT Traversal or Direct (client as server) |

### When to Use NAT Traversal

- **Use Direct Connection** when the server has a public IP (Azure, AWS, VPS, etc.)
- **Use NAT Traversal** when both peers are behind home routers/firewalls

NAT Traversal requires a signaling server running on a machine with a public IP to coordinate the connection.

## Split Tunneling

miniVPN supports two routing modes:

- **Split Tunnel Mode**: Only traffic to VPN network (10.0.0.x) goes through VPN. Your real IP is used for internet traffic.
- **Full VPN Mode**: All traffic goes through VPN. Your public IP shows the VPN server's IP.

### How to Use
1. Open Split Tunneling configuration
2. Select your preferred mode:
   - **Split Tunnel** - Access VPN resources while keeping your real IP for internet
   - **Full VPN** - Route everything through VPN for privacy
3. Click **Save Configuration**
4. Connect (or reconnect) to the VPN server

### When to Use Each Mode

| Mode | Use Case | Your Public IP |
|------|----------|----------------|
| Split Tunnel | Access VPN network resources (10.0.0.x) while keeping normal internet speed | Your real IP |
| Full VPN | Maximum privacy, hide your real IP | VPN server IP |

**Note**: Changes to split tunnel mode take effect on the next connection.

## Firewall Configuration

miniVPN automatically creates Windows Firewall rules on startup. The rules are:
- `miniVPN UDP` - Allows UDP traffic for hole punching
- `miniVPN TCP` - Allows TCP traffic for VPN connections

For cloud servers, you also need to open ports in your cloud provider's firewall:
- **Port 51820 (TCP)** - VPN server
- **Port 51821 (UDP)** - Signaling server (if running)

### Azure NSG Example
```
Inbound Rule: Allow TCP 51820 from Any
Inbound Rule: Allow UDP 51821 from Any
```

## Building from Source

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **wintun.dll** from https://www.wintun.net/

### Build Steps
```bash
cd build
wails build
```

The executable will be created in `build/build/bin/miniVPN.exe`.

### Wintun DLL Setup
1. Download wintun from https://www.wintun.net/
2. Extract `wintun/bin/amd64/wintun.dll` (for 64-bit Windows)
3. Place `wintun.dll` in the same directory as `miniVPN.exe`

### Build Signaling Server (optional)
```bash
cd build
go build -o signaling-server.exe ./cmd/signaling-server/
```

## Project Structure

```
miniVPN/
├── build/                      # Source code
│   ├── main.go                 # Go entry point
│   ├── app.go                  # Application logic + TUN/bridge integration
│   ├── cmd/
│   │   └── signaling-server/   # Standalone signaling server
│   ├── internal/
│   │   ├── vpn/                # VPN protocol implementation
│   │   │   ├── tunnel.go       # Encrypted tunnel management
│   │   │   ├── bridge.go       # TUN ↔ Tunnel traffic bridge
│   │   │   ├── forwarder.go    # Server-side NAT/packet forwarding
│   │   │   ├── ippool.go       # VPN IP address allocation
│   │   │   └── protocol.go     # Wire protocol + IP assignment
│   │   ├── holepunch/          # NAT traversal / hole punching
│   │   ├── firewall/           # Windows Firewall management
│   │   ├── splittunnel/        # Split tunneling logic
│   │   │   ├── routes.go       # VPN route setup/teardown
│   │   │   ├── apps_windows.go # Application enumeration & filtering
│   │   │   └── wfp_windows.go  # Windows Filtering Platform
│   │   └── tun/                # Virtual network adapter (Wintun)
│   │       ├── adapter.go      # TUN adapter abstraction
│   │       └── wintun_windows.go # Wintun DLL integration
│   ├── frontend/               # Svelte UI
│   └── wails.json              # Wails configuration
├── docs/                       # Documentation
│   └── images/                 # Screenshots
└── README.md
```

## How It Works

### Traffic Flow
```
CLIENT:
[Apps] → [Windows Network] → [Wintun TUN 10.0.0.x] → [Bridge] → [Encrypt] → [TCP to Server]

SERVER:
[TCP from Client] → [Decrypt] → [Forwarder/NAT] → [Internet] → [Response] → [Encrypt] → [TCP to Client]

CLIENT (response):
[TCP from Server] → [Decrypt] → [Bridge] → [Write to TUN] → [Apps receive response]
```

### VPN IP Assignment
- Server uses 10.0.0.1
- Clients are assigned 10.0.0.2, 10.0.0.3, etc.
- Subnet mask: 255.255.255.0

## Troubleshooting

### Debug Logging
miniVPN includes built-in debug logging to help diagnose connection issues. Run from command prompt to see logs:
```bash
cd <path-to-miniVPN>
miniVPN.exe
```
Look for `[APP]`, `[SIGNALING]`, and `[HOLEPUNCH-CLIENT]` prefixes in the output.

### "Connection failed" with direct connection
1. Check that the server is running and shows "Running" status
2. Verify the server's firewall allows inbound TCP on the VPN port
3. Confirm you're using the correct IP address and port

### "Peer not found" with hole punching
1. Ensure the signaling server is running and accessible
2. Verify the VPN server has registered with the signaling server
3. Check that the secret code matches exactly

### "Hole punch timeout - NAT may be symmetric"
Some NAT types (symmetric NAT) are difficult to punch through. Try:
1. Using a server with a public IP instead
2. Running the signaling server on a different port
3. Using a relay/TURN server (not yet implemented)

### Connected but traffic not routing through VPN
If your local network uses the same subnet as the VPN (10.0.0.x), routes may conflict. miniVPN handles this automatically by specifying the VPN interface explicitly, but if issues persist:
1. Check `route print` to see if VPN routes point to the correct interface
2. Ensure miniVPN is running with Administrator privileges

### Firewall issues
Run miniVPN as Administrator to allow automatic firewall rule creation.

## Security

- Connections are authenticated using a 20-character secret code
- Traffic is encrypted using AES-256-GCM
- Key exchange uses Curve25519 ECDH
- Secret codes are regenerated on each server start
- All traffic is routed through encrypted tunnel (no leaks)

## Wintun Requirement

miniVPN uses [Wintun](https://www.wintun.net/) - a lightweight TUN driver for Windows developed by the WireGuard project.

**The `wintun.dll` file must be in the same directory as `miniVPN.exe`.**

The release package includes wintun.dll. If building from source, download it from https://www.wintun.net/

## License

MIT License

## Author

alex-luncan
