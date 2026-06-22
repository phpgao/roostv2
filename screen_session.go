package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ============================================================================
// SessionScreen
// ============================================================================

// SessionScreen is the session list screen.
type SessionScreen struct {
	project        *Project
	sessions       []Session
	filtered       []Session
	cursor         int
	platformFilter int
	installed      []Platform
	scanners       []Scanner

	searching   bool
	searchQuery string

	selecting   bool
	selectedSet map[string]bool

	showHelp bool
	width    int
	height   int
}

func NewSessionScreen(project *Project, sessions []Session, installed []Platform, scanners []Scanner) *SessionScreen {
	return &SessionScreen{
		project:        project,
		sessions:       sessions,
		filtered:       sessions,
		platformFilter: -1,
		installed:      installed,
		scanners:       scanners,
	}
}

func (c *SessionScreen) Name() string { return "session" }
func (c *SessionScreen) OnEnter()     {}
func (c *SessionScreen) OnExit()      {}

func (c *SessionScreen) HandleKey(msg tea.KeyPressMsg) tea.Cmd {
	if c.searching {
		return c.handleSearchKey(msg)
	}
	if c.showHelp {
		c.showHelp = false
		return nil
	}

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
		return PopCmd()

	case "?":
		c.showHelp = true

	case "/":
		c.searching = true
		c.searchQuery = ""

	case " ", "space":
		if len(c.filtered) > 0 {
			if !c.selecting {
				c.selecting = true
				c.selectedSet = make(map[string]bool)
			}
			key := c.filtered[c.cursor].ID
			if c.selectedSet[key] {
				delete(c.selectedSet, key)
			} else {
				c.selectedSet[key] = true
			}
		}

	case "D":
		if c.selecting && len(c.selectedSet) > 0 {
			var sessions []Session
			for _, s := range c.filtered {
				if c.selectedSet[s.ID] {
					sessions = append(sessions, s)
				}
			}
			detail := fmt.Sprintf("%d sessions", len(sessions))
			return PushCmd(NewConfirmScreen(
				fmt.Sprintf("Delete %d sessions?", len(sessions)),
				detail,
				nil,
				nil,
				nil,
				deleteTargetSession,
				sessions,
			))
		}

	case "tab":
		c.platformFilter++
		if c.platformFilter >= len(c.installed) {
			c.platformFilter = -1
		}
		c.filter()

	case "up", "k":
		if len(c.filtered) > 0 {
			if c.cursor > 0 {
				c.cursor--
			} else {
				c.cursor = len(c.filtered) - 1
			}
		}
	case "down", "j":
		if len(c.filtered) > 0 {
			if c.cursor < len(c.filtered)-1 {
				c.cursor++
			} else {
				c.cursor = 0
			}
		}
	case "g":
		if len(c.filtered) > 0 {
			c.cursor = 0
		}
	case "G":
		if len(c.filtered) > 0 {
			c.cursor = len(c.filtered) - 1
		}

	case "enter":
		if len(c.filtered) > 0 {
			sess := c.filtered[c.cursor]
			sc := FindScanner(c.scanners, sess.Platform)
			return PushCmd(NewSessionDetail(&sess, sc, sess.Platform.Icon()))
		}

	case "d":
		if len(c.filtered) > 0 {
			sess := c.filtered[c.cursor]
			return PushCmd(NewConfirmScreen(
				"Delete session?",
				sess.Title,
				nil,
				&sess,
				nil,
				deleteTargetSession,
			))
		}

	case "n":
		c.filtered = c.sessions
		c.cursor = 0
		return PushCmd(NewAgentScreen(c.sessions, c.scanners, c.installed))

	case "x":
		// 删除当前项目
		var batches []Project
		for _, sc := range c.scanners {
			var sessions []Session
			for _, s := range c.sessions {
				if s.Platform == sc.Platform() {
					sessions = append(sessions, s)
				}
			}
			if len(sessions) > 0 {
				batches = append(batches, Project{
					Name:     c.project.Name,
					FullPath: c.project.FullPath,
					Sessions: sessions,
				})
			}
		}
		return PushCmd(NewConfirmScreen(
			"Delete project?",
			c.project.FullPath,
			c.project,
			nil,
			batches,
			deleteTargetBatch,
		))
	}

	return nil
}

func (c *SessionScreen) handleSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		c.searching = false
		c.searchQuery = ""
		c.filter()
		return nil
	case "enter":
		c.searching = false
		c.filter()
		return nil
	case "backspace":
		if len(c.searchQuery) > 0 {
			c.searchQuery = c.searchQuery[:len(c.searchQuery)-1]
			c.filter()
		}
	default:
		s := msg.String()
		if len(s) == 1 && s[0] >= 32 && s[0] < 127 {
			c.searchQuery += s
			c.filter()
		}
	}
	return nil
}

func (c *SessionScreen) filter() {
	base := c.sessions
	if c.platformFilter >= 0 && c.platformFilter < len(c.installed) {
		target := c.installed[c.platformFilter]
		var filtered []Session
		for _, s := range base {
			if s.Platform == target {
				filtered = append(filtered, s)
			}
		}
		base = filtered
	}

	if c.searchQuery != "" {
		q := strings.ToLower(c.searchQuery)
		var filtered []Session
		for _, s := range base {
			if strings.Contains(strings.ToLower(s.Title), q) ||
				strings.Contains(strings.ToLower(s.Model), q) {
				filtered = append(filtered, s)
			}
		}
		c.filtered = filtered
	} else {
		c.filtered = base
	}

	if c.cursor >= len(c.filtered) {
		c.cursor = len(c.filtered) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
}

func (c *SessionScreen) platformLabel() string {
	if c.platformFilter < 0 || c.platformFilter >= len(c.installed) {
		return "All"
	}
	return c.installed[c.platformFilter].String()
}

// ============================================================================
// SessionScreen View
// ============================================================================

func (c *SessionScreen) View(width, height int) string {
	c.width = width
	c.height = height

	if c.showHelp {
		return c.renderHelp()
	}

	var sb strings.Builder
	w := maxWidth
	if width < w {
		w = width
	}
	if w < 20 {
		w = 20
	}

	// Breadcrumb header
	breadcrumb := StyleSubtle.Render("roost > ") + StyleTitle.Render(ProjectShortName(c.project.FullPath)) + StyleFaint.Render(" ▸ SESSIONS")
	if len(c.installed) > 0 {
		var parts []string
		for _, p := range c.installed {
			parts = append(parts, "  "+PlatformStyleName(p).Render(p.Icon()+p.String()))
		}
		breadcrumb += StyleSubtle.Render(strings.Join(parts, ""))
	}
	sb.WriteString("  " + breadcrumb + "\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n")

	// Layout
	const (
		markerW = 4
		iconW   = 3
	)
	colSepW := 3 // " │ "

	var modelW, timeW, msgW, titleW int
	totalFixed := markerW + 2 + iconW + colSepW*3

	switch {
	case w >= 100:
		modelW = 16
		timeW = 10
		msgW = 5
	case w >= 80:
		modelW = 14
		timeW = 9
		msgW = 5
	case w >= 60:
		modelW = 0
		timeW = 9
		msgW = 5
	default:
		modelW = 0
		timeW = 8
		msgW = 0
	}
	titleW = w - totalFixed - modelW - timeW - msgW
	if titleW < 10 {
		titleW = 10
	}

	colMarker := lipgloss.NewStyle().Width(markerW)
	colIcon := lipgloss.NewStyle().Width(iconW)
	colTitle := lipgloss.NewStyle().Width(titleW)
	colModel := lipgloss.NewStyle().Width(modelW)
	colTime := lipgloss.NewStyle().Width(timeW).Align(lipgloss.Right)
	colMsg := lipgloss.NewStyle().Width(msgW).Align(lipgloss.Right)

	colSep := " │ "
	colSepStyled := StyleFaint.Render(colSep)

	tLabel, mLabel, aLabel, gLabel := "title", "model", "active", "msgs"
	if w < 70 {
		tLabel = "title"
		mLabel = "mdl"
		aLabel = "act"
		gLabel = "msg"
	}

	var headerLine string
	baseLine := colMarker.Render("") + colIcon.Render("") + "  " +
		colTitle.Render(StyleFaint.Render(tLabel)) + colSepStyled
	if modelW > 0 {
		baseLine += colModel.Render(StyleFaint.Render(mLabel)) + colSepStyled
	}
	baseLine += colTime.Render(StyleFaint.Render(aLabel))
	if msgW > 0 {
		baseLine += colSepStyled + colMsg.Render(StyleFaint.Render(gLabel))
	}
	headerLine = baseLine
	sb.WriteString(headerLine + "\n")

	// List
	viewH := c.height - 5
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

	for i := start; i < end; i++ {
		s := c.filtered[i]

		marker := "  "
		if c.selecting {
			if c.selectedSet[s.ID] {
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

		isCurrent := i == c.cursor || (c.selecting && c.selectedSet[s.ID])
		titleF := TruncateWidth(s.Title, titleW)
		modelF := TruncateWidth(s.Model, modelW)
		timeF := RelativeTime(s.LastActive)
		msgF := fmt.Sprintf("%d", s.MsgCount)

		var titleR, modelR, timeR, msgR string
		if isCurrent {
			titleR = colTitle.Bold(true).Render(titleF)
			modelR = colModel.Bold(true).Render(modelF)
			timeR = colTime.Bold(true).Render(timeF)
			msgR = colMsg.Bold(true).Render(msgF)
		} else {
			titleR = colTitle.Render(titleF)
			modelR = colModel.Render(StyleTime.Render(modelF))
			timeR = colTime.Render(StyleTime.Render(timeF))
			msgR = colMsg.Render(StyleTime.Render(msgF))
		}

		icon := colIcon.Render(PlatformStyleName(s.Platform).Render(s.Platform.Icon()))

		var line string
		rowBase := markerRendered + cursorMark + icon + titleR + colSepStyled
		if modelW > 0 {
			rowBase += modelR + colSepStyled
		}
		rowBase += timeR
		if msgW > 0 {
			rowBase += colSepStyled + msgR
		}
		line = rowBase
		if i == c.cursor {
			sb.WriteString(StyleSelected.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	// Footer
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n")
	if c.searching {
		sb.WriteString(StyleSearch.Render("  / "+c.searchQuery+"_") + "\n")
	} else {
		scroll := scrollHint(start, end, len(c.filtered), c.cursor)
		if c.selecting {
			fmt.Fprintf(
				&sb, "  %s toggle  %s delete(%d)  %s cancel%s\n",
				StyleKey.Render("Space"),
				StyleKey.Render("D"),
				len(c.selectedSet),
				StyleKey.Render("Esc"),
				StyleFaint.Render(scroll),
			)
		} else if w >= 60 {
			fmt.Fprintf(
				&sb, "  [%s] %s resume  %s new  %s %s %s back  %s help%s\n",
				c.platformLabel(),
				StyleKey.Render("Enter"),
				StyleKey.Render("n"),
				StyleKey.Render("↑↓"),
				StyleKey.Render("Space"),
				StyleKey.Render("Esc"),
				StyleKey.Render("?"),
				StyleFaint.Render(scroll),
			)
		} else {
			fmt.Fprintf(
				&sb, "  [%s] %s %s %s %s %s%s\n",
				c.platformLabel(),
				StyleKey.Render("Enter"),
				StyleKey.Render("n"),
				StyleKey.Render("↑↓"),
				StyleKey.Render("Esc"),
				StyleKey.Render("?"),
				StyleFaint.Render(scroll),
			)
		}
	}

	return sb.String()
}

func (c *SessionScreen) renderHelp() string {
	var sb strings.Builder
	sb.WriteString(StyleTitle.Render("  Session View — Keyboard Shortcuts") + "\n\n")

	row := func(k, d string) {
		fmt.Fprintf(&sb, "  %-14s %s\n", StyleKey.Render(k), StyleSubtle.Render(d))
	}

	row("Enter", "Resume session")
	row("n", "New session (select agent)")
	row("d", "Delete session")
	row("x", "Delete project (back to list)")
	row("↑↓ / j k", "Move cursor")
	row("g / G", "Go to top / bottom")
	row("Tab", "Filter by platform")
	row("/", "Search sessions")
	row("Space", "Toggle select for batch delete")
	row("D", "Batch delete selected")
	row("Esc", "Go back to projects")
	row("?", "Toggle this help")
	row("q / Ctrl+C", "Quit")

	return sb.String()
}

// ============================================================================
// AgentScreen
// ============================================================================

type AgentScreen struct {
	sessions  []Session
	scanners  []Scanner
	platforms []Platform
	cursor    int
	width     int
	height    int
	maxW      int
}

func NewAgentScreen(sessions []Session, scanners []Scanner, installed []Platform) *AgentScreen {
	// Build list of platform options
	var platforms []Platform
	platforms = append(platforms, installed...)
	// Add custom agents
	for _, sc := range scanners {
		if name := sc.DisplayName(); name != sc.Platform().String() {
			platforms = append(platforms, sc.Platform())
		}
	}

	return &AgentScreen{
		sessions:  sessions,
		scanners:  scanners,
		platforms: platforms,
		maxW:      maxWidth,
	}
}

func (c *AgentScreen) Name() string { return "agent" }
func (c *AgentScreen) OnEnter()     {}
func (c *AgentScreen) OnExit()      {}

func (c *AgentScreen) HandleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "esc":
		return PopCmd()
	case "up", "k":
		if len(c.platforms) > 0 {
			if c.cursor > 0 {
				c.cursor--
			} else {
				c.cursor = len(c.platforms) - 1
			}
		}
	case "down", "j":
		if len(c.platforms) > 0 {
			if c.cursor < len(c.platforms)-1 {
				c.cursor++
			} else {
				c.cursor = 0
			}
		}
	case "enter":
		if c.cursor >= 0 && c.cursor < len(c.platforms) {
			p := c.platforms[c.cursor]
			// Get project path from the session list
			projectPath := "."
			if len(c.sessions) > 0 {
				projectPath = c.sessions[0].ProjectPath
				if projectPath == "" {
					projectPath = c.sessions[0].ProjectDir
				}
			}
			return func() tea.Msg { return NewSessionMsg{Platform: p, ProjectPath: projectPath} }
		}
	}
	return nil
}

func (c *AgentScreen) View(width, height int) string {
	c.width = width
	c.height = height

	var sb strings.Builder
	w := c.maxW
	if width < w {
		w = width
	}

	sb.WriteString(StyleTitle.Render("  New Session") + "\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n\n")
	sb.WriteString(StyleFaint.Render("  Select an agent:") + "\n\n")

	viewH := c.height - 5
	if viewH < 1 {
		viewH = 1
	}
	start, end := viewport(len(c.platforms), c.cursor, viewH)

	for i := start; i < end; i++ {
		p := c.platforms[i]
		icon := p.Icon()
		name := p.String()

		cursorMark := "  "
		if i == c.cursor {
			cursorMark = StyleCursor.Render("▸ ")
		}

		line := fmt.Sprintf("%s %s %s", cursorMark, PlatformStyleName(p).Render(icon), name)
		if i == c.cursor {
			sb.WriteString(StyleSelected.Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}

	for j := end - start; j < viewH; j++ {
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", w)) + "\n")
	scroll := scrollHint(start, end, len(c.platforms), c.cursor)
	fmt.Fprintf(
		&sb, "  %s select  %s %s back%s\n",
		StyleKey.Render("Enter"),
		StyleKey.Render("↑↓"),
		StyleKey.Render("Esc"),
		StyleFaint.Render(scroll),
	)

	return sb.String()
}

// ============================================================================
// SessionDetail — session detail + manual command confirmation
// ============================================================================

type SessionDetail struct {
	sess   Session
	sc     Scanner
	icon   string
	resume tea.Cmd
}

func NewSessionDetail(sess *Session, sc Scanner, icon string) *SessionDetail {
	var resume tea.Cmd
	if sess != nil {
		resume = func() tea.Msg { return ResumeSessionMsg{Session: *sess} }
	}
	return &SessionDetail{sess: *sess, sc: sc, icon: icon, resume: resume}
}

func (d *SessionDetail) Name() string { return "detail" }
func (d *SessionDetail) OnEnter()     {}
func (d *SessionDetail) OnExit()      {}

func (d *SessionDetail) HandleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		if d.resume != nil {
			return NewContextCmd(d.resume)
		}
	case "esc", "q":
		return PopCmd()
	}
	return nil
}

func (d *SessionDetail) View(width, height int) string {
	var sb strings.Builder
	sess := d.sess

	sb.WriteString(StyleTitle.Render("  Session Detail") + "\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", width)) + "\n\n")

	kvs := []struct{ k, v string }{
		{"Platform", PlatformStyleName(sess.Platform).Render(d.icon + " " + sess.Platform.String())},
		{"Title", sess.Title},
		{"Model", sess.Model},
		{"Project", sess.ProjectPath},
		{"Last Active", RelativeTime(sess.LastActive)},
		{"Messages", fmt.Sprintf("%d", sess.MsgCount)},
		{"Session ID", sess.ID},
	}
	for _, kv := range kvs {
		if kv.v != "" {
			fmt.Fprintf(&sb, "  %-14s %s\n", StyleKey.Render(kv.k), StyleSubtle.Render(kv.v))
		}
	}

	// Manual command — 路径 + shell 参数转义
	cmd := d.manualCmd()
	if cmd != "" {
		sb.WriteString("\n" + StyleFaint.Render("  manual command:") + "\n")
		fmt.Fprintf(&sb, "  %s\n", StyleSubtle.Render("  $ cd "+shellEscape(sess.ProjectPath)))
		fmt.Fprintf(&sb, "  %s\n", StyleSubtle.Render("  $ "+cmd))
	}

	sb.WriteString("\n")
	sb.WriteString(StyleDivider.Render(strings.Repeat("─", width)) + "\n")
	fmt.Fprintf(
		&sb, "  %s confirm  %s back\n",
		StyleKey.Render("[Enter]"),
		StyleKey.Render("[Esc]"),
	)
	return sb.String()
}

func (d *SessionDetail) manualCmd() string {
	if d.sc == nil {
		return ""
	}
	argv := d.sc.ResumeCmd(d.sess)
	var parts []string
	for _, a := range argv {
		parts = append(parts, shellEscape(a))
	}
	return strings.Join(parts, " ")
}

// shellEscape 对包含空格/特殊字符的参数加单引号。
func shellEscape(s string) string {
	needsQuote := strings.ContainsAny(s, " \t\n'\"$&|;(){}[]*?!<>")
	if !needsQuote {
		return s
	}
	// 单引号内不能含单引号，先替换为 '\''
	s = strings.ReplaceAll(s, "'", `'\''`)
	return "'" + s + "'"
}
