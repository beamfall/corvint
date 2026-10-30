package main

import (
	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EFO-V0-001, CEM-V1-210: without the stable flags the verb names the retired
// cem/0.2 form, which admits no map. A cem/0.2 map beside a real impact
// --provider receipt refuses with unsupported-spec and the recovery line,
// writes no stdout, and writes no file.
func TestObligationsCommandRefusesRetiredMap(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	record := providerRecord(t, root)
	code, receipt, stderr := runCLI(t, "--root", root, "impact", "--provider", record, "pkg/main.go")
	if code != 0 || stderr != "" {
		t.Fatalf("impact --provider: exit %d stderr %q", code, stderr)
	}
	scratch := t.TempDir()
	impactPath, cemPath := filepath.Join(scratch, "impact.json"), filepath.Join(scratch, "change.cem.json")
	cem := `{"spec":"cem/0.2","baseRevision":"` + strings.Repeat("1", 40) + `","excludedPath":".corvint/change.cem.json","patchSha256":"` + strings.Repeat("2", 64) + `","evidence":[],"hunks":[{"id":"hunk:sha256:aaaa","path":"pkg/main.go","disposition":"supported"}]}`
	for path, content := range map[string]string{impactPath: receipt, cemPath: cem} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	code, out, stderr := runCLI(t, "obligations", "--cem", cemPath, "--impact", impactPath)
	if code != 2 || out != "" || !strings.Contains(stderr, cemcode.UnsupportedSpec) || !strings.Contains(stderr, wire.RetiredSpecFix) {
		t.Fatalf("obligations over cem/0.2: exit %d out %q stderr %q", code, out, stderr)
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("obligations wrote a file: %v", entries)
	}
}

// EFO-V0-001: option errors exit 2 with a named message and no stdout.
func TestObligationsArguments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		arguments []string
		message   string
	}{
		{[]string{"--impact", "i.json"}, "requires --cem FILE and --impact FILE"},
		{[]string{"--cem", "c.json", "--impact", "i.json", "--limit", "0"}, "must be a positive integer"},
		{[]string{"--cem", "c.json", "--cem", "d.json", "--impact", "i.json"}, "given twice"},
		{[]string{"--cem", "c.json", "--impact"}, "requires exactly one value"},
		{[]string{"--cem", "c.json", "--impact", "i.json", "--root", "."}, "unrecognized arguments"},
		{[]string{"--cem", "absent.json", "--impact", "absent.json"}, "cannot read cem input"},
	}
	for _, entry := range cases {
		code, out, stderr := runCLI(t, append([]string{"obligations"}, entry.arguments...)...)
		if code != 2 || out != "" || !strings.Contains(stderr, entry.message) {
			t.Errorf("%v: exit %d out %q stderr %q", entry.arguments, code, out, stderr)
		}
	}
}
