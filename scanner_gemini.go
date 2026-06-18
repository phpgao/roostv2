package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GeminiScanner 扫描 ~/.gemini/
type GeminiScanner struct {
	dataDir    string
	bin        string
	customName string
}

func NewGeminiScanner(_ Config, ov *ScannerOverrides) *GeminiScanner {
	dataDir := DefaultDirGemini
	if !filepath.IsAbs(dataDir) {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, dataDir)
	}
	s := &GeminiScanner{
		dataDir: dataDir,
		bin:     DefaultBinGemini,
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

func (s *GeminiScanner) Platform() Platform { return PlatformGemini }
func (s *GeminiScanner) DataDir() string    { return s.dataDir }

func (s *GeminiScanner) DisplayName() string {
	if s.customName != "" {
		return s.customName
	}
	return s.Platform().String()
}

func (s *GeminiScanner) SetCustomName(name string) {
	s.customName = name
}

type geminiProjects struct {
	Projects map[string]string `json:"projects"`
}

type geminiSession struct {
	SessionID   string          `json:"sessionId"`
	StartTime   string          `json:"startTime"`
	LastUpdated string          `json:"lastUpdated"`
	Messages    []geminiMessage `json:"messages"`
}

type geminiMessage struct {
	Type    string          `json:"type"`
	Content json.RawMessage `json:"content"`
	Model   string          `json:"model"`
}

func (s *GeminiScanner) ScanProjects() ([]Project, error) {
	projectsFile := filepath.Join(s.dataDir, "projects.json")
	data, err := os.ReadFile(projectsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var gp geminiProjects
	if err := json.Unmarshal(data, &gp); err != nil {
		return nil, err
	}

	var projects []Project
	for fullPath, name := range gp.Projects {
		chatsDir := filepath.Join(s.dataDir, "tmp", name, "chats")
		sessions, err := s.scanSessions(chatsDir, name, fullPath)
		if err != nil || len(sessions) == 0 {
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

func (s *GeminiScanner) scanSessions(chatsDir, projectName, fullPath string) ([]Session, error) {
	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		session, err := s.parseSession(filepath.Join(chatsDir, entry.Name()), projectName, fullPath)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *GeminiScanner) parseSession(filePath, projectName, fullPath string) (Session, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Session{}, err
	}

	var gs geminiSession
	if err := json.Unmarshal(data, &gs); err != nil {
		return Session{}, err
	}

	var lastActive time.Time
	if gs.LastUpdated != "" {
		if t, err := time.Parse(time.RFC3339, gs.LastUpdated); err == nil {
			lastActive = t
		}
	}
	if lastActive.IsZero() && gs.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, gs.StartTime); err == nil {
			lastActive = t
		}
	}

	var title string
	for _, msg := range gs.Messages {
		if msg.Type == roleUser {
			title = Truncate(extractGeminiText(msg.Content), 50)
			break
		}
	}

	var model string
	for i := len(gs.Messages) - 1; i >= 0; i-- {
		if gs.Messages[i].Type == typeGemini && gs.Messages[i].Model != "" {
			model = gs.Messages[i].Model
			break
		}
	}

	displayTitle := title
	if displayTitle == "" {
		displayTitle = untitledTitle
	}

	return Session{
		ID:          gs.SessionID,
		Platform:    PlatformGemini,
		Title:       displayTitle,
		Model:       model,
		LastActive:  lastActive,
		MsgCount:    len(gs.Messages),
		SizeBytes:   int64(len(data)),
		ProjectDir:  projectName,
		FilePath:    filePath,
		ResumeArg:   gs.SessionID,
		ProjectPath: fullPath,
	}, nil
}

func extractGeminiText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		for _, b := range blocks {
			if b.Text != "" {
				return b.Text
			}
		}
	}
	return ""
}

func (s *GeminiScanner) DeleteSession(sess Session) error {
	return os.Remove(sess.FilePath)
}

func (s *GeminiScanner) DeleteProject(p Project) error {
	if len(p.Sessions) == 0 {
		return nil
	}
	chatsDir := filepath.Dir(p.Sessions[0].FilePath)
	projectDir := filepath.Dir(chatsDir)
	return os.RemoveAll(projectDir)
}

func (s *GeminiScanner) ResumeCmd(sess Session) []string {
	return []string{s.bin, flagResume, sess.ResumeArg}
}
