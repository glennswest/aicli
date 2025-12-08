# aicli

A command-line AI coding assistant with tool execution capabilities. Works with any OpenAI-compatible API.

## Features

- Interactive chat with AI models
- Tool execution: shell commands, file operations, git, web search
- Session recording and playback
- Auto-versioning on commits (semver)
- Piped input support for scripting

## Installation

```bash
go build -o aicli .
```

On macOS, you may need to sign the binary:
```bash
codesign --force --sign - ./aicli
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
