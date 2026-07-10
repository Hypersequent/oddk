package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andrianbdn/oddk/internal/cli"
)

func TestBackupRestoreActionSendsOwner(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/rdbms/target/restore" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		received <- body
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{
			"targetDatabase":"restored",
			"sourceBackup":"backup ID 42",
			"owner":"appuser",
			"message":"Successfully restored database restored"
		}`)
	}))
	defer server.Close()

	env := []string{fmt.Sprintf("ODDK_CLI_CONFIG=%s", writeTestConfig(t, server.URL))}
	var output bytes.Buffer
	if err := cli.Run([]string{
		"oddk", "backup", "restore",
		"--instance", "target",
		"--id", "42",
		"--database", "source",
		"--restore-as", "restored",
		"--owner", "appuser",
	}, env, &output); err != nil {
		t.Fatalf("restore command failed: %v", err)
	}

	body := <-received
	if body["databaseName"] != "source" || body["restoreAs"] != "restored" || body["owner"] != "appuser" {
		t.Fatalf("unexpected restore request: %#v", body)
	}
	if !strings.Contains(output.String(), "Owner: appuser") {
		t.Fatalf("expected owner in command output: %q", output.String())
	}
}
