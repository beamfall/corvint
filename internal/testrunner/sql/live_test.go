//go:build darwin || linux

package sql

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// This opt-in test consumes already-built official runtimes and an operator-owned
// disposable database. It never creates or selects a default/user database.
func TestSQLLiveExecution(t *testing.T) {
	runtimeDir, connection, psql := os.Getenv("CORVINT_SQL_LIVE_DIR"), os.Getenv("CORVINT_SQL_CONNECTION"), os.Getenv("CORVINT_SQL_PSQL")
	if runtimeDir == "" || connection == "" || psql == "" {
		t.Skip("explicit SQL qualification runtimes and disposable connection not supplied")
	}
	here, e := os.Getwd()
	if e != nil {
		t.Fatal(e)
	}
	here, e = filepath.EvalSymlinks(here)
	if e != nil {
		t.Fatal(e)
	}
	out, e := os.MkdirTemp("", "sql-evidence-")
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("retained live SQL evidence: %s", out)
	config, e := os.ReadFile(connection)
	if e != nil {
		t.Fatal(e)
	}
	for _, family := range []string{"pgtap", "sqlite"} {
		for _, c := range []struct {
			name     string
			exit     int
			complete bool
		}{{"pass", 0, true}, {"fail", 0, true}, {"skip", 0, true}, {"zero", 3, false}, {"infra", 3, false}} {
			t.Run(family+"/"+c.name, func(t *testing.T) {
				runner, exe, extension := "sql-pgtap", psql, ".sql"
				exit, complete := c.exit, c.complete
				if family == "sqlite" {
					runner = "sql-sqllogictest-sqlite"
					exe = filepath.Join(runtimeDir, "sqlite-slt", "src", "sqllogictest")
					extension = ".slt"
					if c.name == "fail" || c.name == "infra" {
						exit = 1
						complete = true
					} else {
						exit = 0
					}
				}
				root := filepath.Join(here, "testdata", family)
				project := c.name + extension
				source, e := os.ReadFile(filepath.Join(root, project))
				if e != nil {
					t.Fatal(e)
				}
				tool, e := os.ReadFile(exe)
				if e != nil {
					t.Fatal(e)
				}
				r := tr.Request{Runner: runner, Root: root, Executable: exe, ExecutableSha256: tr.Digest(tool), Project: project, InputFiles: map[string]string{project: tr.Digest(source)}, ReportDir: filepath.Join(out, family+"-"+c.name), TimeoutSeconds: 30}
				if family == "pgtap" {
					r.Config = connection
					r.ConfigSha256 = tr.Digest(config)
				}
				inv, e := Build(r)
				if e != nil {
					t.Fatal(e)
				}
				x, execErr := tr.Execute(context.Background(), r, inv)
				o, parseErr := Parse(x.Input)
				o = tr.Normalize(x.Input, o)
				retained := struct {
					Request     tr.Request
					Invocation  tr.Invocation
					Execution   tr.Execution
					Observation tr.Observation
				}{r, inv, x, o}
				raw, e := json.MarshalIndent(retained, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(out, family+"-"+c.name+".json"), append(raw, '\n'), 0600); e != nil {
					t.Fatal(e)
				}
				if execErr != nil || parseErr != nil || x.Input.ExitCode != exit || o.Complete != complete {
					t.Fatalf("execute=%v parse=%v exit=%d observation=%+v", execErr, parseErr, x.Input.ExitCode, o)
				}
			})
		}
	}
}
