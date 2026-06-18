package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CodeBuddyScanner 扫描 ~/.codebuddy/
type CodeBuddyScanner struct {
	dataDir     string
	bin         string
	trustedDirs []string // 从 settings.json trustedDirectories 读取，用于精确还原编码路径
	customName  string
}

// cbSettings 对应 ~/.codebuddy/settings.json
type cbSettings struct {
	TrustedDirectories []string `json:"trustedDirectories"`
}

func NewCodeBuddyScanner(_ Config, ov *ScannerOverrides) *CodeBuddyScanner {
	home, _ := os.UserHomeDir()
	dataDir := DefaultDirCodeBuddy
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(home, dataDir)
	}
	s := &CodeBuddyScanner{
		dataDir: dataDir,
		bin:     DefaultBinCodeBuddy,
	}
	s.loadTrustedDirs()
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

func (s *CodeBuddyScanner) loadTrustedDirs() {
	data, err := os.ReadFile(filepath.Join(s.dataDir, "settings.json"))
	if err != nil {
		return
	}
	var settings cbSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}
	// 过滤掉通配符条目（如 /path/**）
	for _, d := range settings.TrustedDirectories {
		if !strings.Contains(d, "*") {
			s.trustedDirs = append(s.trustedDirs, d)
		}
	}
}

func (s *CodeBuddyScanner) Platform() Platform { return PlatformCodeBuddy }
func (s *CodeBuddyScanner) DataDir() string    { return s.dataDir }

func (s *CodeBuddyScanner) DisplayName() string {
	if s.customName != "" {
		return s.customName
	}
	return s.Platform().String()
}

func (s *CodeBuddyScanner) SetCustomName(name string) {
	s.customName = name
}

// encodePath 将绝对路径编码为 CodeBuddy 目录名格式
// /home/user/code → home-user-code
func encodePath(path string) string {
	// 去掉前缀 /
	path = strings.TrimPrefix(path, "/")
	return strings.ReplaceAll(path, "/", "-")
}

// decodeDirName 将 CodeBuddy 编码的目录名还原为真实路径
// 策略：用 trustedDirs 做精确匹配（路径编码后与 encoded 比对），
// 取最长匹配；无匹配时 fallback 为简单 - → / 替换
func (s *CodeBuddyScanner) decodeDirName(encoded string) string {
	var bestMatch string
	for _, dir := range s.trustedDirs {
		enc := encodePath(dir)
		if enc == encoded {
			// 完全匹配，直接返回
			return dir
		}
		// 前缀匹配：encoded 以某个已知路径的编码开头
		if strings.HasPrefix(encoded, enc+"-") || strings.HasPrefix(encoded, enc) {
			if len(dir) > len(bestMatch) {
				bestMatch = dir
			}
		}
	}
	if bestMatch != "" && encodePath(bestMatch) == encoded {
		return bestMatch
	}
	// fallback：简单替换
	return "/" + strings.ReplaceAll(encoded, "-", "/")
}

func (s *CodeBuddyScanner) ScanProjects() ([]Project, error) {
	projectsDir := filepath.Join(s.dataDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var projects []Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fullPath := s.decodeDirName(entry.Name())
		sessions, err := s.scanSessions(filepath.Join(projectsDir, entry.Name()), entry.Name(), fullPath)
		if err != nil {
			continue
		}
		if len(sessions) == 0 {
			continue
		}
		projects = append(projects, Project{
			Name:     ProjectShortName(fullPath),
			FullPath: fullPath,
			Sessions: sessions,
		})
	}
	return projects, nil
}

// cbLine 用于解析 CodeBuddy JSONL 行
type cbLine struct {
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Timestamp    int64           `json:"timestamp"`
	AITitle      string          `json:"aiTitle"`
	SessionID    string          `json:"sessionId"`
	ProviderData json.RawMessage `json:"providerData"`
}

type cbProviderData struct {
	Model string `json:"model"`
	Agent string `json:"agent"`
}

func (s *CodeBuddyScanner) scanSessions(projectDir, encodedName, fullPath string) ([]Session, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		sid := strings.TrimSuffix(entry.Name(), ".jsonl")
		session, err := s.parseSession(filepath.Join(projectDir, entry.Name()), sid, encodedName, fullPath)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *CodeBuddyScanner) parseSession(filePath, sid, encodedName, fullPath string) (Session, error) {
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

	var title, model, agentType string
	var lastActive time.Time
	msgCount := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, scanBufferSize), scanBufferSize)
	for scanner.Scan() {
		var line cbLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		// 时间戳（毫秒 Unix）
		if line.Timestamp > 0 {
			t := time.UnixMilli(line.Timestamp)
			if t.After(lastActive) {
				lastActive = t
			}
		}
		// 标题
		if line.Type == "ai-title" && line.AITitle != "" {
			title = line.AITitle
		}
		// 消息计数（只计 user/assistant）
		if line.Type == typeMessage && (line.Role == roleUser || line.Role == roleAssistant) {
			msgCount++
		}
		// 模型和 agent（从最后一条 assistant 消息取）
		if line.Type == typeMessage && line.Role == roleAssistant && line.ProviderData != nil {
			var pd cbProviderData
			if err := json.Unmarshal(line.ProviderData, &pd); err == nil {
				if pd.Model != "" {
					model = pd.Model
				}
				if pd.Agent != "" {
					agentType = pd.Agent
				}
			}
		}
	}

	displayTitle := title
	if displayTitle == "" {
		displayTitle = untitledTitle
	}

	return Session{
		ID:          sid,
		Platform:    PlatformCodeBuddy,
		AgentType:   agentType,
		Title:       displayTitle,
		Model:       model,
		LastActive:  lastActive,
		MsgCount:    msgCount,
		SizeBytes:   sizeBytes,
		ProjectDir:  encodedName,
		FilePath:    filePath,
		ResumeArg:   sid,
		ProjectPath: fullPath,
	}, nil
}

func (s *CodeBuddyScanner) DeleteSession(sess Session) error {
	base := filepath.Join(s.dataDir, "projects", sess.ProjectDir)
	_ = os.Remove(filepath.Join(base, sess.ID+".jsonl"))
	_ = os.RemoveAll(filepath.Join(base, sess.ID))
	_ = os.RemoveAll(filepath.Join(s.dataDir, "tasks", sess.ID))
	_ = os.RemoveAll(filepath.Join(s.dataDir, "file-history", sess.ID))
	return nil
}

func (s *CodeBuddyScanner) DeleteProject(p Project) error {
	if len(p.Sessions) == 0 {
		return nil
	}
	projectDir := filepath.Join(s.dataDir, "projects", p.Sessions[0].ProjectDir)
	_ = os.RemoveAll(projectDir)
	for _, sess := range p.Sessions {
		_ = os.RemoveAll(filepath.Join(s.dataDir, "tasks", sess.ID))
		_ = os.RemoveAll(filepath.Join(s.dataDir, "file-history", sess.ID))
	}
	return nil
}

func (s *CodeBuddyScanner) ResumeCmd(sess Session) []string {
	return []string{s.bin, flagResume, sess.ResumeArg}
}
