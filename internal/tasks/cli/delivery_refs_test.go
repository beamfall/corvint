package cli_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCDV0730_ReferencesVetoPerID covers V1-3713 criterion 3: a mid-line
// Delivers delivers every ID of its clause, a veto (ALREADY-DELIVERED,
// BLOCKED, "blocked on", "not started") mentions only the ID it governs, and
// a PR title's final parenthetical delivers its IDs unless the body vetoes
// one (TCD-V0-730).
func TestTCDV0730_ReferencesVetoPerID(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	var l []string
	for _, n := range []string{"alpha", "bravo", "charlie", "delta", "echo", "golf", "foxtrot", "hotel"} {
		_, local := tcdTicket(t, root, n, "")
		l = append(l, local)
	}
	a, b, c, d, e, g, f, h := l[0], l[1], l[2], l[3], l[4], l[5], l[6], l[7]
	body := "Lane summary: this lane Delivers " + a + ", " + b + "; ALREADY-DELIVERED " + c + "; BLOCKED " + d +
		"\n- " + e + " delivered, " + g + " blocked on owner\nResidual: " + h + " not started"
	prs := tdsExport(t, "prs.json", `[{"number":50,"state":"MERGED","title":"`+tdsJSON("CI test selection: narrow config-only changes ("+f+", "+h+")")+`","body":"`+tdsJSON(body)+`","labels":[]}]`)
	x := tdsRun(t, root, append([]string{"ticket", "delivery-screen", "--prs", prs}, l...)...)
	want := map[string]string{a: "DELIVERED", b: "DELIVERED", c: "MENTIONED", d: "MENTIONED", e: "DELIVERED", g: "MENTIONED", f: "DELIVERED", h: "MENTIONED"}
	if got := tdsVerdicts(t, x); x.code != 1 || !mapsEqual(got, want) {
		t.Fatalf("exit %d verdicts %v, want %v", x.code, got, want)
	}
}

// TestTCDV0731_BatchDeliveredPR25And38Shapes covers V1-3713 criteria 1 and
// 2 with fixtures shaped like PR #25 (title parenthetical, a mid-line
// Delivers with ALREADY-DELIVERED and BLOCKED IDs, and a ticket only a lane
// commit delivers) and PR #38 (only a title parenthetical); a merge commit
// that does not resolve leaves the lane commits NOT_OBSERVED (TCD-V0-731).
func TestTCDV0731_BatchDeliveredPR25And38Shapes(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	var l []string
	for _, n := range []string{"alpha", "bravo", "charlie", "delta", "kilo", "echo", "foxtrot", "lima"} {
		_, local := tcdTicket(t, root, n, "")
		l = append(l, local)
	}
	a, b, c, d, k, e, f, lm := l[0], l[1], l[2], l[3], l[4], l[5], l[6], l[7]
	base := revParse(t, root, "HEAD")
	c1 := tdsCommit(t, root, "feat(tasks): doctor scan reports journalAudit ("+k+")", base)
	c2 := tdsCommit(t, root, "docs(build-log): lane decisions\n\nThe release still waits on "+lm+".", c1)
	m25 := tdsCommit(t, root, "Merge pull request #25 from x/lane-linux-w2", base, c2)
	c3 := tdsCommit(t, root, "fix: config-only narrowing", m25)
	m38 := tdsCommit(t, root, "Merge pull request #38 from x/lane-ci-select", m25, c3)
	prs := tdsExport(t, "prs.json", `[
{"number":25,"state":"MERGED","title":"`+tdsJSON("fix: linux-w2 bug lane ("+a+", "+b+", "+c+", "+d+")")+`","body":"`+tdsJSON("## Summary\nBug lane linux-w2. Delivers "+a+", "+b+"; ALREADY-DELIVERED "+c+" (fixed on main); "+d+" BLOCKED on the owner.")+`","headRefOid":"`+c2+`","mergeCommit":{"oid":"`+m25+`"},"labels":[]},
{"number":38,"state":"MERGED","title":"`+tdsJSON("CI test selection: narrow config-only changes ("+e+", "+f+")")+`","body":"`+tdsJSON("## Summary\nNarrows CI selection.")+`","headRefOid":"`+c3+`","mergeCommit":{"oid":"`+m38+`"},"labels":[]},
{"number":39,"state":"MERGED","title":"`+tdsJSON("fix: unfetched ("+e+")")+`","body":"","headRefOid":"`+c3+`","mergeCommit":{"oid":"`+strings.Repeat("a", 40)+`"},"labels":[]}]`)

	rows := func(item wire.Value, key string) string {
		var out []string
		for _, r := range field(item, key).Arr {
			s := field(r, "local").Str
			if cm := field(r, "commit"); cm.Kind != wire.KindNull {
				s += "@" + cm.Str
			}
			out = append(out, s)
		}
		return strings.Join(out, " ")
	}
	x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "25", "--prs", prs)
	item := x.res.Items[0]
	if x.code != 0 || rows(item, "delivered") != a+" "+b+" "+k+"@"+c1 || rows(item, "mentioned") != c+" "+d+" "+lm+"@"+c2 ||
		field(item, "laneCommits").Str != "2" || field(item, "laneCommitsTruncated").Bool {
		t.Fatalf("PR #25 shape: %s", x.stdout)
	}
	x = atm(t, root, nil, "ticket", "batch-delivered", "--pr", "38", "--prs", prs)
	item = x.res.Items[0]
	if x.code != 0 || rows(item, "delivered") != e+" "+f || rows(item, "mentioned") != "" || field(item, "laneCommits").Str != "1" {
		t.Fatalf("PR #38 shape: %s", x.stdout)
	}
	x = atm(t, root, nil, "ticket", "batch-delivered", "--pr", "39", "--prs", prs)
	item = x.res.Items[0]
	if x.code != 0 || rows(item, "delivered") != e || field(item, "laneCommits").Str != "NOT_OBSERVED" ||
		!strings.Contains(strings.Join(x.res.Warnings, "\n"), "TCD-V0-731") {
		t.Fatalf("unresolved merge: %s", x.stdout)
	}
}

// TestTCDV0731_CloseoutDebtSkipsDuplicateMergeRows covers V1-3713
// criterion 4: a merge commit whose message names the tickets its
// constituent already cites adds no umbrella rows, while a merge commit that
// is a ticket's only citer still lists it (TCD-V0-731).
func TestTCDV0731_CloseoutDebtSkipsDuplicateMergeRows(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	one, oneLocal := tcdTicket(t, root, "one", "")
	only, onlyLocal := tcdTicket(t, root, "only", "")
	git(t, root, "checkout", "-q", "-b", "side")
	tcdCommit(t, root, "fix: one ("+oneLocal+")")
	git(t, root, "checkout", "-q", "main")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m",
		"Merge pull request #7 from x/side\n\nfix: lane ("+oneLocal+", "+onlyLocal+")", "side")
	_, res := tcdRun(t, root)
	_, got := tcdRows(t, res)
	var s []string
	for _, r := range got {
		s = append(s, r.ticket+"/"+r.citation+"/"+r.subject)
	}
	want := one + "/direct-id/fix: one (" + oneLocal + ") " + only + "/umbrella/Merge pull request #7 from x/side"
	if strings.Join(s, " ") != want {
		t.Fatalf("rows %v, want %s", s, want)
	}
}
