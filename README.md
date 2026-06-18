# roost

**A terminal TUI session manager for AI coding assistants.**

Roost scans your filesystem for AI coding sessions (CodeBuddy, Claude, Gemini, Codex, Copilot, OpenCode) and gives you a fast, keyboard-driven terminal UI to browse, search, resume, and delete them — all without leaving the shell.

## Features

- **Splash screen** with animated logo on startup
- **Project view** — merged list of all projects across platforms, sorted by last activity
- **Session list** — drill into a project to see all sessions with title, model, message count
- **Session detail** — view full metadata and copy-paste ready manual resume commands
- **Keyboard navigation** — vim-style bindings (`j`/`k`, `g`/`G`, `/` search)
- **Platform filter** — `Tab` cycles through detected platforms to narrow the list
- **Search** — `/` to start typing, real-time filtering by project path or session title
- **Batch delete** — `Space` to select multiple items, `D` to confirm deletion
- **New session** — pick a platform from the agent picker (`n` key) to start a fresh session
- **Custom agents** — configure alternative binaries and data directories for any platform
- **Responsive layout** — adapts to terminal width, caps at 140 columns for readability
- **1-minute auto-refresh** — keeps session data current without interrupting you
- **Cyberpunk dark theme** — OLED-friendly neon palette with per-platform colors
- **CLI mode** — `list`, `resume`, and `delete` subcommands for scripting

## Installation

### Homebrew

```sh
brew install phpgao/tap/roost
```

### Go Install

```sh
go install github.com/phpgao/roost@latest
```

Requires Go 1.26+.

## Usage

### Interactive TUI

```sh
roost
```

Starts the terminal UI. On first run, roost automatically detects installed AI platforms and scans for sessions.

### Keyboard Shortcuts

#### Global

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help |

#### Project List

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | Open project (session list) |
| `d` | Delete selected project (with confirmation) |
| `Space` | Toggle select for selected project |
| `D` | Batch delete selected projects (with confirmation) |
| `Tab` | Cycle platform filter (All → platforms in order) |
| `/` | Start search (filter by path) |
| `Esc` (×2) | Quit from project view |
| `r` | Manual refresh |

#### Session List

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `g` | Jump to top |
| `G` | Jump to bottom |
| `Enter` | Open session detail |
| `d` | Delete selected session (with confirmation) |
| `x` | Delete current project (with confirmation, returns to project list) |
| `Space` | Toggle select for selected session |
| `D` | Batch delete selected sessions (with confirmation) |
| `Tab` | Cycle platform filter |
| `/` | Start search (filter by title) |
| `n` | Open agent picker — start a new session |
| `Esc` | Go back to project list |

#### Session Detail

| Key | Action |
|-----|--------|
| `Enter` | Resume this session |
| `Esc` / `q` | Go back to session list |

#### Agent Picker (`n` from session list)

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Start a new session with selected agent |
| `Esc` | Cancel |

### CLI Subcommands

```sh
# List all projects (table format)
roost list

# List sessions in a project
roost list --type session --project /path/to/repo

# Output formats: table (default), json, yaml
roost list -o json
roost list --type session -p /path/to/repo -o yaml

# Resume a session by ID
roost resume -s <session-id>

# Delete a session by ID
roost delete -s <session-id>

# Show version
roost version
```

## Configuration

Configuration is read from `~/.roost/roost.yaml`. If the file doesn't exist, roost creates one with sensible defaults on first run.

### Full Configuration File

```yaml
# resume mode:
#   replace - process replacement, returns to shell after agent exits (default)
#   suspend - subprocess mode, returns to roost TUI after agent exits
resume_mode: replace

platforms:
  codebuddy:
    args: [-y]
  claude:
    args: [--dangerously-skip-permissions]
  gemini:
    args: [-y]
  codex:
    args: [--full-auto]
  copilot:
    args: []
  opencode:
    args: []

custom_agents:
  - name: My CodeBuddy Dev
    type: codebuddy
    bin: codebuddy-dev
    data_dir: ~/.codebuddy-dev
    args: [--model, gpt-4o]
```

### Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `resume_mode` | string | `replace` | `replace` exits to shell after agent exits; `suspend` returns to roost TUI |
| `platforms` | map | `{}` | Per-platform override map; each entry has `args` — extra CLI arguments appended when resuming |
| `custom_agents` | array | `[]` | Additional agent instances with custom binaries or data directories |

## Custom Agents

Custom agents let you use alternative binaries, data directories, or display names for any supported platform type. For example, running multiple CodeBuddy instances with different configurations, or using a development build of Claude via `claude-dev`.

### Example

```yaml
custom_agents:
  - name: CodeBuddy Production
    type: codebuddy
    bin: codebuddy

  - name: CodeBuddy Staging
    type: codebuddy
    bin: codebuddy-staging
    data_dir: ~/.codebuddy-staging
    args: [--config, staging.toml]

  - name: My Claude Dev
    type: claude
    bin: claude-dev
    args: [--no-permissions-check]
```

### Custom Agent Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Display name shown in the TUI and agent picker |
| `type` | Yes | Platform type — must be one of: `codebuddy`, `claude`, `gemini`, `codex`, `copilot`, `opencode` |
| `bin` | Yes | Executable name or path to the binary |
| `data_dir` | No | Override the default data directory for session scanning |
| `args` | No | Extra CLI arguments appended when starting new sessions |

### Supported Platform Types

| Type | Default Binary | Default Data Directory |
|------|---------------|----------------------|
| `codebuddy` | `codebuddy` | `.codebuddy` |
| `claude` | `claude` | `.claude` |
| `gemini` | `gemini` | `.gemini` |
| `codex` | `codex` | `.codex` |
| `copilot` | `copilot` | `.copilot` |
| `opencode` | `opencode` | (internal db) |

## Screenshots

<!-- TODO: add screenshots of the splash screen, project list, session list, session detail, and delete confirmation -->

## Contributing / Development

### Prerequisites

- Go 1.26+

### Build & Run

```sh
# Run directly
go run .

# Build binary
go build -o roost .

# Vet and test
go vet ./...
go test ./...
```

### Project Structure

```
.
├── main.go              # Entry point, CLI dispatch, TUI bootstrap
├── app.go               # Bubbletea App model, phases, screen stack
├── screen.go            # Screen interface and ContextCmd helpers
├── screen_project.go    # Project list screen
├── screen_session.go    # Session list, session detail, agent picker
├── screen_confirm.go    # Delete confirmation dialog
├── scanner.go           # Scanner interface, data models, parallel scan, utilities
├── scanner_codebuddy.go # CodeBuddy scanner
├── scanner_claude.go    # Claude scanner
├── scanner_gemini.go    # Gemini scanner
├── scanner_codex.go     # Codex scanner
├── scanner_copilot.go   # Copilot scanner
├── scanner_opencode.go  # OpenCode scanner
├── config.go            # YAML config loading and validation
├── cmd_list.go          # CLI list/resume/delete implementation
├── style.go             # Colors, styles, splash logo rendering
├── go.mod / go.sum      # Go module files
└── roost                # Built binary (gitignored)
```

## License

MIT
