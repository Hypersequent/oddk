package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

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

	_, _ = fmt.Fprintf(c.out, "ODDK checklist — generated %s\n\n", result.GeneratedAt)

	if result.Health.CheckedAt != "" {
		_, _ = fmt.Fprintf(c.out, "Overall health: %s (last check %s)\n", result.Health.Overall, result.Health.CheckedAt)
	} else {
		_, _ = fmt.Fprintf(c.out, "Overall health: %s (no health checks recorded yet)\n", result.Health.Overall)
	}
	if result.Health.FailDetails != "" {
		_, _ = fmt.Fprintf(c.out, "Health details: %s\n", result.Health.FailDetails)
	}
	_, _ = fmt.Fprintln(c.out)

	if len(result.Instances) == 0 {
		_, _ = fmt.Fprintln(c.out, "No RDBMS instances found")
	} else {
		headers := []string{"NAME", "PG", "STATUS", "HEALTH", "PARAMETER GROUP", "DAILY BACKUP", "LAST GOOD BACKUP", "SIZE", "COPIES", "TOTAL"}
		var rows [][]string

		for _, inst := range result.Instances {
			dailyBackup := "-"
			if inst.BackupCron != nil {
				dailyBackup = fmt.Sprintf("%02d:00 UTC", inst.BackupCron.UTCHour)
			}

			lastBackup, size, copies := "never", "-", "-"
			if b := inst.LastGoodBackup; b != nil {
				lastBackup = b.Timestamp
				size = formatSize(b.SizeBytes)
				copies = b.Location
			}

			rows = append(rows, []string{
				inst.Name,
				inst.Version,
				inst.Status,
				inst.Health,
				inst.ParameterGroup,
				dailyBackup,
				lastBackup,
				size,
				copies,
				fmt.Sprintf("%d", inst.CompletedBackups),
			})
		}

		if err := writeTable(c.out, headers, rows); err != nil {
			return err
		}
	}
	_, _ = fmt.Fprintln(c.out)

	if len(result.Notifications.Configured) == 0 {
		_, _ = fmt.Fprintln(c.out, "Notifications: none configured")
	} else {
		_, _ = fmt.Fprintf(c.out, "Notifications: %d configured (", len(result.Notifications.Configured))
		for i, n := range result.Notifications.Configured {
			if i > 0 {
				_, _ = fmt.Fprint(c.out, ", ")
			}
			_, _ = fmt.Fprintf(c.out, "%s [%s]", n.Name, n.Type)
		}
		_, _ = fmt.Fprintln(c.out, ")")
	}

	printEvent := func(label string, event *checklistNotificationEvent) {
		if event == nil {
			_, _ = fmt.Fprintf(c.out, "%s: none\n", label)
			return
		}
		detail := event.Detail
		if detail != "" {
			detail = " — " + detail
		}
		_, _ = fmt.Fprintf(c.out, "%s: %s  %s  %s%s\n", label, event.CreatedAt, event.Name, event.Status, detail)
	}
	printEvent("Last notification event", result.Notifications.LastEvent)
	printEvent("Last notification error", result.Notifications.LastError)

	return nil
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
