package operations

import (
	"reflect"
	"testing"
)

func TestBuildDatabaseCreateGrantSQL(t *testing.T) {
	statements, missing := buildDatabaseCreateGrantSQL(
		`restored"db`,
		[]string{`app"user`, "missing role", `app"user`},
		[]string{`app"user`},
	)

	wantStatements := []string{`GRANT CREATE ON DATABASE "restored""db" TO "app""user"`}
	if !reflect.DeepEqual(statements, wantStatements) {
		t.Fatalf("unexpected grants:\n got: %v\nwant: %v", statements, wantStatements)
	}
	if !reflect.DeepEqual(missing, []string{"missing role"}) {
		t.Fatalf("unexpected missing roles: %v", missing)
	}
}

func TestReadDatabaseCreateGranteesSupportsLegacyMetadata(t *testing.T) {
	dir := t.TempDir()
	if err := writeDatabaseMetadata(dir, []DatabaseMeta{{Name: "appdb"}}); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	grantees, err := readDatabaseCreateGrantees(dir, "appdb")
	if err != nil {
		t.Fatalf("read CREATE grantees: %v", err)
	}
	if grantees != nil {
		t.Fatalf("expected no grantees for legacy metadata, got: %v", grantees)
	}
}
