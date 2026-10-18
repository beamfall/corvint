//go:build darwin || linux

package cli_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// progressCLIPolicy declares progressSignal HANDOFF_EVIDENCE (and, when loop,
// loopDetection with bound 2) through policy update.
func progressCLIPolicy(t *testing.T, root string, loop bool) {
	t.Helper()
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, e := intent.Load(repo.PrimaryWorktree)
	if e != nil {
		t.Fatal(e)
	}
	v, err := wire.Parse(loaded.Policy.Raw)
	if err != nil {
		t.Fatal(err)
	}
	v.Obj.Set("policyVersion", wire.String(fmt.Sprint(loaded.Policy.PolicyVersion.Uint64()+1)))
	v.Obj.Set("progressSignal", wire.String(intent.ProgressSignalHandoffEvidence))
	if loop {
		v.Obj.Set("loopDetection", wire.ObjectValue(wire.NewObject().Set("maxAlternatingReturns", wire.String("2")).Set("maxNoProgressGenerations", wire.String("2"))))
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "policy.json")
	fixture.Write(t, path, wire.EncodeFile(v))
	if x := handoffCLI(t, root, "policy", "update", "--request-id", "progress-policy", "--expected-policy-version", string(loaded.Policy.PolicyVersion), "--file", path); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("policy update: %s", x.stdout)
	}
}

// progressCLIHandoffs claims id at stage implement once per ref and releases
// each generation as a no-tree hand-off whose evidence reference is ref.
func progressCLIHandoffs(t *testing.T, root, id string, refs ...string) {
	t.Helper()
	for i, ref := range refs {
		c := handoffCLI(t, root, "claim", id, "--holder", fmt.Sprintf("p%d", i), "--stage", "implement", "--request-id", fmt.Sprintf("pclaim-%d", i))
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, c.stdout)
		}
		a, g := field(c.res.Items[0], "attemptId").Str, field(c.res.Items[0], "generation").Str
		if r := handoffCLI(t, root, "release", "--attempt", a, "--generation", g, "--reason", wire.CodeHandoff, "--evidence", ref, "--request-id", fmt.Sprintf("prelease-%d", i)); r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release %d: %s", i, r.stdout)
		}
	}
}

// ATR-V0-082/083 through the CLI: under a declared HANDOFF_EVIDENCE signal,
// hand-offs whose evidence reference changes raise no NO_PROGRESS_HANDOFF
// finding and no loop hold, while repeated references still do, and both
// name the signal that judged them; the plan entry carries it as
// loop.progressSignal. Without the declaration changing references are
// still reported as before (ATR-V0-082(3)).
func TestATRV0082_CLIProgressSignal(t *testing.T) {
	t.Run("ATR-V0-082 ATR-V0-083 CLIProgressSignal", func(t *testing.T) {
		changing := []string{"local:a", "local:b", "local:c"}
		same := []string{"local:a", "local:a", "local:a", "local:a"}

		// Doctor only (no loopDetection): changed references are progress.
		root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		progressCLIPolicy(t, root, false)
		id := planTicket(t, root, "progressing", "P1", `["src/"]`)
		progressCLIHandoffs(t, root, id, changing...)
		if _, item := doctorOK(t, root); len(findingsOf(item, "NO_PROGRESS_HANDOFF")) != 0 {
			t.Fatalf("changed fingerprints reported: %s", wire.Encode(item))
		}

		root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		progressCLIPolicy(t, root, false)
		id = planTicket(t, root, "spinning", "P1", `["src/"]`)
		progressCLIHandoffs(t, root, id, same...)
		_, item := doctorOK(t, root)
		got := findingsOf(item, "NO_PROGRESS_HANDOFF")
		if len(got) != 1 || !strings.Contains(string(wire.Encode(got[0])), "progress signal HANDOFF_EVIDENCE") {
			t.Fatalf("unchanged fingerprints: %s", wire.Encode(item))
		}

		// Undeclared: changing references are not a progress signal.
		root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		id = planTicket(t, root, "plain", "P1", `["src/"]`)
		progressCLIHandoffs(t, root, id, changing...)
		_, item = doctorOK(t, root)
		got = findingsOf(item, "NO_PROGRESS_HANDOFF")
		if len(got) != 1 || strings.Contains(string(wire.Encode(got[0])), "progress signal") {
			t.Fatalf("undeclared: %s", wire.Encode(item))
		}

		// Loop detection plus the signal: changed references never hold.
		root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		progressCLIPolicy(t, root, true)
		id = planTicket(t, root, "loop-progressing", "P1", `["src/"]`)
		progressCLIHandoffs(t, root, id, changing...)
		if c := handoffCLI(t, root, "claim", id, "--holder", "late", "--stage", "implement", "--request-id", "claim-late"); c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("changed fingerprints held: %s", c.stdout)
		}

		root, _ = leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
		progressCLIPolicy(t, root, true)
		id = planTicket(t, root, "loop-spinning", "P1", `["src/"]`)
		progressCLIHandoffs(t, root, id, same...)
		plan := handoffCLI(t, root, "plan", "preview")
		entries := field(plan.res.Items[0], "entries").Arr
		if len(entries) != 1 || field(entries[0], "reason").Str != wire.CodeLoopDetected || field(field(entries[0], "loop"), "progressSignal").Str != intent.ProgressSignalHandoffEvidence {
			t.Fatalf("plan: %s", plan.stdout)
		}
		show := handoffCLI(t, root, "ticket", "show", id)
		if !strings.Contains(string(show.stdout), "progress signal HANDOFF_EVIDENCE") {
			t.Fatalf("show: %s", show.stdout)
		}
		if obs, err := cli.ObserveDispatch(root); err != nil || len(obs.Tickets) != 1 || obs.Tickets[0].Loop == nil || obs.ProgressSignal != intent.ProgressSignalHandoffEvidence {
			t.Fatalf("dispatch observation: %v %+v", err, obs)
		}
	})
}

// progressCLIFingerprints claims id at stage implement once per ref and
// releases each generation as a HANDOFF with --evidence ref, or, when ref is
// empty, as a plain release that records no work and no reference, and
// returns the dispatcher's CAL-V0-057 fingerprint of the ticket
// observed after each release.
func progressCLIFingerprints(t *testing.T, root, id string, refs ...string) []string {
	t.Helper()
	var fps []string
	for i, ref := range refs {
		c := handoffCLI(t, root, "claim", id, "--holder", fmt.Sprintf("f%d", i), "--stage", "implement", "--request-id", fmt.Sprintf("fclaim-%d", i))
		if c.res.Outcome != wire.OutcomeOK {
			t.Fatalf("claim %d: %s", i, c.stdout)
		}
		args := []string{"release", "--attempt", field(c.res.Items[0], "attemptId").Str, "--generation", field(c.res.Items[0], "generation").Str, "--request-id", fmt.Sprintf("frelease-%d", i)}
		if ref != "" {
			args = append(args, "--reason", wire.CodeHandoff, "--evidence", ref)
		}
		if r := handoffCLI(t, root, args...); r.res.Outcome != wire.OutcomeOK {
			t.Fatalf("release %d: %s", i, r.stdout)
		}
		obs, err := cli.ObserveDispatch(root)
		if err != nil || len(obs.Tickets) != 1 || obs.ProgressSignal != intent.ProgressSignalHandoffEvidence {
			t.Fatalf("observe %d: %v %+v", i, err, obs)
		}
		fps = append(fps, dispatch.Fingerprint(obs, obs.Tickets[0].ID))
	}
	return fps
}

// ATR-V0-084 through the store: the dispatcher's fingerprint carries the
// latest recorded hand-off reference across a generation that records none,
// because the next claim clears the current one. So a reference followed by
// a hand-off without evidence is no progress, a repeated reference is no
// progress, and only a new reference is, with or without loopDetection.
func TestATRV0084_DispatchFingerprintCarriesLatestReference(t *testing.T) {
	t.Run("ATR-V0-084 DispatchFingerprintCarriesLatestReference", func(t *testing.T) {
		for _, loop := range []bool{false, true} {
			for _, c := range []struct {
				name     string
				refs     []string
				progress bool
			}{
				{"reference then none", []string{"local:a", ""}, false},
				{"reference then none then the same", []string{"local:a", "", "local:a"}, false},
				{"same reference", []string{"local:a", "local:a"}, false},
				{"new reference", []string{"local:a", "local:b"}, true},
				{"new reference after none", []string{"local:a", "", "local:b"}, true},
			} {
				root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
				progressCLIPolicy(t, root, loop)
				id := planTicket(t, root, "carry", "P1", `["src/"]`)
				fps := progressCLIFingerprints(t, root, id, c.refs...)
				changed := false
				for _, fp := range fps[1:] {
					changed = changed || fp != fps[0]
				}
				if last := fps[len(fps)-1] != fps[len(fps)-2]; changed != c.progress || last != c.progress {
					t.Fatalf("loopDetection %v, %s: fingerprints %v, want progress %v", loop, c.name, fps, c.progress)
				}
			}
		}
	})
}
