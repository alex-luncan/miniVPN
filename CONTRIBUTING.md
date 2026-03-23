# Contributing to miniVPN

## Repository Rules

### Commit Author Policy

**IMPORTANT**: All commits to this repository MUST be authored by `alex-luncan` only.

- **Author Name**: `alex-luncan`
- **Author Email**: `alex@luncan.dev`
- **No Co-Authors**: Do NOT include `Co-Authored-By` lines in commit messages
- **No AI Attribution**: Do NOT include Claude, GitHub Copilot, or any other AI assistant attribution

### Git Configuration

Before committing, ensure your git config is set correctly:

```bash
git config user.name "alex-luncan"
git config user.email "alex@luncan.dev"
```

### Commit Message Format

```
Short description of the change

Optional longer description if needed.
```

Do NOT include:
- `Co-Authored-By:` lines
- AI assistant mentions
- External contributor names

### Contributors Policy

The only contributor to this repository is **alex-luncan**. This is a solo project and must remain attributed solely to the owner.

### Files to Exclude

The following must NEVER be committed:
- `.claude/` directory and any Claude-related files
- IDE configuration files (`.vscode/`, `.idea/`)
- Build artifacts (`*.exe`, `node_modules/`, `dist/`)
- Environment files (`.env`, credentials)

### Pull Requests

This repository does not accept external pull requests. All changes are made directly by the repository owner.
