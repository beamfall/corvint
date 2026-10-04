// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repository writes a module with one bounded and one unbounded test package.
func repository(t *testing.T, record string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27\n",
		"bounded/b.go":      "package bounded\n",
		"bounded/b_test.go": "package bounded\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
		"reader/r.go":       "package reader\n",
		"untested/u.go":     "package untested\n\nimport \"os\"\n\nfunc U() { _, _ = os.Getwd() }\n",
		"reader/r_test.go":  "package reader\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestR(t *testing.T) { _, _ = os.Getwd() }\n",
		ceilingPath:         record,
	} {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAFPV0025RatchetFailsAboveTheRecordedCeiling(t *testing.T) {
	for name, c := range map[string]struct {
		record string
		code   int
		want   string
	}{
		"at the ceiling":    {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{"reader":"calls os.Getwd"}}`, 0, `"count": 1`},
		"below the ceiling": {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":2,"reasons":{}}`, 0, "lower"},
		"above the ceiling": {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":0,"reasons":{}}`, 1, "above the ceiling 0"},
		"stale reason":      {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{"bounded":"none"}}`, 1, "not an unbounded test package"},
		"unknown member":    {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{},"extra":1}`, 2, "want exactly"},
		"missing ceiling":   {`{"profile":"corvint-unbounded-reader-ceiling/0","reasons":{}}`, 2, "want exactly"},
		"negative ceiling":  {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":-1,"reasons":{}}`, 2, "want exactly"},
		"other profile":     {`{"profile":"other/0","ceiling":1,"reasons":{}}`, 2, "want exactly"},
		"empty reason":      {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{"reader":" "}}`, 2, "want exactly"},
		"duplicate ceiling": {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":0,"ceiling":1,"reasons":{}}`, 2, "want exactly"},
		"member case":       {`{"profile":"corvint-unbounded-reader-ceiling/0","Ceiling":1,"reasons":{}}`, 2, "want exactly"},
		"trailing document": {`{"profile":"corvint-unbounded-reader-ceiling/0","ceiling":1,"reasons":{}} {}`, 2, "want exactly"},
	} {
		var stdout, stderr bytes.Buffer
		code := run(repository(t, c.record), &stdout, &stderr)
		if code != c.code || !strings.Contains(stdout.String()+stderr.String(), c.want) {
			t.Errorf("%s: code=%d want %d; output %q %q lacks %q", name, code, c.code, stdout.String(), stderr.String(), c.want)
		}
	}
}

func TestAFPV0025RatchetWithoutARecordRefuses(t *testing.T) {
	root := repository(t, "{}")
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(ceilingPath))); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run(root, &stdout, &stderr); code != 2 {
		t.Fatalf("code=%d output %q", code, stderr.String())
	}
}
