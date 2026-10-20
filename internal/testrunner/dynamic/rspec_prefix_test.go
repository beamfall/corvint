package dynamic

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// TestRSpecHistoricalArgvUnchanged (TRE-V0-086): without execPrefix, pathMap
// or formatter the rspec argv, report inventory and document bytes are the
// historical ones, and no new member is serialized.
func TestRSpecHistoricalArgvUnchanged(t *testing.T) {
	rd := filepath.Join(t.TempDir(), "reports")
	r := tr.Request{Runner: "rspec", Executable: "/usr/bin/rspec", ReportDir: rd, Config: "config/.rspec", Selectors: []string{"spec/a_spec.rb", "/abs/spec/b_spec.rb:7"}}
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	want := []string{"--format", "json", "--out", filepath.Join(rd, "rspec.json"), "--options", "config/.rspec", "spec/a_spec.rb", "/abs/spec/b_spec.rb:7"}
	if !reflect.DeepEqual(v.Argv, want) || !reflect.DeepEqual(v.ReportPaths, []string{"rspec.json"}) || len(v.Files) != 0 {
		t.Fatalf("historical rspec invocation changed: %+v", v)
	}
	for _, doc := range []any{r, tr.Test{ID: "x"}} {
		b, e := json.Marshal(doc)
		if e != nil {
			t.Fatal(e)
		}
		for _, key := range []string{"execPrefix", "pathMap", "formatter", "inclusionLocations"} {
			if bytes.Contains(b, []byte(`"`+key+`"`)) {
				t.Fatalf("absent %s serialized: %s", key, b)
			}
		}
	}
}

// TestRSpecExecPrefixPathMapArgv (TRE-V0-081, TRE-V0-082, TRE-V0-084): the
// prefix precedes the profile's arguments, every absolute host path the
// profile emits is mapped, relative ones stay literal, and the Corvint
// formatter is the Ruby provider's own bytes written as a profile template.
func TestRSpecExecPrefixPathMapArgv(t *testing.T) {
	rd := filepath.Join(t.TempDir(), "reports")
	root := filepath.Join(t.TempDir(), "app")
	prefix := []string{"run", "--rm", "--network", "none", "-v", rd + ":/reports", "-v", root + ":/app", "-w", "/app", "img@sha256:" + strings.Repeat("a", 64), "bundle", "exec", "rspec"}
	r := tr.Request{Runner: "rspec", Root: root, Executable: "/usr/local/bin/docker", ReportDir: rd, Config: filepath.Join(root, ".rspec"), Selectors: []string{"spec/a_spec.rb:3", filepath.Join(root, "spec", "b_spec.rb")},
		ExecPrefix: prefix, PathMap: []tr.PathMapping{{Host: rd, Container: "/reports"}, {Host: root, Container: "/app"}}, Formatter: RSpecFormatterCorvint}
	v, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	want := append(append([]string{}, prefix...), "--require", "/reports/corvint_rspec_formatter.rb", "--format", "CorvintRSpecJsonFormatter", "--out", "/reports/rspec-corvint.json", "--options", "/app/.rspec", "spec/a_spec.rb:3", "/app/spec/b_spec.rb")
	if !reflect.DeepEqual(v.Argv, want) {
		t.Fatalf("argv\n got %q\nwant %q", v.Argv, want)
	}
	if !reflect.DeepEqual(v.ReportPaths, []string{"rspec-corvint.json"}) || !bytes.Equal(v.Files["corvint_rspec_formatter.rb"], rubytestprovider.CorvintFormatterSource()) || len(v.Files) != 1 {
		t.Fatalf("report inventory or formatter template: %+v", v.ReportPaths)
	}
	// A prefix without a map is the identity: the wrapper mounts host paths
	// at the identical path. The formatter alone runs on the host.
	r.PathMap = nil
	if v, e = Build(r); e != nil || v.Argv[len(prefix)+5] != filepath.Join(rd, "rspec-corvint.json") || v.Argv[len(prefix)+1] != filepath.Join(rd, "corvint_rspec_formatter.rb") {
		t.Fatalf("identity mapping: %v %q", e, v.Argv)
	}
	r.ExecPrefix = nil
	if v, e = Build(r); e != nil || v.Argv[0] != "--require" {
		t.Fatalf("host formatter: %v %q", e, v.Argv)
	}
	// Every member changes the plan identity a receipt binds (TRE-V0-083).
	base := tr.Request{Runner: "rspec", Executable: "/usr/local/bin/docker", ReportDir: rd, ExecPrefix: prefix, PathMap: []tr.PathMapping{{Host: rd, Container: "/reports"}}}
	seen := map[string]bool{}
	for _, edit := range []func(*tr.Request){func(*tr.Request) {}, func(q *tr.Request) { q.ExecPrefix = append(append([]string{}, prefix[:len(prefix)-1]...), "rspec2") }, func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: rd, Container: "/other"}} }, func(q *tr.Request) { q.Formatter = RSpecFormatterCorvint }} {
		q := base
		edit(&q)
		inv, e := Build(q)
		if e != nil {
			t.Fatal(e)
		}
		id := tr.Identity(tr.PlanDocument{Profile: "corvint-test-runner-plan/0", Request: q, Invocation: inv})
		if seen[id] {
			t.Fatalf("plan identity does not bind %+v", q)
		}
		seen[id] = true
	}
}

// TestRSpecExecPrefixRefusals (TRE-V0-081, TRE-V0-082): every invalid or
// unmapped member is refused before a plan exists.
func TestRSpecExecPrefixRefusals(t *testing.T) {
	rd := "/host/reports/run1"
	ok := tr.Request{Runner: "rspec", Executable: "/usr/local/bin/docker", ReportDir: rd, ExecPrefix: []string{"run", "img", "rspec"}, PathMap: []tr.PathMapping{{Host: "/host/reports", Container: "/reports"}}}
	if _, e := Build(ok); e != nil {
		t.Fatal(e)
	}
	long := make([]string, 65)
	for i := range long {
		long[i] = "x"
	}
	many := []tr.PathMapping{}
	for i := 0; i < 9; i++ {
		many = append(many, tr.PathMapping{Host: "/h" + string(rune('a'+i)), Container: "/c" + string(rune('a'+i))})
	}
	for name, edit := range map[string]func(*tr.Request){
		"other runner prefix": func(q *tr.Request) { q.Runner = "pytest"; q.PathMap = nil },
		"other runner formatter": func(q *tr.Request) {
			q.Runner = "minitest"
			q.ExecPrefix = nil
			q.PathMap = nil
			q.Formatter = RSpecFormatterCorvint
		},
		"unknown formatter":    func(q *tr.Request) { q.Formatter = "stock" },
		"empty prefix word":    func(q *tr.Request) { q.ExecPrefix = []string{"run", ""} },
		"newline prefix word":  func(q *tr.Request) { q.ExecPrefix = []string{"run", "a\nb"} },
		"prefix bound":         func(q *tr.Request) { q.ExecPrefix = long },
		"secret-shaped prefix": func(q *tr.Request) { q.ExecPrefix = []string{"run", "--password", "correct horse", "img"} },
		"map without prefix":   func(q *tr.Request) { q.ExecPrefix = nil },
		"map bound":            func(q *tr.Request) { q.PathMap = many },
		"relative host":        func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: "host/reports", Container: "/reports"}} },
		"relative container":   func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: "/host/reports", Container: "reports"}} },
		"unclean host":         func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: "/host/reports/", Container: "/reports"}} },
		"root container":       func(q *tr.Request) { q.PathMap = []tr.PathMapping{{Host: "/host/reports", Container: "/"}} },
		"overlapping host":     func(q *tr.Request) { q.PathMap = append(q.PathMap, tr.PathMapping{Host: "/host", Container: "/h"}) },
		"overlapping container": func(q *tr.Request) {
			q.PathMap = append(q.PathMap, tr.PathMapping{Host: "/src", Container: "/reports/src"})
		},
		"report dir outside map":      func(q *tr.Request) { q.ReportDir = "/elsewhere/run1" },
		"report dir sibling prefix":   func(q *tr.Request) { q.ReportDir = "/host/reports-other/run1" },
		"absolute config outside map": func(q *tr.Request) { q.Config = "/etc/rspec" },
		"absolute selector outside":   func(q *tr.Request) { q.Selectors = []string{"/src/spec/a_spec.rb"} },
		"selector traversal":          func(q *tr.Request) { q.Selectors = []string{"/host/reports/../../etc/a_spec.rb"} },
		"config traversal":            func(q *tr.Request) { q.Config = "/host/reports/../secrets" },
	} {
		t.Run(name, func(t *testing.T) {
			q := ok
			edit(&q)
			if _, e := Build(q); e == nil {
				t.Fatalf("admitted %+v", q)
			}
		})
	}
}

const corvintRSpecReport = `{"version":"3.13.6","seed":1,"examples":[
{"id":"./spec/widget_spec.rb[1:1]","description":"renders","full_description":"Widget renders","status":"passed","file_path":"./spec/widget_spec.rb","line_number":4,"run_time":0.001,"pending_message":null,"shared_group_inclusion_backtrace":[]},
{"id":"./spec/widget_spec.rb[1:2:1]","description":"is valid","full_description":"Widget behaves like a model is valid","status":"passed","file_path":"./spec/support/shared.rb","line_number":2,"run_time":0.001,"pending_message":null,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/widget_spec.rb:7"}]},
{"id":"./spec/widget_spec.rb[1:3:1]","description":"is valid","full_description":"Widget behaves like a model is valid","status":"passed","file_path":"./spec/support/shared.rb","line_number":2,"run_time":0.001,"pending_message":null,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/support/nested.rb:3"},{"inclusion_location":"./spec/widget_spec.rb:10"}]}
],"summary":{"duration":0.01,"example_count":3,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0},"summary_line":"3 examples, 0 failures","corvint_runtime":{"container":"` + "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" + `"}}`

// TestRSpecCorvintInclusionLocations (TRE-V0-085): the Corvint report retains
// each example's inclusion backtrace, refuses an example without one, and the
// stock report ignores the member. The report is a synthetic fixture shaped
// by the shipped formatter's source, not a runner-generated witness.
func TestRSpecCorvintInclusionLocations(t *testing.T) {
	o, e := Parse(tr.Input{Runner: "rspec", ExitCode: 0, Reports: map[string][]byte{"rspec-corvint.json": []byte(corvintRSpecReport)}})
	if e != nil || !o.Complete {
		t.Fatalf("%v %+v", e, o.Problems)
	}
	got := map[string][]string{}
	for _, test := range o.Tests {
		got[test.ID] = test.InclusionLocations
	}
	want := map[string][]string{"./spec/widget_spec.rb[1:1]": nil, "./spec/widget_spec.rb[1:2:1]": {"./spec/widget_spec.rb:7"}, "./spec/widget_spec.rb[1:3:1]": {"./spec/support/nested.rb:3", "./spec/widget_spec.rb:10"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inclusions %q", got)
	}
	stock, e := Parse(tr.Input{Runner: "rspec", ExitCode: 0, Reports: map[string][]byte{"rspec.json": []byte(corvintRSpecReport)}})
	if e != nil {
		t.Fatal(e)
	}
	for _, test := range stock.Tests {
		if test.InclusionLocations != nil {
			t.Fatalf("stock report retained inclusions: %+v", test)
		}
	}
	opaque := strings.Replace(corvintRSpecReport, `"shared_group_inclusion_backtrace":[]`, `"shared_group_inclusion_backtrace":"opaque"`, 1)
	if _, e := Parse(tr.Input{Runner: "rspec", ExitCode: 0, Reports: map[string][]byte{"rspec.json": []byte(opaque)}}); e != nil {
		t.Fatalf("stock report now depends on the inclusion member: %v", e)
	}
	for name, edit := range map[string][2]string{
		"string backtrace":  {`"shared_group_inclusion_backtrace":[]`, `"shared_group_inclusion_backtrace":"opaque"`},
		"missing backtrace": {`,"shared_group_inclusion_backtrace":[]}`, `}`},
		"null backtrace":    {`"shared_group_inclusion_backtrace":[]`, `"shared_group_inclusion_backtrace":null`},
		"missing location":  {`{"inclusion_location":"./spec/widget_spec.rb:7"}`, `{}`},
		"empty location":    {`"inclusion_location":"./spec/widget_spec.rb:7"`, `"inclusion_location":""`},
		"newline location":  {`"inclusion_location":"./spec/widget_spec.rb:7"`, `"inclusion_location":"a\nb"`},
	} {
		t.Run(name, func(t *testing.T) {
			report := strings.Replace(corvintRSpecReport, edit[0], edit[1], 1)
			if report == corvintRSpecReport {
				t.Fatal("edit did not apply")
			}
			if _, e := Parse(tr.Input{Runner: "rspec", ExitCode: 0, Reports: map[string][]byte{"rspec-corvint.json": []byte(report)}}); e == nil {
				t.Fatal("admitted a malformed inclusion backtrace")
			}
		})
	}
}
