package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hypersequent/oddk/internal/cli"
)

const checklistFixture = `{
	"generatedAt": "2026-07-08T12:00:00Z",
	"health": {
		"overall": "degraded",
		"checkedAt": "2026-07-08T11:59:00Z",
		"hostHealthy": true,
		"failDetails": "instance staging: connection refused"
	},
	"instances": [
		{
			"name": "my-app",
			"version": "17",
			"status": "running",
			"health": "ok",
			"parameterGroup": "default:2025-08-27",
			"backupCron": {"utcHour": 3, "cleanupLocalDays": 7, "cleanupRemoteDays": 14},
			"lastGoodBackup": {
				"id": 42,
				"timestamp": "2026-07-08T03:00:12Z",
				"sizeBytes": 1288490188,
				"location": "local+s3"
			},
			"completedBackups": 9
		},
		{
			"name": "staging",
			"version": "18",
			"status": "running",
			"health": "failing",
			"parameterGroup": "default:2025-08-27",
			"completedBackups": 0
		}
	],
	"notifications": {
		"configured": [
			{"name": "ops-mail", "type": "email"},
			{"name": "ops-slack", "type": "slack"}
		],
		"lastEvent": {
			"name": "ops-slack",
			"status": "success",
			"detail": "Health degraded",
			"createdAt": "2026-07-08T11:59:05Z"
		}
	}
}`

func newChecklistServer(t *testing.T) (*httptest.Server, []string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/checklist" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error": "not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(checklistFixture))
	}))
	env := []string{fmt.Sprintf("ODDK_CLI_CONFIG=%s", writeTestConfig(t, server.URL))}
	return server, env
}

func TestChecklistAction_TableOutput(t *testing.T) {
	server, env := newChecklistServer(t)
	defer server.Close()

	var buf bytes.Buffer
	if err := cli.Run([]string{"oddk", "checklist"}, env, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	for _, want := range []string{
		"Overall health: degraded (last check 2026-07-08T11:59:00Z)",
		"instance staging: connection refused",
		"my-app",
		"default:2025-08-27",
		"03:00 UTC",
		"2026-07-08T03:00:12Z",
		"1.2GB",
		"local+s3",
		"failing",
		"never",
		"Notifications: 2 configured (ops-mail [email], ops-slack [slack])",
		"Last notification event: 2026-07-08T11:59:05Z  ops-slack  success — Health degraded",
		"Last notification error: none",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestChecklistAction_JSONOutput(t *testing.T) {
	server, env := newChecklistServer(t)
	defer server.Close()

	var buf bytes.Buffer
	if err := cli.Run([]string{"oddk", "checklist", "--json"}, env, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `"generatedAt": "2026-07-08T12:00:00Z"`) {
		t.Errorf("expected pretty-printed JSON, got:\n%s", out)
	}
	if strings.Contains(out, "PARAMETER GROUP") {
		t.Errorf("expected no table output in JSON mode, got:\n%s", out)
	}
}

func TestChecklistAction_DaemonError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error": "failed to build checklist: boom"}`))
	}))
	defer server.Close()

	env := []string{fmt.Sprintf("ODDK_CLI_CONFIG=%s", writeTestConfig(t, server.URL))}

	var buf bytes.Buffer
	err := cli.Run([]string{"oddk", "checklist"}, env, &buf)
	if err == nil {
		t.Fatal("expected error from daemon")
	}
	if !strings.Contains(err.Error(), "failed to build checklist") {
		t.Errorf("expected daemon error to surface, got: %v", err)
	}
}
