# Changelog

## [v0.9.0] — 2026-02-28

### Added
- **Plan Mode** — Multi-step implementation planning with model optimization
  - `/plan <goal>` creates a structured implementation plan using the best reasoning model
  - `/plan next` executes steps one at a time with a cheaper/faster execution model
  - `/plan run` executes all remaining steps automatically
  - `/plan retry` retries failed steps
  - `/plan status` shows plan progress
  - `/plan reset` clears the current plan
  - `--plan "goal"` CLI flag for non-interactive plan creation
- `plan_model` config option — model for plan generation (defaults to `grok-4` for xAI)
- `exec_model` config option — model for plan step execution (defaults to main `model`)
- Plan saved as `plan.md` (human-readable) and `.aicli/plan.json` (machine-readable)
- Per-step model tier recommendations (premium/standard/economy) from the planning AI
- Project analysis with automatic key file detection for planning context

## [v0.8.0] — 2026-02-28

### Added
- `preload_model` config option (`*bool`, tri-state) to control Ollama model preloading
- Auto-detection of Ollama vs cloud API endpoints (xAI, OpenAI, Anthropic, Mistral, Groq, Together, Fireworks, Perplexity, DeepSeek, Google)
- `config.example.json` with example cloud API configuration

### Fixed
- Nil pointer panic in `NewNonInteractive()` — keylistener was not initialized
- Nil receiver panics on all `Listener` methods (Start, Stop, Events, GetBufferedInput, ClearBuffer, IsActive)
- Ollama-native API calls (`/api/ps`, `/api/generate`, `/api/chat`) no longer fire against cloud endpoints
- Ollama auto-discovery (mDNS) and auto-config skipped for cloud API endpoints
- `tool_permissions` config now correctly expects `map[string]string` with values "always", "ask", "never"

## [v0.7.0] — 2026-01-20

### Added
- Native Ollama API for vision models with images

## [v0.6.9] — 2026-01-20

### Added
- Image support for vision models like llava

## [v0.6.8] — 2026-01-20

### Fixed
- Prevent reading binary files that corrupt model context

## [v0.6.7] — 2026-01-20

### Fixed
- Panic: close of closed channel in keylistener

## [v0.6.6] — 2026-01-20

### Added
- Automatic update check on startup

## [v0.6.5] — 2026-01-20

### Fixed
- TLS endpoint to use hostname instead of IP

## [v0.6.4] — 2026-01-20

### Fixed
- macOS dns-sd hostname resolution fallback
- Prioritize network discovery over localhost

### Added
- Debug flag for discovery troubleshooting

## [v0.6.3] — 2026-01-20

### Fixed
- macOS dns-sd discovery parsing bug

## [v0.6.2] — 2026-01-20

### Added
- macOS dns-sd discovery fallback
- Improved terminal handling

## [v0.6.1] — 2025-12-27

### Fixed
- Version bump alignment

## [v0.6.0] — 2025-12-27

### Added
- Escape key to interrupt AI streaming and command execution

## [v0.5.5] — 2025-12-27

### Fixed
- Certificate error detection to include x509 errors

## [v0.5.4] — 2025-12-27

### Fixed
- avahi-browse stderr handling, suppress mDNS debug logs

## [v0.5.3] — 2025-12-27

### Added
- Avahi fallback for mDNS discovery
- Auto-detect self-signed certificates

## [v0.5.2] — 2025-12-26

### Fixed
- `--update` now runs before Ollama discovery

## [v0.5.1] — 2025-12-26

### Fixed
- mDNS discovery to only return verified Ollama endpoints
- Install script to use existing PATH directories

## [v0.5.0] — 2025-12-26

### Added
- mDNS auto-discovery for Ollama instances on the network
- TLS support for remote Ollama connections
- Self-update feature

## [v0.4.0] — 2025-12-26

### Added
- Proactive detection for models without native tool support
- Auto-continue, session resume, and version display
- Model loading check on startup with status messages
- Persistent todo, changelog, and history tracking

### Fixed
- Tool result handling for models without native tool support
- `list_files` to show all files when pattern is `*` or `.`
- Auto-config to never switch away from a valid configured model

## [v0.3.0] — 2025-12-23

### Added
- GitHub Actions workflow for automated releases

## [v0.2.0] — 2025-12-23

### Added
- Non-interactive mode with tool support
- Model listing API and auto-configuration
- Claude-like tool permission system (y/n/a/!)
- Language-specific error handling and planning phase
- Todo stack for error recovery
- User interrupt injection on command failures
- Real-time streaming of command output
- RPM package support

### Fixed
- Tool call parsing for qwen models
- Nil pointer crash in non-interactive mode
- Model auto-configuration persistence

## [v0.1.0] — 2025-12-08

### Added
- Initial release: AI coding assistant CLI
- Cross-platform builds and installers
