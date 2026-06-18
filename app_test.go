package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// mockScanner 记录 delete 调用，用于验证批量删除。
type mockScanner struct {
	platform   Platform
	deleted    []string // 被删除的 session ID 列表
	projects   []Project
	sessions   []Session
	dataDir    string
	customName string
}

func (m *mockScanner) Platform() Platform { return m.platform }
func (m *mockScanner) DataDir() string    { return m.dataDir }
func (m *mockScanner) DisplayName() string {
	if m.customName != "" {
		return m.customName
	}
	return m.platform.String()
}
func (m *mockScanner) SetCustomName(n string)           { m.customName = n }
func (m *mockScanner) ScanProjects() ([]Project, error) { return m.projects, nil }
func (m *mockScanner) DeleteSession(s Session) error {
	m.deleted = append(m.deleted, s.ID)
	return nil
}

func (m *mockScanner) DeleteProject(p Project) error {
	for _, s := range p.Sessions {
		m.deleted = append(m.deleted, s.ID)
	}
	return nil
}

func (m *mockScanner) ResumeCmd(s Session) []string {
	return []string{DefaultBinFor(m.platform), "--resume", s.ID}
}

func TestBatchSessionDelete(t *testing.T) {
	sc := &mockScanner{platform: PlatformCodeBuddy}
	scanners := []Scanner{sc}

	sessions := []Session{
		{ID: "s1", Platform: PlatformCodeBuddy, Title: "first"},
		{ID: "s2", Platform: PlatformCodeBuddy, Title: "second"},
		{ID: "s3", Platform: PlatformCodeBuddy, Title: "third"},
	}

	// 模拟批量删除
	app := &App{scanners: scanners, phase: PhaseActive, contextStack: []Screen{&mockScreen{}}}
	msg := DeleteDoneMsg{
		Sessions: sessions,
		Target:   deleteTargetSession,
	}

	cmd := app.deleteCmd(msg)
	cmdResult := cmd()

	// 验证所有 session 都被删除
	if len(sc.deleted) != 3 {
		t.Errorf("expected 3 deletions, got %d: %v", len(sc.deleted), sc.deleted)
	}
	for _, sid := range []string{"s1", "s2", "s3"} {
		if !contains(sc.deleted, sid) {
			t.Errorf("session %s was not deleted", sid)
		}
	}

	// 验证返回了 scanDoneMsg
	if _, ok := cmdResult.(scanDoneMsg); !ok {
		t.Errorf("expected scanDoneMsg, got %T", cmdResult)
	}
}

func TestSingleSessionDelete(t *testing.T) {
	sc := &mockScanner{platform: PlatformClaude}
	scanners := []Scanner{sc}

	sess := &Session{ID: "only-one", Platform: PlatformClaude, Title: "single"}

	app := &App{scanners: scanners, phase: PhaseActive, contextStack: []Screen{&mockScreen{}}}
	msg := DeleteDoneMsg{
		Session: sess,
		Target:  deleteTargetSession,
	}

	cmd := app.deleteCmd(msg)
	cmd()

	if len(sc.deleted) != 1 {
		t.Errorf("expected 1 deletion, got %d", len(sc.deleted))
	}
	if sc.deleted[0] != "only-one" {
		t.Errorf("deleted %s, expected only-one", sc.deleted[0])
	}
}

func TestEmptySessionDelete(t *testing.T) {
	sc := &mockScanner{platform: PlatformCodeBuddy}
	scanners := []Scanner{sc}

	app := &App{scanners: scanners, phase: PhaseActive, contextStack: []Screen{&mockScreen{}}}
	msg := DeleteDoneMsg{
		Target: deleteTargetSession, // no Session, no Sessions
	}

	cmd := app.deleteCmd(msg)
	cmd()

	if len(sc.deleted) != 0 {
		t.Errorf("expected 0 deletions with empty target, got %d", len(sc.deleted))
	}
}

func TestBatchDeleteMixedPlatforms(t *testing.T) {
	s1 := &mockScanner{platform: PlatformCodeBuddy}
	s2 := &mockScanner{platform: PlatformClaude}
	scanners := []Scanner{s1, s2}

	sessions := []Session{
		{ID: "cb-1", Platform: PlatformCodeBuddy, Title: "cb session"},
		{ID: "cl-1", Platform: PlatformClaude, Title: "cl session"},
		{ID: "cb-2", Platform: PlatformCodeBuddy, Title: "cb session 2"},
	}

	app := &App{scanners: scanners, phase: PhaseActive, contextStack: []Screen{&mockScreen{}}}
	msg := DeleteDoneMsg{
		Sessions: sessions,
		Target:   deleteTargetSession,
	}

	cmd := app.deleteCmd(msg)
	cmd()

	if len(s1.deleted) != 2 {
		t.Errorf("CodeBuddy scanner: expected 2 deletions, got %d: %v", len(s1.deleted), s1.deleted)
	}
	if len(s2.deleted) != 1 {
		t.Errorf("Claude scanner: expected 1 deletion, got %d: %v", len(s2.deleted), s2.deleted)
	}
}

// mockScreen is a minimal Screen for tests.
type mockScreen struct{}

func (m *mockScreen) Name() string                          { return "mock" }
func (m *mockScreen) OnEnter()                              {}
func (m *mockScreen) OnExit()                               {}
func (m *mockScreen) View(w, h int) string                  { return "" }
func (m *mockScreen) HandleKey(msg tea.KeyPressMsg) tea.Cmd { return nil }

func contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
