package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CodexScanner 扫描 ~/.codex/sessions/
type CodexScanner struct {
	dataDir    string
	bin        string
	customName string
}

func NewCodexScanner(_ Config, ov *ScannerOverrides) *CodexScanner {
	dataDir := DefaultDirCodex
	if !filepath.IsAbs(dataDir) {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, dataDir)
	}
	s := &CodexScanner{
		dataDir: dataDir,
		bin:     DefaultBinCodex,
	}
	if ov != nil {
		if ov.Bin != "" {
			s.bin = ov.Bin
		}
		if ov.DataDir != "" {
			s.dataDir = ov.DataDir
		}
	}
	return s
}

func (s *CodexScanner) Platform() Platform { return PlatformCodex }
func (s *CodexScanner) DataDir() string    { return s.dataDir }

func (s *CodexScanner) DisplayName() string {
	if s.customName != "" {
		return s.customName
	}
	return s.Platform().String()
}

func (s *CodexScanner) SetCustomName(name string) {
	s.customName = name
}

// codexSessionMeta 对应 session_meta 行的 payload
type codexSessionMeta struct {
	ID            string `json:"id"`
	CWD           string `json:"cwd"`
	Timestamp     string `json:"timestamp"`
	ModelProvider string `json:"model_provider"`
	CLIVersion    string `json:"cli_version"`
}

// codexLine 通用 JSONL 行结构
type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// codexEventPayload 用于解析 event_msg payload
type codexEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// codexTurnContext 用于解析 turn_context payload 获取 model
type codexTurnContext struct {
	Model string `json:"model"`
}

func (s *CodexScanner) ScanProjects() ([]Project, error) {
	sessionsDir := filepath.Join(s.dataDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	// 按 cwd 聚合 sessions
	projectMap := make(map[string][]Session)

	// 遍历 sessions/YYYY/MM/DD/*.jsonl
	err := filepath.WalkDir(sessionsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		sess, err := s.parseSession(path)
		if err != nil {
			return nil // 跳过解析失败的文件
		}
		projectMap[sess.ProjectPath] = append(projectMap[sess.ProjectPath], sess)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var projects []Project
	for fullPath, sessions := range projectMap {
		projects = append(projects, Project{
			Name:     ProjectShortName(fullPath),
			FullPath: fullPath,
			Sessions: sessions,
		})
	}
	return projects, nil
}

func (s *CodexScanner) parseSession(filePath string) (Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = f.Close() }()

	info, _ := f.Stat()
	var sizeBytes int64
	if info != nil {
		sizeBytes = info.Size()
	}

	var meta codexSessionMeta
	var title, model string
	var lastActive time.Time
	var firstUserDone bool
	msgCount := 0

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, scanBufferSize), scanBufferSize)

	for sc.Scan() {
		var line codexLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}

		// 更新最后活跃时间
		if line.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
				if t.After(lastActive) {
					lastActive = t
				}
			}
		}

		switch line.Type {
		case typeSessionMeta:
			_ = json.Unmarshal(line.Payload, &meta)

		case typeEventMsg:
			var ev codexEventPayload
			if err := json.Unmarshal(line.Payload, &ev); err != nil {
				continue
			}
			switch ev.Type {
			case typeUserMessage:
				msgCount++
				if !firstUserDone {
					firstUserDone = true
					// 取第一行作为标题
					msg := ev.Message
					if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
						msg = msg[:idx]
					}
					title = Truncate(strings.TrimSpace(msg), 50)
				}
			case "agent_message":
				msgCount++
			}

		case "turn_context":
			var tc codexTurnContext
			if err := json.Unmarshal(line.Payload, &tc); err == nil && tc.Model != "" {
				model = tc.Model
			}
		}
	}

	if meta.ID == "" {
		return Session{}, os.ErrInvalid
	}

	displayTitle := title
	if displayTitle == "" {
		displayTitle = untitledTitle
	}

	return Session{
		ID:          meta.ID,
		Platform:    PlatformCodex,
		Title:       displayTitle,
		Model:       model,
		LastActive:  lastActive,
		MsgCount:    msgCount,
		SizeBytes:   sizeBytes,
		FilePath:    filePath,
		ResumeArg:   meta.ID,
		ProjectPath: meta.CWD,
	}, nil
}

func (s *CodexScanner) DeleteSession(sess Session) error {
	return os.Remove(sess.FilePath)
}

func (s *CodexScanner) DeleteProject(p Project) error {
	for _, sess := range p.Sessions {
		_ = os.Remove(sess.FilePath)
	}
	return nil
}

func (s *CodexScanner) ResumeCmd(sess Session) []string {
	return []string{s.bin, codexCmdResume, sess.ResumeArg}
}
