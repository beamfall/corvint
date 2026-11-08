# Selwide self-review repair: executable hidden packages and embed claims

The repair request identifies two V1-5018 / AFP-V0-9012 integration failures:
the runner compares an indexed hidden-package universe with a wildcard that
omits those packages, and embed discovery claims unrelated hidden metadata
before matching any directive. Both failures reproduce with the new regression
tests over the exact pre-repair production blobs at
`b9a4916f404690af1fd14be9ce5aa3c44c931bd4` using a Go overlay. No golden,
existing assertion, diagnostic, exit code or wire member is changed.

## Decisions and acceptance evidence

For AFP-V0-9012 and STR-V0-004, retain `go list ./...` and its exact-universe
comparison, adding explicit import paths for indexed packages with dot- or
underscore-prefixed components. The root invocation already executes selected
packages by explicit import path; no execution-unit or receipt change is needed.
Nested-module discovery keeps its existing separate contract. Runtime discovery
still refuses a listing error or any universe difference. This preserves tests
in hidden packages rather than removing those packages to make discovery agree.

`TestHiddenGoExecution_AFPV09012` builds the real graph and manifest and runs the
real Go adapter. It covers `.github/cishards`, `.github/testconfine/cmd`,
`nested/.helpers`, `nested/_helpers`, and an unrelated `solo` package. Each
selected package must produce a PASS outcome. Deliberately failing the hidden
tests must produce FAIL, proving their assertions execute. Against the old
runner, every subcase instead returns INCOMPLETE before launching, with four
unlisted packages.

For AZU-V0-005 / AFP-V0-9012, use the existing embed resolver as the source
listing's acceptance predicate, including module boundaries and directory
expansion. Only an actual pattern target reaches representability validation;
the final resolver retains the existing per-unit bounds and ordering. The source
walk, its cache recording, and the default name-validation contract are unchanged.
`TestHiddenUnrepresentableNames_AZU_V0_005` now includes a valid unrelated
`//go:embed payload.txt`, checks its retained EmbedFiles, and compares immutable
and disk graphs. Windows cannot create the invalid UTF-8 disk filename, so that
disk comparison is platform-limited; immutable cases remain cross-platform.
Direct glob, directory expansion and `all:` targets with invalid names still
must refuse ErrWalkUnrepresentable. The original invalid Go-source assertion
is retained. Against the old embed resolver, unrelated `.metadata/name\xff.txt`
aborts graph construction before any pattern claims it.

AFP-V0-9012's spec and implementation/test mapping are updated in the same change.
V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged; their prior
acceptance evidence and host integration limits remain applicable. Rollback
reverts only the discovery argument addition and embed acceptance predicate,
restoring the reproduced failures; it never deletes the new tests or changes
frozen expectations.

## Verification and limits

The current-tree affected plan against the original full base selected all four
lane packages before tests. It retained scope UNKNOWN, with 399 selected units
and 70 unknown entries; the 133 selected Go packages were advice, not a broad
test execution. Only focused packages were run through the existing heavy
wrapper, with the required private TMPDIR and no exported Git identity.

The two targeted regressions pass on the repaired tree (2.329 s) and fail on the
pre-repair production overlay (0.095 s). An initial test-harness attempt panicked
because the new Runner fixture had no Output writer; assigning io.Discard fixes
the fixture without changing production behavior or an existing assertion.
The failed attempt is retained as evidence.

Whole existing packages pass without test-name filters: affected (134.620 s),
its Go provider (11.237 s), and sessiontest (2.451 s).
`make -s spec-requirements-check line-citations-check` passes; the staged
`script/check-line-citations.sh --repin` finds no moved citations. The pinned
Go version check passes. The command-package rerun and final CEM result are
retained in the lane handback; these three package passes do not establish a
passing command package or repository-wide gate.

Raw artifacts are outside the repository under
`/home/russell/lanes/selwide-evidence/`. The digests support the results above:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| `self-review-focused-fixed.log` | 794 | `3e5c242d609f289c3eb202e0b10361b1df5d57b853989e9bfa303097beabbd9e` |
| `self-review-before-regressions.log` | 6708 | `7bb2c105dd1e0e9cf8ad5c3bedf8f1da20eb431479319cda0dbb65af6f4a8a4c` |
| `self-review-whole-affected.log` | 840 | `dc2d2eeffbdf716dfa2ccc453b5f0792f1787f5f8861f819aba688ef73c6e736` |
| `self-review-whole-golang.log` | 780 | `21f99d8630788b02bf6c816ed08d9c105a14bfccdb0fe8834ca6be3159b35072` |
| `self-review-whole-sessiontest.log` | 761 | `1de3d87b63f1443de3486734fcc7ec1c7fd4547b242b30b7cd4b40610faec1e2` |
| `self-review-spec-citations.log` | 3417 | `ca98194cffb4809362f94cb072c8e53a150b2cf9c03d3886a195b6bf1717aafc` |

Pre-change capture is reused. Independent review, repository-wide gates,
selection baseline and cross-platform qualification remain host-owned.
No Tasks write, publication, pre-change recapture or seal is authorized.
