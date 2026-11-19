# Source removal B23 — verifier repair 1

V1-5694; RMV-V1-008/009, AFU-V1-008 and AFU-V1-1655.

The host's whole liverepo package run rejected four stale test links in the
installed Go and JS flow declarations. Both qualification entry points changed
in `02b6cd88d785d216e9cc2703242a1617fa4611d7`, after their review anchors.
Reviewed the changes against both old anchors: the core harness now consumes
the bounded retained-evidence reader, preserving execution/freshness/strength,
stdout byte identity, Go preview and stale-negative checks. The installed runner
no longer requires the removed MCP executable and identifies the local reader
under its retained report key. Both tests still exercise the declared flows;
this review does not claim a new installed-platform qualification.

Class: stale content-bound review declarations after qualification-consumer
changes, not an analyzer soundness gap. The existing closed admissibility rules
remain: a declared link must exist at an ancestor commit that changed its intent,
and its target content must match that anchor. Missing, unknown, inferred,
nonancestor, undeclared-at-anchor and stale links cannot become reviewed. No
analyzer form allowlist or additional regression is needed: the unchanged
repository test already detects the stale bindings and checks every flow link.

Re-declare only these four review anchors, then pin them to the re-declaration
commit. The preparatory commit names the reviewed implementation revision but
cannot itself supply final review authority: that implementation commit did not
change these intents. The following commit supplies the intent-change anchor.
All other links, JSON shapes, test assertions and frozen outputs are unchanged.
Rollback is to revert the two anchor commits, restoring the stale-link failure.

Verification uses the existing whole package
`internal/liveverify/affected/liverepo`, selected with the supplied base
`1ac8b3d4ea94444cb09e3b31a332a421816fa068`, through `script/heavy.sh`.
The whole package passed in 125.272 s at
`56c046c41af1ae90474b29d1316ee2c6439ec176`, with all existing tests and no
`-run` filter. The configured semaphore had no queue delay; total run 126.1 s.
No Go identity overrides were exported during the run. Go version go1.27.4,
diff whitespace checks and the four-value-only declaration audit passed.

Both installed and previous scratch selectors refused stale binaries. A fresh
checkout build through `script/heavy.sh` passed in 10.5 s; its affected plan
selected liverepo. The plan retains UNKNOWN summary scope, 462 selected units,
237 exclusions, 109 unknowns and six unresolved packages; its selection verdict
is FULL. The wider selection remains deferred to the host per repair instructions.
No spec, line citation, Go source, test or assertion changed in this round.
Independent review and live installed-platform qualification remain NOT_RUN here.

Raw evidence is outside Git under `~/lanes/sr-b23-evidence/`. SHA-256:

- `verifier1-flow-affected-current.json` (428829 bytes):
  `34a0defa4c12210ae0a7817426f758087a8dcec1e7c8830d1287c8e09b490d0b`.
- `verifier1-flow-build.log` (10.5 s, exit 0):
  `10d9508f4413dee8846d4547ec06ca0058c86d3c60b1dd4c4e384d877eb2f7c7`.
- `verifier1-flow-liverepo.log` (125.272 s package, exit 0):
  `6be689da80ebe4bb9f0ff5a334babb00c92957a567af98d188f95379aca35323`.

CEM bind/check is unavailable under the retired `docs/DOGFOOD.md`; no seal,
publication, task-store completion or independent review is claimed here.
