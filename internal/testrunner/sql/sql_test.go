package sql

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
)

func nativeInput(t *testing.T, family, name string, exit int) tr.Input {
	t.Helper()
	stdout, e := os.ReadFile(filepath.Join("testdata", family, name+".stdout"))
	if e != nil {
		t.Fatal(e)
	}
	stderr, e := os.ReadFile(filepath.Join("testdata", family, name+".stderr"))
	if e != nil {
		t.Fatal(e)
	}
	in := tr.Input{Runner: "sql-sqllogictest-sqlite", SourceFile: "case.slt", Stdout: stdout, Stderr: stderr, ExitCode: exit, SuccessExitCodes: []int{0}, FailureExitCodes: []int{1}}
	if family == "pgtap" {
		in.Runner = "sql-pgtap"
		in.SourceFile = "case.sql"
		in.SuccessExitCodes = nil
		in.FailureExitCodes = nil
		in.OutcomeNeutralExitCodes = []int{0}
	}
	return in
}
func TestNativeMatrix(t *testing.T) {
	for _, f := range []string{"pgtap", "sqlite"} {
		for _, c := range []struct {
			name, state string
			exit        int
			complete    bool
		}{{"pass", tr.Passed, 0, true}, {"fail", tr.Failed, 0, true}, {"skip", tr.Skipped, 0, true}, {"zero", "", 0, false}, {"infra", "", 3, false}} {
			t.Run(f+"/"+c.name, func(t *testing.T) {
				exit := c.exit
				complete := c.complete
				state := c.state
				if f == "pgtap" && c.name == "zero" {
					exit = 3
				}
				if f == "sqlite" && (c.name == "fail" || c.name == "infra") {
					exit = 1
					complete = true
					state = tr.Failed
				}
				in := nativeInput(t, f, c.name, exit)
				o, e := Parse(in)
				if e != nil {
					t.Fatal(e)
				}
				o = tr.Normalize(in, o)
				if o.Complete != complete || o.RetryInformation != tr.NotReported {
					t.Fatalf("%+v", o)
				}
				if state != "" && (len(o.Tests) != 1 || o.Tests[0].State != state) {
					t.Fatal(o)
				}
				if len(o.Tests) > 0 {
					r := o.Tests[0]
					if r.State == tr.Failed && r.FailureKind != tr.Unknown {
						t.Fatal(r)
					}
					if f == "pgtap" {
						if r.Granularity != "CASE" || r.ExecutedCount != nil || !strings.HasPrefix(r.ID, "case.sql::1") {
							t.Fatal(r)
						}
					} else if r.Granularity != "SUITE_ONLY" || r.ExecutedCount == nil || r.SkippedCount == nil || r.ID != "case.slt" {
						t.Fatal(r)
					}
				}
			})
		}
	}
	in := nativeInput(t, "sqlite", "connect", 1)
	o, e := Parse(in)
	if e != nil || o.Complete {
		t.Fatal(o, e)
	}
}
func TestTAPRefusals(t *testing.T) {
	for _, raw := range []string{"1..1\nok 1 - x\n1..1\n", "1..2\nok 1 - x\n", "ok 1 - x\n", "1..1\nok 2 - x\n", "1..1\nok 1 - x", "1..1\nok 1 - x # TODO reason\n", "1..1\nnot ok 1 # SKIP reason\n", "1..1\nok 1 - x\nunknown status\n", "1..4097\n", "ok 1 - x\n1..2\nok 2 - y\n"} {
		in := tr.Input{Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte(raw)}
		if o, e := Parse(in); e == nil && o.Complete {
			t.Fatalf("accepted %q: %+v", raw, o)
		}
	}
	for _, in := range []tr.Input{{Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte("1..1\nok 1 - x\n"), Stderr: []byte("ERROR: lost connection\n")}, {Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte("1..1\nok 1 - x\nBail out! lost connection\n")}, {Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte("1..1\nok 1 - x\n"), ExitCode: 3}, {Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte("1..0\n")}} {
		o, e := Parse(in)
		if e != nil || o.Complete {
			t.Fatal(o, e)
		}
	}
	o, e := Parse(tr.Input{Runner: "sql-pgtap", SourceFile: "case.sql", Stdout: []byte("ok 1 - one\nok 2 - two\n1..2\n")})
	if e != nil || !o.Complete || len(o.Tests) != 2 {
		t.Fatal(o, e)
	}
}
func TestSQLiteRefusals(t *testing.T) {
	good := nativeInput(t, "sqlite", "pass", 0)
	for _, mutate := range []func(*tr.Input){
		func(x *tr.Input) { x.SourceFile = "other.slt" }, func(x *tr.Input) { x.ExitCode = 1 }, func(x *tr.Input) { x.Stdout = []byte("case.slt:3: halt\n") }, func(x *tr.Input) { x.Stderr = append([]byte("native fatal error\n"), x.Stderr...) }, func(x *tr.Input) { x.Stderr = append(x.Stderr, x.Stderr...) }, func(x *tr.Input) { x.Stderr = []byte("0 errors out of 4097 tests in case.slt - 0 skipped.\n") }, func(x *tr.Input) { x.Stderr = []byte("256 errors out of 1 tests in case.slt - 0 skipped.\n") }, func(x *tr.Input) { x.Stderr = []byte("0 errors out of 1 tests in case.slt - 0 skipped.") }, func(x *tr.Input) { x.SourceFile = "../case.slt" }, func(x *tr.Input) { x.Reports = map[string][]byte{"old.xml": []byte("ignored")} }, func(x *tr.Input) { x.TimedOut = true },
	} {
		in := good
		mutate(&in)
		o, e := Parse(in)
		if e == nil && o.Complete {
			t.Fatalf("accepted %+v", in)
		}
	}
}
func buildRequest(t *testing.T, runner string) tr.Request {
	t.Helper()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	project := "case.sql"
	if runner == "sql-sqllogictest-sqlite" {
		project = "case.slt"
	}
	r := tr.Request{Runner: runner, Root: root, ReportDir: filepath.Join(root, "fresh"), Executable: "/pinned/tool", ExecutableSha256: strings.Repeat("a", 64), Project: project, InputFiles: map[string]string{project: strings.Repeat("b", 64)}}
	if runner == "sql-pgtap" {
		b, e := json.Marshal(Connection{SocketDirectory: "/private/tmp/socket", Port: 55437, Database: "postgres", User: "corvint"})
		if e != nil {
			t.Fatal(e)
		}
		r.Config = "connection.json"
		r.ConfigSha256 = tr.Digest(b)
		if e = os.WriteFile(filepath.Join(root, r.Config), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	return r
}
func TestFixedInvocations(t *testing.T) {
	for _, runner := range Runners() {
		r := buildRequest(t, runner)
		v, e := Build(r)
		if e != nil {
			t.Fatal(e)
		}
		if runner == "sql-pgtap" {
			if !reflect.DeepEqual(v.OutcomeNeutralExitCodes, []int{0}) || v.Argv[len(v.Argv)-1] != "case.sql" {
				t.Fatal(v)
			}
		} else {
			want := []string{"-verify", "-engine", "SQLite", "-connection", filepath.Join(r.ReportDir, "scratch.sqlite"), "case.slt"}
			if !reflect.DeepEqual(v.Argv, want) || len(v.FailureExitCodes) != 255 {
				t.Fatal(v)
			}
		}
		for _, mutate := range []func(*tr.Request){func(x *tr.Request) { x.Project = "../case.sql" }, func(x *tr.Request) { x.Selectors = []string{"one-case"} }, func(x *tr.Request) { x.Target = "arbitrary" }, func(x *tr.Request) { x.InputFiles = map[string]string{} }, func(x *tr.Request) { x.ReportFiles = []string{"stale.tap"} }, func(x *tr.Request) { x.ExecutableSha256 = "" }} {
			q := r
			mutate(&q)
			if _, e := Build(q); e == nil {
				t.Fatalf("accepted %+v", q)
			}
		}
	}
}
func TestClosedConnection(t *testing.T) {
	r := buildRequest(t, "sql-pgtap")
	for _, raw := range []string{`{"socketDirectory":"localhost","port":5432,"database":"postgres","user":"corvint"}`, `{"socketDirectory":"/tmp/a,evil.example","port":5432,"database":"postgres","user":"corvint"}`, `{"socketDirectory":"/tmp/a","port":5432,"database":"host=evil","user":"corvint"}`, `{"socketDirectory":"/tmp/a","port":5432,"port":1,"database":"postgres","user":"corvint"}`, `{"socketDirectory":"/tmp/a","port":5432,"database":"postgres","user":"corvint","password":"secret"}`, `{"socketDirectory":"/tmp/a","port":5432,"database":"postgres","user":"corvint"} {}`, `{"SocketDirectory":"/tmp/a","port":5432,"database":"postgres","user":"corvint"}`} {
		b := []byte(raw)
		if e := os.WriteFile(filepath.Join(r.Root, r.Config), b, 0600); e != nil {
			t.Fatal(e)
		}
		r.ConfigSha256 = tr.Digest(b)
		if _, e := Build(r); e == nil {
			t.Fatal(raw)
		}
	}
	r = buildRequest(t, "sql-pgtap")
	r.ConfigSha256 = strings.Repeat("0", 64)
	if _, e := Build(r); e == nil {
		t.Fatal("hash mismatch accepted")
	}
	r = buildRequest(t, "sql-pgtap")
	target := filepath.Join(r.Root, r.Config)
	link := filepath.Join(r.Root, "linked.json")
	if e := os.Symlink(target, link); e != nil {
		t.Fatal(e)
	}
	r.Config = link
	if _, e := Build(r); e == nil {
		t.Fatal("symlink configuration accepted")
	}
}
