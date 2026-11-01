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

	// A lane merge and the batch merge that encloses it are a ticket's only
	// citers: only the innermost (oldest) merge lists it.
	dup, dupLocal := tcdTicket(t, root, "dup", "")
	merge := func(msg, branch string) {
		git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", msg, branch)
	}
	git(t, root, "checkout", "-q", "-b", "lane2")
	tcdCommit(t, root, "lane two work")
	git(t, root, "checkout", "-q", "main")
	git(t, root, "checkout", "-q", "-b", "batch")
	merge("Merge #8 (x/lane2) into batch\n\nfix: lane two ("+dupLocal+")", "lane2")
	git(t, root, "checkout", "-q", "main")
	merge("Merge pull request #9 from x/batch\n\nbatch: lane two ("+dupLocal+")", "batch")
	_, res = tcdRun(t, root)
	_, got = tcdRows(t, res)
	var dups []string
	for _, r := range got {
		if r.ticket == dup {
			dups = append(dups, r.subject)
		}
	}
	if strings.Join(dups, "|") != "Merge #8 (x/lane2) into batch" {
		t.Fatalf("merge-only citer rows %v, want the lane merge only", dups)
	}
}

// TestTCDV0730_PRTitleMidLineDelivers covers the TCD-V0-730 rule for PR
// titles: a delivery word anywhere in the ID's clause of a title delivers
// it, and a veto in another clause of the title governs only its own ID,
// both in delivery-screen and in batch-delivered.
func TestTCDV0730_PRTitleMidLineDelivers(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, a := tcdTicket(t, root, "alpha", "")
	_, b := tcdTicket(t, root, "bravo", "")
	_, c := tcdTicket(t, root, "charlie", "")
	m := tdsCommit(t, root, "Merge pull request #60 from x/lane", revParse(t, root, "HEAD"), tdsCommit(t, root, "lane work", revParse(t, root, "HEAD")))
	prs := tdsExport(t, "prs.json", `[{"number":60,"state":"MERGED","title":"`+tdsJSON("Lane summary: Delivers "+a+", "+b+"; BLOCKED "+c)+`","body":"","mergeCommit":{"oid":"`+m+`"},"labels":[]}]`)
	x := tdsRun(t, root, "ticket", "delivery-screen", "--prs", prs, a, b, c)
	want := map[string]string{a: "DELIVERED", b: "DELIVERED", c: "MENTIONED"}
	if got := tdsVerdicts(t, x); !mapsEqual(got, want) {
		t.Fatalf("exit %d verdicts %v, want %v", x.code, got, want)
	}
	y := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "60", "--prs", prs)
	var delivered []string
	for _, r := range field(y.res.Items[0], "delivered").Arr {
		delivered = append(delivered, field(r, "local").Str)
	}
	if y.code != 0 || strings.Join(delivered, " ") != strings.Join(sortedLocals(a, b), " ") {
		t.Fatalf("batch-delivered title: %s", y.stdout)
	}
}

// TestTCDV0731_BatchOfLaneMergesReadsEveryLaneCommit: a batch PR whose
// branch merges two lane merges, each with its own ticket commits, delivers
// every lane commit's ticket in batch-delivered (laneCommits counts the
// non-merge commits only), and closeout-debt --prs proposes BATCH_CLOSE for
// those tickets (TCD-V0-731, TCD-V0-871).
func TestTCDV0731_BatchOfLaneMergesReadsEveryLaneCommit(t *testing.T) {
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	_, t1 := tcdTicket(t, root, "one", "")
	_, t2 := tcdTicket(t, root, "two", "")
	_, t3 := tcdTicket(t, root, "three", "")
	base := revParse(t, root, "HEAD")
	a1 := tdsCommit(t, root, "fix: lane a first ("+t1+")", base)
	a2 := tdsCommit(t, root, "lane a docs\n\nRefs "+t2, a1)
	b1 := tdsCommit(t, root, "feat: lane b ("+t3+")", base)
	ma := tdsCommit(t, root, "Merge #1 (x/lane-a) into batch", base, a2)
	mb := tdsCommit(t, root, "Merge #2 (x/lane-b) into batch", ma, b1)
	m := tdsCommit(t, root, "Merge pull request #40 from x/batch", base, mb)
	git(t, root, "update-ref", "refs/heads/main", m)
	prs := tdsExport(t, "prs.json", `[{"number":40,"state":"MERGED","title":"batch: two lanes","body":"Lanes merged locally.","headRefOid":"`+mb+`","mergeCommit":{"oid":"`+m+`"},"labels":[]}]`)

	x := atm(t, root, nil, "ticket", "batch-delivered", "--pr", "40", "--prs", prs)
	item := x.res.Items[0]
	got := map[string]string{}
	for _, r := range field(item, "delivered").Arr {
		got[field(r, "local").Str] = field(r, "commit").Str
	}
	want := map[string]string{t1: a1, t2: a2, t3: b1}
	if x.code != 0 || !mapsEqual(got, want) || field(item, "laneCommits").Str != "3" || field(item, "laneCommitsTruncated").Bool {
		t.Fatalf("batch of lane merges: delivered %v, want %v: %s", got, want, x.stdout)
	}

	_, res := tcdRun(t, root, "--prs", prs)
	cands, _ := res.Items[0].Obj.Get("candidates")
	proposal := map[string]string{}
	for _, c := range cands.Arr {
		proposal[field(c, "local").Str] = field(c, "proposal").Str + "#" + field(c, "pr").Str
	}
	for _, id := range []string{t1, t2, t3} {
		if proposal[id] != "BATCH_CLOSE#40" {
			t.Fatalf("closeout-debt proposals %v, want BATCH_CLOSE#40 for %s", proposal, id)
		}
	}
}
