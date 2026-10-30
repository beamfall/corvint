# Decision 0432: urfave/cli replaces the ineligible cobra repository selection

Date: 2026-09-29. Status: accepted owner selection, conditional on the final provenance check;
that bounded check found no prior use. Preregistration review and public integration are separate.
Tickets: V1-0019, V1-0431; V1-0020 and V1-0021 retain their qualification and promotion gates.

## Owner answer and admission finding

The owner answered **"Use urfave/cli (recommended)"** to a choice explicitly conditioned on the
final provenance check finding no prior Corvint use. The integration coordinator reserved 0427.
This prospectively supersedes decision 0425 item 4's choice of spf13/cobra only, and the naming
of spf13/cobra in decision 0427 (close the three unproven README rows) item 4(a).

Numbering: this record was prepared on 2026-09-29 as 0427 but stayed unpublished while `main`
accepted a different decision 0427 on 2026-10-01. It is published as 0432 on 2026-10-05 with its
content unchanged. The sealed `urfave/preregistration.json` (deviation 1, `decision`) and
`urfave/admission.json` keep the reserved label `0427`; in those two files it means this decision.
They are not edited, so the preregistration digest below stays valid.

Sequencing deviation: item 4 required the independent review before the `.sha256` seal was
written, and no review of the draft digests was retained before commit `b436fadd` wrote it. On
2026-10-05 an independent read-only Codex review of the published files confirmed the three digests
(preregistration `eedf799f...`, corpus `b9cf1407...`, harness `b67cc95a...`), the unchanged
measurements, thresholds and invalidation rules, and the corpus's internal consistency, and returned
the missing pre-seal record as its only blocking finding. The owner then accepted the seal as it
stands ("you have my explicit approval", 2026-10-05). No Corvint invocation had run against these
cases, so the property the order protects, no observation before the seal, holds; the seal is not
rewritten.

The old choice does not meet the repository-independence premise in PRS-V1-008's protocol:
`benchmarks/manifest.json` already pins spf13/cobra at
`adbc8813901bba65827259daa8e22ff94ec1f30e`, and retained `benchmarks/results/v4-development.json`
and `benchmarks/results/v4-development-go.json` each record six scored cases on that pin.
Their partitions are five development and one development-after-observation, with corpus digest
`d744bec14c6ee54f03082b5db4a2897f4d77e1cf928dd4a886c63af0dbe8e3ad`.
The later case set's seal does not make that repository untouched. Its original preregistration,
seal and prior records remain unchanged, including the overbroad earlier no-use assertion; this
new decision records the correction instead of rewriting history.

## Prospective selection and unchanged protocol

1. Select `https://github.com/urfave/cli.git`, module `github.com/urfave/cli/v3`.
   Pin `7389061eb0182169c496021a09729735ac2b797b`, tree `7b3d3d699268a5e0a08afff798b9e09f6680c8da`,
   is the public `main` HEAD advertised before case derivation. No candidate output selected it.
   The clone is complete, not shallow. The initial selected commit is retained even if main moves.
2. Use the existing `benchmarks/untouched-repository-v1/harness.py` without modification,
   SHA-256 `b67cc95abd92ffffeef9bf1bccfce4d0316e270716d6cd065948e802994f9eb2`. Its `corpus` command uses Git reads only.
   Derive the 20 most recent first-parent commits ending at the pin without a pathspec or new
   path exclusions. The corpus has 20 cases, 20 orientation-eligible and
   18 consequence-eligible; no E1-E3 corpus exclusion. E4/E5 eligibility is preserved.
   Task extraction, lexical baseline, root-module boundary, limits, three measurements,
   C1/C2/C3 controls and thresholds, failure treatment and no-tuning rules are unchanged.
3. C3 uses PIN^..PIN, with parent `afa0a306aca344a64200a1d680bd8de46dfaad7b`. It adds no
   sealed CEM. README.md lines 1-3 exist unchanged for the inherited literal citation fixture.
   These structural checks are not a successful loop or semantic citation qualification.
4. Retain the preregistration under `benchmarks/untouched-repository-v1/urfave/`.
   Corpus SHA-256 `b9cf140717e58ab1656111dd962240e37866fa0b4356402a221d4d33a5ed48cc`;
   preregistration SHA-256 `eedf799f9444014e64f1bab1cf5d3296ea06f784e4e88930d385a990b230a185`.
   An independent reviewer must inspect these exact inputs before the `.sha256` seal is written.
   Integrate and publish the preregistration before executing the candidate. No case bodies or
   candidate outputs are sent to implementation builders; independent corpus review is permitted.
5. Target version remains `1.0.0-rc.2` under the existing protocol; the candidate is not created or
   qualified by this selection. Complete implementation changes, freeze candidate/source/runtime
   identities and exact archive bytes, and only then run the cases. A changed target version,
   corpus, exclusion, threshold or harness needs a prospective numbered deviation/reseal.
   After output is observed, no implementation tuning may preserve that candidate's qualification;
   the observed cases become development if used for tuning. Losing and aborted runs stay.

## Provenance boundary and qualification limits

The final targeted search found no urfave identity in current development/blind manifest repository
metadata, existing preregistrations, retained benchmark results, declared performance manifests,
ARB v2 repository directories or its frozen fold inventory. A search of all locally reachable Git
refs over those manifests/preregistrations, benchmark results, build-log records and retrieval
plans found no historical match. `admission.json` retains the searched path identities and scope.
No agent transcripts, sealed case bodies, unreachable Git objects or unrecorded private experiments
were searched. This is a bounded absence finding, not omniscience; contrary actual-use evidence
stops admission. No Corvint/candidate or source tests ran against the selected repository during
preparation, and there is no qualification verdict.

All three Core jobs still need candidate-bound results on Corvint, Beamfall and this admitted public
repository. Keep Corvint's separate C3 loopTarget deviation and Beamfall's private corpus unchanged.
Keep chi run-001, Beamfall's failed run and Corvint's aborted start marker unchanged. This decision
waives no native platform, frozen regression, two-repeat, proof, signing, review or owner-promotion
requirement, creates no candidate, completes no ticket and authorizes no release publication.

## Rollback

Withdraw this prospective admission before execution while retaining its files, hashes, reviews
and any later starts/results. A rollback does not make cobra eligible again or turn observed chi
cases into heldout cases. The owner selects another repository under the same admission contract;
record a new prospective decision and preserve the failed or withdrawn packet.
