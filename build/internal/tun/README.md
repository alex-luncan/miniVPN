# Wintun DLL Setup

Before building miniVPN, you need to download the Wintun DLL:

1. Visit https://www.wintun.net/
2. Download the latest release (wintun-X.X.zip)
3. Extract and copy `wintun/bin/amd64/wintun.dll` to this directory (`internal/tun/`)

The `wintun.dll` file will be embedded into the binary during compilation.

## Architecture

- For 64-bit Windows: Use `wintun/bin/amd64/wintun.dll`
- For 32-bit Windows: Use `wintun/bin/x86/wintun.dll`
- For ARM64 Windows: Use `wintun/bin/arm64/wintun.dll`

## Note

The wintun.dll is not included in this repository due to licensing. You must download it separately from the official Wintun website.
