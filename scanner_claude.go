package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeScanner 扫描 ~/.claude/projects/<encoded-path>/*.jsonl 。
type ClaudeScanner struct {
	dataDir    string
	bin        string
	knownPaths []string
	customName string
}

type claudeConfig struct {
	Projects map[string]json.RawMessage `json:"projects"`
}

func NewClaudeScanner(_ Config, ov *ScannerOverrides) *ClaudeScanner {
	home, _ := os.UserHomeDir()
	dataDir := DefaultDirClaude
	if !filepath.IsAbs(dataDir) {
		dataDir = filepath.Join(home, dataDir)
	}
	s := &ClaudeScanner{dataDir: dataDir, bin: DefaultBinClaude}
	s.loadKnownPaths()
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

func (s *ClaudeScanner) loadKnownPaths() {
	data, err := os.ReadFile(filepath.Join(s.dataDir, ".claude.json"))
	if err != nil {
		return
	}
	var cfg claudeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	for path := range cfg.Projects {
		s.knownPaths = append(s.knownPaths, path)
	}
}

func (s *ClaudeScanner) Platform() Platform { return PlatformClaude }
func (s *ClaudeScanner) DataDir() string    { return s.dataDir }

func (s *ClaudeScanner) DisplayName() string {
	if s.customName != "" {
		return s.customName
	}
	return s.Platform().String()
}

func (s *ClaudeScanner) SetCustomName(name string) {
	s.customName = name
}

func claudeEncodePath(path string) string {
	trimmed := strings.TrimPrefix(path, "/")
	result := strings.ReplaceAll(trimmed, "/", "-")
	result = strings.ReplaceAll(result, ".", "-")
	result = strings.ReplaceAll(result, "_", "-")
	return "-" + result
}

func (s *ClaudeScanner) decodeDirName(encoded string) string {
	for _, path := range s.knownPaths {
		if claudeEncodePath(path) == encoded {
			return path
		}
	}
	trimmed := strings.TrimPrefix(encoded, "-")
	return "/" + strings.ReplaceAll(trimmed, "-", "/")
}

func (s *ClaudeScanner) ScanProjects() ([]Project, error) {
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

type claudeLine struct {
	Type      string          `json:"type"`
	UUID      string          `json:"uuid"`
	Timestamp string          `json:"timestamp"`
	Message   json.RawMessage `json:"message"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"`
}

type claudeContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *ClaudeScanner) scanSessions(projectDir, encodedName, fullPath string) ([]Session, error) {
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

func (s *ClaudeScanner) parseSession(filePath, sid, encodedName, fullPath string) (Session, error) {
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

	var title, model string
	var lastActive time.Time
	var firstUserDone bool
	msgCount := 0

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, scanBufferSize), scanBufferSize)
	for sc.Scan() {
		var line claudeLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		if line.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, line.Timestamp); err == nil {
				if t.After(lastActive) {
					lastActive = t
				}
			}
		}
		if line.Message == nil {
			continue
		}
		var msg claudeMessage
		if err := json.Unmarshal(line.Message, &msg); err != nil {
			continue
		}
		if msg.Role == roleUser || msg.Role == roleAssistant {
			msgCount++
		}
		if msg.Role == roleUser && !firstUserDone {
			firstUserDone = true
			title = extractClaudeText(msg.Content)
			title = Truncate(title, 50)
		}
		if msg.Role == roleAssistant && msg.Model != "" {
			model = msg.Model
		}
	}

	displayTitle := title
	if displayTitle == "" {
		displayTitle = untitledTitle
	}

	return Session{
		ID:          sid,
		Platform:    PlatformClaude,
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

func extractClaudeText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	var blocks []claudeContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

func (s *ClaudeScanner) DeleteSession(sess Session) error {
	base := filepath.Join(s.dataDir, "projects", sess.ProjectDir)
	_ = os.Remove(filepath.Join(base, sess.ID+".jsonl"))
	_ = os.RemoveAll(filepath.Join(base, sess.ID))
	return nil
}

func (s *ClaudeScanner) DeleteProject(p Project) error {
	if len(p.Sessions) == 0 {
		return nil
	}
	projectDir := filepath.Join(s.dataDir, "projects", p.Sessions[0].ProjectDir)
	return os.RemoveAll(projectDir)
}

func (s *ClaudeScanner) ResumeCmd(sess Session) []string {
	return []string{s.bin, flagResume, sess.ResumeArg}
}
