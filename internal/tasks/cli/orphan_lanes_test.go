package cli_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0744_OrphanLanesListsUnbatchedLaneDeliveries pins V1-3856: a lane
// commit that delivers an OPEN ticket and that neither the integration ref
// nor a batch branch contains is an orphan; a lane already merged into a
// batch is not, a mention that does not deliver is not, and without a
// matching batch branch the batched lane is listed with a NOT_OBSERVED
// warning.
func TestTCLV0744_OrphanLanesListsUnbatchedLaneDeliveries(t *testing.T) {
	parallelTest(t) // actor through cli.Env.Actor; no env, cwd, hook, exec-mode or package-variable writes (V1-4187)
	setActor(t, "tester")
	root, _ := ergStore(t)
	orphanID, orphan := tcdTicket(t, root, "orphaned", "")
	_, batched := tcdTicket(t, root, "batched", "")
	_, cited := tcdTicket(t, root, "cited", "")
	out, err := exec.Command("git", "-C", root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	main := strings.TrimSpace(string(out))
	head := func() string {
		out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}
	git(t, root, "checkout", "-q", "-b", "claude/lane-x")
	tcdCommit(t, root, "tasks: orphaned work\n\nRefs "+orphan)
	orphanCommit := head()
	tcdCommit(t, root, "docs: build log notes "+cited+" as NOT DELIVERED")
	git(t, root, "checkout", "-q", main)
	git(t, root, "checkout", "-q", "-b", "claude/lane-y")
	tcdCommit(t, root, "tasks: batched work\n\nRefs "+batched)
	git(t, root, "checkout", "-q", main)
	git(t, root, "checkout", "-q", "-b", "claude/batch-1")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "merge lane-y", "claude/lane-y")
	git(t, root, "checkout", "-q", main)

	x := atm(t, root, nil, "ticket", "orphan-lanes")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
		t.Fatalf("orphan-lanes: %s", x.stdout)
	}
	item := x.res.Items[0]
	rows := field(item, "orphans").Arr
	if field(item, "profile").Str != "taskman-orphan-lanes/0" || field(item, "orphanCount").Str != "1" || len(rows) != 1 ||
		field(rows[0], "ticketId").Str != orphanID || field(rows[0], "local").Str != orphan ||
		field(rows[0], "ref").Str != "refs/heads/claude/lane-x" || field(rows[0], "commit").Str != orphanCommit {
		t.Fatalf("orphans: %s", x.stdout)
	}
	if b := field(item, "batchRefs").Arr; len(b) != 1 || field(b[0], "ref").Str != "refs/heads/claude/batch-1" || len(x.res.Warnings) != 0 {
		t.Fatalf("batchRefs or warnings: %s", x.stdout)
	}
	if lh := field(item, "laneHeads").Arr; len(lh) != 2 {
		t.Fatalf("laneHeads: %s", wire.Encode(field(item, "laneHeads")))
	}

	x = atm(t, root, nil, "ticket", "orphan-lanes", "--batch", "nomatch/*")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "orphanCount").Str != "2" ||
		!strings.Contains(strings.Join(x.res.Warnings, " "), "in-flight batches are NOT_OBSERVED") {
		t.Fatalf("without a batch branch the batched lane is not listed with a warning: %s", x.stdout)
	}
	for _, args := range [][]string{{"V1-1"}, {"--limit", "0"}, {"--batch", "["}} {
		if x := atm(t, root, nil, append([]string{"ticket", "orphan-lanes"}, args...)...); x.res.Outcome == wire.OutcomeOK || x.res.Codes[0] != wire.CodeMalformed {
			t.Fatalf("%v was not refused MALFORMED: %s", args, x.stdout)
		}
	}
}

// TestTCLV0744_OrphanLanesBatchSharingACommitAndTruncatedBatch pins two
// review findings: a batch branch at the same commit as a lane branch still
// counts as batched (LaneHeads keeps one ref per commit), and a batch scan cut
// at --limit checks a candidate with merge-base instead of calling it an
// orphan because the scan did not reach it.
func TestTCLV0744_OrphanLanesBatchSharingACommitAndTruncatedBatch(t *testing.T) {
	parallelTest(t) // seam-carried actor; no env, cwd, hook, package-variable or in-process executable writes (V1-4208)
	setActor(t, "tester")
	root, _ := ergStore(t)
	_, shared := tcdTicket(t, root, "shared", "")
	_, deep := tcdTicket(t, root, "deep", "")
	main := strings.TrimSpace(gitOutput(t, root, "rev-parse", "--abbrev-ref", "HEAD"))
	// A lane branch and a batch branch whose name sorts after it, at one commit.
	git(t, root, "checkout", "-q", "-b", "claude/lane-v")
	tcdCommit(t, root, "tasks: shared work\n\nRefs "+shared)
	git(t, root, "branch", "claude/zz-batch-3")
	// A batch holding a lane commit two commits back.
	git(t, root, "checkout", "-q", main)
	git(t, root, "checkout", "-q", "-b", "claude/lane-d")
	tcdCommit(t, root, "tasks: deep work\n\nRefs "+deep)
	git(t, root, "checkout", "-q", main)
	git(t, root, "checkout", "-q", "-b", "claude/zz-batch-4")
	git(t, root, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "merge", "-q", "--no-ff", "-m", "merge lane-d", "claude/lane-d")
	tcdCommit(t, root, "batch: a later commit on the batch")
	git(t, root, "checkout", "-q", main)

	x := atm(t, root, nil, "ticket", "orphan-lanes", "--batch", "claude/zz-batch-*")
	if x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "orphanCount").Str != "0" {
		t.Fatalf("a batch at a lane's commit or holding the lane was not seen: %s", x.stdout)
	}
	x = atm(t, root, nil, "ticket", "orphan-lanes", "--batch", "claude/zz-batch-*", "--limit", "1")
	item := x.res.Items[0]
	if x.res.Outcome != wire.OutcomeOK || !field(item, "batchScanTruncated").Bool {
		t.Fatalf("--limit 1 did not truncate the batch scan: %s", x.stdout)
	}
	for _, r := range field(item, "orphans").Arr {
		if field(r, "local").Str == deep || field(r, "local").Str == shared {
			t.Fatalf("a truncated batch scan made batched work an orphan: %s", x.stdout)
		}
	}
	if len(field(item, "unknown").Arr) != 0 {
		t.Fatalf("a resolvable ancestry check was reported UNKNOWN: %s", x.stdout)
	}
}
