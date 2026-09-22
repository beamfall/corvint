# Decision 0041 — GPK-V0-049, DR-0021, and perf grants for the two corvint harness invocations

Date: 2026-09-03. Status: accepted. Authority: repository owner, verbatim instruction
"ratify a grant for those two invocations" (2026-09-03), following the diagnosis of
`harness-user-prompt` and `harness-file-change` recorded in `docs/BUILD-LOG.md` wave 13.

## What was asked, and what it required

The owner ratified a grant. A grant is only writable over an adjudicated register entry, and the
divergence behind `harness-user-prompt` was not registered at all — wave 13 diagnosed it and left it
unadjudicated because the clause deciding it did not exist. Delivering the grant therefore required
three things in order, and all three are accepted here: the clause, the entry, the grant. The clause
comes first on purpose. Writing the grant first would have adjudicated a disagreement by pointing at
the candidate's output, which `GPK-V0-033` forbids in terms.

## 1. `GPK-V0-049` — refused-grammar disclosure

`GPK-V0-048` (decision 0039, accepted the same day) already requires a Go source the parser refused
to carry its `Unparsed` row, "so a parse failure narrows the symbol set visibly rather than
silently." Nothing in that reasoning is specific to Go; the clause simply left every other grammar
undetermined. `GPK-V0-049` states it for all of them: a source the index admits and counts but
extracts no facts from MUST carry an `unparsed` row, and a compiled receipt MUST carry the resulting
table when it is non-empty, under the `GPK-V0-045` envelope compaction.

The clause is derived from `GPK-V0-048`'s own reasoning, from AGENTS.md invariant 2 (a receipt that
drops facts and says nothing asserts a coverage it never measured), and from `GPK-V0-014`, which
names "silently narrowing Python support" as a parity failure. It is not derived from what the
candidate happens to emit.

## 2. `DR-0021` — `python-defect`, known-divergent

The oracle has no such member: `unparsed` does not occur anywhere in `src/`. It therefore
contradicts `GPK-V0-049` and the candidate satisfies it unchanged. `src/` is NOT repaired — it is
frozen and scheduled for deletion, and `GPK-V0-033` records rather than repairs an oracle defect.

## 3. The grants

| Task | Entries | Argv |
|---|---|---|
| `harness-user-prompt` | `DR-0021` | `harness event … --event user-prompt --input -` |
| `harness-file-change` | `DR-0004`, `DR-0009`, `DR-0021` | `harness event … --event file-change --input -` |

`harness-file-change` names all three because it carries all three: `DR-0021`'s member, `DR-0009`'s
coverage denominators (`requested_results` 10 against 60), and `DR-0004`'s two permuted test-marker
evidence rows. Absorbing one while the others still invalidated would report a diagnosis nobody
made — the same reasoning decision 0040 gave for the Beamfall impact task.

Every constraint on the mechanism is unchanged and still tested. A grant lives in Go source where no
manifest edit can widen it; it is matched by entry, task id, and exact argv; it relaxes
`unequal-stdout` and nothing else. The guard test enumerates every grant, so an unratified addition
fails there before it can pass quietly, and it transcribes the two harness argv lists independently
of the `harnessEventArgv` helper the grants use, so a typo in the helper fails rather than silently
stopping a grant from matching. A new test shows `DR-0021` absorbing nothing on the Beamfall harness
event, which runs over a corpus with no refused source and stays measured against the unrelaxed
predicate.

## What this does not do

It does not make the two tasks pass. It makes them measurable: their thresholds can now report
`PASS` or `FAIL` on evidence instead of `NOT_RUN` forever, and every criterion measured across the
relaxation renders with the absorbed reasons named inline. It grants nothing to any other task, to
any other repository, or to `beamfall-impact-path-snapshot-present`, whose oracle exceeds the
30-second sample timeout and whose stdout is unstable across samples — none of which is a diagnosed
disagreement and none of which any grant may relax. That task remains the open packet-5 blocker.

## Rollback

Delete the `DR-0021` map entry and the two decision-0041 grants from
`conformance/perf-v0/manifest.go`, drop `acceptedDivergences` from the two tasks in
`manifest.json`, and revert the two guard-test expectations. The tasks return to
`insufficient_evidence` with `unequal-stdout`, which is exactly their state before this decision.
`GPK-V0-049` and `DR-0021` are independent of the grant and would stand.
