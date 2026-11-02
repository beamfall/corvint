package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/execfile"
	tr "github.com/Beamfall/corvint/internal/testrunner"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
)

// fakeContainer is a docker-shaped stand-in: `run --rm -v HOST:CTR IMAGE
// rspec ...`. It maps --require and --out from the container path back to the
// host mount, refuses an unmapped path, requires the profile-written formatter
// at the mapped path, and writes a fixed Corvint-formatter report. It uses
// only sh builtins.
const fakeContainer = `#!/bin/sh
host= ; ctr=
while [ $# -gt 0 ]; do
  case "$1" in
    run|--rm) shift ;;
    -v) host=${2%%:*}; ctr=${2#*:}; shift 2 ;;
    fake-image) shift; break ;;
    *) echo "unexpected prefix word $1" >&2; exit 9 ;;
  esac
done
[ "$1" = rspec ] || exit 9
shift
req= ; out=
while [ $# -gt 0 ]; do
  case "$1" in
    --require) req=$2; shift 2 ;;
    --format) shift 2 ;;
    --out) out=$2; shift 2 ;;
    *) shift ;;
  esac
done
case "$req" in "$ctr"/*) req="$host${req#"$ctr"}" ;; *) echo "unmapped $req" >&2; exit 8 ;; esac
case "$out" in "$ctr"/*) out="$host${out#"$ctr"}" ;; *) echo "unmapped $out" >&2; exit 8 ;; esac
[ -f "$req" ] || { echo "formatter missing at $req" >&2; exit 7; }
printf '%s' 'REPORT' > "$out"
`

const fakeReport = `{"version":"3.13.6","examples":[{"id":"./spec/widget_spec.rb[1:1]","description":"renders","status":"passed","file_path":"./spec/widget_spec.rb","line_number":4,"shared_group_inclusion_backtrace":[]},{"id":"./spec/widget_spec.rb[1:2:1]","description":"is valid","status":"passed","file_path":"./spec/support/shared.rb","line_number":2,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/widget_spec.rb:7"}]},{"id":"./spec/widget_spec.rb[1:3:1]","description":"is valid","status":"passed","file_path":"./spec/support/shared.rb","line_number":2,"shared_group_inclusion_backtrace":[{"inclusion_location":"./spec/widget_spec.rb:10"}]}],"summary":{"example_count":3,"failure_count":0,"pending_count":0,"errors_outside_of_examples_count":0},"corvint_runtime":{}}`

// TestContainerisedRSpecPlanRunReceipt (TRE-V0-081..085): a plan through a
// fake container prefix with a host-to-container report mapping runs, and its
// receipt binds the plan and names each shared-example inclusion. A plan
// whose prefix or mapping was edited after planning is refused for execution.
// No Ruby, RSpec or Docker runs here; live qualification is NOT_RUN.
func TestContainerisedRSpecPlanRunReceipt(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable process containment is only supported on Darwin/Linux")
	}
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	exe := filepath.Join(dir, "bin", "docker")
	script := []byte(strings.Replace(fakeContainer, "REPORT", fakeReport, 1))
	if e = os.MkdirAll(filepath.Dir(exe), 0700); e != nil {
		t.Fatal(e)
	}
	if e = execfile.WriteFile(exe, script, 0700); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(dir, "app")
	spec := []byte("RSpec.describe 'Widget' do\nend\n")
	if e = os.MkdirAll(filepath.Join(root, "spec"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(root, "spec", "widget_spec.rb"), spec, 0600); e != nil {
		t.Fatal(e)
	}
	reports := filepath.Join(dir, "reports")
	rd := filepath.Join(reports, "run1")
	if e = os.MkdirAll(reports, 0700); e != nil {
		t.Fatal(e)
	}
	r := tr.Request{Runner: "rspec", Root: root, ReportDir: rd, TimeoutSeconds: 60, InputFiles: map[string]string{"spec/widget_spec.rb": tr.Digest(spec)}, Selectors: []string{"spec/widget_spec.rb"},
		ExecPrefix: []string{"run", "--rm", "-v", reports + ":/reports", "fake-image", "rspec"}, PathMap: []tr.PathMapping{{Host: reports, Container: "/reports"}}, Formatter: "corvint-rspec-json/0"}
	req := filepath.Join(dir, "request.json")
	b, _ := json.Marshal(r)
	if e = os.WriteFile(req, b, 0600); e != nil {
		t.Fatal(e)
	}
	identity := []string{"--executable", exe, "--executable-sha256", tr.Digest(script)}
	var out, errout bytes.Buffer
	if command(context.Background(), append([]string{"plan", "--request", req}, identity...), &out, &errout) != 0 {
		t.Fatal(errout.String())
	}
	var p plan
	if e = tr.DecodeDocument(out.Bytes(), &p); e != nil {
		t.Fatal(e)
	}
	planPath := filepath.Join(dir, "plan.json")
	if e = os.WriteFile(planPath, out.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	receiptPath := filepath.Join(dir, "receipt.json")
	out.Reset()
	errout.Reset()
	if code := command(context.Background(), append([]string{"run", "--out", receiptPath, "--plan", planPath, "--approve", tr.Identity(p), "--experimental", "--trusted-local"}, identity...), &out, &errout); code != 0 {
		raw, _ := os.ReadFile(receiptPath)
		t.Fatalf("exit %d: %s %s", code, errout.String(), raw)
	}
	raw, e := os.ReadFile(receiptPath)
	if e != nil {
		t.Fatal(e)
	}
	var result receipt
	if e = tr.DecodeDocument(raw, &result); e != nil {
		t.Fatal(e)
	}
	if result.PlanSha256 != tr.Identity(p) || !result.Observation.Complete || result.Error != "" || len(result.Observation.Tests) != 3 {
		t.Fatalf("receipt: %s", raw)
	}
	inclusions := map[string]string{}
	for _, test := range result.Observation.Tests {
		if test.State != tr.Passed {
			t.Fatalf("state %+v", test)
		}
		inclusions[test.ID] = strings.Join(test.InclusionLocations, ",")
	}
	if inclusions["./spec/widget_spec.rb[1:2:1]"] != "./spec/widget_spec.rb:7" || inclusions["./spec/widget_spec.rb[1:3:1]"] != "./spec/widget_spec.rb:10" || inclusions["./spec/widget_spec.rb[1:1]"] != "" {
		t.Fatalf("inclusions %q", inclusions)
	}
	for name, edit := range map[string]func(*plan){
		"prefix": func(q *plan) {
			q.Request.ExecPrefix = []string{"run", "--rm", "-v", reports + ":/reports", "other-image", "rspec"}
		},
		"path map":  func(q *plan) { q.Request.PathMap = []tr.PathMapping{{Host: reports, Container: "/elsewhere"}} },
		"formatter": func(q *plan) { q.Request.Formatter = "" },
	} {
		t.Run(name, func(t *testing.T) {
			q := p
			q.Request.ReportDir = filepath.Join(reports, "run-"+strings.ReplaceAll(name, " ", "-"))
			// Keep the invocation consistent with the new report directory
			// only, so the edited member alone is stale.
			fresh := q
			fresh.Request.ExecPrefix, fresh.Request.PathMap, fresh.Request.Formatter = p.Request.ExecPrefix, p.Request.PathMap, p.Request.Formatter
			inv, e := registry.Build(fresh.Request)
			if e != nil {
				t.Fatal(e)
			}
			q.Invocation = inv
			edit(&q)
			if tr.Identity(q) == tr.Identity(p) {
				t.Fatal("edit did not change the plan identity")
			}
			b, _ := json.Marshal(q)
			file := filepath.Join(t.TempDir(), "plan.json")
			if e := os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			var out, errout bytes.Buffer
			code := command(context.Background(), append([]string{"run", "--plan", file, "--approve", tr.Identity(q), "--out", filepath.Join(t.TempDir(), "receipt.json"), "--experimental", "--trusted-local"}, identity...), &out, &errout)
			if code != 1 || !strings.Contains(errout.String(), "plan does not match current fixed runner profile") {
				t.Fatalf("edited %s admitted (exit %d): %s", name, code, errout.String())
			}
			if _, e := os.Stat(q.Request.ReportDir); !os.IsNotExist(e) {
				t.Fatal("refused plan launched", e)
			}
		})
	}
}
