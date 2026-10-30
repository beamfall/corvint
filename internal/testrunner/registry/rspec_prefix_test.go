package registry_test

import (
	"strings"
	"testing"

	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// TestRSpecOnlyMembersRefusedForOtherRunners traces TRE-V0-081: the exec
// prefix, path map and formatter members are admitted only for rspec, so no
// other fixed profile silently ignores them.
func TestRSpecOnlyMembersRefusedForOtherRunners(t *testing.T) {
	base := tr.Request{Runner: "boost-test-junit", Executable: "/test", ReportDir: "/fresh", Target: "M", ExpectedTests: []string{"M::s/ok"}}
	if _, e := registry.Build(base); e != nil {
		t.Fatal(e)
	}
	for name, edit := range map[string]func(*tr.Request){
		"prefix":    func(r *tr.Request) { r.ExecPrefix = []string{"run"} },
		"path map":  func(r *tr.Request) { r.PathMap = []tr.PathMapping{{Host: "/fresh", Container: "/reports"}} },
		"formatter": func(r *tr.Request) { r.Formatter = "corvint-rspec-json/0" },
	} {
		for _, runner := range []string{"boost-test-junit", "pytest", "go-test"} {
			r := base
			r.Runner = runner
			edit(&r)
			if _, e := registry.Build(r); e == nil || !strings.Contains(e.Error(), "admitted only for rspec") {
				t.Fatalf("%s %s admitted: %v", runner, name, e)
			}
		}
	}
	r := tr.Request{Runner: "rspec", Executable: "/usr/bin/docker", ReportDir: "/host/reports/run", ExecPrefix: []string{"run", "img", "rspec"}, PathMap: []tr.PathMapping{{Host: "/host/reports", Container: "/reports"}}, Formatter: "corvint-rspec-json/0"}
	inv, e := registry.Build(r)
	if e != nil || len(inv.ReportPaths) != 1 || inv.ReportPaths[0] != "rspec-corvint.json" || !strings.Contains(strings.Join(inv.Argv, " "), "--out /reports/run/rspec-corvint.json") {
		t.Fatal(inv, e)
	}
}
