# aicli

A command-line AI coding assistant with tool execution capabilities. Works with any OpenAI-compatible API, including Ollama, Hugging Face, and cloud providers.

## Features

### Core Capabilities
- **Interactive chat** with AI models for code assistance, debugging, and explanations
- **Tool execution** - The AI can run shell commands, modify files, perform git operations, search the web, and more
- **Session recording** - All conversations are automatically saved for playback and resume
- **Session resume** - Detects incomplete sessions and offers to continue where you left off
- **Persistent todos** - Track tasks across sessions with automatic detection on startup
- **Auto-versioning** - Semantic version bumping on each git commit (x.y.z format)
- **Piped input** - Process files and logs through AI for scripting automation
- **Auto-continue** - Detects when the AI describes an action without executing it and prompts to continue
- **Smart error handling** - Language-specific error detection with suggested fixes

### Intelligent Automation
- **Model auto-configuration** - Automatically detects and configures available models
- **Model loading check** - Verifies model is loaded on startup, loads if needed (24h keep-alive)
- **Tool permissions** - Granular control over tool execution (always/ask/never per tool)
- **Changelog tracking** - Automatic logging of file changes and commits
- **Project history** - Complete activity log of requests, todos, changes, and commits

### Network & Security
- **mDNS discovery** - Automatically discovers Ollama instances on your local network
- **TLS/HTTPS support** - Secure encrypted connections to remote Ollama servers
- **Encryption warnings** - Warns when using unencrypted HTTP connections to remote hosts
- **Self-update** - Check for and install updates directly from GitHub releases

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

### Configuration Files

aicli uses a two-tier configuration system:

1. **Local config** (`.aicli/config.json` in project directory) - Project-specific settings
2. **Global config** (`~/.config/aicli/config.json`) - Default settings for all projects

Local config takes precedence over global config.

### Basic Configuration

Create or initialize configuration:
```bash
./aicli --init
```

This creates `~/.config/aicli/config.json` with defaults:

```json
{
  "api_endpoint": "http://localhost:11434/v1",
  "api_key": "",
  "model": "default",
  "max_tokens": 4096,
  "temperature": 0.3,
  "system_prompt": "...",
  "tool_permissions": {},
  "user_interrupts": false
}
```

### Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `api_endpoint` | OpenAI-compatible API URL | `http://localhost:11434/v1` |
| `api_key` | API key (if required) | `""` |
| `model` | Model name or "default" for auto-detect | `"default"` |
| `max_tokens` | Maximum tokens in response | `4096` |
| `temperature` | Creativity (0.0-2.0, lower = more focused) | `0.3` |
| `system_prompt` | Custom system prompt for the AI | (built-in coding assistant prompt) |
| `tool_permissions` | Per-tool permission settings | `{}` |
| `user_interrupts` | Enable user interrupts for weaker models | `false` |

### Example Configurations

**For Ollama (local):**
```json
{
  "api_endpoint": "http://localhost:11434/v1",
  "model": "qwen2.5:72b"
}
```

**For OpenAI:**
```json
{
  "api_endpoint": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "model": "gpt-4o"
}
```

**For Hugging Face Inference API:**
```json
{
  "api_endpoint": "https://api-inference.huggingface.co/v1",
  "api_key": "hf_...",
  "model": "meta-llama/Llama-3.1-70B-Instruct"
}
```

## Usage

### Interactive Mode

```bash
./aicli
```

Starts an interactive chat session. The AI has access to all tools and can execute actions in your project.

### Single Prompt

```bash
./aicli -p "explain this code" main.go
```

Run a single prompt with file context, then exit.

### Piped Input

```bash
cat error.log | ./aicli
git diff | ./aicli -p "review these changes"
```

Process piped content through the AI.

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
| `-v, --version` | Show aicli and project version |
| `--sessions` | List recorded sessions |
| `--playback` | Replay a session file |
| `--auto` | Auto-execute mode (skip confirmations) |
| `--insecure` | Skip TLS certificate verification |
| `--update` | Check for updates and install if available |

### Chat Commands

| Command | Description |
|---------|-------------|
| `/help`, `/h` | Show help |
| `/quit`, `/q` | Exit |
| `/clear`, `/new` | Clear conversation history |
| `/file <path>` | Add file as context |
| `/files <paths>` | Add multiple files |
| `/cd <dir>` | Change working directory |
| `/run <cmd>` | Execute shell command directly |
| `/git <cmd>` | Git operations (status, diff, log, add, commit) |
| `/version`, `/v` | Show version |
| `/auto` | Toggle auto-execute mode |
| `/search <query>` | Web search (DuckDuckGo) |
| `/screenshot` | Capture screenshot |
| `/sessions` | List sessions |
| `/playback <file>` | Replay session |
| `/config` | Show config |
| `/models` | List available models |
| `/model [name]` | Show or switch model |
| `/permissions` | View/manage tool permissions |
| `/todos` | View/manage persistent todos |
| `/changelog` | View/add changelog entries |
| `/history [n]` | View recent project history |

## AI Tools

The AI has access to these tools for autonomous operation:

### File Operations
| Tool | Description |
|------|-------------|
| `read_file` | Read file contents |
| `write_file` | Create or overwrite files (source code, config, etc.) |
| `write_doc` | Write documentation files (README, guides, etc.) |
| `list_files` | List source files in the project |

### Shell Execution
| Tool | Description |
|------|-------------|
| `run_command` | Execute shell commands (builds, tests, installs) |

### Git Operations
| Tool | Description |
|------|-------------|
| `git_status` | Show repository status |
| `git_diff` | Show changes (staged or unstaged) |
| `git_add` | Stage files for commit |
| `git_commit` | Create commit with auto-version bump |
| `git_log` | Show recent commits |

### Web Access
| Tool | Description |
|------|-------------|
| `web_search` | Search the web via DuckDuckGo |
| `fetch_url` | Fetch and parse web page content |

### System
| Tool | Description |
|------|-------------|
| `screenshot` | Capture screen or window |
| `get_version` | Get current project version |
| `set_version` | Set project version manually |

### Tool Permissions

Control which tools require confirmation:

```bash
# View current permissions
/permissions

# Set tool to always allow
/permissions set run_command always

# Set tool to never allow
/permissions set git_commit never

# Reset to default (ask)
/permissions reset run_command
```

During execution, you'll be prompted:
- `(y)es` - Approve once
- `(n)o` - Decline once
- `(a)lways` - Always allow this tool
- `(!)` - Never allow this tool

## AI Model Support

### Tested Models

aicli has been tested with various AI models. Here's our compatibility matrix:

#### Ollama Models (Local)

| Model | Tool Calling | Quality | Notes |
|-------|-------------|---------|-------|
| `qwen2.5:72b` | Excellent | Excellent | Best overall performance, follows instructions well |
| `qwen2.5:32b` | Good | Good | Good balance of speed and quality |
| `qwen2.5-coder:32b` | Excellent | Excellent | Best for code-focused tasks |
| `llama3.1:70b` | Good | Excellent | Strong reasoning, good tool use |
| `llama3.1:8b` | Fair | Good | Fast but may need prompting for tools |
| `codellama:34b` | Good | Good | Specialized for code generation |
| `deepseek-coder:33b` | Good | Excellent | Strong code understanding |
| `mixtral:8x7b` | Fair | Good | Good for varied tasks |

#### Cloud API Models

| Provider | Model | Tool Calling | Notes |
|----------|-------|-------------|-------|
| OpenAI | `gpt-4o` | Excellent | Best cloud option |
| OpenAI | `gpt-4-turbo` | Excellent | Strong tool use |
| OpenAI | `gpt-3.5-turbo` | Good | Cost-effective |
| Anthropic* | Claude 3 | N/A | Requires adapter |
| Together AI | Llama/Qwen | Good | Various model options |

*Anthropic's API uses a different format; use an OpenAI-compatible adapter.

### Model Selection Tips

1. **For best results**: Use `qwen2.5:72b` or `qwen2.5-coder:32b` locally
2. **For speed**: Use smaller models (8B-14B) for simple tasks
3. **For code generation**: Use coder-specific models
4. **For complex reasoning**: Use larger models (70B+)

## Ollama Integration

aicli is optimized for [Ollama](https://ollama.ai/), a local AI model server.

### Setup

1. Install Ollama:
```bash
curl -fsSL https://ollama.com/install.sh | sh
```

2. Pull a model:
```bash
ollama pull qwen2.5:72b
```

3. Run aicli (auto-configures to Ollama):
```bash
./aicli
```

### Features

- **Auto-detection**: Automatically uses `http://localhost:11434/v1` endpoint
- **Model discovery**: Lists available models via `/models` command
- **Running model preference**: Auto-configures to already-loaded models
- **Model loading**: Loads models on startup with 24h keep-alive
- **Status display**: Shows model loading progress

### Model Management

```bash
# List available models
/models

# Switch models
/model qwen2.5:32b

# Check current model
/model
```

## Hugging Face Integration

aicli works with Hugging Face's inference endpoints.

### Hugging Face Inference API

Use HF's serverless API:

```json
{
  "api_endpoint": "https://api-inference.huggingface.co/v1",
  "api_key": "hf_...",
  "model": "meta-llama/Llama-3.1-70B-Instruct"
}
```

Get your API key at: https://huggingface.co/settings/tokens

### Hugging Face Inference Endpoints

For dedicated inference:

```json
{
  "api_endpoint": "https://your-endpoint.endpoints.huggingface.cloud/v1",
  "api_key": "hf_...",
  "model": "tgi"
}
```

### Text Generation Inference (TGI)

Run TGI locally for HF models:

```bash
# Start TGI server
docker run --gpus all -p 8080:80 \
  ghcr.io/huggingface/text-generation-inference:latest \
  --model-id meta-llama/Llama-3.1-8B-Instruct

# Configure aicli
{
  "api_endpoint": "http://localhost:8080/v1",
  "model": "tgi"
}
```

## Sessions

### Automatic Recording

All chat sessions are automatically recorded to `.aicli/` in your project directory:
```
.aicli/
├── session_20241215_103000.json
├── session_20241215_140522.json
├── debug/              # Request/response logs
├── config.json         # Local project config
└── ...
```

### Session Resume

On startup, aicli detects incomplete sessions and offers to resume:
```
>>> Previous session appears incomplete
    Last session: session_20241215_140522.json

Should I continue where we stopped? (y/n): y
✓ Restored 15 conversation entries
```

### Playback

Replay sessions for debugging or review:

```bash
# List sessions
./aicli --sessions

# Replay a session
./aicli --playback session_20241215_140522.json
```

## Project Files

aicli creates these files in your project root:

| File | Description |
|------|-------------|
| `VERSION` | Semantic version (x.y.z), auto-bumped on commits |
| `TODOS.md` | Persistent todo list, survives across sessions |
| `CHANGELOG.md` | Track of changes made during sessions |
| `HISTORY.md` | Complete activity log (requests, todos, changes, commits) |

### Version Management

- Version auto-bumps on each `git_commit` tool call
- Default: patch bump (0.0.1 → 0.0.2)
- Use `bump:"minor"` or `bump:"major"` for larger bumps
- Initialize with `./aicli --init`

## Network Discovery

aicli can automatically discover Ollama instances on your local network using mDNS (multicast DNS).

### How It Works

When no configuration file exists and no local Ollama is found at `localhost:11434`:

1. aicli broadcasts an mDNS query for `_ollama._tcp` services
2. Discovers Ollama servers advertising on the network
3. Prefers HTTPS endpoints over HTTP
4. Auto-configures and saves the discovered endpoint

```bash
$ aicli
🔍 No local Ollama found, searching network...
✓ Discovered Ollama at server.local 🔒
✓ Model qwen2.5:72b is ready
```

### Encryption Warnings

When connecting to remote Ollama servers over HTTP, aicli warns about unencrypted connections:

```
⚠ Warning: Connection is not encrypted (using HTTP)
  Data sent to http://192.168.1.100:11434/v1 may be visible on the network
```

Localhost connections are exempt from this warning.

### TLS Certificate Verification

For self-signed certificates, use the `--insecure` flag:

```bash
aicli --insecure -e "https://myserver:443/v1"
```

## Updates

aicli can check for and install updates directly from GitHub.

### Check for Updates

```bash
$ aicli --update
Checking for updates...

⬆ Update available!
  Current version: 0.4.0
  Latest version:  0.5.0
  Download size:   2.49 MB

Release notes:
  ## What's New
  - mDNS network discovery
  - TLS encryption support
  ...

Do you want to update? [y/N]: y

Downloading update... 100.0%
✓ Successfully updated to version 0.5.0
Please restart aicli to use the new version.
```

### Already Up to Date

```bash
$ aicli --update
Checking for updates...
✓ You are running the latest version (0.5.0)
```

## Troubleshooting

### Model Not Loading

If the model fails to load:
```
⏳ Loading model qwen2.5:72b (this may take a moment)...
✗ Failed to load model: ...
```

1. Ensure Ollama is running: `ollama serve`
2. Check model is pulled: `ollama list`
3. Verify endpoint in config: `/config`

### Tool Execution Errors

If commands fail:
1. Check working directory: `/cd`
2. Verify permissions: `/permissions`
3. Run manually: `/run <command>`

### API Connection Issues

1. Verify endpoint is correct
2. Check API key is valid
3. Test connection: `curl <endpoint>/models`

## License

MIT
