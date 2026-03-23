# Contributing to miniVPN

Thank you for your interest in contributing to miniVPN! This document provides guidelines and instructions for contributing.

## Code of Conduct

By participating in this project, you agree to maintain a respectful and inclusive environment for everyone.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/alex-luncan/miniVPN/issues)
2. If not, create a new issue with:
   - Clear, descriptive title
   - Steps to reproduce the bug
   - Expected vs actual behavior
   - Windows version and system details
   - Debug logs (run miniVPN from command prompt to capture logs)

### Suggesting Features

1. Check existing issues for similar suggestions
2. Create a new issue with the `enhancement` label
3. Describe the feature and its use case
4. Explain why it would benefit users

### Contributing Code

#### Branch Workflow

**All contributions must be made in separate branches, not directly to `main`.**

1. **Fork the repository** to your GitHub account

2. **Clone your fork**
   ```bash
   git clone https://github.com/YOUR-USERNAME/miniVPN.git
   cd miniVPN
   ```

3. **Create a new branch** for your changes
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

   Branch naming conventions:
   - `feature/` - for new features
   - `fix/` - for bug fixes
   - `docs/` - for documentation changes
   - `refactor/` - for code refactoring

4. **Make your changes** and commit them
   ```bash
   git add .
   git commit -m "feat: Add your feature description"
   ```

   Commit message format:
   - `feat:` - new feature
   - `fix:` - bug fix
   - `docs:` - documentation changes
   - `refactor:` - code refactoring
   - `test:` - adding tests
   - `chore:` - maintenance tasks

5. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```

6. **Create a Pull Request**
   - Go to the original repository
   - Click "New Pull Request"
   - Select your branch
   - Fill in the PR template with details about your changes

#### Pull Request Guidelines

- Keep PRs focused on a single feature or fix
- Update documentation if needed
- Ensure code compiles without errors
- Test your changes on Windows 10/11
- Respond to review feedback promptly

### Development Setup

#### Prerequisites

- Go 1.21+
- Node.js 18+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- wintun.dll from https://www.wintun.net/

#### Building

```bash
cd build
wails build
```

#### Running in Development Mode

```bash
cd build
wails dev
```

### Code Style

- Follow standard Go formatting (`go fmt`)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions focused and concise

### Testing

- Test VPN connections in both server and client modes
- Verify split tunneling functionality
- Test on different Windows versions if possible
- Run from command prompt to check debug logs

## Questions?

If you have questions about contributing, feel free to open an issue with the `question` label.

## License

By contributing to miniVPN, you agree that your contributions will be licensed under the MIT License.
