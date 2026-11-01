package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// DCW-V0-057: a change is a record only when it touches at least one path
// under docs/build-log/ or .agent-evidence/ and every other path is a CEM
// sidecar path; a mixed change is a change and a sidecar-only change is a
// chore, never a record.
func TestRecordChangeDCWV0057Classifies(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  bool
	}{
		{"build-log entry", []string{"docs/build-log/2026-10-13-x.md"}, true},
		{"build-log subdirectory", []string{"docs/build-log/evidence/run.json"}, true},
		{"agent evidence", []string{".agent-evidence/v1-1344/report.json"}, true},
		{"record and bind sidecar", []string{".corvint/change.cem.json", "docs/build-log/x.md"}, true},
		{"record and sealed sidecar", []string{".agent-evidence/a", ".corvint/changes/abc.cem.json"}, true},
		{"mixed with code", []string{"docs/build-log/x.md", "internal/x.go"}, false},
		{"mixed with a decision", []string{"docs/build-log/x.md", "docs/decisions/0001-x.md"}, false},
		{"sidecar only", []string{".corvint/change.cem.json"}, false},
		{"sealed sidecar only", []string{".corvint/changes/abc.cem.json"}, false},
		{"empty", nil, false},
		{"directory itself", []string{"docs/build-log/"}, false},
		{"no slash", []string{"docs/build-log"}, false},
		{"closed build log", []string{"docs/BUILD-LOG.md"}, false},
		{"quoted path", []string{`"docs/build-log/caf\303\251.md"`}, false},
		{"nested sealed sidecar", []string{"docs/build-log/x.md", ".corvint/changes/a/b.cem.json"}, false},
	} {
		if got := RecordChange(tc.paths); got != tc.want {
			t.Errorf("%s: RecordChange(%q) = %v; want %v", tc.name, tc.paths, got, tc.want)
		}
		if tc.want && ChoreRole(tc.paths) != "" {
			t.Errorf("%s: a record change is also a chore", tc.name)
		}
	}
	for path, want := range map[string]bool{
		"docs/build-log/2026-10-13-x.md": true, "docs/decisions/0527-x.md": true,
		"docs/build-log/sub/x.md": false, "docs/decisions/x.txt": false, "docs/specs/x.md": false, "docs/decisions/.md": true,
	} {
		if got := recordIntentPath(path); got != want {
			t.Errorf("recordIntentPath(%q) = %v; want %v", path, got, want)
		}
	}
}

// DCW-V0-058: a record change accepts a uniform plan and a build-log entry it
// touches, or a decision present at the target, as its intent, published as
// the no-intent declaration; a mixed change, an absent decision or an entry the
// change does not touch keeps the named intent as a path intent, and a manifest
// changed during the run is drift, never published. Such a path intent that
// ocm-prepare cannot derive (a build-log entry or a decision is no
// requirements spec, an absent one is no blob) now refuses before any step,
// as the bind would after it (DCW-V0-945).
func TestChangeDCWV0058RecordBindsUniformPlanAndRecordIntent(t *testing.T) {
	for _, tc := range []struct {
		name, extra, intents string
		drift                bool
		record, recordIntent bool
		invalid              bool
	}{
		{name: "record", intents: "docs/build-log/2026-10-13-x.md\n", record: true, recordIntent: true},
		{name: "record naming a decision", intents: "docs/build-log/2026-10-13-x.md\ndocs/decisions/0001-x.md\n", record: true, recordIntent: true},
		{name: "mixed", extra: "a.txt", intents: "docs/build-log/2026-10-13-x.md\n", invalid: true},
		{name: "absent decision", intents: "docs/decisions/0002-never.md\n", record: true, invalid: true},
		{name: "untouched entry", intents: "docs/build-log/2026-10-01-old.md\n", record: true, invalid: true},
		{name: "drift", intents: "docs/build-log/2026-10-13-x.md\n", drift: true, record: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			testGit(t, root, "init", "-q")
			write := func(path, content string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0o777); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o666); err != nil {
					t.Fatal(err)
				}
			}
			write("docs/decisions/0001-x.md", "decision\nline\n")
			write("docs/build-log/2026-10-01-old.md", "old\n")
			write("a.txt", "base\n")
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "base")
			base := testGit(t, root, "rev-parse", "HEAD")
			write("docs/build-log/2026-10-13-x.md", "entry\n")
			write(".agent-evidence/x/run.txt", "run\n")
			if tc.extra != "" {
				write(tc.extra, "changed\n")
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-q", "-m", "record")
			scratch := t.TempDir()
			plan := filepath.Join(scratch, "cites.tsv")
			intents := filepath.Join(scratch, "intents.txt")
			cites := 0
			steps := Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, _, _ io.Writer) int {
				switch {
				case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
					hunks := `{"disposition": "unknown", "id": "hunk:a", "path": "x", "reason": "no-evidence"}, {"disposition": "unknown", "id": "hunk:b", "path": "x", "reason": "no-evidence"}`
					if os.WriteFile(filepath.Join(root, ".corvint/change.cem.json"), []byte(`{"hunks": [`+hunks+"]}\n"), 0o666) != nil {
						return 1
					}
					return 0
				case len(args) > 1 && args[0] == "cem" && args[1] == "cite":
					cites++
					if tc.drift && os.WriteFile(intents, []byte("docs/decisions/0001-x.md\n"), 0o666) != nil {
						return 1
					}
					return 0
				}
				return 1
			}}
			if err := os.WriteFile(plan, []byte("1\tdocs/decisions/0001-x.md\t1:2\tdecision\n2\tdocs/decisions/0001-x.md\t1:2\tdecision\n"), 0o666); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(intents, []byte(tc.intents), 0o666); err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			code, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents}, &stderr)
			if err != nil {
				t.Fatal(err)
			}
			if tc.invalid {
				if code != 2 || cites != 0 || !strings.Contains(stderr.String(), "dogfood-change: REFUSE intent-invalid\n  intent="+strings.TrimSpace(tc.intents)+" code=") {
					t.Errorf("an underivable path intent was not refused before any step: code=%d cites=%d\n%s", code, cites, &stderr)
				}
				return
			}
			report, err := os.ReadFile(filepath.Join(root, ".corvint/dogfood-report.json"))
			if err != nil {
				t.Fatal(err)
			}
			uniformRefused := bytes.Contains(report, []byte(`"reason": "citation-plan-uniform"`))
			declared := bytes.Contains(report, []byte(noIntentOCMStatus))
			published, _ := os.ReadFile(filepath.Join(root, ".corvint/change.ocm-intents"))
			if tc.record == uniformRefused || (tc.record && cites != 2) || (!tc.record && cites != 0) {
				t.Errorf("uniform plan refused=%v after %d cites; want refused=%v\n%s", uniformRefused, cites, !tc.record, report)
			}
			switch {
			case tc.drift:
				if !bytes.Contains(report, []byte(`{"name": "ocm-aggregate", "status": "NOT_PRODUCED", "reason": "intent-scope-drift"}`)) || published != nil {
					t.Errorf("a manifest changed during the run was published (%q)\n%s", published, report)
				}
			case tc.recordIntent:
				if !declared || !bytes.Equal(published, noIntentManifest) || bytes.Contains(report, []byte("invalid-requirements-section")) {
					t.Errorf("intent not published as the no-intent declaration (published %q)\n%s", published, report)
				}
			default:
				if declared || bytes.Equal(published, noIntentManifest) || !strings.Contains(string(report), `"name": "ocm-`) {
					t.Errorf("the intent was read as a record intent or not assessed\n%s", report)
				}
			}
		})
	}
}

// DCW-V0-057, CEM-V1-170: the status class and the bind-loop classifier
// apply the same record rule to the same paths.
func TestRecordChangeDCWV0057MatchesStatusClass(t *testing.T) {
	for _, paths := range [][]string{
		{"docs/build-log/x.md"}, {"docs/build-log/a/b.json"}, {".agent-evidence/a"}, {".corvint/change.cem.json", "docs/build-log/x.md"},
		{".agent-evidence/a", ".corvint/changes/abc.cem.json"}, {"docs/build-log/x.md", "internal/x.go"}, {"docs/build-log/x.md", "docs/decisions/0001-x.md"},
		{".corvint/change.cem.json"}, {".corvint/changes/abc.cem.json"}, nil, {"docs/build-log/"}, {".agent-evidence/"}, {"docs/build-log"},
		{"docs/BUILD-LOG.md"}, {`"docs/build-log/caf\303\251.md"`}, {"docs/build-log/x.md", ".corvint/changes/a/b.cem.json"},
		{"docs/build-log/x.md", ".corvint/changes/.cem.json"}, {"docs/build-log/x.md", ".corvint/changes/abc.cem.jsonx"},
	} {
		status := workflow.StableCitationClass(wire.StableRoleChange, paths)["class"] == workflow.StableCitationClassRecord
		if got := RecordChange(paths); got != status {
			t.Errorf("RecordChange(%q) = %v; status class record = %v", paths, got, status)
		}
	}
}
