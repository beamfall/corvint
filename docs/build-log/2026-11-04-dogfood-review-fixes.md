# Dogfood review fix round

SPDX-License-Identifier: Apache-2.0

Owner intent: the cx-dogfood fix-round request for V1-4545, V1-4546, V1-4548,
V1-4541 and V1-4573. The independent review returned CHANGES_REQUIRED; this
entry records the bounded repairs, without claiming the next review passed.

- P2-1 / V1-4548 / DWC-V0-046: shell seal evidence selects the validated
  external header when present, otherwise the single legacy pin. Duplicate
  legacy pins refuse before checking. Shell regression coverage wraps legacy
  output in an external header with a different pin and checks that the durable
  message contains exactly one evidence line. Native and shell fixtures cover
  a legacy report over a metadata-only descendant with verdict=NOT_OBSERVED;
  both refuse duplicate legacy pins. ASD-V0-1024 now notes DWC-V0-046's
  supersession of its Go-seal non-goal.
- P2-2 / V1-4546 / DWC-V0-045: DCW-V0-842 drops the contradictory carried
  citation requirement and records its amendment, retaining ordinary drafting
  for unmatched hunks. No new requirement ID was added.
- P3 / V1-4573 / DWC-V0-048: ignore the fallback's temporary filename pattern
  and chmod its file to the normal stage's effective mode before publication.
  Copying the stage's permissions preserves umask without changing global
  process state. The publication regression compares the two real paths under
  the running process's umask and retains cleanup/failure coverage. This
  fallback belongs to V1-4573; its requirement citation and trace state now
  reflect that ownership and passing implementation evidence.
- P3 / V1-4541 / DWC-V0-047: handback lint accepts general uppercase/digit
  ticket segments separated by hyphens. The cumulative regression accepts
  CORE-FOO-1, BUG-X-0930 and OSC-Y-1 and still rejects lowercase IDs.
- V1-4545 / DWC-V0-044: existing rebind behavior is unchanged and its focused
  regression is rerun with the other DWC-V0-04x cases.

Observed verification: `script/heavy.sh go test -count=1 -timeout 30m -run
'TestDWCV004' ./internal/dogfoodflow` PASS (1.207s); shell review-head-check
fixtures PASS; `make spec-requirements` PASS after setting the shared writable
heavy lock directory. The initial unconfigured test and generator invocation
refused the host's unwritable /home/russell/bench/rt/.locks path before tests.
The generator had already refreshed REQUIREMENTS.tsv; its admitted retry
reported no changes. The affected receipt retains mixed-worktree UNKNOWN
scope and stale-installed-binary degradation; selected package coverage does
not claim repository-wide coverage.

The lane handback retains final binding/check output and static checks.
Independent rereview, native ticket writes, publication and actual lane seal
are reserved to the orchestrator. Exhaustive tests, frozen wire evaluations
and external live qualification are NOT_RUN for this scoped fix; no wire
format changed. Rollback each repair independently; reverting the publication
mode/ignore fix restores the reviewed deficiency, while dropping the fallback
entirely restores the original cross-mount refusal.
