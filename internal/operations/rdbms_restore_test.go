package operations

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildPgRestoreCommand(t *testing.T) {
	t.Run("keeps existing restore behavior without owner", func(t *testing.T) {
		cmd := buildPgRestoreCommand(5432, "appdb", "")
		if slices.ContainsFunc(cmd, func(arg string) bool { return strings.HasPrefix(arg, "--role=") }) {
			t.Fatalf("unexpected restore role in command: %v", cmd)
		}
	})

	t.Run("restores objects as explicit owner", func(t *testing.T) {
		cmd := buildPgRestoreCommand(5432, "appdb", "appuser")
		if !slices.Contains(cmd, "--role=appuser") {
			t.Fatalf("expected restore role in command: %v", cmd)
		}
	})
}

func TestBuildRestoreCreateSQLWithOwner(t *testing.T) {
	t.Run("overrides metadata owner and preserves locale", func(t *testing.T) {
		dir := t.TempDir()
		metas := []DatabaseMeta{{
			Name: "appdb", Owner: "sourceowner", Encoding: "UTF8", Collate: "C", Ctype: "C", LocProvider: "c",
		}}
		if err := writeDatabaseMetadata(dir, metas); err != nil {
			t.Fatalf("write metadata: %v", err)
		}

		got, err := buildRestoreCreateSQL(dir, "appdb", "appdb_copy", "appuser")
		if err != nil {
			t.Fatalf("build restore SQL: %v", err)
		}
		want := `CREATE DATABASE "appdb_copy" OWNER = "appuser" TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C' LC_CTYPE 'C'`
		if got != want {
			t.Fatalf("unexpected restore SQL:\n got: %s\nwant: %s", got, want)
		}
	})

	t.Run("sets owner for legacy archive without metadata", func(t *testing.T) {
		got, err := buildRestoreCreateSQL(t.TempDir(), "appdb", "appdb_copy", "appuser")
		if err != nil {
			t.Fatalf("build restore SQL: %v", err)
		}
		want := `CREATE DATABASE "appdb_copy" OWNER = "appuser"`
		if got != want {
			t.Fatalf("unexpected restore SQL:\n got: %s\nwant: %s", got, want)
		}
	})
}
