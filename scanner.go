package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// ============================================================================
// Config
// ============================================================================

// Config is the config interface scanners need.
type Config interface {
	ArgsFor(p Platform) []string
}

// ScannerOverrides 自定义 agent 时覆盖默认 bin/data_dir。
type ScannerOverrides struct {
	Bin     string
	DataDir string
	Args    []string
}

// ============================================================================
// Platform
// ============================================================================

// Platform 标识一个 AI 平台。
type Platform int

const (
	PlatformCodeBuddy Platform = iota
	PlatformClaude
	PlatformGemini
	PlatformCodex
	PlatformCopilot
	PlatformOpenCode
)

const PlatformCount = 6

var (
	customPlatforms     []Platform
	customPlatformNames = map[Platform]string{}
)

// RegisterCustomPlatform registers a custom platform with a display name.
// Returns a new Platform ID that can be used like other platforms.
func RegisterCustomPlatform(name string) Platform {
	p := Platform(len(customPlatforms) + int(PlatformCount))
	customPlatforms = append(customPlatforms, p)
	customPlatformNames[p] = name
	return p
}

func (p Platform) String() string {
	if name, ok := customPlatformNames[p]; ok {
		return name
	}
	switch p {
	case PlatformCodeBuddy:
		return "CodeBuddy"
	case PlatformClaude:
		return "Claude"
	case PlatformGemini:
		return "Gemini"
	case PlatformCodex:
		return "Codex"
	case PlatformCopilot:
		return "Copilot"
	case PlatformOpenCode:
		return "OpenCode"
	default:
		return "Unknown"
	}
}

func (p Platform) Icon() string { return "●" }

func (p Platform) ShortName() string {
	switch p {
	case PlatformCodeBuddy:
		return "CB"
	case PlatformClaude:
		return "CL"
	case PlatformGemini:
		return "GE"
	case PlatformCodex:
		return "CX"
	case PlatformCopilot:
		return "Co"
	case PlatformOpenCode:
		return "OC"
	default:
		return "??"
	}
}

// AllPlatforms 返回所有已知平台，按名称排序。
func AllPlatforms() []Platform {
	base := []Platform{
		PlatformCodeBuddy,
		PlatformClaude,
		PlatformGemini,
		PlatformCodex,
		PlatformCopilot,
		PlatformOpenCode,
	}
	return append(base, customPlatforms...)
}

// ============================================================================
// Default Binary & Data Directory Names
// ============================================================================

const (
	DefaultBinCodeBuddy = "codebuddy"
	DefaultBinClaude    = "claude"
	DefaultBinGemini    = "gemini"
	DefaultBinCodex     = "codex"
	DefaultBinCopilot   = "copilot"
	DefaultBinOpenCode  = "opencode"

	DefaultDirCodeBuddy = ".codebuddy"
	DefaultDirClaude    = ".claude"
	DefaultDirGemini    = ".gemini"
	DefaultDirCodex     = ".codex"
	DefaultDirCopilot   = ".copilot"
)

// DefaultBinFor 返回平台默认的二进制名称。
func DefaultBinFor(p Platform) string {
	switch p {
	case PlatformCodeBuddy:
		return DefaultBinCodeBuddy
	case PlatformClaude:
		return DefaultBinClaude
	case PlatformGemini:
		return DefaultBinGemini
	case PlatformCodex:
		return DefaultBinCodex
	case PlatformCopilot:
		return DefaultBinCopilot
	case PlatformOpenCode:
		return DefaultBinOpenCode
	default:
		return ""
	}
}

// ============================================================================
// Session & Project
// ============================================================================

// Session 代表单条 AI 会话记录。
type Session struct {
	ID          string
	Platform    Platform
	AgentType   string
	Title       string
	Model       string
	LastActive  time.Time
	MsgCount    int
	SizeBytes   int64
	ProjectDir  string
	FilePath    string
	ResumeArg   string
	ProjectPath string
}

// Project 代表一个工作目录下的所有会话。
type Project struct {
	Name     string
	FullPath string
	Sessions []Session
}

// LastActive 返回项目内所有 session 中最新的活跃时间。
func (p *Project) LastActive() time.Time {
	var t time.Time
	for _, s := range p.Sessions {
		if s.LastActive.After(t) {
			t = s.LastActive
		}
	}
	return t
}

// ============================================================================
// Scanner Common Constants
// ============================================================================

const (
	roleUser      = "user"
	roleAssistant = "assistant"
	untitledTitle = "(untitled)"
	flagResume    = "--resume"

	// scanBufferSize is the max token size for bufio.Scanner (1 MB).
	scanBufferSize = 1024 * 1024
)

const (
	typeAgentCLI    = "cli"
	typeMessage     = "message" // CodeBuddy message type
	typeGemini      = "gemini"  // Gemini platform type
	typeSessionMeta = "session_meta"
	typeEventMsg    = "event_msg"
	typeUserMessage = "user_message"
	codexCmdResume  = "resume"
)

// ============================================================================
// Scanner Interface
// ============================================================================

// Scanner 定义各平台扫描器接口。
type Scanner interface {
	Platform() Platform
	DataDir() string
	ScanProjects() ([]Project, error)
	DeleteSession(s Session) error
	DeleteProject(p Project) error
	ResumeCmd(s Session) []string
	// DisplayName returns the display name for this scanner instance.
	// For custom agents, this overrides Platform().String().
	DisplayName() string
	// SetCustomName overrides the display name.
	SetCustomName(name string)
}

// ============================================================================
// Utilities
// ============================================================================

// RelativeTime 返回相对当前时间的可读字符串。
func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// Truncate 截断字符串到最多 n 个 rune。
func Truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// TruncateWidth 截断字符串，使视觉宽度不超过 n（保留头部，末尾加 …）。
func TruncateWidth(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	const ellipsis = "…"
	budget := n - lipgloss.Width(ellipsis)
	if budget <= 0 {
		return ellipsis
	}
	w, keep := 0, 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > budget {
			break
		}
		w += rw
		keep++
	}
	return string([]rune(s)[:keep]) + ellipsis
}

// TruncateKeepEnd 截断字符串，使视觉宽度不超过 n（去掉头部，保留末尾，开头加 …）。
func TruncateKeepEnd(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	const ellipsis = "…"
	budget := n - lipgloss.Width(ellipsis)
	if budget <= 0 {
		return ellipsis
	}
	runes := []rune(s)
	w, start := 0, len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		rw := lipgloss.Width(string(runes[i]))
		if w+rw > budget {
			break
		}
		w += rw
		start = i
	}
	return ellipsis + string(runes[start:])
}

// ============================================================================
// ScanProjectsParallel
// ============================================================================

// MergeProjects 将多个 Scanner 返回的 []Project 按 FullPath 合并，按最近活跃时间倒序。
func MergeProjects(all [][]Project) []Project {
	m := make(map[string]*Project)
	var order []string
	for _, projects := range all {
		for i := range projects {
			p := &projects[i]
			if existing, ok := m[p.FullPath]; ok {
				existing.Sessions = append(existing.Sessions, p.Sessions...)
			} else {
				cp := *p
				m[p.FullPath] = &cp
				order = append(order, p.FullPath)
			}
		}
	}
	result := make([]Project, 0, len(m))
	for _, key := range order {
		result = append(result, *m[key])
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastActive().After(result[j].LastActive())
	})
	return result
}

// ScanProjectsParallel 并行扫描所有平台的 projects，返回合并后的列表。
func ScanProjectsParallel(scanners []Scanner) []Project {
	var all [][]Project
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, sc := range scanners {
		wg.Add(1)
		go func(sc Scanner) {
			defer wg.Done()
			projects, err := sc.ScanProjects()
			if err != nil || len(projects) == 0 {
				return
			}
			mu.Lock()
			all = append(all, projects)
			mu.Unlock()
		}(sc)
	}
	wg.Wait()

	return MergeProjects(all)
}

// resolveEncodedPath 将用 - 编码的路径解码为真实路径，通过检查文件系统是否存在。
// 与简单的 - → / 替换不同：目录名本身可能包含 -（如 my-project），
// 简单替换会错误地将其拆分为 my/project。
// 该函数从 "/" 开始，逐步尝试 - 作为路径分隔符或目录名的一部分。
func resolveEncodedPath(encoded string) string {
	parts := strings.Split(encoded, "-")
	if len(parts) == 0 {
		return "/"
	}
	result := resolveFromDir("/", parts)
	if result == "" {
		// 无法通过文件系统解析，回退到简单替换
		return "/" + strings.ReplaceAll(encoded, "-", "/")
	}
	return result
}

// resolveFromDir 从 baseDir 开始，尝试将 parts 中的 - 解释为路径分隔符或目录名的一部分。
func resolveFromDir(baseDir string, parts []string) string {
	if len(parts) == 0 {
		return baseDir
	}
	// 尝试逐渐合并更多的 parts 作为单个目录名
	for i := 1; i <= len(parts); i++ {
		candidate := strings.Join(parts[:i], "-")
		fullPath := filepath.Join(baseDir, candidate)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			result := resolveFromDir(fullPath, parts[i:])
			if result != "" {
				return result
			}
		}
	}
	return ""
}

// ProjectShortName 从绝对路径取最后两段作为短名。
func ProjectShortName(fullPath string) string {
	fullPath = strings.TrimRight(fullPath, "/")
	if fullPath == "" {
		return "/"
	}
	dir := fullPath[:strings.LastIndex(fullPath, "/")]
	if dir == "" {
		return fullPath
	}
	base := fullPath[strings.LastIndex(fullPath, "/")+1:]
	parent := dir[strings.LastIndex(dir, "/")+1:]
	if parent == "" || parent == "." || parent == fullPath {
		return base
	}
	return parent + "/" + base
}
