# aicli

A command-line AI coding assistant with tool execution capabilities. Works with any OpenAI-compatible API.

## Features

- Interactive chat with AI models
- Tool execution: shell commands, file operations, git, web search
- Session recording and playback
- Auto-versioning on commits (semver)
- Piped input support for scripting

## Installation

### Pre-built Binaries

Download the latest release for your platform from the `dist/` folder:

| Platform | File | Notes |
|----------|------|-------|
| macOS ARM64 (Apple Silicon) | `aicli-darwin-arm64.zip` | M1/M2/M3 Macs |
| Linux AMD64 | `aicli-linux-amd64.tar.gz` | Elementary OS, Ubuntu, Debian, etc. |
| Linux ARM64 | `aicli-linux-arm64.tar.gz` | NVIDIA GB10 Grace, Raspberry Pi 4, etc. |
| RHEL / Fedora | `aicli-0.1.0-1.x86_64.rpm` | RHEL 8/9, Fedora 38+ |
| Windows AMD64 | `aicli-windows-amd64.zip` | Windows 10/11 64-bit |

### macOS (Apple Silicon)

```bash
unzip aicli-darwin-arm64.zip
sudo mv aicli /usr/local/bin/
codesign --force --sign - /usr/local/bin/aicli
```

### Linux (Elementary OS, Ubuntu, etc.)

```bash
tar xzf aicli-linux-amd64.tar.gz
sudo mv aicli /usr/local/bin/
```

### Linux ARM64 (NVIDIA GB10 Grace)

```bash
tar xzf aicli-linux-arm64.tar.gz
sudo mv aicli /usr/local/bin/
```

### RHEL / Fedora

```bash
sudo rpm -i aicli-0.1.0-1.x86_64.rpm
```

Or with dnf:
```bash
sudo dnf install ./aicli-0.1.0-1.x86_64.rpm
```

### Windows

1. Extract `aicli-windows-amd64.zip`
2. Move `aicli.exe` to a folder in your PATH, or run the installer:

```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

### Using Install Scripts

**Linux/macOS:**
```bash
./install.sh
```

**Windows (PowerShell as Admin):**
```powershell
powershell -ExecutionPolicy Bypass -File install.ps1
```

### Build from Source

Requires Go 1.24+:

```bash
# Build for current platform
go build -o aicli .

# Build all platforms
make all

# Build specific platform
make darwin-arm64
make linux-amd64
make linux-arm64
make windows-amd64
```

## Configuration

Create `~/.config/aicli/config.json`:

```json
{
  "api_endpoint": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "model": "gpt-4o",
  "max_tokens": 4096,
  "temperature": 0.7
}
```

Or initialize with defaults:
```bash
./aicli --init
```

## Usage

### Interactive Mode

```bash
./aicli
```

### Single Prompt

```bash
./aicli -p "explain this code" main.go
```

### Piped Input

```bash
cat error.log | ./aicli
```

### Command Line Options

| Flag | Description |
|------|-------------|
| `-e, --endpoint` | API endpoint URL |
| `-k, --key` | API key |
| `-m, --model` | Model name |
| `-p, --prompt` | Single prompt (non-interactive) |
| `-t, --temperature` | Temperature (0.0-2.0) |
| `--max-tokens` | Max response tokens |
| `--config` | Show configuration |
| `--init` | Initialize config and VERSION |
| `--version` | Show project version |
| `--sessions` | List recorded sessions |
| `--playback` | Replay a session file |
| `--auto` | Auto-execute mode (skip confirmations) |

### Chat Commands

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/quit` | Exit |
| `/clear` | Clear conversation |
| `/file <path>` | Add file as context |
| `/files <paths>` | Add multiple files |
| `/cd <dir>` | Change working directory |
| `/run <cmd>` | Execute shell command |
| `/git <cmd>` | Git operations |
| `/version` | Show version |
| `/auto` | Toggle auto-execute |
| `/search <query>` | Web search |
| `/screenshot` | Capture screenshot |
| `/sessions` | List sessions |
| `/playback <file>` | Replay session |
| `/config` | Show config |

## AI Tools

The AI can use these tools:

- `run_command` - Execute shell commands
- `write_file` - Create/modify files
- `write_doc` - Write documentation
- `read_file` - Read file contents
- `web_search` - Search the web
- `fetch_url` - Fetch web pages
- `screenshot` - Capture screenshots
- `git_status`, `git_diff`, `git_add`, `git_commit`, `git_log` - Git operations
- `list_files` - List project files
- `get_version`, `set_version` - Version management

## Sessions

All chat sessions are automatically recorded to `.aicli/` in the working directory. Replay with:

```bash
./aicli --playback session-2024-01-15-10-30-00.json
```

## Version Management

Projects use a `VERSION` file (semver format: x.y.z). Version auto-bumps on each commit:
- Default: patch bump (0.0.1 -> 0.0.2)
- Use `bump:"minor"` or `bump:"major"` in commit for larger bumps

## License

MIT
