package main

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ============================================================================
// 颜色 — Cyberpunk Dark (OLED)
// ============================================================================

const (
	// 强调色 — 霓虹绿
	ColorAccent    = "#22C55E"
	ColorAccentDim = "#166534"

	ColorRed    = "#EF4444"
	ColorYellow = "#EAB308"

	// 平台色 — 高饱和霓虹
	ColorGreen  = "#4ADE80" // CodeBuddy
	ColorOrange = "#FB923C" // Claude
	ColorCyan   = "#22D3EE" // Gemini
	ColorPurple = "#C084FC" // Codex
	ColorPink   = "#F472B6" // Copilot
	ColorGray   = "#9CA3AF" // OpenCode

	// 文字层级 — Slate
	ColorTextPrimary   = "#F1F5F9"
	ColorTextSecondary = "#94A3B8"
	ColorTextTertiary  = "#64748B"

	// 背景 / 分割线
	ColorBgSelected = "rgba(34,197,94,0.12)"
	ColorDivider    = "#334155"
)

// ============================================================================
// 基础样式
// ============================================================================

var (
	StyleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color(ColorBgSelected)).
			Foreground(lipgloss.Color(ColorTextPrimary))

	StyleSelectedAlt = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E293B")).
				Foreground(lipgloss.Color(ColorTextPrimary))

	StyleNormal = lipgloss.NewStyle()

	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))

	StyleSubtitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorTextPrimary))

	StyleSubtle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecondary))

	StyleFaint = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextTertiary))

	StyleTime = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecondary))

	StyleModel = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecondary))

	StyleMsgCount = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextSecondary))

	StyleSearch = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorYellow)).
			Bold(true)

	StyleDeleteBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color(ColorRed)).
			Padding(1, 2)

	StyleDeleteWarn = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true)

	StyleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDivider))

	StyleCursor = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)

	StyleCursorOff = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccentDim))

	StyleKey = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)

	StyleKeyDim = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccentDim))
)

// ============================================================================
// 平台颜色 — 霓虹高亮
// ============================================================================

var (
	StylePlatformCodeBuddy = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGreen)).Bold(true)
	StylePlatformClaude    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Bold(true)
	StylePlatformGemini    = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCyan)).Bold(true)
	StylePlatformCodex     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple)).Bold(true)
	StylePlatformCopilot   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPink)).Bold(true)
	StylePlatformOpenCode  = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGray)).Bold(true)
)

func PlatformStyleName(p Platform) lipgloss.Style {
	switch p {
	case PlatformCodeBuddy:
		return StylePlatformCodeBuddy
	case PlatformClaude:
		return StylePlatformClaude
	case PlatformGemini:
		return StylePlatformGemini
	case PlatformCodex:
		return StylePlatformCodex
	case PlatformCopilot:
		return StylePlatformCopilot
	case PlatformOpenCode:
		return StylePlatformOpenCode
	default:
		return StyleSubtle
	}
}

// ============================================================================
// Splash Logo
// ============================================================================

const logo = `
              ▄▄▄▄▄▄▄
            ▄███████████▄
           ███████████████
         ▄████████████████▄
        ████████████████████
       ██████████████████████
       ██████████▀  ▀████████
        ██████▀      ▀██████
         ▀███▀  ▄  ▄  ▀███▀
          ▀█▀  ▄██▄  ▀█▀
           ▀   ████   ▀
                ██
                ▀▀
`

func RenderLogo(_, _ int, version string) string {
	logoLines := strings.Split(strings.Trim(logo, "\n"), "\n")
	tagline := "manage your AI sessions"
	versionText := "roost " + version

	maxLogoW := 0
	for _, line := range logoLines {
		if w := lipgloss.Width(line); w > maxLogoW {
			maxLogoW = w
		}
	}

	rightW := lipgloss.Width(versionText)
	if w := lipgloss.Width(tagline); w > rightW {
		rightW = w
	}

	var sb strings.Builder

	pad := "  " // 左上，小缩进

	for i, line := range logoLines {
		lineW := lipgloss.Width(line)
		logoPadded := line + strings.Repeat(" ", maxLogoW-lineW)

		var right string
		switch i {
		case 2:
			right = StyleTitle.Render(versionText)
		case 3:
			right = StyleSubtle.Render(tagline)
		}
		rightPadded := right + strings.Repeat(" ", rightW-lipgloss.Width(right))

		sb.WriteString(pad + logoPadded + "    " + rightPadded + "\n")
	}

	return sb.String()
}

// ============================================================================
// 动画 — 光标脉冲
// ============================================================================

// CursorTickMsg 光标脉冲 tick。
type CursorTickMsg struct{}

// CursorTick 返回驱动光标脉冲的 Cmd。
func CursorTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return CursorTickMsg{}
	})
}

// CursorFrame 返回当前帧的光标样式（脉冲闪烁）。
func CursorFrame(visible bool) lipgloss.Style {
	if visible {
		return StyleCursor
	}
	return StyleCursorOff
}

// RenderCursor 渲染光标指示符 — 霓虹竖线。
func RenderCursor(visible bool) string {
	return CursorFrame(visible).Render("│")
}
