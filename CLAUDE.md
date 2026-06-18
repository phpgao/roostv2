# CLAUDE.md

Roost — a terminal TUI session manager for AI coding assistants.

## Project Overview

Roost scans `~/.codebuddy`, `~/.claude`, `~/.gemini`, `~/.codex`, `~/.copilot`, and OpenCode's internal DB for session history. It presents the results in a keyboard-driven terminal UI built with bubbletea v2 + lipgloss v2. A `list`/`resume`/`delete` CLI mode is available for scripting.

## Architecture

### Flat package main

The entire codebase is a single Go package (`package main`). There are no sub-packages. All types, interfaces, and logic live in the same namespace. This keeps the codebase simple — there are only ~3000 lines of Go across 16 files.

### Core abstractions

**`Screen` interface** (`screen.go`) — each interactive view implements `Name()`, `HandleKey(tea.KeyPressMsg) tea.Cmd`, `View(width, height int) string`, `OnEnter()`, `OnExit()`. Screens are managed on a stack inside `App.contextStack`.

**`Scanner` interface** (`scanner.go`) — each AI platform has a scanner that implements `Platform()`, `DataDir()`, `ScanProjects()`, `DeleteSession()`, `DeleteProject()`, `ResumeCmd()`, `DisplayName()`, `SetCustomName()`. Scanners are the only code that touches platform-specific data formats.

**`App` model** (`app.go`) — the bubbletea Model. Manages phases (Splash → Scanning → Active), the screen stack, keyboard dispatch, and background refresh ticks. `Update()` delegates key events to the top screen via `HandleKey()`.

### Data flow

1. `main.go` detects scanners (built-in + custom agents from config), then starts the TUI or CLI path
2. `App.Init()` triggers a 2-second splash, then launches parallel scan via `scanCmd()` → `ScanProjectsParallel()`
3. `ScanProjectsParallel()` fans out goroutines per scanner, each calling `scanner.ScanProjects()` which walks the filesystem / SQLite
4. Results are merged by `MergeProjects()` — combines same-path projects from different scanners, sorts by last activity
5. After scan, the project screen is pushed onto the context stack, and the 1-minute refresh tick starts
6. Screens communicate state changes (resume, new session, delete) via bubbletea messages back to `App`

### Context / Screen Stack

`App.contextStack []Screen` is a manual stack — not bubbletea native. Screens return `*ContextCmd` from `HandleKey()` to request push/pop operations. `ContextCmd` wraps a bubbletea `tea.Cmd` plus optional `Push` or `Pop` flags. `App.Update()` processes these by adjusting the stack before delegating the inner Cmd.

### Phase lifecycle

```
PhaseSplash (2s timer)
  → splashDoneMsg
  → PhaseScanning (spinner)
    → scanDoneMsg
    → PhaseActive (screen stack)
```

## Key Files

| File | Role |
|------|------|
| `main.go` | Entry point, CLI arg dispatch, TUI bootstrap, scanner detection, session exec via `syscall.Exec` |
| `app.go` | Bubbletea App model, phases, screen stack management, keyboard dispatch |
| `screen.go` | `Screen` interface, `ContextCmd` type, push/pop helpers |
| `screen_project.go` | Project list — filtering, bulk delete, search |
| `screen_session.go` | Session list, session detail, agent picker (new session) |
| `screen_confirm.go` | Delete confirmation dialog |
| `scanner.go` | `Scanner` interface, data models (`Session`, `Project`), parallel scan, merge, utilities |
| `scanner_claude.go` | Claude session scanner (JSONL files) |
| `scanner_codebuddy.go` | CodeBuddy session scanner (SQLite) |
| `scanner_gemini.go` | Gemini session scanner |
| `scanner_codex.go` | Codex session scanner |
| `scanner_copilot.go` | Copilot session scanner |
| `scanner_opencode.go` | OpenCode session scanner (SQLite) |
| `config.go` | YAML config loading from `~/.roost/roost.yaml` |
| `cmd_list.go` | CLI `list`/`resume`/`delete` implementation with table/JSON/YAML output |
| `style.go` | Colors (cyberpunk dark theme), lipgloss styles, splash logo, cursor animation |

## How to Run

```sh
# Run directly (reads ~/.codebuddy etc. for session data)
go run .

# Build
go build -o roost .

# Vet
go vet ./...

# Test
go test ./...
```

## Testing

There are no tests yet. The CI workflow runs `go vet` and `golangci-lint`, but `go test` currently fails due to a duplicate function declaration (`TestRelativeTime` in both `scanner_test.go` and `config_test.go`). Fix that before adding new tests.

## Style and Conventions

- **Flat package main** — no sub-packages. All types coexist in one namespace.
- **lipgloss v2** for all TUI styling — colors, borders, text formatting. Styles are defined as package-level `var`s in `style.go`.
- **bubbletea v2** for state management — the `tea.Model` interface, commands via `tea.Cmd`, messages as typed structs.
- **Bubbletea v2 APIs** — uses `tea.NewProgram(m)`, `tea.NewView(s)`, `tea.KeyPressMsg`, `tea.WindowSizeMsg`, `tea.Tick()`.
- **lipgloss v2 APIs** — uses `lipgloss.NewStyle()`, `lipgloss.Color()`, `lipgloss.Width()`, `lipgloss.DoubleBorder()`.
- **Screen pattern** — each view is a struct implementing `Screen`. State is local. Navigation is push/pop via `ContextCmd`.
- **Parallel scan** — `ScanProjectsParallel` uses `sync.WaitGroup` + `sync.Mutex` to fan out scanner goroutines. Results merged in deterministic order.
- **`syscall.Exec` for resume** — resumes the target AI binary via process replacement, not subprocess. The `suspend` mode (not yet fully plumbed) would use subprocess instead.
- **1-minute auto-refresh** — `RefreshTickMsg` triggers a background scan that updates project data silently.

## Common Patterns

### Adding a new screen

1. Create a struct implementing `Screen` in a new or existing `screen_*.go` file
2. Implement `Name()`, `HandleKey()`, `View()`, `OnEnter()`, `OnExit()`
3. For navigation: return `PushCmd(newScreen)` to push, `PopCmd()` to pop
4. For bubbletea commands (animations, async work): return `NewContextCmd(cmd)`
5. Wire it from an existing screen's `HandleKey()`

### Adding a new scanner

1. Create `scanner_newplatform.go` implementing `Scanner` interface
2. Register it in `main.go`'s `detectScanners()`:
   - Add a `PlatformXxx` constant in `scanner.go`
   - Add the constructor to `newScannerFor` map
   - Add default bin/dir constants
   - Add to `AllPlatforms()`, `platformKey()`, `parseType()`
3. Add to `config.go` default template and `platformKey()` switch

### Screens that need async data

Return a bubbletea `tea.Cmd` from `HandleKey()` that performs the work and sends back a typed message. `App.Update()` catches the message and pushes the screen with populated data.
