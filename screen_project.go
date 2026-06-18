package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ============================================================================
// ProjectScreen
// ============================================================================

// ProjectScreen 是项目列表屏幕。
type ProjectScreen struct {
	projects       []Project
	filtered       []Project
	cursor         int
	platformFilter int // -1=All, 0-N=installedPlatforms index
	installed      []Platform
	scanners       []Scanner

	searching   bool
	searchQuery string

	selecting   bool
	selectedSet map[string]bool

	showHelp    bool
	escHint     bool
	lastEscTime time.Time
	escHintGen  int

	width  int
	height int
	maxW   int // max layout width
}

const maxWidth = 140

// NewProjectScreen 创建项目列表 Context。
func NewProjectScreen(projects []Project, scanners []Scanner) *ProjectScreen {
	// 收集已安装的平台
	installed := collectPlatforms(projects)

	return &ProjectScreen{
		projects:       projects,
		filtered:       projects,
		platformFilter: -1,
		installed:      installed,
		scanners:       scanners,
		maxW:           maxWidth,
	}
}

// collectPlatforms 从项目列表中提取所有平台（去重、排序）。
func collectPlatforms(projects []Project) []Platform {
	seen := map[Platform]bool{}
	for _, p := range projects {
		for _, s := range p.Sessions {
			seen[s.Platform] = true
		}
	}
	var result []Platform
	for p := range seen {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].String() < result[j].String()
	})
	return result
}

func (c *ProjectScreen) Name() string { return "project" }
func (c *ProjectScreen) OnEnter()     {}
func (c *ProjectScreen) OnExit()      {}

func (c *ProjectScreen) HandleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit

	case "esc":
		if c.selecting {
			c.selecting = false
			c.selectedSet = nil
			return nil
		}
		if c.searching {
			c.searching = false
			c.searchQuery = ""
			c.filter()
			return nil
		}
		if c.showHelp {
			c.showHelp = false
			return nil
		}
		// 双击 Esc 退出
		if c.escHint && time.Since(c.lastEscTime) < 2*time.Second {
			return tea.Quit
		}
		c.escHint = true
		c.lastEscTime = time.Now()
		c.escHintGen++
		gen := c.escHintGen
		return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return escHintTimeoutMsg{gen: gen}
		})

	case "/":
		c.searching = true
		c.searchQuery = ""

	case "?":
		c.showHelp = !c.showHelp

	case "tab":
		c.platformFilter++
		if c.platformFilter >= len(c.installed) {
			c.platformFilter = -1
		}
		c.filter()

	case " ", "space":
		if len(c.filtered) > 0 {
			if !c.selecting {
				c.selecting = true
				c.selectedSet = make(map[string]bool)
			}
			key := c.filtered[c.cursor].FullPath
			if c.selectedSet[key] {
				delete(c.selectedSet, key)
			} else {
				c.selectedSet[key] = true
			}
		}

	case "D":
		if c.selecting && len(c.selectedSet) > 0 {
			return PushCmd(c.projectsToDelete())
		}

	case "up", "k":
		if c.cursor > 0 {
			c.cursor--
		}
	case "down", "j":
		if c.cursor < len(c.filtered)-1 {
			c.cursor++
		}
	case "g":
		c.cursor = 0
	case "G":
		if len(c.filtered) > 0 {
			c.cursor = len(c.filtered) - 1
		}

	case "enter":
		if len(c.filtered) > 0 {
			p := c.filtered[c.cursor]
			sessions := make([]Session, len(p.Sessions))
			copy(sessions, p.Sessions)
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].LastActive.After(sessions[j].LastActive)
			})
			return PushCmd(NewSessionScreen(&p, sessions, c.installed, c.scanners))
		}

	case "d":
		if !c.selecting && len(c.filtered) > 0 {
			return PushCmd(c.projectToDelete())
		}

	case "r":
		return NewContextCmd(refreshCmd())
	}

	return nil
}

func (c *ProjectScreen) filter() {
	q := strings.ToLower(c.searchQuery)
	var filtered []Project

	for _, p := range c.projects {
		if c.platformFilter >= 0 {
			target := c.installed[c.platformFilter]
			has := false
			for _, s := range p.Sessions {
				if s.Platform == target {
					has = true
					break
				}
			}
			if !has {
				continue
			}
		}
		if q != "" && !strings.Contains(strings.ToLower(p.FullPath), q) {
			continue
		}
		filtered = append(filtered, p)
	}

	c.filtered = filtered
	if c.cursor >= len(filtered) && len(filtered) > 0 {
		c.cursor = len(filtered) - 1
	}
	if len(filtered) == 0 {
		c.cursor = 0
	}
}

func (c *ProjectScreen) platformLabel() string {
	if c.platformFilter < 0 || c.platformFilter >= len(c.installed) {
		return "All"
	}
	return c.installed[c.platformFilter].String()
}

func (c *ProjectScreen) projectToDelete() Screen {
	proj := c.filtered[c.cursor]
	return NewConfirmScreen(
		"Delete project?",
		fmt.Sprintf("%s (%d sessions)", ProjectShortName(proj.FullPath), len(proj.Sessions)),
		&proj,
		nil,
		nil,
		deleteTargetProject,
	)
}

func (c *ProjectScreen) projectsToDelete() Screen {
	var projs []Project
	for _, p := range c.filtered {
		if c.selectedSet[p.FullPath] {
			projs = append(projs, p)
		}
	}
	return NewConfirmScreen(
		fmt.Sprintf("Delete %d projects?", len(projs)),
		"",
		nil,
		nil,
		projs,
		deleteTargetBatch,
	)
}

// ============================================================================
// View
// ============================================================================

func (c *ProjectScreen) View(width, height int) string {
	c.width = width
	c.height = height

	if c.showHelp {
		return c.renderHelp()
	}

	var sb strings.Builder
	w := c.maxW
	if width < w {
		w = width
	}
	if w < 20 {
		w = 20
	}

	// Header
	sb.WriteString(StyleTitle.Render("  roost") + StyleFaint.Render(" ▸ PROJECTS"))
	if len(c.installed) > 0 {
		var parts []string
		for _, p := range c.installed {
			parts = append(parts, "  "+PlatformStyleName(p).Render(p.Icon()+p.String()))
		}
		sb.WriteString(StyleSubtle.Render(strings.Join(parts, "")))
	}
	sb.WriteString("\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n")

	// Responsive column widths
	makerW := 4
	colSepW := 3 // " │ "

	var timeColW, platColW, nameColW int
	totalFixed := makerW + 2 + colSepW*2 // marker + cursorMark + 2 separators
	remaining := w - totalFixed

	switch {
	case w >= 100:
		timeColW = 12
		platColW = 24
	case w >= 80:
		timeColW = 10
		platColW = 20
	case w >= 60:
		timeColW = 10
		platColW = 16
	default:
		timeColW = 8
		platColW = 12
	}
	nameColW = remaining - timeColW - platColW
	if nameColW < 12 {
		nameColW = 12
		platColW = remaining - nameColW - timeColW
		if platColW < 8 {
			platColW = 0 // 极窄屏，隐藏平台列
			nameColW = remaining - timeColW
		}
	}

	colMarker := lipgloss.NewStyle().Width(makerW)
	colName := lipgloss.NewStyle().Width(nameColW)
	colPlatform := lipgloss.NewStyle().Width(platColW)
	colTime := lipgloss.NewStyle().Width(timeColW).Align(lipgloss.Right)
	colSep := " │ "
	colSepStyled := StyleFaint.Render(colSep)

	if platColW > 0 {
		projLabel := "project"
		agtLabel := "agents"
		if w < 70 {
			projLabel = "proj"
			agtLabel = "agts"
		}
		headerLine := colMarker.Render("") + "  " +
			colName.Render(StyleFaint.Render(projLabel)) + colSepStyled +
			colPlatform.Render(StyleFaint.Render(agtLabel)) + colSepStyled +
			colTime.Render(StyleFaint.Render("active"))
		sb.WriteString(headerLine + "\n")
	} else {
		headerLine := colMarker.Render("") + "  " +
			colName.Render(StyleFaint.Render("project")) + colSepStyled +
			colTime.Render(StyleFaint.Render("active"))
		sb.WriteString(headerLine + "\n")
	}

	// List
	viewH := c.height - 5 // title + separator + header + footer = 4
	if viewH < 1 {
		viewH = 1
	}
	start, end := viewport(len(c.filtered), c.cursor, viewH)

	if len(c.filtered) == 0 {
		if c.searchQuery != "" {
			sb.WriteString(StyleFaint.Render("  no results for: "+c.searchQuery) + "\n")
		} else {
			sb.WriteString(StyleFaint.Render("  no sessions found") + "\n")
			sb.WriteString(StyleFaint.Render("  press r to refresh") + "\n")
		}
	}

	platformOrder := AllPlatforms()

	for i := start; i < end; i++ {
		p := c.filtered[i]

		// Count sessions per platform
		var counts [PlatformCount]int
		for _, s := range p.Sessions {
			counts[s.Platform]++
		}

		// Build platform string — narrow columns: icons only
		var platParts []string
		for _, plat := range platformOrder {
			if n := counts[plat]; n > 0 {
				if platColW < 15 {
					platParts = append(platParts, PlatformStyleName(plat).Render(plat.Icon()))
				} else {
					platParts = append(platParts, fmt.Sprintf("%s %d",
						PlatformStyleName(plat).Render(plat.Icon()),
						n,
					))
				}
			}
		}
		platStr := strings.Join(platParts, "  ")

		marker := "  "
		if c.selecting {
			if c.selectedSet[p.FullPath] {
				marker = " ●"
			} else {
				marker = " ○"
			}
		}
		markerRendered := colMarker.Render(marker)

		cursorMark := "  "
		if i == c.cursor {
			cursorMark = RenderCursor(cursorVisible) + " "
		}

		pathStr := TruncateKeepEnd(p.FullPath, nameColW)
		timeStr := RelativeTime(p.LastActive())
		isCurrent := i == c.cursor || (c.selecting && c.selectedSet[p.FullPath])

		var nameRendered, platRendered, timeRendered string
		if isCurrent {
			nameRendered = colName.Bold(true).Render(pathStr)
			platRendered = colPlatform.Bold(true).Render(platStr)
			timeRendered = colTime.Bold(true).Render(timeStr)
		} else {
			nameRendered = colName.Render(pathStr)
			platRendered = colPlatform.Render(StyleTime.Render(platStr))
			timeRendered = colTime.Render(StyleTime.Render(timeStr))
		}

		var line string
		if platColW > 0 {
			line = markerRendered + cursorMark + nameRendered + colSepStyled + platRendered + colSepStyled + timeRendered
		} else {
			line = markerRendered + cursorMark + nameRendered + colSepStyled + timeRendered
		}
		if i == c.cursor {
			sb.WriteString(StyleSelected.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	// Pad to viewHeight if scrolling
	contentLines := end - start
	if len(c.filtered) > viewH {
		for j := contentLines; j < viewH && j < len(c.filtered); j++ {
			sb.WriteString("\n")
		}
	}

	// Footer
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n")
	if c.searching {
		sb.WriteString(StyleSearch.Render("  / "+c.searchQuery+"_") + "\n")
	} else if c.escHint {
		sb.WriteString(StyleDeleteWarn.Render("  press Esc again to quit") + "\n")
	} else {
		scroll := scrollHint(start, end, len(c.filtered), c.cursor)
		if c.selecting {
			fmt.Fprintf(&sb, "  %s toggle  %s delete(%d)  %s cancel%s\n",
				StyleKey.Render("Space"),
				StyleKey.Render("D"),
				len(c.selectedSet),
				StyleKey.Render("Esc"),
				StyleFaint.Render(scroll),
			)
		} else if w >= 60 {
			fmt.Fprintf(&sb, "  [%s] %s open  %s %s %s back  %s help%s\n",
				c.platformLabel(),
				StyleKey.Render("Enter"),
				StyleKey.Render("↑↓"),
				StyleKey.Render("Space"),
				StyleKey.Render("Esc"),
				StyleKey.Render("?"),
				StyleFaint.Render(scroll),
			)
		} else {
			fmt.Fprintf(&sb, "  [%s] %s %s %s %s %s%s\n",
				c.platformLabel(),
				StyleKey.Render("Enter"),
				StyleKey.Render("↑↓"),
				StyleKey.Render("Space"),
				StyleKey.Render("Esc"),
				StyleKey.Render("?"),
				StyleFaint.Render(scroll),
			)
		}
	}

	return sb.String()
}

func (c *ProjectScreen) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("  roost — Keyboard Shortcuts") + "\n\n")

	row := func(k, d string) {
		fmt.Fprintf(&sb, "  %-14s %s\n", StyleKey.Render(k), StyleSubtle.Render(d))
	}

	sb.WriteString("  Navigation\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	row("↑/k", "Move up")
	row("↓/j", "Move down")
	row("g", "Jump to first")
	row("G", "Jump to last")
	row("Enter", "Open project")
	row("Esc", "Back / Exit select")
	row("Esc Esc", "Quit from main screen")
	sb.WriteString("\n")

	sb.WriteString("  Actions\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	row("/", "Search (Esc to cancel)")
	row("d", "Delete project")
	row("r", "Refresh")
	row("Tab", "Cycle platform filter")
	row("?", "Toggle help")
	sb.WriteString("\n")

	sb.WriteString("  Batch\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	row("Space", "Toggle selection")
	row("D", "Delete selected")
	sb.WriteString("\n")

	sb.WriteString("  Quit\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", 60)) + "\n")
	row("q / Ctrl+C", "Quit")
	sb.WriteString("\n")
	sb.WriteString(StyleFaint.Render("  Press any key to close help."))

	return sb.String()
}

// ============================================================================
// Shared utilities
// ============================================================================

func viewport(total, cursor, viewH int) (int, int) {
	if viewH <= 0 || total == 0 {
		return 0, 0
	}
	if total <= viewH {
		return 0, total
	}
	start := cursor - viewH/2
	if start < 0 {
		start = 0
	}
	end := start + viewH
	if end > total {
		end = total
		start = end - viewH
	}
	return start, end
}

func scrollHint(start, end, total, cursor int) string {
	if total == 0 {
		return ""
	}
	hint := ""
	if start > 0 {
		hint += "↑"
	}
	if end < total {
		hint += "↓"
	}
	if hint != "" {
		return fmt.Sprintf(" [%d/%d %s]", cursor+1, total, hint)
	}
	return fmt.Sprintf(" [%d/%d]", cursor+1, total)
}

// ============================================================================
// Messages
// ============================================================================

type escHintTimeoutMsg struct {
	gen int
}

// ============================================================================
// Commands (exported for app to use)
// ============================================================================

// RefreshCmd 返回一个触发刷新的 Cmd。

func refreshCmd() tea.Cmd {
	return func() tea.Msg { return RefreshMsg{} }
}

// RefreshMsg 表示需要刷新数据。
type RefreshMsg struct{}
