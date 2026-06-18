package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ============================================================================
// Delete Target
// ============================================================================

type deleteTarget int

const (
	deleteTargetSession deleteTarget = iota
	deleteTargetProject
	deleteTargetBatch
)

// ============================================================================
// ConfirmScreen
// ============================================================================

// ConfirmScreen 是删除确认弹窗。
type ConfirmScreen struct {
	action   string
	detail   string
	project  *Project
	session  *Session
	batches  []Project
	sessions []Session
	target   deleteTarget
}

func NewConfirmScreen(action, detail string, project *Project, session *Session, batches []Project, target deleteTarget, ss ...[]Session) *ConfirmScreen {
	s := []Session{}
	if len(ss) > 0 {
		s = ss[0]
	}
	return &ConfirmScreen{
		action:   action,
		detail:   detail,
		project:  project,
		session:  session,
		batches:  batches,
		sessions: s,
		target:   target,
	}
}

func (c *ConfirmScreen) Name() string { return "confirm" }
func (c *ConfirmScreen) OnEnter()     {}
func (c *ConfirmScreen) OnExit()      {}

func (c *ConfirmScreen) HandleKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return NewContextCmd(deleteAction(c.target, c.project, c.session, c.batches, c.sessions))
	case "esc", "n":
		return PopCmd()
	case "q", "ctrl+c":
		return tea.Quit
	}
	return nil
}

// DeleteDoneMsg 删除确认后由 App 处理。
type DeleteDoneMsg struct {
	Project  *Project
	Session  *Session
	Batches  []Project
	Sessions []Session // batch session delete
	Target   deleteTarget
}

func deleteAction(target deleteTarget, proj *Project, sess *Session, batches []Project, sessions []Session) tea.Cmd {
	return func() tea.Msg {
		return DeleteDoneMsg{
			Project:  proj,
			Session:  sess,
			Batches:  batches,
			Sessions: sessions,
			Target:   target,
		}
	}
}

func (c *ConfirmScreen) View(width, height int) string {
	var lines []string
	lines = append(lines, StyleDeleteWarn.Render(c.action))
	if c.detail != "" {
		lines = append(lines, "  "+c.detail)
	}
	lines = append(lines,
		"",
		StyleFaint.Render("This cannot be undone."),
		"",
		"  ["+StyleDeleteWarn.Render("Enter")+" confirm]  ["+StyleSubtle.Render("Esc/N")+" cancel]",
	)
	body := strings.Join(lines, "\n")
	box := StyleDeleteBox.Render(body)

	boxLines := strings.Split(box, "\n")

	var sb strings.Builder
	topPad := (height - len(boxLines)) / 2
	if topPad < 0 {
		topPad = 0
	}
	sb.WriteString(strings.Repeat("\n", topPad))
	centerStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	for _, l := range boxLines {
		sb.WriteString(centerStyle.Render(l) + "\n")
	}
	return sb.String()
}
