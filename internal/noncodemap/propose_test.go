// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNCMV1003ProposalWritesStdoutOnly(t *testing.T) {
	root := writeMap(t, `{"profile":"corvint-data-map/0","entries":[]}`)
	before := snapshot(t, root)
	recorded := strings.Join([]string{
		`{"test":"go:internal/app::TestLoad","path":"config/app.yml"}`,
		`{"test":"go:internal/app::TestReload","path":"config/app.yml"}`,
		`{"test":"go:internal/web::TestServe","path":"config/app.yml"}`,
		`{"test":"go:internal/app::TestLoad","path":"config/app.yml"}`,
		``,
		`{"test":"go:internal/app::TestLoad","path":"go.sum"}`,
		`{"test":"go:internal/app::TestLoad","path":".corvint/data-map.json"}`,
	}, "\n")
	out, err := Propose(strings.NewReader(recorded), "run.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "profile": "corvint-data-map/0",
  "entries": [
    {
      "path": "config/app.yml",
      "tests": [
        "go:internal/app::TestLoad",
        "go:internal/app::TestReload",
        "go:internal/web::TestServe"
      ],
      "evidence": "proposed from recorded reads in run.jsonl: 3 test(s) read this path"
    }
  ]
}
`
	if string(out) != want {
		t.Fatalf("proposal:\n%s\nwant:\n%s", out, want)
	}
	read, err := Parse(out)
	if err != nil {
		t.Fatalf("the proposal does not read back: %v", err)
	}
	if status, _ := Evaluate(read.Entries[0], allFacts); status != StatusProposed {
		t.Fatalf("status = %s, want proposed: a proposal never narrows", status)
	}
	again, err := Propose(strings.NewReader(recorded), "run.jsonl")
	if err != nil || !bytes.Equal(out, again) {
		t.Fatalf("a second proposal differs: %v", err)
	}
	if after := snapshot(t, root); !bytes.Equal(before, after) {
		t.Fatal("proposing changed the tree")
	}
	if body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(Path))); err != nil || string(body) != `{"profile":"corvint-data-map/0","entries":[]}` {
		t.Fatalf("the map file changed: %q %v", body, err)
	}
	for name, bad := range map[string]string{
		"unknown member": `{"test":"t","path":"a","extra":1}`,
		"unclean path":   `{"test":"t","path":"/etc/passwd"}`,
		"spaced test":    `{"test":"a b","path":"a"}`,
	} {
		if _, err := Propose(strings.NewReader(bad), "run.jsonl"); err == nil {
			t.Fatalf("%s: Propose accepted %s", name, bad)
		}
	}
}
