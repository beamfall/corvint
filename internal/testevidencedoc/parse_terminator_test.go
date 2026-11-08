// SPDX-License-Identifier: Apache-2.0
package testevidencedoc

import "testing"

// TestRSPV0051ParseEndOfOptions pins that a path after "--" stays a path, so an
// MCP caller cannot smuggle an option such as --format=text through "path".
func TestRSPV0051ParseEndOfOptions(t *testing.T) {
	q, format, err := Parse([]string{"--revision", "HEAD", "--", "--format=text"})
	if err != nil || q.Path != "--format=text" || format != "json" {
		t.Fatalf("q=%+v format=%q err=%v", q, format, err)
	}
	if _, f, err := Parse([]string{"--format=text", "a.go"}); err != nil || f != "text" {
		t.Fatalf("option form changed: %q %v", f, err)
	}
	if _, _, err := Parse([]string{"--", "a.go", "b.go"}); err == nil {
		t.Fatal("two positional paths accepted after --")
	}
}
