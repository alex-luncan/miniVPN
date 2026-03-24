# Security Policy

## Disclaimer

As stated in the [MIT License](../license), this software is provided "AS IS", without warranty of any kind. The authors and copyright holders are not liable for any claims, damages, or other liability arising from the use of this software.

Users are responsible for evaluating the security of this software for their specific use case.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 2.1.x   | :white_check_mark: |
| 2.0.x   | :white_check_mark: |
| 1.0.x   | :x:                |

## Reporting a Vulnerability

If you discover a security issue, please report it responsibly.

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, create a private security advisory:
1. Go to the [Security tab](https://github.com/alex-luncan/miniVPN/security)
2. Click "Report a vulnerability"
3. Fill in the details

### What to Include

- Type of vulnerability
- Location of the affected code
- Steps to reproduce
- Potential impact

### Response

Reports will be reviewed on a best-effort basis. There are no guaranteed response times or resolution commitments, consistent with the "AS IS" nature of this open source project.

## Security Overview

miniVPN uses the following security measures:

- **Key Exchange:** Curve25519 ECDH
- **Encryption:** AES-256-GCM
- **Authentication:** Secret codes with secure hashing

## Scope

**In scope:**
- Encryption weaknesses
- Authentication bypasses
- Data exposure vulnerabilities

**Out of scope:**
- Social engineering attacks
- Physical attacks
- Third-party dependency issues (report upstream)
- User misconfiguration

## Recognition

With your permission, security researchers may be acknowledged in release notes.
