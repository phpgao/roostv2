package main

import (
	"testing"
	"time"
)

func TestMergeProjects(t *testing.T) {
	p1 := Project{FullPath: "/a", Sessions: []Session{{ID: "1", Platform: PlatformCodeBuddy}}}
	p2 := Project{FullPath: "/a", Sessions: []Session{{ID: "2", Platform: PlatformClaude}}}
	p3 := Project{FullPath: "/b", Sessions: []Session{{ID: "3", Platform: PlatformGemini}}}

	merged := MergeProjects([][]Project{{p1}, {p2}, {p3}})
	if len(merged) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(merged))
	}
	if merged[0].FullPath != "/a" {
		t.Errorf("first project = %s, want /a", merged[0].FullPath)
	}
	if len(merged[0].Sessions) != 2 {
		t.Errorf("/a sessions = %d, want 2", len(merged[0].Sessions))
	}
	if merged[1].FullPath != "/b" {
		t.Errorf("second project = %s, want /b", merged[1].FullPath)
	}
}

func TestMergeProjects_EmptyInput(t *testing.T) {
	merged := MergeProjects(nil)
	if len(merged) != 0 {
		t.Errorf("expected 0 projects, got %d", len(merged))
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello wo…"},
		{"", 10, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc…"},
	}
	for _, tt := range tests {
		got := Truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestTruncateWidth(t *testing.T) {
	got := TruncateWidth("hello world", 8)
	if got != "hello w…" {
		t.Errorf("TruncateWidth = %q, want %q", got, "hello w…")
	}
	if TruncateWidth("", 10) != "" {
		t.Error("empty string should stay empty")
	}
	if TruncateWidth("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
}

func TestProjectShortName(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/Users/jimmy/code/github/roost", "github/roost"},
		{"/home/user/project", "user/project"},
		{"/", "/"},
		{"/a", "/a"},
	}
	for _, tt := range tests {
		got := ProjectShortName(tt.path)
		if got != tt.want {
			t.Errorf("ProjectShortName(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestRelativeTime_Basic(t *testing.T) {
	now := time.Now()

	if RelativeTime(time.Time{}) != "-" {
		t.Errorf("zero time should return '-'")
	}
	if RelativeTime(now) != "just now" {
		t.Errorf("now should return 'just now'")
	}
	// RelativeTime 的具体格式取决于实现，只验证非空
	if r := RelativeTime(now.Add(-2 * time.Minute)); r == "" {
		t.Error("should return non-empty for 2m ago")
	}
	if r := RelativeTime(now.Add(-2 * time.Hour)); r == "" {
		t.Error("should return non-empty for 2h ago")
	}
}

func TestPlatform_order(t *testing.T) {
	platforms := AllPlatforms()
	if len(platforms) < 6 {
		t.Errorf("expected at least 6 platforms, got %d", len(platforms))
	}
	seen := map[Platform]bool{}
	for _, p := range platforms {
		if seen[p] {
			t.Errorf("duplicate platform: %s", p)
		}
		seen[p] = true
	}
}

func TestDefaultBin_AllPlatforms(t *testing.T) {
	for _, p := range AllPlatforms() {
		bin := DefaultBinFor(p)
		if bin == "" {
			t.Errorf("DefaultBinFor(%s) is empty", p)
		}
	}
}
