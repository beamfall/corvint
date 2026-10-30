# Expanded range impact V0

Owner: Russell Lewis
Date: 2026-09-09
Intent status: proposed
Delivery status: experimental
Authoritative inputs: the owner's instruction to reach full protected-runner support;
`AGENTS.md`'s explicitly experimental prototype allowance;
`go-production-kernel-migration-v0.md` (`GPK-V0-030`, `GPK-V0-040`, `GPK-V0-051`);
the accepted `CF-V0-031`/`CF-V0-032` clauses in `change-frontier-v0.md`;
`local-completion-policy-v0.md`; `../DOGFOOD.md`.

## Agent digest
- Claim: An explicit experimental fixed256 range profile preserves default100 and global evidence semantics; operator promotion remains pending.
- Status: proposed/experimental; implementation is not acceptance or promotion.
- Exists: a separate entrypoint and CLI selector, shared range compiler, and explicit prototype coordinator invocation.
- Blocked on: focused evidence, independent source review, final integrated gates and operator acceptance/promotion.
- Read next: Requirements; Resource and trust boundaries; Acceptance and traceability.

## User and measurable job

The protected-runner release from `3e3b042518367642051085f7a38ba572da2e1574` to
`871423898a5d413bebada15ebe4a7bf2b93df970` changes 140 paths, including 102 Go paths.
The default range profile rejects it at its 100-path bound before producing impact evidence.
The job is to describe this complete bounded range without changing its base, filtering membership,
weakening unsupported-member checks, or treating truncated results as complete impact proof.

The baseline is the unchanged `corvint-range-impact/0` profile and its honest refusal. A fixed
256-member experimental profile covers the evidenced input and this small extension. Batching or
union of limited receipts is unnecessary: the existing global algorithm already binds complete
membership and must retain whole-change authority, marker selection and result accounting.

## Requirements

- `ERI-V0-001`: `corvint impact --base FULL_COMMIT_ID --range-profile expanded-256` MUST explicitly
  select `corvint-range-impact-expanded/experimental`, with fixed capacity 256 changed paths.
  Its request MUST include `rangeProfile: expanded-256`. Only that exact option value is accepted;
  an absent value, repeated option, unknown value, or use without `--base` MUST fail as
  `invalid-arguments`. Existing base/path/working-tree mutual exclusions remain. No arbitrary
  numeric capacity, ambient opt-in, or unsupported-error fallback is permitted.
- `ERI-V0-002`: The experimental profile MUST use the complete original base-to-captured-HEAD
  change set and the same global semantics as `GPK-V0-030`/`GPK-V0-051`: global copy/rename discovery,
  exact path/status/mode/blob and admitted-language hunk binding, whole-change authored authority withholding,
  marker/ADR selection, evidence order and caps, deduplication, ranking, Go-member package/module verification,
  and full outside-profile omission accounting. The separately proposed
  `non-go-impact-v0.md` extends both profiles with Ruby/JS/TS syntax evidence
  and structured dynamic unknowns; it does not widen this capacity profile. Existing canonical change/hunk/omission digest preimages
  MUST remain unchanged. Coverage MUST count all admitted results before the output limit;
  `BUDGETED`, critical misses and uncertainty remain visible.
- `ERI-V0-003`: Only this profile's path capacity changes. The cumulative 10,000-admitted-hunk and
  200,000-target-line bounds and single 30-second range-compilation deadline MUST remain.
  Complete change-discovery and binary-numstat Git outputs each retain an 8 MiB bound; each
  changed-path hunk diff retains its 2,065,536-byte output bound. Per-source 1,000,000-byte and
  Git stderr 64 KiB bounds remain. These are distinct per-call byte bounds, not an aggregate
  8 MiB request allowance. Every existing unsupported member, malformed source, identity/status
  drift and exhausted bound MUST still fail explicitly, without partial success JSON.
- `ERI-V0-004`: Without the new option, the existing range profile, its 100-path refusal, positional
  impact, working-tree impact and `prove --base` MUST retain their receipt bytes and refusal conditions/codes for unchanged Go-only inputs.
  Module-versus-snapshot/base error priority follows the explicit proposed
  `NGI-V0-004` validation-order limitation.
  Ruby/JS/TS admission is separately governed by `NGI-V0-001` through `NGI-V0-006`.
  For an input accepted by both range profiles, experimental semantic bytes MUST equal default
  bytes after removing only the new profile/request discriminator and recomputing packet-byte
  accounting. The experimental selector is additive CLI/help surface, not a default cutover.
- `ERI-V0-005`: The experimental compiler MUST retain one immutable clean base/head snapshot and
  existing final currentness checks, bounded Git calls and descendant cleanup. It MUST not write
  source, index, refs, configuration, CEM/OCM or traces. The pre-existing bounded unsupported-event
  self-observation exception remains unchanged; it grants no ranking or execution authority.
- `ERI-V0-006`: The prototype repository coordinator MUST select the experimental profile by an
  explicit committed `--range-profile expanded-256` argument for the complete unchanged range.
  It MUST NOT retry failed default receipts with a larger bound or introduce a new environment,
  enrollment-plan or strict-checker exception. Actual experimental receipt production, semantic
  impact coverage, local completion and operator acceptance/promotion MUST remain distinct.
  The repository coordinator MAY preserve a typed `unsupported-impact-range` as explicit
  `NOT_PRODUCED` context evidence under the separately accepted owning-change contract. That path
  does not produce an impact receipt, relax this profile, or exempt any completion proof or checker.
- Accepted amendment (decision 0388, V1-0264, DCW-V0-025): the repository coordinator MAY also
  preserve a typed `unsupported-impact-repository` or `unsupported-impact-path` as explicit
  `NOT_PRODUCED` context evidence under the same terms, each under its own code.
- `ERI-V0-060`: (accepted 2026-10-21, V1-1881, standing lane auto-accept of 2026-10-11) Both
  range profiles MUST refuse an over-bound range with the exact bound refusal before the
  copy-detecting diff when a rename-free lower bound proves the outcome. The bound reads the
  base-to-captured-HEAD diff without rename or copy detection; detection never adds an entry, so
  the exact count is at least the rename-free count minus the smaller of its additions and
  deletions. It refuses only when that bound exceeds the profile's capacity, every changed path
  passes the range path validator, the repository is not a partial clone (no `extensions.partialclone`, no
  `remote.<name>.partialclonefilter` and no `remote.<name>.promisor` that is not false), and no unchanged
  base-tree path that fails the validator is a copy source for any addition (checked with that
  path as the only copy source). Otherwise, or on any Git failure, the exact diff decides as
  before. Every refusal and receipt that precedes the count is unchanged, including snapshot,
  worktree, module and ancestry refusals. The probe adds at most four bounded Git reads and
  writes nothing. ERI-V0-261 moves a proved refusal ahead of the snapshot load.
- `ERI-V0-261`: (accepted 2026-10-21, V1-1881, standing lane auto-accept of 2026-10-11) `impact
  --base` (both profiles, without `--worktree`) MUST return the ERI-V0-060 bound refusal before
  loading or decoding the index snapshot when it is proved without the index, with the same
  error bytes, exit code and unsupported-run observation as the loaded path. It is proved only
  when the limit is valid, `--base` is a full object ID of HEAD's format, the repository is not
  history-cut, `git status --porcelain=v1 -z --untracked-files=all` is empty, a regular snapshot
  file for HEAD's tree and the running engine exists (no byte of it is read), the root `go.mod`
  is a regular blob that Build admits as a decodable, non-generated, non-LFS source and declares a
  slash-qualified module, `--base` resolves to a commit that is the only merge base with HEAD,
  and the ERI-V0-060 lower bound exceeds the profile capacity. Otherwise, or on any Git failure,
  the loaded path decides as before, so every in-bound range, dirty worktree, missing snapshot or
  unqualified module is unchanged. Failure mode: a snapshot file whose body no longer decodes
  makes the loaded path rebuild the same clean tree, so its outcome matches unless that rebuild
  itself fails (for example on a deadline), where this refusal answers first. Non-goals: the
  `prove` range route and MCP keep loading the index. Falsifier: a proved refusal whose bytes or
  exit differ from the loaded path, or a refusal where the loaded path would answer differently.
  Rollback: remove the `RangeBoundRefusalBeforeLoad` call from the impact route.
- `ERI-V0-262`: (accepted 2026-10-21, V1-2221, standing lane auto-accept of 2026-10-11; amended
  2026-10-27, V1-2796, experimental under the standing lane auto-accept) Range
  impact, and the ERI-V0-261 pre-load route, MUST return the exact diff's path refusal (the
  ERI-V0-263 refusal naming the smallest failing path, ERI-V0-264) without running
  the copy-detecting diff when the rename-free diff proves it, with the same bytes and exit code.
  It is proved only when every rename-free record is well formed as the exact parse requires with
  a one-letter status and a UTF-8 path, at least one changed path fails the range path validator,
  the answer is the refusal of the lexicographically smallest failing changed path, the repository
  is not a partial clone, and, when the range adds any path, every base-tree path (each a possible
  copy source) is UTF-8 and none that sorts below that smallest failing changed path fails, and the
  exact diff's output stays within the status bound even if every addition gains its longest
  possible source. The path refusal is tried before
  the ERI-V0-060 bound, since the exact parse refuses before it counts. Otherwise, or on any Git
  failure, the exact diff decides as before. Human intent: `impact --base HEAD~300` on this
  repository spent 11 s in copy detection over 6,921 paths only to refuse on `café.txt`
  fixtures. Failure mode: as for ERI-V0-060, a range whose exact diff would itself exceed the Git
  deadline now refuses on the path instead of reporting the deadline. Falsifier: a proved refusal
  whose bytes differ from the exact diff's. Rollback: call `rangeCertainlyExceeds` alone from
  `rangeCertainRefusal`.
- `ERI-V0-263`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2796, GitHub
  #1893) The range path validator MUST admit a normalized repository-relative path made only of
  ASCII letters, digits, `.`, `_`, `/`, `-`, `@` and `+`, and every range path refusal MUST name
  the quoted path and the rule it fails: `Git range contains an unsupported path (it must be a
  non-empty, normalized, repository-relative path within the path-length bound): "<path>"` or `Git
  range contains a path unsafe for verification planning (allowed characters are ASCII letters,
  digits, '.', '_', '/', '-', '@' and '+'): "<path>"`. A binary refusal names the smallest binary
  path (`impact range contains binary content: "<path>"`). `@` and `+` are neither shell, glob nor
  Git pathspec magic, and `+` is valid in a Go import path; because `go test ./dir@v/...` is a
  module version query, a changed Go file whose package directory contains `@` refuses as
  `unsupported-impact-path` naming the file instead of planning a pattern Go would misread. Human
  intent: an adopter range touching `@scope/...` or `c++` paths refused whole without saying which
  path. Non-goals: spaces, non-ASCII, quotes, `*`, `?`, `[`, `:`, `\`, `$` and every other
  character stay refused. Failure modes: a consumer that matched the earlier unnamed message text
  no longer matches (the error code is unchanged); a downstream tool that mishandles `@` or `+` in a
  non-Go path now receives one. Falsifier: an admitted path that changes the meaning of a planned
  command, or a range path refusal that does not name its path. Rollback: restore the pattern
  `^[A-Za-z0-9._/-]+$` and the unnamed messages.
- `ERI-V0-264`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2796, GitHub
  #1893) The exact parse MUST refuse with the refusal of the lexicographically smallest failing
  path among changed paths and rename or copy sources, so the named path does not depend on Git's
  record order. `PerPathRangeImpact`, used only by `corvint witness` (AGW-V0-004), MUST instead
  refuse each offending change alone: it removes the change from analysis and lists it under
  `refusedPaths` as `{path, rule, detail}`, with `rule` one of `UNSAFE_PATH`, `DELETED`,
  `UNSUPPORTED_STATUS`, `MODE_CHANGE`, `UNSUPPORTED_FILE_MODE`, `BINARY_CONTENT`,
  `ROOT_GO_PACKAGE`, `GO_PACKAGE_DIRECTORY_AT`, `NOT_INDEXED`, `INVALID_GO_SYNTAX` or
  `NO_TEXT_HUNKS`, and adds the coverage uncertainty `N changed paths were refused per path and
  are not analysed; this receipt plans no verification for them`. Both members are absent when
  nothing is refused, so the receipt is then byte-identical to `RangeImpact`'s. Repository-level
  refusals (module, snapshot drift, bounds other than those ERI-V0-285 degrades per path, ancestry, a malformed diff) and a binary path that no
  change carries as its path or rename source still refuse the whole range. `RangeImpact`,
  `ExpandedRangeImpact`, `impact --base` and `prove` keep whole-range refusal so they never
  under-select. Non-goals: a per-path profile for `impact`, `affected` or `prove`; analysing a
  deleted path's former dependents (it stays a refused path). Failure mode: the ERI-V0-262 fast
  path now defers to the exact diff when a failing base-tree path, a possible copy source, sorts
  below the smallest failing changed path, costing one copy-detecting diff for that rare range.
  Falsifier: a per-path receipt that omits a changed path from both its analysis and
  `refusedPaths`, or a non-empty `refusedPaths` without the uncertainty line. Rollback: have
  `witness` call `RangeImpact` again and remove `PerPathRangeImpact`.
- `ERI-V0-265`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2797, GitHub
  #1894) `CommitPinnedPerPathRangeImpact`, used only by `corvint witness` for a historical head
  (AGW-V0-007), MUST compile `PerPathRangeImpact` over an index `BuildRevisionContext` built at an
  explicit commit that need not be checked out. It reads only the Git object store: the worktree
  identity, status and untracked-path checks do not apply, and each drift check instead MUST
  re-resolve the index's commit to itself and to the tree the index read, refusing
  `impact-range-drift` on any mismatch or Git failure and `unsupported-impact-range` when either
  identity is not a full object ID. Ancestry, bounds, path rules and every other refusal are
  `PerPathRangeImpact`'s. The range binding's `status` is `COMMIT-PINNED` and it carries no
  `statusSha256`; otherwise the receipt equals the per-path receipt compiled while that commit was
  checked out, its `packet_bytes` smaller by exactly the bytes the binding sheds. Non-goals: a
  commit-pinned `impact --base`, `affected` or `prove`; a pinned profile tolerating a missing
  object. Failure mode: a promisor or shallow clone missing the head's objects refuses in the
  index build rather than compiling a partial range. Falsifier: a pinned receipt that differs from
  the checked-out receipt beyond its binding and packet size, or one that compiles after the commit
  stops naming the indexed tree. Rollback: remove `CommitPinnedPerPathRangeImpact`,
  `verifyPinnedRange` and the `pinned` parameter.
- `ERI-V0-285`: (experimental under the standing lane auto-accept, 2026-10-28, V1-2922, GitHub
  #1928) Per path (ERI-V0-264/265 only), a range of more than 100 changed paths (counted as the
  range's records after rename and copy detection, so a rename is one record named by its
  destination) MUST analyse the first 100 in path order and list every later one under `refusedPaths` with rule `PATH_BOUND`; a
  path whose hunks would take the range past the 10,000-hunk or 200,000-target-line bound MUST be
  listed as `HUNK_BOUND` or `TARGET_LINE_BOUND` while later paths that still fit are analysed. A
  range of more than 1000 changed paths still refuses whole with the 1000-path bound, which also
  bounds the copy-detecting diff the per-path profile reads. `RangeImpact`, `ExpandedRangeImpact`,
  `impact --base` and `prove` keep the whole-range refusals. Falsifier: a per-path receipt over
  the bound that drops a changed path from both its analysis and `refusedPaths`. Rollback: restore
  the whole-range refusal in the per-path mode.
- `ERI-V0-305`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2919, GitHub
  #1925) `impact --base` (both profiles, without `--worktree`) MUST compile with the ERI-V0-264
  per-path rules: a deleted path or an unsafe path, and every other path-level refusal ERI-V0-264
  lists, is named under `refusedPaths` with the coverage uncertainty line instead of refusing the
  range, and the other paths are analysed. The ERI-V0-261 pre-load route then proves only the
  ERI-V0-060 bound refusal, not the ERI-V0-262 path refusal. With no refused path the receipt is
  unchanged. `prove` and the library `RangeImpact` and `ExpandedRangeImpact` keep whole-range
  refusal. This supersedes the `impact --base` clause of ERI-V0-264. Non-goal: analysing a deleted
  path's former dependents. Failure mode: a consumer that ignored `refusedPaths` would treat the
  receipt as covering the whole range; the uncertainty line and `critical` set name the gap.
  Falsifier: `impact --base` exiting 2 on a range whose only problem is a deleted or unsafe path.
  Rollback: call `RangeImpact` and `ExpandedRangeImpact` again from the impact route.

## Resource and trust boundaries

The two public Go entrypoints choose fixed 100 or 256 capacity; one private compiler owns every
other step. The shared positional `maxImpactPaths` remains 100. Index construction retains its
separate existing resource limits; this extension adds no worker, process, cache, network request
or alternate Git reader. Copy sources and caller-authored authorities are resolved over the entire
range, never a subset. A new profile identity does not turn caller assertions into authority.

Delete/type/mode changes, symlinks, submodules, binary content, unsafe paths, excluded or malformed
Go sources, ancestry failures and snapshot drift stay refused; the per-path profile of ERI-V0-264
refuses each such path alone, names it, and still refuses the whole range for repository-level
failures. A 257-member range stays unsupported.
A receipt containing 102 admitted Go paths with limit 20 still omits 82 ranked paths; successful
bounded enumeration does not close those critical misses or the non-Go semantic frontier.

## Acceptance and traceability

| Requirement | Implementation | Executable evidence |
|---|---|---|
| `ERI-V0-001` | explicit CLI parsing and `ExpandedRangeImpact` | `TestExpandedRangeImpactBoundaries`, `TestExpandedRangeImpactCLIClosedSelection` |
| `ERI-V0-002` | unchanged global range compiler/reducer | `TestExpandedRangeImpactGlobalCoverage`, `TestExpandedRangeImpactGlobalAuthority`, `TestExpandedRangeImpactPreservesDefaultSemantics` |
| `ERI-V0-003` | existing validators and correctly scoped bounds | `TestExpandedRangeImpactRejectsUnsupportedMembers`, `TestExpandedRangeImpactBoundaries`; unchanged range/Git bound tests |
| `ERI-V0-004` | default wrapper and unchanged positional limit | `TestExpandedRangeImpactPreservesDefaultSemantics`, `TestImpactRejectsInvalidBoundsBeforeCompilation`, `TestExpandedRangeImpactProveRejectsSelector`, existing committed-range tests |
| `ERI-V0-005` | shared immutable snapshot and Git runner | `TestExpandedRangeImpactPreservesSnapshotRefusal`, `TestExpandedRangeImpactCLIExplicitReadOnly`; existing Git interruption/cleanup regressions |
| `ERI-V0-006` | `script/dogfood-change.sh` explicit prototype argv | actual final coordinator receipt and independent strict checker; final integrated evidence pending |
| `ERI-V0-060` | `rangeCertainlyExceeds`, `mayLackObjects` in `compileRangeImpact` | `TestRangeImpactRefusesOverBoundBeforeCopyDetection`, `TestRangeImpactOverBoundKeepsMissingObjectRefusalForPromisorRemotes`; measured in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-range-bound-before-copy-detection.md` |
| `ERI-V0-261` | `RangeBoundRefusalBeforeLoad` (`internal/contextindex/range_bound_precheck.go`) before `overSnapshot` in the `impact` route of `cmd/corvint/main.go` | `TestERIV0261RangeBoundRefusesWithoutDecodingTheSnapshot` (SHA-1/SHA-256 and expanded over-bound refusals equal to the loaded path's with the snapshot file emptied; in-bound, missing snapshot, dirty worktree, unqualified module, non-ancestor and invalid limit answer nil); measured in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-partial-1021-ticket-completions.md` |
| `ERI-V0-262` | `rangeCertainRefusal`, `rangeCertainPathRefusal` in `compileRangeImpact` and `RangeBoundRefusalBeforeLoad` | `TestRangeImpactRefusesUnsafePathBeforeCopyDetection` (modified and added unsafe paths refuse with the exact bytes and no exact diff; a smaller unsafe copy source reaches the exact diff), `TestRangeCertainPathRefusalProvesSmallestFailingPath`; measured in `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-21-index-perf-1021i-hot-read-paths.md` |
| `ERI-V0-263` | `rangePathPattern`, `rangePathRule`, `validateRangePath`, `rangeBinaryError` and the Go-directory `@` guard in `internal/contextindex/range_impact.go` | `TestRangePathRefusalNamesPathAndRule`, `TestRangeImpactRefusesGoPackageDirectoryWithAt` |
| `ERI-V0-264` | `parseRangeChanges`, `parseRangeChangeRecords`, `PerPathRangeImpact` and `compileRangeImpactMode` | `TestRangePathRefusalNamesPathAndRule` (order-independent refusal), `TestPerPathRangeImpactNamesEachRefusedPath`, `TestPerPathRangeImpactKeepsBytesWithoutRefusals` |
| `ERI-V0-285` | `pastBound`, the per-path hunk bound and `maxPerPathRangeRecords` in `compileRangeImpactMode` | `TestPerPathRangeImpactRefusesPathsPastTheBoundEachAlone`, `TestPerPathRangeImpactRefusesAPathPastTheHunkBoundAlone` |
| `ERI-V0-265` | `CommitPinnedPerPathRangeImpact`, `verifyPinnedRange` and the `pinned` mode of `compileRangeImpactMode` | `TestCommitPinnedRangeImpactMatchesTheCheckedOutHead` (parity after HEAD moved and the worktree went dirty; the worktree-bound profile refuses the same index), `TestCommitPinnedRangeImpactFailsClosed` (tree drift, short identity, absent commit) |

Boundary evidence includes 0, 1, 100, 101, 140, 256 and 257 members; mixed 102-Go/38-non-Go
coverage; SHA-1/SHA-256 semantic parity; same-change ADR/ledger withholding; unsupported members
beyond the original cap; and snapshot drift. A test name here identifies evidence, not a passing
run. Actual results, exclusions and independent review belong in new entries under `../build-log/`.

## Non-goals, rollout and rollback

No default capacity widening, dynamic capacity, batching, new membership digests,
reverse-import/test closure, accepted root, native FULL qualification or latency claim.
Ruby/JS/TS syntax admission is an independent proposed extension in `non-go-impact-v0.md`.
The prototype coordinator uses the explicit experimental profile, while ordinary public `impact
--base` remains default 100. Existing frozen enrollment plans and failed receipts are not rewritten.

Operator acceptance and promotion remain unresolved. The prototype may proceed through focused
tests and independent review, then final integrated CEM/OCM and required gates; no source boolean
or passing local check promotes it. Kill the extension if it changes default semantics, weakens
global authority/refusal behavior, or cannot handle the actual bounded input within existing limits.
Rollback removes the opt-in profile and coordinator argument, restoring the visible default refusal
without erasing its historical evidence or claiming that rollback reaches full support.
