package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// AFP-V0-11033 (V1-5012): `corvint affected audit` replays this binary's
// selector over historical changes by design, so its report names the binary
// it planned with: the `producer` member carries the commit and commit state
// stamped into the build, the same identity PUB-V0-027 gives the impact
// receipt. A fresh binary names HEAD's commit, an accepted stale one its own
// older commit, and an unstamped one the explicit `unknown` marker.
func TestAffectedAuditNamesItsPlanningBinary_V15012(t *testing.T) {
	root, built := selfBinaryRepository(t)
	commitSelfBinaryFiles(t, root, map[string]string{"self.go": "package self\n\nfunc Self() int { return 2 }\n"})
	head := strings.TrimSpace(affectedGit(t, root, "rev-parse", "HEAD"))
	manifest := filepath.Join(t.TempDir(), "changes.json")
	if err := os.WriteFile(manifest, []byte(`{"profile":"corvint-affected-audit-input/0","changes":[{"id":"self","base":"`+built+`","head":"`+head+`"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct {
		name    string
		info    *debug.BuildInfo
		options []string
		commit  string
		clean   string
	}{
		{"fresh", selfBinaryInfo("example.test/self", head), nil, head, "clean"},
		{"accepted stale", selfBinaryInfo("example.test/self", built), []string{"--accept-stale-binary"}, built, "clean"},
		{"unstamped", &debug.BuildInfo{Main: debug.Module{Path: "example.test/self"}}, nil, producerUnknown, producerUnknown},
	} {
		stubSelfBuildInfo(t, state.info)
		stdout, stderr, code := runAffectedForm(t, root, false, append([]string{"audit", "--changes", manifest}, state.options...)...)
		var report struct {
			Producer *producerIdentity `json:"producer"`
		}
		if err := json.Unmarshal([]byte(stdout), &report); err != nil || code != 3 {
			t.Fatalf("%s: affected audit exited %d, want 3 with a report (%v):\n%s%s", state.name, code, err, stdout, stderr)
		}
		want := producerIdentity{Version: version, Build: producerFrom(version, build, nil).Build, Commit: state.commit, CommitState: state.clean}
		if report.Producer == nil || *report.Producer != want {
			t.Fatalf("%s: the audit report names producer %+v, want %+v:\n%s", state.name, report.Producer, want, stdout)
		}
	}
}
