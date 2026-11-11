// SPDX-License-Identifier: Apache-2.0
package repopath

import (
	"errors"
	"testing"
)

// The canonical form admits only slash-separated, dot-free, non-empty UTF-8
// components; everything else is refused.
func TestValidAdmitsOnlyCanonicalRelativePaths(t *testing.T) {
	for _, ok := range []string{"a", "a/b.go", "a.b/c-d_e/f"} {
		if !Valid(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "/a", "a/", "a//b", "./a", "a/./b", "../a", "a/../b", "a\\b", "a\x00b", "\xff"} {
		if Valid(bad) {
			t.Errorf("%q admitted", bad)
		}
	}
}

// A committed range is decoded with the same path rule as the worktree
// status: every name is a valid relative path or the whole capture fails,
// and the result is sorted and deduplicated.
func TestDecodeNameListNormalizesAndFailsClosed(t *testing.T) {
	paths, err := DecodeNameList([]byte("b/moved.x\x00a/removed.x\x00b/moved.x\x00"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "a/removed.x" || paths[1] != "b/moved.x" {
		t.Fatalf("paths=%v", paths)
	}
	for name, raw := range map[string][]byte{"escaping": []byte("../outside.x\x00"), "absolute": []byte("/etc/passwd\x00"), "empty name": []byte("\x00a.x\x00")} {
		if _, err := DecodeNameList(raw); !errors.Is(err, ErrNameListMalformed) {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
	if got := Normalize([]string{"b", "a", "b", "/x", ""}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Normalize=%v", got)
	}
}
