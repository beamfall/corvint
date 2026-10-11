# V1-0038: cmd/corvint read-scope qualification blocked

Status: **BLOCKED**, not implemented. The attempted declaration and its focused
regression test were reverted after a reproduced confinement failure. The final
change is this evidence entry only; production root-location calls, declarations,
requirements, use-case receipts and the ticket store remain unchanged.

## Existing intent and scope

The owner requested a correct measured package declaration under AFP-V0-023,
passing `cmd/corvint` package evidence under GL-V0-004, and removal of that package
from `gate-affected-select -unresolved`. No requirement is added or amended;
AFP-V0-023 retains its existing proposed status. The pre-change HEAD was
`29e1f653486fca13216606683d3a5c2b05cacd36`. Another run concurrently changed
`tools/gate-ledger` and its spec/index; those changes are outside this task.

The attempted scope covered direct fixture reads, all-platform import-source
copies, embedded assets and nested Go/shell/Node builds: `internal/`,
`integrations/`, `cmd/corvint-work-queue/`, workflow files, two spec fixtures,
two conformance fixtures, the frozen source-view manifest, module files, VERSION,
.gitignore, work-queue policy and the five used scripts. Its focused regression
checked those inputs, source-copy closures, canonical forms and unrelated-path
exclusion. No production call was replaced with an undetected equivalent.

## Reproduced confinement blocker

`TestFlowDocsCLIEndToEnd` initializes its normal fixture and passes unconfined,
then fails under the repository's protected `.github/testconfine` wrapper on the
same source snapshot and Linux Go 1.27.1 runtime (arm64, Landlock ABI 4):

```text
unconfined: ok github.com/Beamfall/corvint/cmd/corvint 0.861s
confined:   --- FAIL: TestFlowDocsCLIEndToEnd (0.07s)
            docs_flows_test.go:57: CLI refused:
            flowdocs-refused: cannot open filesystem root
            FAIL github.com/Beamfall/corvint/cmd/corvint 0.137s
```

`internal/flowdocs/files.go`'s `openDirectory` starts with `os.OpenRoot("/")`
before walking and validating every nonsymlink ancestor. The wrapper cannot grant
read access to `/` without granting the repository hierarchy too. `/` is not an
admitted declaration entry. This is a confirmed existing blocker, already retained
in `.corvint/unbounded-readers.json`'s `cmd/corvint` reason from 2026-10-04 and
2026-10-06 (V1-0868), not a newly discovered defect or a new ticket.

Fixing it requires a bounded file-opening implementation that preserves the
symlink-ancestor and race protections. Its owning `internal/flowdocs` files and,
if needed, confinement policy are outside the authorized edit list. Leaving the
candidate declaration would make normal confined CI tests fail, so it was removed.

## Independent ledger-contract blocker

The candidate declaration did remove `cmd/corvint` from the actual affected-test
selection for `unrelated-v1-0038.txt`. The unbounded-reader gate also recognized it
as declared/bounded (a non-failing stale-baseline note). However, both before and
with the declaration, `-unresolved` printed:

```text
unresolved github.com/Beamfall/corvint/cmd/corvint: "cmd/corvint/breakage.go calls os.Getwd"
```

`unresolvedPackages` and `packageBounds` in `tools/gate-affected-select/main.go`
intentionally use the unresolved index without declarations. AFP-V0-023 explicitly
retains whole-tree treatment for declared packages in `-unresolved`/`-bounds`;
GL-V0-009 uses the latter to prove reusable package-receipt input bounds. Removing
a package only from `-unresolved` would leave this proof inconsistent. A correct
extension needs both helpers, their regression tests, the governing AFP/GL clauses
and generated requirement traceability, together with an explicit qualification
policy for metadata-only dependencies and tolerated denied reads. Landlock does
not prove those dependencies absent. The necessary spec files were outside the
owner's authorized edit list; no helper or spec was changed.

An independent native reviewer checked the candidate source/fixture closure and
then the failure evidence, confirmed both blockers, and agreed with reverting the
unsupported declaration. The ticket remains open; no completion write was made.

## Verification and retained artifacts

All Go checks used `GOTOOLCHAIN=local`,
`TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`, and a 30-minute package
hang detector. Linux checks additionally used `GOFLAGS=-buildvcs=false`, as the
protected wrapper requires. The wrapper was built from separately copied source
with its own standard-library-only module, matching CI's isolation mechanism.

- PASS: candidate `TestCmdCorvintDeclaredReadScope`, 3.266s. The candidate test
  was subsequently removed with the unsupported declaration.
- PASS: `go test -p 1 -count=1 -timeout 30m ./tools/gate-affected-select -run
  'TestSelectPackagesHonorsDeclaredReadScopes|Test.*ReadScope'`, 1.734s.
- PASS/FAIL control pair: Linux `go test -p 1 -count=1 -timeout 30m ./cmd/corvint
  -run '^TestFlowDocsCLIEndToEnd$'` without/with `-exec /proof/test-confine`.
  Original logs: `/private/tmp/v1-0038-flowdocs-unconfined.log` and
  `/private/tmp/v1-0038-flowdocs-confined.log`.
- CANCELLED: full Linux confined package audit after the deterministic focused
  failure, exit 137. Its partial `strace -f -qq -yy -s 4096 -e
  trace=openat,open,chdir` observation retained 188 unique absolute `/repo/`
  open paths and 123 denied opens of `/`; these are partial observations,
  **not** complete-package scope qualification or passing package evidence.
  The owned container and its descendants were retired and its absence checked.
- FAIL: host full-package baseline `go test -count=1 -timeout 30m
  ./cmd/corvint -test.testlogfile=/private/tmp/v1-0038-test-reads.log`
  reached its package hang detector: `panic: test timed out after 30m0s`,
  exit 1, `FAIL github.com/Beamfall/corvint/cmd/corvint 1800.459s`.
  Original output is `/private/tmp/v1-0038-package-before.log`. This is not
  passing GL-V0-004 package evidence and is not attributed to the reverted
  candidate. Before timeout it also reported
  `TestIndexedCoreVerbsRefuseAPromisorFetchGitStartsAnyway` failing; the panic
  listed `TestAggregateFinishNativeForeignRepository` as active. Their causes
  were not investigated or classified as new defects within this bounded task.
  The owned test invocation exited; host descendant inventory could
  not be attested because process enumeration is unavailable in this sandbox
  (`ps` denied; `pgrep` reported `sysmond service not found`).
- PASS: `requirement-definitions-check`, `traceability-tests-check` (3 planned),
  `decision-numbers-check`, `line-citations-check`, `error-code-ownership-check`,
  `unbounded-readers-check`, `use-case-receipts-check` (all 18 current), and
  `use-case-receipts-test` (5 cases). Remaining-gates invocation exited 0.
- FAIL: `spec-requirements-check` / `TestIndexCoversSpecsAndHeaders`: the concurrent
  `docs/specs/gate-ledger-v0.md` Agent digest status did not agree with its
  intent/delivery headers. No task-owned spec was edited or staged. Line-citation
  checking explicitly excluded the other run's unstaged spec/index edits.
- Initial doc-gate launches were blocked because Darwin's default `mktemp`
  template ignored TMPDIR and used an unwritable `/var/folders` directory.
  A scratch PATH wrapper under `/private/tmp/v1-0038-tools` changed only the
  missing/default temporary-file template to the allowed TMPDIR. Gates then ran
  unchanged; no repository tooling was edited for that environment workaround.

The source snapshot archive is `/private/tmp/v1-0038-linux/source.tar`, 113489920
bytes, SHA-256
`04324138e40f3f2f0182d290ddc47b0e5565803f38b1a140d2e03ad9f13e193a`.
It contains Git-listed worktree files plus the candidate test/log, with concurrent
unrelated edits, and is not advertised as a clean committed tree. The partial
trace is `/private/tmp/v1-0038-linux/results/linux-read-trace.log`, 35308382 bytes,
SHA-256 `f8c261d856462f5212d00172dd3a09a78f53c074a0a33b3758cbe7a8d46743e1`.
Other logs are retained under `/private/tmp/v1-0038-*`.

Corvint self-use: **used** `query --task ... --limit 1` and `affected --base`
against the captured base; original outputs are
`/private/tmp/v1-0038-context.json` and `/private/tmp/v1-0038-affected.json`.
Their omissions and uncertainty remain open; neither establishes test adequacy.
**Unavailable** snapshot refresh: writing the linked index under the owning
repository's `.git/corvint/index` was denied by this sandbox. **Not produced**
post-commit CEM/OCM/seal: the owner prohibited commits, shared sidecar edits and
out-of-scope writes. Repository-wide `make gate` and `./...` were not run, as
explicitly directed. Use-case repinning was not needed because main.go did not
change. Installed Core is 1.0.0-rc.3 build 407; Tasks is
0.0.0-tcp01-unverified+build.407. Official update checks failed their version smoke
with `descendant snapshot unavailable`; available-version freshness remains
UNKNOWN, and verification was not bypassed. Account usage and billed-token
telemetry were unavailable; no cost or savings claim is made.

Rollback of the attempted code/config work is complete. Further implementation
requires owner-expanded file and contract scope; neither ticket completion nor
read-scope qualification is claimed.
