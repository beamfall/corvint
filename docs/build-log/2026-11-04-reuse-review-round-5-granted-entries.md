# reuse: review round 5 (declaration bytes, granted directory entries, publish-time drift)

Base: `6f12f4704235dc8f9bea19fc12995bfb68c746c3`. Tickets: V1-1097, V1-1099. Requirements amended
in place, with IDs unchanged: AFP-V0-2320, AFP-V0-2321 and AFP-V0-2325 in
`docs/specs/affected-plan-reuse-v0.md`. An independent review of the round-4 head `fdf175cb1`
returned two P1 findings and one P2. Each is fixed with a regression test that fails against the
reviewed implementation, and each fix refuses where it cannot prove soundness.

## Findings and decisions

1. **The read-scope declaration was parsed from its blob but loaded from the worktree (P1,
   AFP-V0-2320).** The confinement wrapper reads `.corvint/test-read-scopes.json` from disk. A
   clean filter that maps a widened worktree declaration back to the committed blob keeps the
   status clean, so the digest bound the narrow scopes while the wrapper granted the wide ones.
   Decision: after parsing, the read-scope and split declarations get the same raw worktree check
   as every digest input (regular file, blob hash, executable bit). Any mismatch sets the
   declaration error, so no package is eligible. Evidence: `TestFilteredDeclarationRuns`. The
   filter over the unchanged declaration is the negative control, with both packages reused. On
   the reviewed code the widened declaration was also reused. Now both packages are
   `DIGEST_ERROR` with `worktree-bytes: .corvint/test-read-scopes.json`.
2. **Granted directories were bound only by their tracked files (P1, AFP-V0-2321, AFP-V0-2325).**
   Git does not report an empty untracked directory or a FIFO, yet the confinement grants the
   package directory and each declared `DIR/` whole, so a test that counts `os.ReadDir` entries
   changed result with an unchanged digest. Decision, made by the coordinator: walk each granted
   subtree on disk without following links. Every directory must hold a tracked path, and every
   file or link must be tracked. Every input the digest records as `absent` must not exist in any
   form. Otherwise the package runs with the new reason `UNTRACKED_GRANTED_ENTRY`, which names the
   path, under the existing AFP-V0-2321 and AFP-V0-2325. The check runs after the dirty and raw
   byte checks, so an untracked or ignored file still reports `dirty-input`. Walks are cached per
   snapshot and per directory. Rejected alternative: binding a directory listing into the digest,
   which would make every recorded digest depend on untracked build output. Evidence:
   `TestGrantedDirectoryEntriesRun`. A real `go test` passes on the committed tree. Then each of
   five faults is added and removed in turn: an empty directory below the package, an empty
   directory below a declared `fixtures/` (each also fails the real rerun), an empty directory at
   an `absent` `gone/`, one at an `absent` `missing.txt`, and a FIFO. The undeclared sibling `b`
   and the restored tree are the negative controls. On the reviewed code `d` was reused under
   every fault.
3. **The final recheck preceded the record, the decision and cached validations (P2,
   AFP-V0-2321).** The CLI rechecked before digesting the confinement wrapper and before
   `BuildRecord` or `Decide`, and those then reused cached file validations. Decision: the check
   (`testReuseFinalCheck`) moves to after the record or decision is built and immediately before
   anything is written. `Snapshot.Recheck` now reruns every cached validation uncached (raw
   bytes, granted walks, absent inputs) and fails on any changed detail. The confinement wrapper
   is digested again and compared. Any change is `snapshot-drift`, exit 3, and no output is
   written. Evidence:
   - `TestTestReuseLateEditIsDrift`: a test hook edits a scoped input inside that window, for
     both `decide` and `record`, and then rewrites the confinement wrapper. The unedited decision
     is the negative control. On the reviewed code the decision was published (exit 0).
     Disabling only the wrapper comparison fails the wrapper step.
   - `TestRecheckRevalidatesWorktreeBytes`: with `core.checkStat=minimal` and
     `core.trustctime=false`, a same-size edit with a restored mtime is hidden from status after
     the harness file was validated. On the reviewed code `Recheck` returned nil.

## Self-review finding

A hostile self-review of the round diff found that the first walk returned `SkipDir` for a root
`.git` entry of any type. In a linked worktree `.git` is a file, and `SkipDir` on a file makes
`filepath.WalkDir` skip the rest of that directory, so a root package's walk ignored every root
entry sorted after `.git`. Decision: skip only a `.git` directory and pass over a `.git` file.
Evidence: `TestWalkGrantedRootGitFile` fails on the first walk (the empty `zz` directory went
unseen) and passes with the fix. A `.git` directory is the negative control.

## Residuals

The walk is POSIX-tested only: the fixtures use a shell filter, FIFOs and `os.ReadDir` over
POSIX paths. Windows has no FIFO, and its reparse points are refused as special files. The
confinement wrapper may live outside the checkout. It is bound by content at digest time and at
publication, and trusted between those points as before.
