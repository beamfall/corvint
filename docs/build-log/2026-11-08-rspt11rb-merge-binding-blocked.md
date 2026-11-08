# rspt11rb merge-forward binding blocked by process namespace

The merge-forward commit is `526f4275e555d3ba90e975f2c5a9d11a4dcd536d`;
the regenerated catalogs and resolution record are committed in
`12c3c2b111dbdc8959c61aeb86d2b4d45c6811e6`. No pre-round commit was rewritten.

`make dogfood-change BASE=9761ea1f459c0d3b3c963953e24a76eaeda16370`
returned make rc 2 before preparation, with `sealed-cem-in-change` for
`.corvint/changes/3f32d7ad7ca93f9328cb492d53f3f08dc347b1bb.cem.json`,
whose stable verifier reported `unsupported-process-containment`. A direct
`cem status` on that unchanged constituent also returned rc 2 with stage
`repository`, outcome `UNSUPPORTED`, and the same reason. This is no evidence
that the constituent is invalid. Its historical seal remains unchanged.

A separate process probe observed `os.getpid() == 3`, while `/proc/self/stat`
reported PID `1227932`, and `/proc/3/stat` was absent. Corvint's Linux reaper
requires consistent process identity to prove cleanup; the sandbox does not
provide it. This matches the previously retained rspt3 namespace finding.
No containment check, evidence policy or historical map was weakened or removed.
Binding is NOT_PRODUCED: unsupported-process-containment. Independent review of
a newly bound CEM is NOT_PRODUCED because no new binding exists. A subsequent
`dogfood-check` returned make rc 2 (`dogfood-report-missing`), as preparation
could not produce this range's report. No seal was requested for this round.

The requested `make -s spec-requirements-check line-citations-check` passed
(rc 0), including its specindex and citation-reader package checks. No Go file
was conflict-resolved or regenerated, so no additional package command was due.
The affected read returned rc 0; its selection was not executed. Whole-selection,
doc and repository-wide gates remain NOT_RUN by explicit owner instruction.

Original local logs live under `/home/agent/lanes/rspt11rb.tmp/`:

| File | SHA-256 |
| --- | --- |
| merge-process-namespace.txt | a611e63cba3e31e32075500be0271c87f81b85634bf6082c93e6c58614b64ac9 |
| merge-prepare.log | 3a19bd9ecb86eb0b0a2f4c61b398719986f392b9213e7a12a42db2b0c5705106 |
| merge-constituent-status.json | ea6cee7cdf122f511aa529a9ab139619ff3f9672323f40de48fb8227f355a6fb |
| merge-focused-checks.log | b66497607926c1946357b0b6a611c0feec744f8b2c1d72fb1509b25496d92152 |

Remaining work: run the same whole-range rebind and dogfood-check on a host
whose proc filesystem agrees with its process namespace. Keep all reasoned
unknowns visible, refresh and review the retained citation plan at the new base,
and stage citation plans if the resulting plan exceeds 256 rows. Stop after
check passes; the lane was not sealed before this round.
