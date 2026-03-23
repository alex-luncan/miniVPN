# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 2.1.x   | :white_check_mark: |
| 2.0.x   | :white_check_mark: |
| 1.0.x   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, report them via email to: **security@miniVPN** (or create a private security advisory on GitHub)

To create a private security advisory:
1. Go to the [Security tab](https://github.com/alex-luncan/miniVPN/security)
2. Click "Report a vulnerability"
3. Fill in the details

### What to Include

- Type of vulnerability (e.g., encryption flaw, authentication bypass, data leak)
- Location of the affected code (file path, function name)
- Step-by-step instructions to reproduce
- Potential impact of the vulnerability
- Any suggested fixes (if you have them)

### Response Timeline

- **Acknowledgment:** Within 48 hours
- **Initial Assessment:** Within 7 days
- **Resolution Target:** Within 30 days (depending on severity)

### What to Expect

1. We will acknowledge receipt of your report
2. We will investigate and validate the vulnerability
3. We will work on a fix and coordinate disclosure timing with you
4. We will credit you in the release notes (unless you prefer to remain anonymous)

## Security Best Practices for Users

### Running miniVPN Securely

1. **Always run as Administrator** - Required for TUN adapter and routing
2. **Keep miniVPN updated** - Use the latest version for security fixes
3. **Protect your secret codes** - Treat them like passwords
4. **Use trusted networks** - Be cautious on public networks when setting up VPN server
5. **Verify downloads** - Only download from official GitHub releases

### Encryption Details

miniVPN uses industry-standard encryption:

- **Key Exchange:** Curve25519 ECDH
- **Encryption:** AES-256-GCM
- **Authentication:** 20-character secret codes with secure hashing

### Network Security

- All traffic between client and server is encrypted
- Secret codes are never transmitted in plain text
- Session keys are derived using secure key derivation

## Scope

The following are **in scope** for security reports:

- Encryption weaknesses
- Authentication/authorization bypasses
- Data leaks or exposure
- Remote code execution
- Privilege escalation
- Denial of service vulnerabilities

The following are **out of scope**:

- Social engineering attacks
- Physical attacks requiring device access
- Issues in third-party dependencies (report to upstream)
- Issues requiring user misconfiguration

## Recognition

We appreciate security researchers who help keep miniVPN secure. With your permission, we will acknowledge your contribution in:

- Release notes
- Security advisories
- Contributors list

Thank you for helping keep miniVPN and its users safe!
