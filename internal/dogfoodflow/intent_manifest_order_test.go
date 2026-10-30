package dogfoodflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-387 (V1-3141): a missing-intent-scope refusal of a manifest that was
// read names the first line it refused and, for an ordering fault, the sort
// rule; an accepted ordering stays accepted, and a manifest never read keeps
// the generic fix.
func TestDCWV0387IntentManifestRefusalNamesTheLine(t *testing.T) {
	cases := []struct {
		name, content string
		want          []string
	}{
		{"out of order", "docs/specs/b.md\ndocs/specs/a.md\n", []string{`line 2 "docs/specs/a.md" does not sort after line 1 "docs/specs/b.md"`, "strictly increasing byte order", "LC_ALL=C sort -u"}},
		{"first fault of several", "docs/a.md\ndocs/c.md\ndocs/b.md\ndocs/0.md\n", []string{`line 3 "docs/b.md" does not sort after line 2 "docs/c.md"`}},
		{"duplicate line", "docs/a.md\ndocs/a.md\n", []string{`line 2 "docs/a.md" does not sort after line 1 "docs/a.md"`, "no duplicates"}},
		{"CRLF line ending sorts after the bare path", "docs/a.md\r\ndocs/a.md\n", []string{`line 2 "docs/a.md" does not sort after line 1 "docs/a.md\r"`}},
		{"control byte is quoted", "docs/b\x1b.md\ndocs/a.md\n", []string{`line 1 "docs/b\x1b.md"`, `line 2 "docs/a.md"`}},
		{"non-canonical path", "./docs/a.md\n", []string{`line 1 "./docs/a.md" is not a canonical repository-relative path`}},
		{"blank line", "docs/a.md\n\ndocs/b.md\n", []string{`line 2 "" is not a canonical repository-relative path`}},
		{"no trailing LF", "docs/a.md", []string{"the last line does not end with LF"}},
		{"empty file", "", []string{"the file is empty"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := filepath.Join(t.TempDir(), "intents")
			if err := os.WriteFile(manifest, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			c := new(change)
			c.options.IntentsFile = manifest
			if got := c.validateIntentManifest(); got != "missing-intent-scope" {
				t.Fatalf("reason %q, want missing-intent-scope", got)
			}
			hint := c.fixHint(step{name: "ocm-aggregate", reason: "missing-intent-scope"})
			if !strings.HasPrefix(hint, "DOGFOOD_INTENTS_FILE "+manifest+": ") || !strings.HasSuffix(hint, intentScopeHint) {
				t.Fatalf("hint does not name the file and keep the rule: %q", hint)
			}
			for _, want := range tc.want {
				if !strings.Contains(hint, want) {
					t.Fatalf("hint %q lacks %q", hint, want)
				}
			}
		})
	}
	// Byte order, not locale order: an uppercase path sorts before lowercase,
	// and a prefix sorts before its extension. Each ordering is padded past the
	// scope bound so the order check completes and the run stops at
	// intent-scope-limit before any Git read; the no-intent declaration passes.
	pad := ""
	for i := 0; i < maxAggregateScopes; i++ {
		pad += fmt.Sprintf("zz/s%02d.md\n", i)
	}
	for _, valid := range []string{"Docs/Z.md\ndocs/a.md\n" + pad, "docs/a.md\ndocs/a.md.bak\ndocs/b.md\n" + pad, "#no-intent-declared\n"} {
		manifest := filepath.Join(t.TempDir(), "intents")
		if err := os.WriteFile(manifest, []byte(valid), 0o600); err != nil {
			t.Fatal(err)
		}
		c := new(change)
		c.options.IntentsFile = manifest
		want := "intent-scope-limit"
		if !strings.HasSuffix(valid, pad) {
			want = ""
		}
		if got := c.validateIntentManifest(); got != want || c.manifestIssue != "" {
			t.Fatalf("%q refused: %q %q", valid, got, c.manifestIssue)
		}
	}
	// A manifest that is not read, and a second validation after a refused
	// one, keep only the generic fix.
	c := new(change)
	c.options.IntentsFile = filepath.Join(t.TempDir(), "absent")
	c.manifestIssue = "stale"
	if got := c.validateIntentManifest(); got != "missing-intent-scope" {
		t.Fatalf("absent manifest: %q", got)
	}
	if hint := c.fixHint(step{name: "ocm-aggregate", reason: "missing-intent-scope"}); hint != intentScopeHint {
		t.Fatalf("unread manifest hint %q", hint)
	}
}
