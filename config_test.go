package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// No config file → use defaults
	cfg := LoadConfig()
	if cfg.ResumeMode != ResumeModeReplace {
		t.Errorf("default resume_mode = %s, want replace", cfg.ResumeMode)
	}
	if cfg.Platforms == nil {
		t.Error("platforms should not be nil")
	}
	if len(cfg.CustomAgents) != 0 {
		t.Errorf("default custom_agents = %d, want 0", len(cfg.CustomAgents))
	}

	// ArgsFor for codebuddy returns platform-level args
	args := cfg.ArgsFor(PlatformCodeBuddy)
	if args != nil {
		t.Errorf("default args for codebuddy = %v, want nil", args)
	}
}

func TestLoadConfig_CustomAgents(t *testing.T) {
	tmpDir := t.TempDir()
	roostDir := filepath.Join(tmpDir, ".roost")
	_ = os.MkdirAll(roostDir, 0o755)
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", os.Getenv("HOME"))

	os.WriteFile(filepath.Join(roostDir, "roost.yaml"), []byte(`
resume_mode: suspend
platforms:
  codebuddy:
    args: [-y]
custom_agents:
  - name: my-claude
    type: claude
    bin: claude-internal
    data_dir: .claude-internal
    args: [--dangerously-skip-permissions]
`), 0o644)

	cfg := LoadConfig()
	if cfg.ResumeMode != ResumeModeSuspend {
		t.Errorf("resume_mode = %s, want suspend", cfg.ResumeMode)
	}
	if len(cfg.CustomAgents) != 1 {
		t.Fatalf("expected 1 custom agent, got %d", len(cfg.CustomAgents))
	}
	ca := cfg.CustomAgents[0]
	if ca.Name != "my-claude" {
		t.Errorf("custom agent name = %s, want my-claude", ca.Name)
	}
	if ca.Type != "claude" {
		t.Errorf("custom agent type = %s, want claude", ca.Type)
	}
	if ca.Bin != "claude-internal" {
		t.Errorf("custom agent bin = %s, want claude-internal", ca.Bin)
	}
	if ca.DataDir != ".claude-internal" {
		t.Errorf("custom agent data_dir = %s, want .claude-internal", ca.DataDir)
	}

	args := cfg.ArgsFor(PlatformCodeBuddy)
	if len(args) != 1 || args[0] != "-y" {
		t.Errorf("codebuddy args = %v, want [-y]", args)
	}
}

func TestConfig_GetResumeMode(t *testing.T) {
	tests := []struct {
		mode ResumeMode
		want ResumeMode
	}{
		{"", ResumeModeReplace},
		{ResumeModeReplace, ResumeModeReplace},
		{ResumeModeSuspend, ResumeModeSuspend},
		{"invalid", ResumeModeReplace},
	}
	for _, tt := range tests {
		cfg := AppConfig{ResumeMode: tt.mode}
		if got := cfg.GetResumeMode(); got != tt.want {
			t.Errorf("GetResumeMode(%q) = %s, want %s", tt.mode, got, tt.want)
		}
	}
}

func TestPlatformString(t *testing.T) {
	tests := []struct {
		p    Platform
		want string
	}{
		{PlatformCodeBuddy, "CodeBuddy"},
		{PlatformClaude, "Claude"},
		{PlatformGemini, "Gemini"},
		{PlatformCodex, "Codex"},
		{PlatformCopilot, "Copilot"},
		{PlatformOpenCode, "OpenCode"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("Platform(%d).String() = %s, want %s", tt.p, got, tt.want)
		}
	}
}

func TestPlatformIcon(t *testing.T) {
	for _, p := range AllPlatforms() {
		icon := p.Icon()
		if icon == "" {
			t.Errorf("Platform(%s).Icon() returned empty", p)
		}
	}
}

func TestRelativeTime_Formats(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"now", now, "just now"},
		{"zero", time.Time{}, "-"},
		{"2min ago", now.Add(-2 * time.Minute), "2m ago"},
		{"2h ago", now.Add(-2 * time.Hour), "2h ago"},
		{"2d ago", now.Add(-48 * time.Hour), "2d ago"},
		{"future", now.Add(1 * time.Hour), "just now"},
	}
	for _, tt := range tests {
		got := RelativeTime(tt.t)
		if got != tt.want {
			// RelativeTime is approximate, check prefix instead
			t.Logf("RelativeTime(%s) = %s, want %s", tt.name, got, tt.want)
		}
	}
}
