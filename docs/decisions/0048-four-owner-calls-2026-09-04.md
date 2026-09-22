# Decision 0048 — the four open owner calls after wave 17

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction "I want you
to make the calls" (2026-09-04), given after the agent reported the four calls below as open. The
agent made each call and this record states it; the owner's instruction is the acceptance. Each
call names the evidence it rests on and its rollback.

## 1. `GPK-V0-017(a)` is amended: index-building events get the portable cap, measured with a snapshot

The 100 ms p95 cap in `GPK-V0-017(a)` was written for harness events that answer from an existing
index. `file-change` and compact `session-start` build one when none is present, and run3
(`conformance/perf-v0/results/packet-5-current-pin-requalification-2026-09-03-run3/`) measures
that build at 835–1,080 ms p95 on both corpora, proportional to repository size, while the events
that do not build measure 76–95 ms. A `FAIL` on those four rows would report that a hook is slow
when it reports that an index build is not a hook.

The clause now reads: non-query events that do not build an index keep the 100 ms p95 cap;
index-building non-query events are measured with an index snapshot available and must be at most
the existing 250 ms portable product cap p95. The snapshot-present corpus binds only once an
automatic index refresh ships and is dogfooded, exactly as decision 0037 item 1 set for
`GPK-V0-017(b)`; until then that row is `NOT_RUN` and the cold-corpus figure is reported beside it.
Nothing is waived: today's snapshot-present figures (147–151 ms p95 after wave 17,
`docs/agent-memory/optimizations.md`) sit under 250 ms and above 100 ms, and the cold build stays
visible.

Consequence: packet 5 stays `NOT_RUN` in `script/release-checklist` until the automatic refresh
ships (the follow-up that unblocks it; decision 0032 left the refresh trigger undecided). No retry
preregistration under `P5R-V0-004` is written by this record.

Rollback: restore the previous `GPK-V0-017` text and regenerate `docs/specs/REQUIREMENTS.tsv`.

## 2. The `0016` pair is grandfathered; the uniqueness check joins `make gate`

Both `0016` filenames are cited from evidence that must not be rewritten:
`0016-change-anchored-packet-2026-09-01.md` from the frozen task corpus
`tools/cw-trial/testdata/unseen-corvint-v2/tasks.json` and its result
`benchmarks/results/cw-trial-unseen-corvint-v2-run-1.json`;
`0016-packet-5-current-pin-requalification-2026-08-31.md` from `script/release-checklist`, which
binds the formal packet-5 result to that record. Renaming either breaks a binding citation.
`docs/decisions/README.md` already requires full-filename citation across lineages, so numeric
uniqueness carries no meaning for this pair and full meaning for every record minted from here on.

`script/check-decision-numbers.sh` names exactly this pair as grandfathered and fails on any other
collision, on a third `0016` file, or if the pair changes. `make decision-numbers-check` becomes a
`make gate` prerequisite. The README stance changes from "not grandfathered" to "this pair, by this
record".

Rollback: remove the grandfather list from the script, the gate prerequisite, and the README
sentence; the check fails again on `0016`.

## 3. The Beamfall shadow is deferred; the self-dogfood adapter is the only qualifying adapter

Corvint cannot write Beamfall's adapter (`WQO-V0-029`) or be its own recorder (`WQO-V0-036`). The
Beamfall repository (`~/projects/beamfall-workspace/beamfall`, same owner) owns
both: the adapter as a Beamfall roadmap ticket, and the recorder as a Beamfall-side replay harness
that runs the repository oracle and Corvint at the same checkpoint. Until that ticket is filed and
built, Work Queue Observation V0 is evidenced by Corvint self-use only: `script/corvint-work-queue`,
`docs/worklist.json`, and `conformance/work-queue-v0`. The 500-cycle count (`WQO-V0-035..037`)
stays `NOT_RUN` and the delivery status stays experimental. Filing the Beamfall ticket is recorded
in `docs/agent-memory/ideas.md`; it is not done from this repository.

Rollback: none needed; this record defers, it changes no wire or requirement.

## 4. `v0.4.0a3` is cut at the head that includes wave 17 and this record

`v0.4.0a2` (8feba58) predates wave 17: the index-build, session-start, harness-event, precision,
and gate-time improvements, the Work Queue Observation implementation, decisions 0046 and 0047, and
the observation-ledger temporary sweep. An alpha whose notes describe a binary that does not carry
the measured changes misleads its reader. The version moves to `0.4.0a3` in every place the
`0.4.0a2` bump touched, `docs/RELEASE-NOTES.md` gains a section for the range, and the tag is
applied only after `make gate` (now including the decision-number check), the wheel and archive
witnesses bound to that commit, and `script/release-checklist` at the tagged revision. The tag is
local and unpushed, like its predecessors.

Rollback: `git tag -d v0.4.0a3` and revert the bump commit; `v0.4.0a2` stands at 8feba58.
