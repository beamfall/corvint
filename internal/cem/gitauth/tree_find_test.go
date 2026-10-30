package gitauth

import (
	"bytes"
	"fmt"
	"testing"
)

// TestFindTreeEntryMatchesDecodedLookup pins that the single-scan
// findTreeEntry (V1-2740) answers exactly as treeEntries(raw)[name]: the same
// entry and the same refusal for sorted, Git-ordered, unsorted, duplicated
// and malformed bodies, at both object widths.
func TestFindTreeEntryMatchesDecodedLookup(t *testing.T) {
	for _, width := range []int{20, 32} {
		oid := func(i int) []byte { return bytes.Repeat([]byte{byte(i)}, width) }
		entry := func(mode, name string, i int) []byte {
			return append([]byte(mode+" "+name+"\x00"), oid(i)...)
		}
		join := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
		var flat []byte
		for i := 0; i < 300; i++ {
			flat = append(flat, entry("100644", fmt.Sprintf("f%03d.js", i), i)...)
		}
		bodies := map[string][]byte{
			"empty":            nil,
			"flat":             flat,
			"modes":            join(entry("100664", "a", 1), entry("100755", "b", 2), entry("40000", "c", 3), entry("120000", "d", 4), entry("160000", "e", 5), entry("100775", "f", 6)),
			"git-order":        join(entry("100644", "a.c", 1), entry("40000", "a", 2), entry("100644", "a0", 3)),
			"unsorted":         join(entry("100644", "b", 1), entry("100644", "a", 2), entry("100644", "c", 3)),
			"dup-adjacent":     join(entry("100644", "a", 1), entry("100644", "a", 2)),
			"dup-apart":        join(entry("100644", "b", 1), entry("100644", "c", 2), entry("100644", "b", 3)),
			"dup-then-bad":     join(entry("100644", "b", 1), entry("100644", "b", 2), []byte("bad")),
			"bad-name-late":    join(entry("100644", "a", 1), entry("100644", "..", 2)),
			"bad-name-unsort":  join(entry("100644", "b", 1), entry("100644", "a", 2), entry("100644", ".", 3)),
			"empty-name":       join(entry("100644", "a", 1), entry("100644", "", 2)),
			"bad-mode":         join(entry("100644", "a", 1), entry("10x644", "b", 2)),
			"no-space":         join(entry("100644", "a", 1), append([]byte("100644b\x00"), oid(2)...)),
			"short-oid":        join(entry("100644", "a", 1), []byte("100644 b\x00\x01\x02")),
			"no-nul":           join(entry("100644", "a", 1), []byte("100644 b")),
			"match-before-bad": join(entry("100644", "a", 1), entry("100644", "b", 2), []byte("100644 c")),
		}
		names := []string{"", "a", "a.c", "b", "c", "d", "e", "f", "f000.js", "f150.js", "f299.js", "f300.js", "zzz", ".."}
		for label, raw := range bodies {
			for _, name := range names {
				got, gotOK, gotErr := findTreeEntry(raw, name, width)
				entries, wantErr := treeEntries(raw, width)
				want, wantOK := entries[name]
				if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || (wantErr == nil && (got != want || gotOK != wantOK)) || (wantErr != nil && (gotOK || got != TreeEntry{})) {
					t.Errorf("width %d %s %q: findTreeEntry = %+v, %v, %v; decoded = %+v, %v, %v", width, label, name, got, gotOK, gotErr, want, wantOK, wantErr)
				}
			}
		}
	}
}
