package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// OpenCodeScanner scans OpenCode sessions from its SQLite database.
// DB path is obtained via `opencode db path`.
type OpenCodeScanner struct {
	bin        string
	dbPath     string
	customName string
}

// opencodeModel represents the JSON model field in session table
type opencodeModel struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
}

func NewOpenCodeScanner(_ Config, ov *ScannerOverrides) *OpenCodeScanner {
	bin := DefaultBinOpenCode
	if ov != nil {
		if ov.Bin != "" {
			bin = ov.Bin
		}
	}
	dbPath := getOpenCodeDBPath(bin)
	return &OpenCodeScanner{
		bin:    bin,
		dbPath: dbPath,
	}
}

// getOpenCodeDBPath runs `bin db path` to obtain the SQLite database path.
func getOpenCodeDBPath(bin string) string {
	out, err := exec.CommandContext(context.Background(), bin, "db", "path").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *OpenCodeScanner) Platform() Platform { return PlatformOpenCode }
func (s *OpenCodeScanner) DataDir() string {
	if s.dbPath == "" {
		return ""
	}
	return filepath.Dir(s.dbPath)
}

func (s *OpenCodeScanner) DisplayName() string {
	if s.customName != "" {
		return s.customName
	}
	return s.Platform().String()
}

func (s *OpenCodeScanner) SetCustomName(name string) {
	s.customName = name
}

func (s *OpenCodeScanner) ScanProjects() ([]Project, error) {
	if s.dbPath == "" {
		return nil, nil
	}

	db, err := sql.Open("sqlite", s.dbPath+"?mode=ro")
	if err != nil {
		return nil, nil
	}
	defer func() { _ = db.Close() }()

	// Verify the database is accessible
	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		return nil, nil
	}

	query := `
		SELECT s.id, s.title, s.model, s.time_updated, s.time_created, p.worktree,
		       (SELECT COUNT(*) FROM session_message sm WHERE sm.session_id = s.id) AS msg_count
		FROM session s
		JOIN project p ON s.project_id = p.id
		ORDER BY s.time_updated DESC
	`

	rows, err := db.QueryContext(context.Background(), query)
	if err != nil {
		return nil, nil
	}
	defer func() { _ = rows.Close() }()

	// Group sessions by worktree
	projectMap := make(map[string][]Session)
	var worktreeOrder []string

	for rows.Next() {
		var id, title, modelJSON, worktree string
		var timeUpdated, timeCreated int64
		var msgCount int

		if err := rows.Scan(&id, &title, &modelJSON, &timeUpdated, &timeCreated, &worktree, &msgCount); err != nil {
			continue
		}

		model := parseOpenCodeModel(modelJSON)
		lastActive := time.Unix(timeUpdated, 0)
		if created := time.Unix(timeCreated, 0); created.After(lastActive) {
			lastActive = created
		}

		if title == "" {
			title = untitledTitle
		}

		sess := Session{
			ID:          id,
			Platform:    PlatformOpenCode,
			Title:       Truncate(title, 50),
			Model:       model,
			LastActive:  lastActive,
			MsgCount:    msgCount,
			ProjectDir:  worktree,
			FilePath:    fmt.Sprintf("%s#%s", s.dbPath, id),
			ResumeArg:   id,
			ProjectPath: worktree,
		}

		if _, exists := projectMap[worktree]; !exists {
			worktreeOrder = append(worktreeOrder, worktree)
		}
		projectMap[worktree] = append(projectMap[worktree], sess)
	}

	var projects []Project
	for _, wt := range worktreeOrder {
		projects = append(projects, Project{
			Name:     ProjectShortName(wt),
			FullPath: wt,
			Sessions: projectMap[wt],
		})
	}
	return projects, nil
}

// parseOpenCodeModel extracts the model ID from the JSON model field.
func parseOpenCodeModel(modelJSON string) string {
	var m opencodeModel
	if err := json.Unmarshal([]byte(modelJSON), &m); err == nil && m.ID != "" {
		return m.ID
	}
	return modelJSON
}

func (s *OpenCodeScanner) DeleteSession(sess Session) error {
	db, err := sql.Open("sqlite", s.dbPath+"?mode=rw")
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Delete messages first (foreign key constraint)
	if _, err := tx.ExecContext(context.Background(), "DELETE FROM session_message WHERE session_id = ?", sess.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(context.Background(), "DELETE FROM session WHERE id = ?", sess.ID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *OpenCodeScanner) DeleteProject(p Project) error {
	for _, sess := range p.Sessions {
		_ = s.DeleteSession(sess)
	}
	return nil
}

func (s *OpenCodeScanner) ResumeCmd(sess Session) []string {
	return []string{s.bin, "-s", sess.ResumeArg}
}
