# Selwide FULL universe and declaration repair

V1-5018 / AFP-V0-9012: self-review found two execution soundness gaps.
The graph admits hidden editor/cache directories, but runtime enumeration
applied the generic skipped-directory set; wildcard enumeration could not
recover those tests. The workflow also trusted a partial non-source parser:
an invalid declaration of `.github/` could erase hidden CI tests during FULL.

The directory class is now handled by the graph's admission rule: all dot
names are source-capable except `.git`, `.hg` and `.svn`. Existing visible
skipped directories, testdata and nested-module boundaries remain. No list of
known editor/cache names grants source authority or exclusion. The regression
executes intentionally failing tests in `.vscode`, `.idea`, `.venv` and an
unfamiliar `.unknown-source-kind` in both unsharded and four-shard FULL runs.

The declaration class is exclusion without complete authority validation.
The workflow accepts only the closed `corvint-non-source-paths/0` object and
`path`/`reason` entry grammar: canonical UTF-8 paths, protected path
restrictions, strictly ascending unique paths, nonempty control-free reasons
of at most 512 bytes, 1–256 entries, and a regular file of at most 64 KiB.
Duplicate/unknown members, unknown profiles, unrecognized entry forms and
invalid content refuse before enumeration or placement. Absence excludes
nothing. The Go driver continues to use the existing strict core parser.

Decision: enforce the complete core declaration contract in the embedded
workflow parser. Introducing a new CLI or requiring a current-tree compiled
helper in the workflow would widen this repair's command and trust surface.
The workflow regression extracts the actual Python block and exercises the
reported path plus unknown forms, bounds, order, protected/canonical paths,
UTF-8 and non-regular files; valid output exclusion is compared with
`DeclaredNonSourceHolds`. The fake go-list command emits only explicit
arguments, making wildcard omission visible.

Acceptance witnesses: `TestHiddenSkippedNamesFullExecution_AFPV09012`,
`TestHiddenUniverseDeclarations_AFPV09012`, and the existing complete
`tools/corvint-pr-tests` and `tools/ci-forward-row` suites. V1-4941 /
AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged. No frozen output,
golden, existing assertion or core diagnostic is changed. Rollback removes
this repair and its added witnesses, retaining the reported execution gap
as unresolved rather than claiming safe FULL coverage.

The pre-change capture is reused as instructed. Final bind/check occurs once
after the code, tests and spec are final, with no seal. Independent review,
host-wide qualification and native completion remain host-owned.

## Verification evidence

| Artifact | SHA-256 | Result |
|---|---|---|
| `review-4-affected.json` | `b5554cce9d959b5acc09a47213e61380f700eee5c23400c0fc7ad64895cfe071` | UNKNOWN retained |
| `review-4-focused-workflow.log` | `1443e3c1271b9518244e0613ab682133613c47792d9eff41085729a7c3ba9496` | ok  	github.com/Beamfall/corvint/tools/ci-forward-row	3.223s |
| `review-4-focused-driver.log` | `3d90eb9cd6656a6683043c5a32ca85bb2c8c459aae71a31de45dd06c9252e729` | ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	19.004s |
| `review-4-forward-whole.log` | `029b993705b191614965955055348d1bce8bfabdf3a33f879963e8015cc51900` | ok  	github.com/Beamfall/corvint/tools/ci-forward-row	50.738s |
| `review-4-driver-whole.log` | `a82fe08f9eb59789d4caf8d469804b7f06d268e11a21072e60430cee6a48310d` | ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	134.096s |
| `review-4-doc-checks.log` | `9fe737cff8eddadb42271547a26a6b7cc59bf93c71addf3167712f3381e9495f` | ok  	github.com/Beamfall/corvint/internal/specindex	0.863s; ok  	github.com/Beamfall/corvint/internal/lrfrepo	0.062s |
| `review-4-repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` | PASS; citation skip limit retained |

Both touched packages passed as whole packages without -run filters or Git
identity exports. Spec requirement and citation checks passed; repin moved none.
The installed affected planner is stale and retains UNKNOWN. No broader gate
or independent-review verdict is claimed. Raw logs remain outside the repository.
