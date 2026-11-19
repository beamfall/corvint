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
Final results and evidence digests will be retained here after the run.
CEM bind/check is unavailable under the retired `docs/DOGFOOD.md`; no seal,
publication, task-store completion or independent review is claimed here.
