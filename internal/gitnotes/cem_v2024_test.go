package gitnotes

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// CEM-V1-345 (V1-2024): a pinned verdict ledger tip that is no longer the
// live refs/notes/corvint-verdicts tip or one of its ancestors reads
// `rollback`, lists no records, and writes nothing; an ancestor or equal tip
// reads `current` beside the verified listing.
func TestCEMV1345RewoundVerdictTipReadsRollback(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	ctx := context.Background()
	// An older ledger commit: a note on the first commit, not HEAD.
	fixtureGit(t, root, nil, "notes", "--ref="+VerdictRef, "add", "-m", "older", "HEAD~2")
	older := fixtureGit(t, root, nil, "rev-parse", VerdictRef)
	if _, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true); err != nil {
		t.Fatal(err)
	}
	newer := fixtureGit(t, root, nil, "rev-parse", VerdictRef)
	if newer == older {
		t.Fatal("anchor did not advance the ledger ref")
	}
	read := func(pinned string) map[string]any {
		t.Helper()
		before := storeState(t, root)
		result, err := ProvenanceWithVerdictsTip(ctx, root, "HEAD", "", true, pinned)
		if err != nil {
			t.Fatalf("pinned %s: %v", pinned, err)
		}
		if storeState(t, root) != before {
			t.Fatal("the pinned-tip read changed the repository")
		}
		return result["verdicts"].(map[string]any)
	}
	for _, pinned := range []string{newer, older} {
		member := read(pinned)
		tip := member["tip"].(map[string]any)
		if tip["state"] != "current" || tip["live"] != newer || tip["ref"] != VerdictRef ||
			member["state"] != "verified" || len(member["records"].([]VerdictRecord)) != 1 {
			t.Fatalf("pinned %s: %v", pinned, member)
		}
	}

	fixtureGit(t, root, nil, "update-ref", VerdictRef, older)
	member := read(newer)
	tip := member["tip"].(map[string]any)
	if tip["state"] != "rollback" || tip["live"] != older || tip["pinned"] != newer ||
		member["state"] != "rollback" || len(member["records"].([]VerdictRecord)) != 0 {
		t.Fatalf("rewound ledger: %v", member)
	}

	fixtureGit(t, root, nil, "update-ref", VerdictRef, newer)
	fixtureGit(t, root, nil, "update-ref", "-d", VerdictRef)
	member = read(newer)
	if tip := member["tip"].(map[string]any); tip["state"] != "rollback" || tip["live"] != nil || member["state"] != "rollback" {
		t.Fatalf("deleted ledger ref: %v", member)
	}

	if _, err := ProvenanceWithVerdictsTip(ctx, root, "HEAD", "", false, newer); cemcode.CodeOf(err) != cemcode.InvalidArguments {
		t.Fatalf("--verdicts-tip without --verdicts: %v", err)
	}
	if _, err := ProvenanceWithVerdictsTip(ctx, root, "HEAD", "", true, VerdictRef); cemcode.CodeOf(err) != cemcode.InvalidArguments ||
		!strings.Contains(err.Error(), "--verdicts-tip") {
		t.Fatalf("revision name accepted as --verdicts-tip: %v", err)
	}
	plain, err := ProvenanceWithVerdicts(ctx, root, "HEAD", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := plain["verdicts"].(map[string]any)["tip"]; present {
		t.Fatal("a listing without --verdicts-tip carries a tip")
	}
}

// anchorProcessEnv names the helper-process mode of TestCEMV1346AnchorProcess.
const anchorProcessEnv = "CORVINT_GITNOTES_ANCHOR_PROCESS"

// TestCEMV1346AnchorProcess is not a test on its own: run as a separate
// process by TestCEMV1346TwoProcessAnchorsOneWins, it anchors the map with a
// verdict in the repository its environment names. With the "hold" role it
// reports READY once its ledger is validated and staged, then waits for one
// line on stdin before publishing. It prints one RESULT line.
func TestCEMV1346AnchorProcess(t *testing.T) {
	root := os.Getenv(anchorProcessEnv)
	if root == "" {
		t.Skip("helper process for TestCEMV1346TwoProcessAnchorsOneWins")
	}
	if os.Getenv(anchorProcessEnv+"_ROLE") == "hold" {
		previous := beforePublish
		beforePublish = func() {
			beforePublish = previous
			fmt.Println("READY")
			_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		}
	}
	receipt, err := AnchorWithVerdict(context.Background(), root, mapPath, "HEAD", root, true)
	line, _ := json.Marshal(map[string]any{"code": cemcode.CodeOf(err), "written": receipt != nil && receipt["written"] == true})
	fmt.Printf("RESULT %s\n", line)
}

// CEM-V1-346 (V1-2024): two separate corvint processes anchor the same map
// concurrently. The first validates and stages, then holds before publishing
// while the second runs to completion; exactly one wins, the held writer is
// refused verdict-ledger-conflict, and the ledger holds only the winner's
// record with no staging ref left behind.
func TestCEMV1346TwoProcessAnchorsOneWins(t *testing.T) {
	root := fixtureRepo(t)
	head := fixtureGit(t, root, nil, "rev-parse", "HEAD")
	process := func(role string) *exec.Cmd {
		command := exec.Command(os.Args[0], "-test.run=^TestCEMV1346AnchorProcess$", "-test.count=1")
		command.Env = append(os.Environ(), anchorProcessEnv+"="+root, anchorProcessEnv+"_ROLE="+role)
		return command
	}
	held := process("hold")
	stdin, err := held.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := held.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Process.Kill(); _ = held.Wait() })
	heldOut := bufio.NewReader(stdout)
	if line, err := heldOut.ReadString('\n'); err != nil || line != "READY\n" {
		t.Fatalf("held writer never reached publish: %q %v", line, err)
	}
	if ledgerNote(t, root, head) != "" {
		t.Fatal("the held writer published before it was released")
	}
	free, err := process("free").Output()
	if err != nil {
		t.Fatalf("free writer: %v\n%s", err, free)
	}
	afterWinner := ledgerNote(t, root, head)
	if _, err := io.WriteString(stdin, "go\n"); err != nil {
		t.Fatal(err)
	}
	rest, err := io.ReadAll(heldOut)
	if err != nil {
		t.Fatal(err)
	}
	if err := held.Wait(); err != nil {
		t.Fatalf("held writer: %v\n%s", err, rest)
	}
	winner, loser := helperResult(t, free), helperResult(t, rest)
	if winner["code"] != "" || winner["written"] != true {
		t.Fatalf("free writer did not win: %v", winner)
	}
	if loser["code"] != CodeVerdictConflict || loser["written"] != false {
		t.Fatalf("held writer was not refused: %v", loser)
	}
	if afterWinner == "" || strings.Count(afterWinner, "\n") != 1 || ledgerNote(t, root, head) != afterWinner {
		t.Fatalf("ledger %q, want only the winner's record %q", ledgerNote(t, root, head), afterWinner)
	}
	if fixtureGit(t, root, nil, "for-each-ref", stagingRefPrefix) != "" {
		t.Fatal("a staging ref survived")
	}
}

func helperResult(t *testing.T, out []byte) map[string]any {
	t.Helper()
	for _, line := range strings.Split(string(out), "\n") {
		if payload, found := strings.CutPrefix(line, "RESULT "); found {
			var result map[string]any
			if err := json.Unmarshal([]byte(payload), &result); err != nil {
				t.Fatal(err)
			}
			return result
		}
	}
	t.Fatalf("helper printed no RESULT line:\n%s", out)
	return nil
}

// CEM-V1-345 (V1-2024, review finding): with --verdicts-tip the ledger is
// read from the captured live commit by object ID. A ledger ref moved A -> B
// -> A between the tip capture and the unmoved check passes that check, so
// the listing must still reflect only A's content, never B's.
func TestCEMV1345PinnedTipReadIgnoresARefMovedAwayAndBack(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	ctx := context.Background()
	if _, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true); err != nil {
		t.Fatal(err)
	}
	a := fixtureGit(t, root, nil, "rev-parse", VerdictRef)
	// B: the same ledger with HEAD's note replaced by a malformed one.
	fixtureGit(t, root, nil, "notes", "--ref="+VerdictRef, "add", "-f", "-m", "not a record", "HEAD")
	b := fixtureGit(t, root, nil, "rev-parse", VerdictRef)
	fixtureGit(t, root, nil, "update-ref", VerdictRef, a)
	if b == a {
		t.Fatal("B did not change the ledger")
	}

	previous := verdictsRead
	t.Cleanup(func() { verdictsRead = previous })
	verdictsRead = func(stage string) {
		switch stage {
		case "before":
			fixtureGit(t, root, nil, "update-ref", VerdictRef, b)
		case "after":
			fixtureGit(t, root, nil, "update-ref", VerdictRef, a)
		}
	}
	result, err := ProvenanceWithVerdictsTip(ctx, root, "HEAD", "", true, a)
	if err != nil {
		t.Fatal(err)
	}
	member := result["verdicts"].(map[string]any)
	tip := member["tip"].(map[string]any)
	if tip["state"] != "current" || tip["live"] != a || member["state"] != "verified" ||
		len(member["records"].([]VerdictRecord)) != 1 {
		t.Fatalf("a ref moved A->B->A leaked B's ledger: %v", member)
	}
}
