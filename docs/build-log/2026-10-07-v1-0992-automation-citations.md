# V1-0992: docs/AUTOMATION.md help.go citations repinned

`docs/AUTOMATION.md` cited `cmd/corvint/help.go` by bare line number, stale since the root-help
compaction (V1-0945). Ten citations were repinned to the current lines with `@hex` content anchors
(`script/check-line-citations.sh --hash`). Two sentences were adjusted minimally: the network quote now
cites the root-help support boundary ("Local-only; no network or telemetry"), and the experimental-verb
list now cites the command-maturity list. `docs/AUTOMATION.md` is not in the checker's `scanned_doc`
set, so these anchors are not verified by `line-citations-check`; extending that set would require
anchoring every other citation in the page and is left as a follow-up.

## Batch integration

Later batch C merges shifted `help.go` by up to four lines; three anchors (`569-570`, `569` and `1195-1207`) were moved to `571-572`, `571` and `1199-1211`, keeping their content hashes, after the batch integration review found the drift (commit fd5aedb3). V1-0997 tracks adding `docs/AUTOMATION.md` to the checker.
