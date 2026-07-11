package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v3"
)

// checklistResponse mirrors operations.ChecklistResult (GET /api/checklist).
type checklistResponse struct {
	GeneratedAt string `json:"generatedAt"`
	Health      struct {
		Overall     string `json:"overall"`
		CheckedAt   string `json:"checkedAt"`
		HostHealthy bool   `json:"hostHealthy"`
		FailDetails string `json:"failDetails"`
	} `json:"health"`
	Instances []struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		Status         string `json:"status"`
		Health         string `json:"health"`
		ParameterGroup string `json:"parameterGroup"`
		BackupCron     *struct {
			UTCHour           int `json:"utcHour"`
			CleanupLocalDays  int `json:"cleanupLocalDays"`
			CleanupRemoteDays int `json:"cleanupRemoteDays"`
		} `json:"backupCron"`
		LastGoodBackup *struct {
			ID        int    `json:"id"`
			Timestamp string `json:"timestamp"`
			SizeBytes int64  `json:"sizeBytes"`
			Location  string `json:"location"`
			Comment   string `json:"comment"`
		} `json:"lastGoodBackup"`
		CompletedBackups int `json:"completedBackups"`
		BackupCopies     struct {
			Both   int `json:"both"`
			Remote int `json:"remote"`
			Local  int `json:"local"`
			None   int `json:"none"`
		} `json:"backupCopies"`
	} `json:"instances"`
	Notifications struct {
		Configured []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"configured"`
		LastEvent *checklistNotificationEvent `json:"lastEvent"`
		LastError *checklistNotificationEvent `json:"lastError"`
	} `json:"notifications"`
}

type checklistNotificationEvent struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	CreatedAt string `json:"createdAt"`
}

func (c *Client) checklistAction(ctx context.Context, cmd *cli.Command) error {
	resp, err := c.request("GET", "/api/checklist", nil)
	if err != nil {
		return err
	}

	if cmd.Bool("json") {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, resp, "", "  "); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
		_, _ = fmt.Fprintln(c.out, pretty.String())
		return nil
	}

	var result checklistResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	out := c.out
	_, _ = fmt.Fprintf(out, "ODDK checklist — generated %s\n\n", result.GeneratedAt)

	// Overall (daemon-wide) health line.
	if result.Health.CheckedAt != "" {
		_, _ = fmt.Fprintf(out, "Overall health: %s (last check %s)\n", result.Health.Overall, result.Health.CheckedAt)
	} else {
		_, _ = fmt.Fprintf(out, "Overall health: %s (no health checks recorded yet)\n", result.Health.Overall)
	}
	if result.Health.FailDetails != "" {
		_, _ = fmt.Fprintf(out, "  %s %s\n", glyphBad, result.Health.FailDetails)
	}
	_, _ = fmt.Fprintln(out)

	// Per-instance blocks. Most hosts run one or a few instances, so a detailed
	// vertical block per instance reads better than a wide table.
	if len(result.Instances) == 0 {
		_, _ = fmt.Fprintln(out, "Instances: none found")
	} else {
		_, _ = fmt.Fprintf(out, "Instances (%d)\n", len(result.Instances))

		// detail prints one aligned "  <glyph> <label>  <value>" line under a
		// block; an empty glyph renders as a blank column so labels stay aligned.
		detail := func(glyph, label, value string) {
			if glyph == "" {
				glyph = " "
			}
			_, _ = fmt.Fprintf(out, "  %s %-16s %s\n", glyph, label, value)
		}

		for _, inst := range result.Instances {
			_, _ = fmt.Fprintln(out)

			// Header: status glyph + name · PostgreSQL <ver> · <status>.
			header := inst.Name
			if inst.Version != "" {
				header += " · PostgreSQL " + inst.Version
			}
			if inst.Status != "" {
				header += " · " + inst.Status
			}
			_, _ = fmt.Fprintf(out, "%s %s\n", statusGlyph(inst.Status), header)

			detail(healthGlyph(inst.Health), "health", inst.Health)

			paramGroup := inst.ParameterGroup
			if paramGroup == "" {
				paramGroup = "(none)"
			}
			detail(glyphOK, "parameter group", paramGroup)

			if inst.BackupCron != nil {
				detail(glyphOK, "daily backup", fmt.Sprintf("%02d:00 UTC · keep local %dd, remote %dd",
					inst.BackupCron.UTCHour, inst.BackupCron.CleanupLocalDays, inst.BackupCron.CleanupRemoteDays))
			} else {
				detail(glyphTodo, "daily backup", "not scheduled")
			}

			if b := inst.LastGoodBackup; b != nil {
				val := fmt.Sprintf("%s · %s · %s · #%d", b.Timestamp, formatSize(b.SizeBytes), b.Location, b.ID)
				if b.Comment != "" {
					val += fmt.Sprintf(" · %q", b.Comment)
				}
				detail(glyphOK, "last good backup", val)
			} else {
				detail(glyphTodo, "last good backup", "never")
			}

			bc := inst.BackupCopies
			detail("", "backups stored", formatBackupsStored(inst.CompletedBackups, bc.Both, bc.Remote, bc.Local, bc.None))
		}
	}
	_, _ = fmt.Fprintln(out)

	// Notifications are global (not per-instance).
	_, _ = fmt.Fprintln(out, "Notifications")
	if len(result.Notifications.Configured) == 0 {
		_, _ = fmt.Fprintln(out, "  none configured")
	} else {
		_, _ = fmt.Fprintf(out, "  %d configured: ", len(result.Notifications.Configured))
		for i, n := range result.Notifications.Configured {
			if i > 0 {
				_, _ = fmt.Fprint(out, ", ")
			}
			_, _ = fmt.Fprintf(out, "%s [%s]", n.Name, n.Type)
		}
		_, _ = fmt.Fprintln(out)
	}

	printEvent := func(label string, event *checklistNotificationEvent) {
		if event == nil {
			_, _ = fmt.Fprintf(out, "  %-11s none\n", label)
			return
		}
		msg := event.Detail
		if msg != "" {
			msg = " — " + msg
		}
		_, _ = fmt.Fprintf(out, "  %-11s %s · %s · %s%s\n", label, event.CreatedAt, event.Name, event.Status, msg)
	}
	printEvent("last event", result.Notifications.LastEvent)
	printEvent("last error", result.Notifications.LastError)

	return nil
}

// Checklist status glyphs: ✓ good/configured, ✗ problem, ○ absent/pending.
const (
	glyphOK   = "✓"
	glyphBad  = "✗"
	glyphTodo = "○"
)

// statusGlyph maps an instance's lifecycle status to a block-header glyph.
func statusGlyph(status string) string {
	switch status {
	case "running":
		return glyphOK
	case "error":
		return glyphBad
	default: // stopped, creating, ...
		return glyphTodo
	}
}

// healthGlyph maps an instance's health verdict to a checklist glyph.
func healthGlyph(health string) string {
	switch health {
	case "ok":
		return glyphOK
	case "failing":
		return glyphBad
	default: // not-checked, unknown
		return glyphTodo
	}
}

// formatBackupsStored renders the completed-backup count with a breakdown of
// where the copies live. With a single bucket it collapses to "9 local+s3";
// with several it shows the total plus each non-empty bucket, e.g.
// "9 · 7 local+s3, 1 s3, 1 local". Buckets are mutually exclusive and sum to total.
func formatBackupsStored(total, both, remote, local, none int) string {
	var parts []string
	if both > 0 {
		parts = append(parts, fmt.Sprintf("%d local+s3", both))
	}
	if remote > 0 {
		parts = append(parts, fmt.Sprintf("%d s3", remote))
	}
	if local > 0 {
		parts = append(parts, fmt.Sprintf("%d local", local))
	}
	if none > 0 {
		parts = append(parts, fmt.Sprintf("%d no copies", none))
	}
	switch len(parts) {
	case 0:
		return fmt.Sprintf("%d", total)
	case 1:
		return parts[0]
	default:
		return fmt.Sprintf("%d · %s", total, strings.Join(parts, ", "))
	}
}

// formatSize renders a byte count in a human-readable binary unit
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
