<!-- SPDX-License-Identifier: Apache-2.0 -->
# Clicontract mainline merge-forward

The owner requested a merge of mainline cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419
into lane head e2f1791b5d7065435b0a450968a89f8c082082a1, preserving both sides and
rebinding the complete lane range against that mainline commit. The prior base was
d436dff76de1b8e75c427069ae7f80003ae99bc2. Merge commit:
50e706d0aa60dbd308e89e0dd264e7acc5a165fa. Existing commits remain unchanged.

Conflict dispositions:

- Makefile: retain both corvint-test and linecov capability snapshot generation.
- docs/specs/REQUIREMENTS.tsv: take mainline during the merge; regenerate afterward
  with make spec-requirements, which owns REQUIREMENTS.tsv, INDEX.json and the README
  table. All eleven lane-added requirement rows return. No added IDs collide.
- docs/specs/README.md: preserve both sides' rows and their complete requirement ranges.
- docs/specs/corvint-tasks-cli-v0.md: preserve lane TCL-V0-7703/7704 and mainline's
  store/read-path repair requirements.
- docs/SPEC-TOOLCHAIN-INTEGRATION.md,
  docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md,
  docs/specs/falsifiable-packet-v0.md and docs/specs/index-snapshot-v0.md:
  preserve the prose and relocate seven pinned citations to the merged content with
  script/check-line-citations.sh --repin; every original content hash survives.
- script/batch-precheck.sh: retain the exact-body anonymous local mirror and mainline's
  named CI-only classification and drift refusal. A named step with a different name
  cannot acquire the anonymous mirror. Update the lane's list assertion accordingly.
  An unnecessary newline trim introduced during resolution caused a sync refusal;
  removing it restored the mainline comparison. Both results remain in local evidence.

Verification: corvint affected --base cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419
returned 0. make spec-requirements returned 0, and make -s spec-requirements-check
line-citations-check returned 0. bash -n script/batch-precheck.sh and the final
script/batch-precheck.sh --sync-check returned 0. No Go file conflicted or was
regenerated, so the requested conflict-package tests have no applicable package.
The generator's obligation-bound check and the spec check's internal/specindex tests
ran as part of their owning targets. Whole selections, doc-gates, corvint test,
gate-affected, publication and Tasks writes are excluded by the owner for this round.
The host verifier and batch CI own broader integration qualification.

Evidence and losing outputs are retained outside the worktree under
/home/russell/lanes/clicontract.tmp/merge-forward. The unsealed lane will receive a
new committed CEM and a clean dogfood-check against the merged mainline, without a
seal. Native reports retain NOT_PRODUCED and NOT_OBSERVED limits; this record does
not substitute for those reports or independent external qualification.
