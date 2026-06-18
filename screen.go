package main

import tea "charm.land/bubbletea/v2"

// Screen 代表 TUI 中的一个交互屏幕（如项目列表、Session 列表、删除确认）。
// 每个 Screen 自包含其状态、事件处理和渲染逻辑。
type Screen interface {
	// Name 返回 Screen 名称，用于日志和调试。
	Name() string

	// HandleKey 处理键盘事件。返回的 Cmd 会被 App 转发给 bubbletea。
	// 如需切换 Screen（push/pop），可返回特殊的 ContextCmd。
	HandleKey(msg tea.KeyPressMsg) tea.Cmd

	// View 渲染当前 Screen，参数为终端宽高。
	View(width, height int) string

	// OnEnter 在 Screen 被 push 到栈顶时调用。
	OnEnter()

	// OnExit 在 Screen 被 pop 时调用。
	OnExit()
}

// ContextCmd 用于 Screen.HandleKey 返回时请求 Context 栈操作。
// 它不是 bubbletea 的 Cmd，而是包装了实际 Cmd + 栈操作意图。
type ContextCmd struct {
	Cmd  tea.Cmd // 实际的 bubbletea Cmd（可为 nil）
	Push Screen  // 非 nil 时 push 新 Screen
	Pop  bool    // true 时 pop 当前 Screen
}

// NewContextCmd 创建带 Cmd 的 ContextCmd。
func NewContextCmd(cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		return &ContextCmd{Cmd: cmd}
	}
}

// PushCmd 创建 push 新 Screen 的 ContextCmd。
func PushCmd(ctx Screen) tea.Cmd {
	return func() tea.Msg {
		return &ContextCmd{Push: ctx}
	}
}

// PopCmd 创建 pop 当前 Screen 的 ContextCmd。
func PopCmd() tea.Cmd {
	return func() tea.Msg {
		return &ContextCmd{Pop: true}
	}
}

// PushAndCmd 创建 push Screen + 执行 Cmd 的 ContextCmd。
func PushAndCmd(ctx Screen, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		return &ContextCmd{Push: ctx, Cmd: cmd}
	}
}
