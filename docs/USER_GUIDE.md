# miniVPN User Guide

## Getting Started

### System Requirements
- Windows 10 (version 1903 or later)
- Windows 11
- Windows Server 2019 or later
- Administrator privileges (required for TUN adapter and routing)

### Installation
1. Download `miniVPN.zip` from the releases page
2. Extract all files to a folder (keep `miniVPN.exe` and `wintun.dll` together)
3. Run `miniVPN.exe` as Administrator
4. Allow administrator access when prompted

**Important:** The `wintun.dll` file must be in the same directory as `miniVPN.exe`. This file is required for the VPN to route network traffic.

## Server Mode

### Starting a VPN Server

1. Launch miniVPN
2. Click on "Server Mode"
3. Note your automatically generated secret code
4. Configure the server port (default: 51820)
5. Click "Start Server"

### Server Configuration

| Setting | Description | Default |
|---------|-------------|---------|
| Port | UDP port for VPN connections | 51820 |
| Secret Code | Authentication code for clients | Auto-generated |

### Managing Secret Codes

- Codes are automatically generated when entering server mode
- Click the regenerate button to create a new code
- Share codes securely with authorized users
- Codes change when regenerated or when the server restarts

### Sharing Connection Details

Provide clients with:
1. Your server's IP address (shown in the dashboard)
2. The server port
3. The current secret code

## Client Mode

### Connecting to a Server

1. Launch miniVPN
2. Click on "Client Mode"
3. Enter the server IP address
4. Enter the secret code
5. Click "Connect to VPN"

### Connection Status

When connected, you'll see:
- Green status indicator
- Server address
- Active connection status

## Split Tunneling

Split tunneling allows you to route only specific ports through the VPN while keeping other traffic on your normal network.

### Why Use Split Tunneling?

- Test database connections on specific ports
- Keep video streaming on your local connection
- Reduce VPN bandwidth usage
- Access local network resources

### Configuring Split Tunneling

1. Click "Configure Split Tunneling"
2. Choose a mode:
   - **Include Mode**: Only selected ports use VPN
   - **Exclude Mode**: Everything except selected ports uses VPN

### Adding Ports

**Manual Entry:**
1. Enter a port number (1-65535)
2. Click "Add"

**Quick Add:**
Click common service buttons:
- MySQL (3306)
- PostgreSQL (5432)
- Redis (6379)
- MongoDB (27017)
- SSH (22)
- RDP (3389)

### Example: Database Testing

To route only database traffic through VPN:

1. Select "Include Mode"
2. Add port 14700 (or your database port)
3. Click "Save Configuration"
4. Connect to VPN

Now only port 14700 traffic goes through VPN. All other traffic uses your normal connection.

## Troubleshooting

### Connection Failed

**Possible causes:**
- Incorrect server IP or secret code
- Server not running
- Firewall blocking connection
- Network connectivity issues
- Missing wintun.dll

**Solutions:**
1. Verify server IP and secret code
2. Check if server is running
3. Allow miniVPN through firewall
4. Test network connectivity
5. Ensure `wintun.dll` is in the same folder as `miniVPN.exe`

### "wintun.dll not found" Error

**Solution:**
1. Download the release ZIP file (not just the exe)
2. Extract both `miniVPN.exe` and `wintun.dll` to the same folder
3. Or download wintun.dll manually from https://www.wintun.net/

### Split Tunneling Not Working

**Possible causes:**
- Application running without admin rights
- Conflicting network software
- Incorrect port configuration

**Solutions:**
1. Run miniVPN as Administrator
2. Temporarily disable other VPN software
3. Verify port numbers are correct

### Server Won't Start

**Possible causes:**
- Port already in use
- Insufficient permissions
- Firewall blocking

**Solutions:**
1. Try a different port
2. Run as Administrator
3. Add firewall exception

### TUN Adapter Not Created

**Possible causes:**
- Missing wintun.dll
- Not running as Administrator
- Wintun driver conflict

**Solutions:**
1. Ensure wintun.dll is present
2. Run as Administrator
3. Reboot if another VPN was recently used

## Security Best Practices

1. **Regenerate codes regularly** - Don't reuse the same code indefinitely
2. **Share codes securely** - Use encrypted messaging
3. **Use strong network security** - Keep your server behind a firewall
4. **Update regularly** - Keep miniVPN updated

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| Escape | Go back / Cancel |
| Enter | Submit form |

## FAQ

**Q: Can I run multiple servers?**
A: Each instance of miniVPN can run one server. For multiple servers, run multiple instances on different ports.

**Q: Is my traffic encrypted?**
A: Yes, all VPN traffic is encrypted using AES-256-GCM with Curve25519 key exchange.

**Q: What happens if the server restarts?**
A: A new secret code is generated. Clients must reconnect with the new code.

**Q: Can I save server settings?**
A: Settings are not persisted to prevent storing sensitive data.

**Q: What IP address will I get?**
A: The server assigns IP addresses from 10.0.0.2-254. The server uses 10.0.0.1.

**Q: Does all my traffic go through the VPN?**
A: Yes, all traffic is routed through the VPN when connected. Use split tunneling to exclude specific ports.

**Q: Why do I need wintun.dll?**
A: Wintun is a TUN driver that creates a virtual network adapter. It's required to route traffic through the VPN.

**Q: Can I verify my traffic is going through the VPN?**
A: Yes, visit https://api.ipify.org when connected - it should show the server's public IP.
