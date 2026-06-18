package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

// 版本信息
var (
	Version   = "v2.0.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	cfg := LoadConfig()

	scanners := detectScanners(cfg)
	if len(scanners) == 0 {
		fmt.Fprintln(os.Stderr, "No AI platforms detected.")
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		runTUI(scanners, cfg)
		return
	}

	switch args[0] {
	case "list":
		listArgs, err := ParseListArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			os.Exit(1)
		}
		if err := RunList(scanners, listArgs); err != nil {
			fmt.Fprintf(os.Stderr, "list: %v\n", err)
			os.Exit(1)
		}
	case "resume":
		resumeArgs, err := ParseResumeArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "resume: %v\n", err)
			os.Exit(1)
		}
		projects := ScanProjectsParallel(scanners)
		sess := FindSession(projects, resumeArgs.SessionID)
		if sess == nil {
			fmt.Fprintf(os.Stderr, "session not found: %s\n", resumeArgs.SessionID)
			os.Exit(1)
		}
		resumeSession(sess, scanners, cfg)
	case "delete":
		deleteArgs, err := ParseDeleteArgs(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "delete: %v\n", err)
			os.Exit(1)
		}
		projects := ScanProjectsParallel(scanners)
		sess := FindSession(projects, deleteArgs.SessionID)
		if sess == nil {
			fmt.Fprintf(os.Stderr, "session not found: %s\n", deleteArgs.SessionID)
			os.Exit(1)
		}
		sc := FindScanner(scanners, sess.Platform)
		if sc == nil {
			fmt.Fprintf(os.Stderr, "no scanner for platform %s\n", sess.Platform)
			os.Exit(1)
		}
		if err := sc.DeleteSession(*sess); err != nil {
			fmt.Fprintf(os.Stderr, "delete: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("deleted: %s (%s)\n", sess.Title, sess.ID)
	case "version", "--version", "-v":
		fmt.Printf("roost %s (built %s, commit %s)\n", Version, BuildTime, GitCommit)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: roost [list|resume|delete]")
		os.Exit(1)
	}
}

func runTUI(scanners []Scanner, cfg AppConfig) {
	m := NewApp(scanners, cfg, Version)
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	app, ok := final.(*App)
	if !ok {
		return
	}
	if app.ResumeSession != nil {
		resumeSession(app.ResumeSession, scanners, cfg)
	}
	if app.NewSession.Pending {
		execNewSession(app.NewSession.Platform, app.NewSession.ProjectPath, cfg)
	}
}

func detectScanners(cfg AppConfig) []Scanner {
	var scanners []Scanner

	newScannerFor := map[Platform]func(AppConfig, *ScannerOverrides) Scanner{
		PlatformCodeBuddy: func(c AppConfig, ov *ScannerOverrides) Scanner { return NewCodeBuddyScanner(c, ov) },
		PlatformClaude:    func(c AppConfig, ov *ScannerOverrides) Scanner { return NewClaudeScanner(c, ov) },
		PlatformGemini:    func(c AppConfig, ov *ScannerOverrides) Scanner { return NewGeminiScanner(c, ov) },
		PlatformCodex:     func(c AppConfig, ov *ScannerOverrides) Scanner { return NewCodexScanner(c, ov) },
		PlatformCopilot:   func(c AppConfig, ov *ScannerOverrides) Scanner { return NewCopilotScanner(c, ov) },
		PlatformOpenCode:  func(c AppConfig, ov *ScannerOverrides) Scanner { return NewOpenCodeScanner(c, ov) },
	}

	isAvailable := func(bin string) bool {
		_, err := exec.LookPath(bin)
		return err == nil
	}

	isOpenCodeUsable := func(bin string) bool {
		if !isAvailable(bin) {
			return false
		}
		_, err := exec.Command(bin, "db", "path").Output()
		return err == nil
	}

	for _, p := range AllPlatforms() {
		constructor, ok := newScannerFor[p]
		if !ok {
			continue
		}
		if p == PlatformOpenCode && !isOpenCodeUsable(DefaultBinOpenCode) {
			continue
		}
		scanners = append(scanners, constructor(cfg, nil))
	}

	// 自定义 agents
	for _, ca := range cfg.CustomAgents {
		if ca.Type == "" || ca.Bin == "" || ca.Name == "" {
			continue
		}
		platform := parseType(ca.Type)
		if platform < 0 {
			continue
		}
		constructor, ok := newScannerFor[platform]
		if !ok {
			continue
		}
		ov := &ScannerOverrides{Bin: ca.Bin, DataDir: ca.DataDir}
		scanner := constructor(cfg, ov)
		scanner.SetCustomName(ca.Name)
		scanners = append(scanners, scanner)
	}

	return scanners
}

// parseType 将类型字符串映射到 Platform。
func parseType(s string) Platform {
	switch strings.ToLower(s) {
	case "claude":
		return PlatformClaude
	case "codebuddy":
		return PlatformCodeBuddy
	case "gemini":
		return PlatformGemini
	case "codex":
		return PlatformCodex
	case "copilot":
		return PlatformCopilot
	case "opencode":
		return PlatformOpenCode
	default:
		return -1
	}
}

func resumeSession(sess *Session, scanners []Scanner, cfg AppConfig) {
	if err := os.Chdir(sess.ProjectPath); err != nil {
		fmt.Fprintf(os.Stderr, "chdir: %v\n", err)
		os.Exit(1)
	}
	sc := FindScanner(scanners, sess.Platform)
	if sc == nil {
		fmt.Fprintf(os.Stderr, "no scanner for platform %s\n", sess.Platform)
		os.Exit(1)
	}
	argv := sc.ResumeCmd(*sess)
	extra := cfg.ArgsFor(sess.Platform)
	if len(extra) > 0 {
		argv = append(argv, extra...)
	}
	binPath, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "not found: %s\n", argv[0])
		os.Exit(1)
	}
	if err := syscall.Exec(binPath, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "exec: %v\n", err)
		os.Exit(1)
	}
}

func execNewSession(p Platform, projectPath string, cfg AppConfig) {
	if err := os.Chdir(projectPath); err != nil {
		fmt.Fprintf(os.Stderr, "chdir: %v\n", err)
		os.Exit(1)
	}
	bin := DefaultBinFor(p)
	argv := []string{bin}
	extra := cfg.ArgsFor(p)
	if len(extra) > 0 {
		argv = append(argv, extra...)
	}
	binPath, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "not found: %s\n", argv[0])
		os.Exit(1)
	}
	if err := syscall.Exec(binPath, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "exec: %v\n", err)
		os.Exit(1)
	}
}
