// SPDX-License-Identifier: Apache-2.0
package forwardrow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAFPV0760ListNamesSplitsATruncatedPage runs list_names of names.sh against a gh spy that
// serves an artifact listing at any page size, fails the named pages as a truncated response
// does, and logs every read: a failed page of 100 is read as its four pages of 25 and the
// names, pages_read, exhausted and limit equal those of a clean listing of pages of 100.
func TestAFPV0760ListNamesSplitsATruncatedPage(t *testing.T) {
	type result struct {
		names, reads              []string
		pagesRead, exhausted, lim string
	}
	// list lists `total` artifacts, every seventh of another branch, with PAGES `pages` and the
	// named "SIZE-PAGE" reads failing.
	list := func(t *testing.T, total, pages int, fail ...string) result {
		dir := t.TempDir()
		bin := filepath.Join(dir, "bin")
		writeFile(t, filepath.Join(bin, "gh"), []byte(`#!/bin/sh
d='`+dir+`'
q=${2#*per_page=}; pp=${q%%&*}; p=${q##*page=}
echo "$pp-$p" >> "$d/reads"
[ ! -f "$d/fail-$pp-$p" ] || { echo "unexpected end of JSON input" >&2; exit 1; }
jq -r --argjson pp "$pp" --argjson p "$p" '{artifacts: .artifacts[($p - 1) * $pp:$p * $pp]}' "$d/all.json" | jq -r "$4"
`))
		writeFile(t, filepath.Join(bin, "sleep"), []byte("#!/bin/sh\necho \"sleep $1\" >> '"+dir+"/reads'\n"))
		for _, tool := range []string{"gh", "sleep"} {
			if err := os.Chmod(filepath.Join(bin, tool), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		var all []any
		for i := 0; i < total; i++ {
			branch := "main"
			if i%7 == 6 {
				branch = "claude/x"
			}
			all = append(all, doc{"name": fmt.Sprintf("a-%d", i), "expired": false, "created_at": fmt.Sprintf("t%d", i),
				"workflow_run": doc{"head_branch": branch, "repository_id": 1, "head_repository_id": 1}})
		}
		writeDoc(t, filepath.Join(dir, "all.json"), doc{"artifacts": all})
		for _, f := range fail {
			writeFile(t, filepath.Join(dir, "fail-"+f), nil)
		}
		out := run(t, []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "REPO=beamfall/corvint", "TMPDIR=" + dir},
			"sh", "-c", `. "$1"; list_names "$2" "$3/names" "$3/times"; printf '%s|%s|%s\n' "$pages_read" "$exhausted" "$limit"`,
			"sh", script(t, "names.sh"), fmt.Sprint(pages), dir)
		f := strings.SplitN(strings.TrimSuffix(out, "\n"), "|", 3)
		names, _ := os.ReadFile(filepath.Join(dir, "names"))
		times, _ := os.ReadFile(filepath.Join(dir, "times"))
		reads, _ := os.ReadFile(filepath.Join(dir, "reads"))
		r := result{names: strings.Fields(string(names)), reads: strings.Fields(strings.ReplaceAll(string(reads), "sleep ", "sleep-")),
			pagesRead: f[0], exhausted: f[1], lim: f[2]}
		if n := len(strings.Split(strings.TrimSpace(string(times)), "\n")); len(r.names) > 0 && n != len(r.names) {
			t.Fatalf("%d times for %d names", n, len(r.names))
		}
		return r
	}
	mainNames := func(from, to int) []string {
		var names []string
		for i := from; i < to; i++ {
			if i%7 != 6 {
				names = append(names, fmt.Sprintf("a-%d", i))
			}
		}
		return names
	}
	check := func(t *testing.T, r result, names []string, pagesRead, exhausted, reads string) {
		t.Helper()
		if strings.Join(r.names, " ") != strings.Join(names, " ") || r.pagesRead != pagesRead || r.exhausted != exhausted ||
			strings.Join(r.reads, " ") != reads {
			t.Fatalf("names %d (want %d), pages_read %q exhausted %q limit %q reads %q; want pages_read %q exhausted %q reads %q",
				len(r.names), len(names), r.pagesRead, r.exhausted, r.lim, strings.Join(r.reads, " "), pagesRead, exhausted, reads)
		}
	}
	clean := list(t, 230, 5)
	check(t, clean, mainNames(0, 230), "3", "1", "100-1 100-2 100-3")
	if clean.lim != "" {
		t.Fatalf("clean listing limit %q", clean.lim)
	}
	// Page P of 100 is pages 4P-3 to 4P of 25, so a split page lists the same artifacts.
	split := list(t, 230, 5, "100-1")
	check(t, split, clean.names, "3", "1", "100-1 25-1 25-2 25-3 25-4 100-2 100-3")
	// A split last page ends the listing at its short page of 25.
	check(t, list(t, 230, 5, "100-3"), clean.names, "3", "1", "100-1 100-2 100-3 25-9 25-10")
	// The page budget still bounds a split page: two pages of 100 cover 200 artifacts, not exhausted.
	check(t, list(t, 500, 2, "100-2"), mainNames(0, 200), "2", "", "100-1 100-2 25-5 25-6 25-7 25-8")
	// A listing of exactly 200 ends at the empty third page, as with pages of 100.
	check(t, list(t, 200, 5, "100-2"), mainNames(0, 200), "3", "1", "100-1 100-2 25-5 25-6 25-7 25-8 100-3")
	// An unreadable page of 25 names the limit: three tries 5 s and 10 s apart, the artifacts
	// before it listed, the page of 100 not counted, never exhausted.
	failed := list(t, 230, 5, "100-2", "25-6")
	check(t, failed, mainNames(0, 125), "1", "", "100-1 100-2 25-5 25-6 sleep-5 25-6 sleep-10 25-6")
	if want := "artifact page 2 is unreadable: its page 6 of 25 failed after 3 tries (unexpected end of JSON input ); the count covers the artifacts before it"; failed.lim != want {
		t.Fatalf("limit %q, want %q", failed.lim, want)
	}
}
