package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// List
// ============================================================================

// ListArgs 是 list 命令的参数。
type ListArgs struct {
	Type    string // project 或 session
	Project string // project path，session 模式时必需
	Output  string // table, json, yaml
}

// ParseListArgs 从 os.Args 解析 list 子命令参数。
func ParseListArgs(args []string) (ListArgs, error) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	a := ListArgs{Type: "project", Output: "table"}
	fs.StringVar(&a.Type, "type", "project", "project or session")
	fs.StringVar(&a.Type, "t", "project", "shorthand for --type")
	fs.StringVar(&a.Project, "project", "", "project path (for session mode)")
	fs.StringVar(&a.Project, "p", "", "shorthand for --project")
	fs.StringVar(&a.Output, "output", "table", "output format: table, json, yaml")
	fs.StringVar(&a.Output, "o", "table", "shorthand for --output")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	return a, nil
}

// RunList 执行 list 命令。
func RunList(scanners []Scanner, args ListArgs) error {
	projects := ScanProjectsParallel(scanners)

	if args.Type == "session" {
		return listSessions(projects, args)
	}
	return listProjects(projects, args)
}

func listProjects(projects []Project, args ListArgs) error {
	switch args.Output {
	case "json":
		return outputJSON(projects)
	case "yaml":
		return outputYAML(projects)
	default:
		return outputTable(projects)
	}
}

func listSessions(projects []Project, args ListArgs) error {
	if args.Project == "" {
		return fmt.Errorf("session mode requires -p <project-path>")
	}

	var target *Project
	for _, p := range projects {
		if p.FullPath == args.Project {
			target = &p
			break
		}
	}
	if target == nil {
		return fmt.Errorf("project not found: %s", args.Project)
	}

	switch args.Output {
	case "json":
		return outputSessionsJSON(target)
	case "yaml":
		return outputSessionsYAML(target)
	default:
		return outputSessionsTable(target)
	}
}

// ============================================================================
// Output formats — Projects
// ============================================================================

func outputTable(projects []Project) error {
	if len(projects) == 0 {
		fmt.Println("(no sessions found)")
		return nil
	}

	// 列宽
	const (
		pathW   = 60
		agentsW = 30
		activeW = 12
	)
	divider := strings.Repeat("─", pathW+agentsW+activeW+4)

	fmt.Println(divider)
	fmt.Printf("%-*s  %-*s  %s\n", pathW, "PROJECT", agentsW, "AGENTS", "LAST ACTIVE")
	fmt.Println(divider)

	for _, p := range projects {
		path := TruncateKeepEnd(p.FullPath, pathW)

		// Count sessions per platform
		var counts [PlatformCount]int
		for _, s := range p.Sessions {
			counts[s.Platform]++
		}

		var agents []string
		platformOrder := AllPlatforms()
		for _, plat := range platformOrder {
			if n := counts[plat]; n > 0 {
				agents = append(agents, fmt.Sprintf("%s %d", plat.ShortName(), n))
			}
		}
		agentStr := strings.Join(agents, "  ")

		timeStr := RelativeTime(p.LastActive())

		fmt.Printf("%-*s  %-*s  %s\n", pathW, path, agentsW, agentStr, timeStr)
	}
	fmt.Println(divider)
	fmt.Printf("%d projects\n", len(projects))
	return nil
}

type jsonProject struct {
	Path     string        `json:"path"`
	Sessions []jsonSession `json:"sessions"`
}

type jsonSession struct {
	ID         string `json:"id"`
	Platform   string `json:"platform"`
	AgentType  string `json:"agent_type,omitempty"`
	Title      string `json:"title"`
	Model      string `json:"model"`
	LastActive string `json:"last_active"`
	MsgCount   int    `json:"msg_count"`
}

func outputJSON(projects []Project) error {
	var out []jsonProject
	for _, p := range projects {
		jp := jsonProject{Path: p.FullPath}
		for _, s := range p.Sessions {
			jp.Sessions = append(jp.Sessions, jsonSession{
				ID:         s.ID,
				Platform:   s.Platform.String(),
				AgentType:  s.AgentType,
				Title:      s.Title,
				Model:      s.Model,
				LastActive: s.LastActive.Format("2006-01-02 15:04:05"),
				MsgCount:   s.MsgCount,
			})
		}
		out = append(out, jp)
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputYAML(projects []Project) error {
	data, err := yaml.Marshal(projects)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

// ============================================================================
// Output formats — Sessions
// ============================================================================

func outputSessionsTable(proj *Project) error {
	if len(proj.Sessions) == 0 {
		fmt.Println("(no sessions)")
		return nil
	}

	const (
		titleW  = 40
		modelW  = 18
		activeW = 12
		msgW    = 6
	)
	divider := strings.Repeat("─", titleW+modelW+activeW+msgW+8)

	fmt.Println(divider)
	fmt.Printf("%-*s  %-*s  %s  %s\n", titleW, "TITLE", modelW, "MODEL", "LAST ACTIVE", "MSGS")
	fmt.Println(divider)

	for _, s := range proj.Sessions {
		title := sanitizeTitle(s.Title)
		if s.AgentType != "" && s.AgentType != "cli" {
			title += " [" + s.AgentType + "]"
		}
		title = TruncateWidth(title, titleW)
		model := TruncateWidth(s.Model, modelW)
		timeStr := RelativeTime(s.LastActive)

		fmt.Printf("%-*s  %-*s  %-*s  %*d\n",
			titleW, title, modelW, model, activeW, timeStr, msgW, s.MsgCount)
	}
	fmt.Println(divider)
	fmt.Printf("%d sessions in %s\n", len(proj.Sessions), proj.FullPath)
	return nil
}

func outputSessionsJSON(proj *Project) error {
	var sessions []jsonSession
	for _, s := range proj.Sessions {
		sessions = append(sessions, jsonSession{
			ID:         s.ID,
			Platform:   s.Platform.String(),
			AgentType:  s.AgentType,
			Title:      s.Title,
			Model:      s.Model,
			LastActive: s.LastActive.Format("2006-01-02 15:04:05"),
			MsgCount:   s.MsgCount,
		})
	}
	data, _ := json.MarshalIndent(sessions, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputSessionsYAML(proj *Project) error {
	data, err := yaml.Marshal(proj.Sessions)
	if err != nil {
		return err
	}
	fmt.Print(string(data))
	return nil
}

func sanitizeTitle(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
}

// ============================================================================
// Resume
// ============================================================================

// ResumeArgs 是 resume 命令的参数。
type ResumeArgs struct {
	SessionID string
}

// ParseResumeArgs 解析 resume 子命令参数。
func ParseResumeArgs(args []string) (ResumeArgs, error) {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	a := ResumeArgs{}
	fs.StringVar(&a.SessionID, "session", "", "session ID to resume")
	fs.StringVar(&a.SessionID, "s", "", "shorthand for --session")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	return a, nil
}

// FindSession 在项目列表中按 ID 查找 session。
func FindSession(projects []Project, id string) *Session {
	for _, p := range projects {
		for i := range p.Sessions {
			if p.Sessions[i].ID == id {
				return &p.Sessions[i]
			}
		}
	}
	return nil
}

// FindScanner 在 scanner 列表中按 platform 查找。
func FindScanner(scanners []Scanner, p Platform) Scanner {
	for _, sc := range scanners {
		if sc.Platform() == p {
			return sc
		}
	}
	return nil
}

// ============================================================================
// Delete
// ============================================================================

// DeleteArgs 是 delete 命令的参数。
type DeleteArgs struct {
	SessionID string
}

// ParseDeleteArgs 解析 delete 子命令参数。
func ParseDeleteArgs(args []string) (DeleteArgs, error) {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	a := DeleteArgs{}
	fs.StringVar(&a.SessionID, "session", "", "session ID to delete")
	fs.StringVar(&a.SessionID, "s", "", "shorthand for --session")
	if err := fs.Parse(args); err != nil {
		return a, err
	}
	return a, nil
}
