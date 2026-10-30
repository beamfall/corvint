package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-343 (V1-2811): the commits of the previous-binding window, which lie
// before BASE, print one NOTE line with the count, the newest and oldest commit
// and the private file that lists every commit; when that file cannot be
// written the commits are listed inline, so none is dropped.
func TestDCWV0343WindowNoteIsOneLineWithList(t *testing.T) {
	commits := []string{}
	for i := range 91 {
		commits = append(commits, fmt.Sprintf("%040x", 91-i))
	}
	evidence := t.TempDir()
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", stderr: &stderr, base: "b"}, evidence: evidence}
	run.windowNote("unbound-commits", "unbound", commits, "p")
	list := evidence + "/dogfood-check.unbound-commits"
	want := fmt.Sprintf("dogfood-check: NOTE unbound-commits count=91 window=p..b newest=%s oldest=%s before-base list=%s\n", commits[0], commits[90], list)
	if stderr.String() != want {
		t.Fatalf("note = %q, want %q", stderr.String(), want)
	}
	listed, err := os.ReadFile(list)
	if err != nil || string(listed) != "unbound "+strings.Join(commits, "\nunbound ")+"\n" {
		t.Fatalf("list = %q (%v)", listed, err)
	}
	// Retroactive rows keep their binding in the list; newest and oldest name
	// the commits alone.
	stderr.Reset()
	run.windowNote("retroactive-bound-commits", "retroactive", []string{commits[0] + " binding=x", commits[1] + " binding=x"}, "p")
	if !strings.HasPrefix(stderr.String(), "dogfood-check: NOTE retroactive-bound-commits count=2 window=p..b newest="+commits[0]+" oldest="+commits[1]+" before-base list=") {
		t.Fatalf("retroactive note = %q", stderr.String())
	}
	// No commits print nothing.
	stderr.Reset()
	run.windowNote("unbound-commits", "unbound", nil, "p")
	if stderr.Len() != 0 {
		t.Fatalf("empty window printed %q", stderr.String())
	}
	// An unwritable evidence directory keeps the inline list.
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	run.evidence = blocked
	run.windowNote("unbound-commits", "unbound", commits[:2], "p")
	if stderr.String() != "dogfood-check: NOTE unbound-commits count=2 window=p..b\n  unbound "+commits[0]+"\n  unbound "+commits[1]+"\n" {
		t.Fatalf("fallback = %q", stderr.String())
	}
}

// DCW-V0-344 (V1-2811): below-anchored hunks print one aggregated WARN line
// with the hunk, tier and path counts, the cause and the fix, naming the
// private file that holds every per-hunk line; the unanchorable NOTE follows
// unchanged. Without a writable evidence directory the bounded per-hunk form
// of DCW-V0-112 stands.
func TestDCWV0344TierWarningsAggregateWithCause(t *testing.T) {
	paths, tiers := []string{}, []string{}
	for i := range maxTierWarnings + 11 {
		paths = append(paths, fmt.Sprintf("docs/p%d.md", i%7))
		tier := "cited"
		if i%10 == 0 {
			tier = "unknown"
		}
		tiers = append(tiers, tier)
	}
	line := tierStatusLine(paths, tiers)
	evidence := t.TempDir()
	var stderr bytes.Buffer
	run := &check{flow: flow{ctx: context.Background(), prefix: "dogfood-check", root: t.TempDir(), stderr: &stderr}, evidence: evidence}
	got := run.tierWarnings(line)
	list := evidence + "/dogfood-check.tier-below-anchored"
	want := "dogfood-check: WARN tier-below-anchored hunks=61 cited=54 unknown=7 paths=7 first=\"docs/p0.md\" list=" + list + " " + tierBelowAnchoredCause + " " + tierBelowAnchoredFix
	if len(got) != 1 || got[0] != want {
		t.Fatalf("tier lines = %q, want [%q]", got, want)
	}
	detail, err := os.ReadFile(list)
	perHunk := strings.Split(strings.TrimSuffix(string(detail), "\n"), "\n")
	if err != nil || len(perHunk) != 61 || perHunk[0] != belowAnchoredWarnings(line, nil)[0] {
		t.Fatalf("detail: %d lines (%v)", len(perHunk), err)
	}
	// No below-anchored hunk: no line, and a former list is removed.
	if got := run.tierWarnings(tierStatusLine([]string{"a.go"}, []string{"anchored"})); len(got) != 0 {
		t.Fatalf("anchored: %q", got)
	}
	if _, err := os.Stat(list); !os.IsNotExist(err) {
		t.Fatalf("stale list kept: %v", err)
	}
	// Unwritable evidence: the bounded per-hunk lines.
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	run.evidence = blocked
	if got := run.tierWarnings(line); len(got) != maxTierWarnings+1 || got[maxTierWarnings] != "dogfood-check: WARN tier-below-anchored more=11" {
		t.Fatalf("fallback: %d lines", len(got))
	}
}
