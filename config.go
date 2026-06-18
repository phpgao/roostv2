// Package main 处理 ~/.roost/roost.yaml 配置。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ResumeMode 控制 resume 的行为。
type ResumeMode string

const (
	ResumeModeReplace ResumeMode = "replace"
	ResumeModeSuspend ResumeMode = "suspend"
)

// PlatformConfig 单个平台的配置 — 只保留 args。
type PlatformConfig struct {
	Args []string `yaml:"args"`
}

// AppConfig 根配置。
type AppConfig struct {
	ResumeMode   ResumeMode                `yaml:"resume_mode"`
	Platforms    map[string]PlatformConfig `yaml:"platforms"`
	CustomAgents []CustomAgent             `yaml:"custom_agents"`
}

// CustomAgent 自定义 agent，复用内置平台的解析逻辑。
type CustomAgent struct {
	Name    string   `yaml:"name"`     // 显示名
	Type    string   `yaml:"type"`     // 解析类型: claude/gemini/codebuddy/...
	Bin     string   `yaml:"bin"`      // 二进制名
	DataDir string   `yaml:"data_dir"` // 数据目录
	Args    []string `yaml:"args"`     // 额外参数
}

const defaultTemplate = `# roost configuration

# resume mode:
#   replace - process replacement, returns to shell after agent exits (default)
#   suspend - subprocess mode, returns to roost TUI after agent exits
resume_mode: replace

platforms:
  codebuddy:
    # args: [-y]
  claude:
    # args: [--dangerously-skip-permissions]
  gemini:
    # args: [-y]
  codex:
    # args: [--full-auto]
  copilot:
    # args: []
  opencode:
    # args: []
`

func LoadConfig() AppConfig {
	cfg := defaultConfig()
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			writeDefault(path)
			return cfg
		}
		fmt.Fprintf(os.Stderr, "warning: cannot read config: %v\n", err)
		return cfg
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid config: %v\n", err)
	}
	return cfg
}

func (c AppConfig) GetResumeMode() ResumeMode {
	if c.ResumeMode == ResumeModeSuspend {
		return ResumeModeSuspend
	}
	return ResumeModeReplace
}

// ArgsFor 返回指定平台的额外参数。
func (c AppConfig) ArgsFor(p Platform) []string {
	name := platformKey(p)
	if pc, ok := c.Platforms[name]; ok {
		return pc.Args
	}
	return nil
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".roost", "roost.yaml")
}

func defaultConfig() AppConfig {
	return AppConfig{
		ResumeMode: ResumeModeReplace,
		Platforms:  make(map[string]PlatformConfig),
	}
}

func writeDefault(path string) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(defaultTemplate), 0o644)
}

func platformKey(p Platform) string {
	switch p {
	case PlatformCodeBuddy:
		return "codebuddy"
	case PlatformClaude:
		return "claude"
	case PlatformGemini:
		return "gemini"
	case PlatformCodex:
		return "codex"
	case PlatformCopilot:
		return "copilot"
	case PlatformOpenCode:
		return "opencode"
	default:
		return ""
	}
}
