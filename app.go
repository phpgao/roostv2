package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ============================================================================
// Messages
// ============================================================================

type splashDoneMsg struct{}

type scanDoneMsg struct {
	projects []Project
}

type spinnerTickMsg struct{}

// RefreshTickMsg 1 分钟自动刷新。
type RefreshTickMsg struct{}

// ResumeSessionMsg 由 SessionScreen 发送，触发 resume 并退出 TUI。
type ResumeSessionMsg struct {
	Session Session
}

// NewSessionMsg 由 AgentScreen 发送，触发新建 session 并退出 TUI。
type NewSessionMsg struct {
	Platform    Platform
	ProjectPath string
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// cursorVisible 光标脉冲动画状态（包级共享）。
var cursorVisible = true

// ============================================================================
// Phase
// ============================================================================

type Phase int

const (
	PhaseSplash Phase = iota
	PhaseScanning
	PhaseActive
)

// ============================================================================
// App
// ============================================================================

type App struct {
	scanners []Scanner
	cfg      AppConfig

	phase  Phase
	width  int
	height int

	projects []Project

	err    error
	errOut string

	spinnerIdx int

	contextStack []Screen

	splashVersion string

	// Resume/New session — set by screens, checked by main.go after TUI exits
	ResumeSession *Session
	NewSession    struct {
		Platform    Platform
		ProjectPath string
		Pending     bool
	}

	// lastProjectPath 记录删除前所在的 project，用于删除后回到 session 列表
	lastProjectPath string
}

func NewApp(scanners []Scanner, cfg AppConfig, version string) *App {
	return &App{
		scanners:      scanners,
		cfg:           cfg,
		phase:         PhaseSplash,
		width:         80,
		height:        24,
		splashVersion: version,
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case splashDoneMsg:
		a.phase = PhaseScanning
		return a, tea.Batch(a.scanCmd(), a.spinnerTick())

	case spinnerTickMsg:
		if a.phase == PhaseScanning {
			a.spinnerIdx = (a.spinnerIdx + 1) % len(spinnerFrames)
			return a, a.spinnerTick()
		}
		return a, nil

	case scanDoneMsg:
		a.projects = msg.projects
		if a.phase == PhaseSplash || a.phase == PhaseScanning || a.contextStack == nil {
			a.phase = PhaseActive
			cursorVisible = true
			a.contextStack = []Screen{NewProjectScreen(a.projects, a.scanners)}

			// 如果有 lastProjectPath，且该 project 仍然存在且有 session，进入 session 页面
			if a.lastProjectPath != "" {
				for i := range a.projects {
					if a.projects[i].FullPath == a.lastProjectPath && len(a.projects[i].Sessions) > 0 {
						installed := collectPlatforms(a.projects)
						proj := a.projects[i]
						sessions := make([]Session, len(proj.Sessions))
						copy(sessions, proj.Sessions)
						sort.Slice(sessions, func(i, j int) bool {
							return sessions[i].LastActive.After(sessions[j].LastActive)
						})
						a.contextStack = append(a.contextStack,
							NewSessionScreen(&proj, sessions, installed, a.scanners))
						break
					}
				}
				a.lastProjectPath = ""
			}
			return a, tea.Batch(CursorTick(), a.refreshTick())
		}
		return a, nil

	case CursorTickMsg:
		cursorVisible = !cursorVisible
		if a.phase == PhaseActive {
			return a, CursorTick()
		}
		return a, nil

	case RefreshTickMsg:
		if a.phase == PhaseActive {
			// 后台刷新，更新数据不打断用户
			return a, tea.Batch(a.scanCmd(), a.refreshTick())
		}
		return a, nil

	case DeleteDoneMsg:
		// 记住当前所在的 project 路径（从 context stack 中找到 SessionScreen）
		a.lastProjectPath = ""
		for i := len(a.contextStack) - 1; i >= 0; i-- {
			if ss, ok := a.contextStack[i].(*SessionScreen); ok {
				a.lastProjectPath = ss.project.FullPath
				break
			}
		}
		// pop + 重建 Context，确保数据同步
		a.contextStack = nil
		return a, a.deleteCmd(msg)

	case ResumeSessionMsg:
		a.ResumeSession = &msg.Session
		return a, tea.Quit

	case NewSessionMsg:
		a.NewSession.Platform = msg.Platform
		a.NewSession.ProjectPath = msg.ProjectPath
		a.NewSession.Pending = true
		return a, tea.Quit

	case *ContextCmd:
		if msg.Push != nil {
			msg.Push.OnEnter()
			a.contextStack = append(a.contextStack, msg.Push)
		}
		if msg.Pop && len(a.contextStack) > 1 {
			top := a.contextStack[len(a.contextStack)-1]
			top.OnExit()
			a.contextStack = a.contextStack[:len(a.contextStack)-1]
		}
		if msg.Cmd != nil {
			return a, msg.Cmd
		}
		return a, nil

	case tea.KeyPressMsg:
		return a.handleKey(msg)
	}

	return a, nil
}

func (a *App) View() tea.View {
	var s string
	switch a.phase {
	case PhaseSplash:
		s = a.renderSplash()
	case PhaseScanning:
		s = a.renderScanning()
	case PhaseActive:
		s = a.renderActive()
	}
	return tea.NewView(s)
}

func (a *App) scanCmd() tea.Cmd {
	scanners := a.scanners
	return func() tea.Msg {
		projects := ScanProjectsParallel(scanners)
		return scanDoneMsg{projects: projects}
	}
}

func (a *App) deleteCmd(msg DeleteDoneMsg) tea.Cmd {
	scanners := a.scanners
	return func() tea.Msg {
		switch msg.Target {
		case deleteTargetSession:
			if msg.Session != nil {
				for _, sc := range scanners {
					if sc.Platform() == msg.Session.Platform {
						_ = sc.DeleteSession(*msg.Session)
						break
					}
				}
			}
			// 批量 session 删除
			for _, sess := range msg.Sessions {
				for _, sc := range scanners {
					if sc.Platform() == sess.Platform {
						_ = sc.DeleteSession(sess)
						break
					}
				}
			}
		case deleteTargetProject:
			if msg.Project != nil {
				for _, sc := range scanners {
					if len(msg.Project.Sessions) > 0 && sc.Platform() == msg.Project.Sessions[0].Platform {
						_ = sc.DeleteProject(*msg.Project)
					}
				}
			}
		case deleteTargetBatch:
			for _, proj := range msg.Batches {
				// 按平台分组，每个 Scanner 只删自己的会话
				byPlatform := map[Platform][]Session{}
				for _, s := range proj.Sessions {
					byPlatform[s.Platform] = append(byPlatform[s.Platform], s)
				}
				for _, sc := range scanners {
					if sessions, ok := byPlatform[sc.Platform()]; ok {
						_ = sc.DeleteProject(Project{
							Name:     proj.Name,
							FullPath: proj.FullPath,
							Sessions: sessions,
						})
					}
				}
			}
		}
		return scanDoneMsg{projects: ScanProjectsParallel(scanners)}
	}
}

func (a *App) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func (a *App) refreshTick() tea.Cmd {
	return tea.Tick(1*time.Minute, func(t time.Time) tea.Msg {
		return RefreshTickMsg{}
	})
}

func (a *App) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if a.err != nil {
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "esc", "enter":
			a.err = nil
			a.errOut = ""
			return a, nil
		}
		return a, nil
	}

	if len(a.contextStack) == 0 {
		return a, nil
	}
	top := a.contextStack[len(a.contextStack)-1]
	cmd := top.HandleKey(msg)
	if cmd != nil {
		return a, cmd
	}
	return a, nil
}

func (a *App) renderSplash() string {
	return RenderLogo(a.width, a.height, a.splashVersion)
}

func (a *App) renderScanning() string {
	var sb strings.Builder

	frame := spinnerFrames[a.spinnerIdx]
	line := fmt.Sprintf("  %s  scanning sessions…", StyleFaint.Render(frame))

	sb.WriteString(line)

	return sb.String()
}

func (a *App) renderActive() string {
	if top := a.topContext(); top != nil {
		return top.View(a.width, a.height)
	}
	return ""
}

func (a *App) topContext() Screen {
	if len(a.contextStack) == 0 {
		return nil
	}
	return a.contextStack[len(a.contextStack)-1]
}

func (a *App) PopContext() {
	if len(a.contextStack) > 1 {
		a.topContext().OnExit()
		a.contextStack = a.contextStack[:len(a.contextStack)-1]
	}
}

func (a *App) PushContext(ctx Screen) {
	ctx.OnEnter()
	a.contextStack = append(a.contextStack, ctx)
}
