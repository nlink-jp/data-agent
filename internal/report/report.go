package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nlink-jp/data-agent/internal/session"
)

// Report holds a generated analysis report.
type Report struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	SessionID string    `json:"session_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"` // Markdown
	CreatedAt time.Time `json:"created_at"`
}

// GenerateFromSession creates a Markdown report from a completed session.
func GenerateFromSession(sess *session.Session) (*Report, error) {
	if sess.Plan == nil {
		return nil, fmt.Errorf("session has no plan")
	}

	var sb strings.Builder

	// Header
	fmt.Fprintf(&sb, "# Analysis Report: %s\n\n", sess.Plan.Objective)
	fmt.Fprintf(&sb, "> Session: %s\n", sess.ID)
	fmt.Fprintf(&sb, "> Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// 1. Investigation Plan
	sb.WriteString("## 1. Investigation Plan\n\n")
	fmt.Fprintf(&sb, "**Objective:** %s\n\n", sess.Plan.Objective)

	for _, p := range sess.Plan.Perspectives {
		fmt.Fprintf(&sb, "### %s: %s\n\n", p.ID, p.Description)
		fmt.Fprintf(&sb, "Status: %s\n\n", p.Status)
		sb.WriteString("| Step | Type | Description | Status |\n")
		sb.WriteString("|------|------|-------------|--------|\n")
		for _, s := range p.Steps {
			deps := ""
			if len(s.DependsOn) > 0 {
				deps = " (depends: " + strings.Join(s.DependsOn, ", ") + ")"
			}
			fmt.Fprintf(&sb, "| %s | %s | %s%s | %s |\n", s.ID, s.Type, s.Description, deps, s.Status)
		}
		sb.WriteString("\n")
	}

	// Plan revision history
	if len(sess.Plan.History) > 0 {
		sb.WriteString("### Plan Revision History\n\n")
		for _, rev := range sess.Plan.History {
			fmt.Fprintf(&sb, "- **v%d** (%s): %s — %s\n", rev.Version, rev.Timestamp.Format("15:04:05"), rev.Reason, rev.Changes)
		}
		sb.WriteString("\n")
	}

	// 2. Execution Record
	sb.WriteString("## 2. Execution Record\n\n")
	if len(sess.ExecLog) == 0 {
		sb.WriteString("No executions recorded.\n\n")
	} else {
		for _, entry := range sess.ExecLog {
			fmt.Fprintf(&sb, "### Step %s (%s)\n\n", entry.StepID, entry.Type)
			if entry.SQL != "" {
				fmt.Fprintf(&sb, "```sql\n%s\n```\n\n", entry.SQL)
			}
			if entry.Result != nil {
				fmt.Fprintf(&sb, "**Result:** %s\n\n", entry.Result.Summary)
			}
			if entry.Error != "" {
				fmt.Fprintf(&sb, "**Error:** %s\n\n", entry.Error)
				if entry.Decision != "" {
					fmt.Fprintf(&sb, "**Decision:** %s\n\n", entry.Decision)
				}
			}
			fmt.Fprintf(&sb, "Duration: %s | Plan v%d\n\n", entry.Duration, entry.PlanVersion)
		}
	}

	// 3. Findings
	sb.WriteString("## 3. Findings\n\n")
	if len(sess.Findings) == 0 {
		sb.WriteString("No findings recorded.\n\n")
	} else {
		sb.WriteString("| ID | Severity | Description | Step |\n")
		sb.WriteString("|----|----------|-------------|------|\n")
		for _, f := range sess.Findings {
			fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", f.ID, f.Severity, f.Description, f.StepID)
		}
		sb.WriteString("\n")
	}

	// 4. Metadata
	sb.WriteString("## 4. Metadata\n\n")
	fmt.Fprintf(&sb, "- Session ID: %s\n", sess.ID)
	fmt.Fprintf(&sb, "- Case ID: %s\n", sess.CaseID)
	fmt.Fprintf(&sb, "- Created: %s\n", sess.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "- Completed: %s\n", sess.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "- Plan version: %d\n", sess.Plan.Version)
	fmt.Fprintf(&sb, "- Total executions: %d\n", len(sess.ExecLog))
	fmt.Fprintf(&sb, "- Total findings: %d\n", len(sess.Findings))

	return &Report{
		ID:        uuid.New().String(),
		CaseID:    sess.CaseID,
		SessionID: sess.ID,
		Title:     sess.Plan.Objective,
		Content:   sb.String(),
		CreatedAt: time.Now(),
	}, nil
}

// SaveToCase persists the report in the case's reports directory.
func (r *Report) SaveToCase(reportsDir string) error {
	if err := os.MkdirAll(reportsDir, 0o700); err != nil {
		return fmt.Errorf("create reports dir: %w", err)
	}

	// Save metadata
	metaPath := filepath.Join(reportsDir, r.ID+".json")
	metaData, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0o600); err != nil {
		return err
	}

	// Save markdown
	mdPath := filepath.Join(reportsDir, r.ID+".md")
	return os.WriteFile(mdPath, []byte(r.Content), 0o600)
}

// ExportFile writes the report markdown to the given path.
func (r *Report) ExportFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(r.Content), 0o600)
}

// ListReports returns all reports in a reports directory.
func ListReports(reportsDir string) ([]Report, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var reports []Report
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(reportsDir, e.Name()))
		if err != nil {
			continue
		}
		var r Report
		if json.Unmarshal(data, &r) == nil {
			reports = append(reports, r)
		}
	}
	return reports, nil
}
