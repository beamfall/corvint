//go:build darwin || linux

package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTQDV0041_RepeatRefusalHonoursProgressSignal: under a declared
// HANDOFF_EVIDENCE signal, review returns of one inherited candidate tree
// separated by hand-offs whose evidence fingerprint changed are not repeats;
// with an unchanged fingerprint they still are and the finding names the
// signal; without the declaration the count is as before.
func TestTQDV0041_RepeatRefusalHonoursProgressSignal(t *testing.T) {
	t.Run("TQD-V0-041 RepeatRefusalHonoursProgressSignal", func(t *testing.T) {
		gens := func(refs ...string) []cli.DoctorTestEnded {
			var out []cli.DoctorTestEnded
			for i, ref := range refs {
				tree := ""
				if i == 0 {
					tree = "1111111111111111111111111111111111111111"
				}
				out = append(out,
					cli.DoctorTestEnded{Generation: string(rune('1' + 2*i)), Disposition: wire.CodeHandoff, Tree: tree, Evidence: ref},
					cli.DoctorTestEnded{Generation: string(rune('2' + 2*i)), Disposition: wire.CodeReviewReturned})
			}
			return out
		}
		changing := gens("local:a", "local:b", "local:c")
		same := gens("local:a", "local:a", "local:a")
		if got := cli.DoctorRepeatRefusalDetails(changing, ""); len(got) != 1 || strings.Contains(got[0], "progress signal") || !strings.Contains(got[0], "refused 3 times") {
			t.Fatalf("undeclared: %q", got)
		}
		if got := cli.DoctorRepeatRefusalDetails(changing, intent.ProgressSignalHandoffEvidence); len(got) != 0 {
			t.Fatalf("changed fingerprints reported: %q", got)
		}
		if got := cli.DoctorRepeatRefusalDetails(same, intent.ProgressSignalHandoffEvidence); len(got) != 1 || !strings.Contains(got[0], "refused 3 times") || !strings.Contains(got[0], "(progress signal HANDOFF_EVIDENCE)") {
			t.Fatalf("unchanged fingerprints: %q", got)
		}
	})
}

// TestTQDV0042_SetupOnlyProofIgnoresProgressSignal: a declared progress
// signal is not an obligation witness, so SETUP_ONLY_PROOF judges and words
// a completion exactly as without it.
func TestTQDV0042_SetupOnlyProofIgnoresProgressSignal(t *testing.T) {
	t.Run("TQD-V0-042 SetupOnlyProofIgnoresProgressSignal", func(t *testing.T) {
		r, id, head := obligationRepo(t)
		seedObligations(t, r.Root, id)
		progressCLIPolicy(t, r.Root, false)
		if x := atm(t, r.Root, nil, declareArgs(id, "w-setup", obligationRevision(t, r.Root, id), head, "AC-3")...); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("witness: %s", x.stdout)
		}
		if x := atm(t, r.Root, nil, "ticket", "complete-manual", "--request-id", "complete", "--target", id, "--expected-revision", obligationRevision(t, r.Root, id),
			"--payload", `{"evidence":[],"reason":"manual"}`); x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("complete: %s", x.stdout)
		}
		_, item := doctorOK(t, r.Root)
		got := findingsOf(item, "SETUP_ONLY_PROOF")
		if len(got) != 1 || field(got[0], "detail").Str != "completed with 0 of 2 core obligations witnessed (1 of 9 total)" {
			t.Fatalf("setup-only: %s", wire.Encode(item))
		}
	})
}
