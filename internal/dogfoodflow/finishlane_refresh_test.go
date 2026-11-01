package dogfoodflow

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
)

// TestDCWV0947FinishRefreshesAMovedPlan (V1-3638): a plan drafted before
// HEAD moved is refreshed before any step: a refresh that only re-keys
// continues on the refreshed file, one that must draft a hunk afresh stops,
// and a current plan is used unchanged. The caller's plan is never rewritten.
func TestDCWV0947FinishRefreshesAMovedPlan(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q", "-b", "main")
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a\n")
	write(".gitignore", ".corvint/\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	write("a.txt", "lane\n")
	testGit(t, root, "commit", "-q", "-am", "lane")
	var drafted, notes bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &drafted, &notes); err != nil || code != 0 {
		t.Fatalf("draft: code=%d err=%v\n%s", code, err, &notes)
	}
	oldID, _, _ := strings.Cut(drafted.String(), "\t")
	plan := filepath.Join(t.TempDir(), "plan.tsv")
	original := oldID + "\tunknown\tinsufficient-evidence\treviewed\n"
	if err := os.WriteFile(plan, []byte(original), 0o666); err != nil {
		t.Fatal(err)
	}
	intents := filepath.Join(t.TempDir(), "intents.txt")
	if err := os.WriteFile(intents, []byte("#no-intent-declared\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	gitDir := testGit(t, root, "rev-parse", "--absolute-git-dir")
	refreshed := gitDir + "/corvint/" + finishRefreshedPlan
	run := func() (int, string, int) {
		t.Helper()
		calls := 0
		steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
			calls++
			return 1
		}}
		var stdout, stderr bytes.Buffer
		code, err := FinishLane(context.Background(), FinishLaneOptions{Preflight: true, Change: ChangeOptions{Root: root, Base: base, Steps: steps, Citations: plan, IntentsFile: intents, Verify: "go version", Outcome: "passed"}}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		return code, stderr.String(), calls
	}

	// A current plan is used unchanged.
	if _, stderr, _ := run(); strings.Contains(stderr, "citations-refresh") || isRegular(refreshed) {
		t.Fatalf("a current plan was refreshed:\n%s", stderr)
	}
	// So is a current plan rerun over a leftover map that already records its
	// row's hunk (reasoned unknown, so no longer owed), as after a failed bind:
	// staleness is judged against the canonical patch, not the owed hunks.
	var changeErr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Citations: plan, IntentsFile: intents, Steps: Runner{Path: "corvint", Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		if len(args) > 0 && args[0] == "cem" {
			return cemcli.Run(ctx, root, args[1:], stdout, stderr)
		}
		return 1
	}}}, &changeErr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readFile(filepath.Join(root, ".corvint/change.cem.json"))), oldID) {
		t.Fatalf("no leftover map naming %s:\n%s", oldID, &changeErr)
	}
	if _, stderr, _ := run(); strings.Contains(stderr, "citations-refresh") || isRegular(refreshed) {
		t.Fatalf("a current plan over a leftover map was refreshed:\n%s", stderr)
	}
	// HEAD moves: the same BASE range gets new content, so its hunk ID changes
	// and the row is re-keyed; the run continues on the refreshed plan.
	write("a.txt", "lane 2\n")
	testGit(t, root, "commit", "-q", "-am", "lane 2")
	_, stderr, calls := run()
	if !strings.Contains(stderr, "dogfood-inputs-check: NOTE citations-refreshed plan="+refreshed+" carried=1 rekeyed=1\n") || calls == 0 {
		t.Fatalf("re-keyed plan: calls=%d\n%s", calls, stderr)
	}
	data := string(readFile(refreshed))
	newID, rest, _ := strings.Cut(strings.TrimSuffix(data, "\n"), "\t")
	if newID == oldID || !strings.HasPrefix(newID, "hunk:sha256:") || rest != "unknown\tinsufficient-evidence\t"+staleNotePrefix+oldID+", review] reviewed" {
		t.Fatalf("refreshed plan %q (old ID %s)", data, oldID)
	}
	if string(readFile(plan)) != original {
		t.Fatal("the caller's plan was rewritten")
	}
	// The refreshed plan, reviewed and passed back as CITATIONS after HEAD moves
	// again, is refreshed into the other file and kept byte-identical; the two
	// files alternate, so a refresh never writes over its own input. The review
	// drops the stale mark the re-key added (V1-3289).
	reviewed := strings.Replace(data, staleNotePrefix+oldID+", review] reviewed\n", "reviewed again\n", 1)
	if reviewed == data {
		t.Fatalf("no stale mark to review in %q", data)
	}
	if err := os.WriteFile(refreshed, []byte(reviewed), 0o600); err != nil {
		t.Fatal(err)
	}
	write("a.txt", "lane 3\n")
	testGit(t, root, "commit", "-q", "-am", "lane 3")
	alternate := gitDir + "/corvint/" + finishRefreshedPlanAlt
	firstPlan := plan
	plan = refreshed
	_, stderr, _ = run()
	if !strings.Contains(stderr, "dogfood-inputs-check: NOTE citations-refreshed plan="+alternate+" carried=1 rekeyed=1\n") {
		t.Fatalf("refreshing the refreshed plan:\n%s", stderr)
	}
	if string(readFile(refreshed)) != reviewed {
		t.Fatalf("the reviewed refreshed plan, the refresh's own input, was rewritten: %q", readFile(refreshed))
	}
	if _, rest, _ := strings.Cut(strings.TrimSuffix(string(readFile(alternate)), "\n"), "\t"); rest != "unknown\tinsufficient-evidence\t"+staleNotePrefix+newID+", review] reviewed again" {
		t.Fatalf("alternate plan %q", readFile(alternate))
	}
	plan = firstPlan
	// A new hunk the plan never named must be drafted afresh: the run stops
	// before any step, naming it and the plan to review.
	write("b.txt", "new\n")
	testGit(t, root, "add", "b.txt")
	testGit(t, root, "commit", "-q", "-m", "new file")
	code, stderr, calls := run()
	if code != 2 || calls != 0 {
		t.Fatalf("incomplete refresh: code=%d calls=%d\n%s", code, calls, stderr)
	}
	for _, want := range []string{
		"dogfood-inputs-check: REFUSE citations-refresh-incomplete plan=" + refreshed + " carried=1 rekeyed=1 dropped=0 drafted=1\n",
		"path=b.txt BASE 0,0\n",
		"  fix: review the drafted rows in " + refreshed + " (a drafted citation is a proposal, not evidence), then rerun with CITATIONS=" + refreshed + "\n",
	} {
		if !strings.Contains(stderr, want) {
			t.Errorf("refusal lacks %q:\n%s", want, stderr)
		}
	}
}
