# miniVPN

A lightweight, modern VPN application with split tunneling support for Windows.

![miniVPN Screenshot](docs/images/screenshot.png)

## Features

- **Server Mode**: Host a VPN server with auto-generated secret codes
- **Client Mode**: Connect to VPN servers with IP and secret code
- **Split Tunneling**: Route specific ports through VPN while keeping other traffic on normal network
- **Modern UI**: Clean, dark-themed interface built with Svelte
- **Single Executable**: Standalone Windows application, no installation required

## Requirements

- Windows 10, Windows 11, or Windows Server 2019+
- Administrator privileges (required for VPN functionality)

## Quick Start

### Server Mode
1. Launch miniVPN and select "Server Mode"
2. Note the generated secret code
3. Click "Start Server"
4. Share your IP address and secret code with clients

### Client Mode
1. Launch miniVPN and select "Client Mode"
2. Enter the server IP address
3. Enter the secret code provided by the server
4. Configure split tunneling (optional)
5. Click "Connect to VPN"

## Split Tunneling

miniVPN allows you to route only specific ports through the VPN:

- **Include Mode**: Only selected ports go through VPN (e.g., database port 14700)
- **Exclude Mode**: All traffic except selected ports goes through VPN

This is useful for:
- Testing database connections on specific ports
- Keeping general browsing on your normal connection
- Reducing VPN bandwidth usage

## Building from Source

### Prerequisites
- Go 1.21+
- Node.js 18+
- Wails CLI

### Build Steps
```bash
cd build
wails build
```

The executable will be created in `build/build/bin/`.

## Project Structure

```
miniVPN/
├── build/              # Source code
│   ├── main.go         # Go entry point
│   ├── app.go          # Application logic
│   ├── frontend/       # Svelte UI
│   └── wails.json      # Wails configuration
├── docs/               # Documentation
└── README.md
```

## License

MIT License

## Author

alex-luncan
