# Tasks: `release-process`

Each task names the requirement IDs it delivers and its proof. Tick tasks in order.

## Tasks

- [ ] 1.1 File the ticket and reserve prefix `REL-V1` (001 to 015). Requirements: all. Proof:
      `corvint-tasks ticket show` with `requirementRefs` listing REL-V1-001 to 015.
- [ ] 1.2 First commit: create `docs/specs/release-process-v1.md` from the spec delta, its
      `docs/specs/README.md` row and the regenerated index; amend PRS-V1-004 in the same commit.
      Requirements: REL-V1-001 to 015, PRS-V1-004. Proof: `make spec-requirements-check` passes.
- [ ] 2.1 M0 mapping: list every open `v1-0` ticket; give each a `requirementRefs` entry naming a
      `PROD` requirement or move it to `post-1-0` with a recorded reason. Requirements: REL-V1-001.
      Proof: the M0 audit output with zero unmapped tickets.
- [ ] 2.2 M0 release object: decide which existing gate IDs implement PROD-INT-017, then rewrite the
      acceptance criteria and required gates with `corvint-tasks release update`. Requirements:
      REL-V1-001, REL-V1-012. Proof: `corvint-tasks release show` before and after. The nightly
      `release-gates.yml` run is cut to exactly the REL-V1-012 gates (M5); tickets V1-4848,
      V1-4849 and V1-4850 close against that rule, and a dropped gate closes its ticket as
      superseded. `greptile-gate` is not added as a release gate (D14).
- [ ] 2.3 Cut `binaries.json` to three targets and group binaries by product. Requirements:
      REL-V1-008. Proof: `make release-binaries-check` and a reproducibility check of one target.
- [ ] 2.4 Tag workflow: build job (read, OIDC and attestation permissions only), full suite, three
      targets, `SHA256SUMS`, manifest, licences, attestation; publish job in a protected
      environment, the only job with `contents: write`. `.github`-only PR. Requirements:
      REL-V1-005, REL-V1-006, REL-V1-007. Proof: a run on a test tag showing the publish job waiting
      for approval; `actionlint` or the repository's workflow check output.
- [ ] 2.5 Calendar and clock record: a release-status document or command output giving freeze,
      candidate, RC publish, soak and trial clocks and the computed stable date. Requirements:
      REL-V1-002, REL-V1-003. Proof: a fixture where a code change in the RC moves the stable date.
- [ ] 2.6 Demotion verdict record: FIH and NMS verdicts recorded on the freeze-candidate SHA and
      compared on the RC SHA. Requirements: REL-V1-004. Proof: a fixture regression reported as a
      blocking miss.
- [ ] 2.7 Soak and trial counters and the measurement-session record: each timing result records
      host, kernel, load and concurrent processes (D8). Requirements: REL-V1-010, REL-V1-011.
      Proof: a counted dry run over a past 7-day window and one session record.
- [ ] 2.8 Removal deadline check in the milestone record. Requirements: REL-V1-009. Proof: the M3
      record on 2026-12-01 listing merged and moved batches.
- [ ] 2.9 Rebuild evidence classes: write and freeze each class's input path set before M5a, and
      the path-set reuse check. Requirements: REL-V1-014. Proof: the committed path sets and a
      fixture where a path in no class forces every class to be redone.
- [ ] 2.10 macOS release gate on the freeze-candidate and release-candidate commits; record the
      per-merge check's state on 2026-12-15. Requirements: REL-V1-015. Proof: the two macOS runs
      and the flake-ticket and consecutive-run count.
- [ ] 2.11 Publish job check of the signed acceptance tag against the earlier allowed-signers
      file. Requirements: REL-V1-007, REL-V1-013. Proof: a test tag signed with a key added only
      in the released tree is refused.
- [ ] 3.1 M5a: push the qualification candidate tag; run the per-product qualifications.
      Requirements: REL-V1-002, REL-V1-012, REL-V1-013. Proof: the workflow run and the three
      qualification records.
- [ ] 3.2 Build-log entry; complete the ticket through the Tasks gate. Requirements: all. Proof:
      `corvint-tasks ticket show` and `receipt audit`.
