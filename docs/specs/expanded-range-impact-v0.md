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
  exact path/status/mode/blob and Go hunk binding, whole-change authored authority withholding,
  marker/ADR selection, evidence order and caps, deduplication, ranking, package/module verification,
  and full non-Go omission accounting. Existing canonical change/hunk/omission digest preimages
  MUST remain unchanged. Coverage MUST count all admitted results before the output limit;
  `BUDGETED`, critical misses and uncertainty remain visible.
- `ERI-V0-003`: Only this profile's path capacity changes. The cumulative 10,000-Go-hunk and
  200,000-target-Go-line bounds and single 30-second range-compilation deadline MUST remain.
  Complete change-discovery and binary-numstat Git outputs each retain an 8 MiB bound; each
  changed-path hunk diff retains its 2,065,536-byte output bound. Per-source 1,000,000-byte and
  Git stderr 64 KiB bounds remain. These are distinct per-call byte bounds, not an aggregate
  8 MiB request allowance. Every existing unsupported member, malformed source, identity/status
  drift and exhausted bound MUST still fail explicitly, without partial success JSON.
- `ERI-V0-004`: Without the new option, the existing range profile, its 100-path refusal, positional
  impact, working-tree impact and `prove --base` MUST retain their receipt bytes and error behavior.
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

## Resource and trust boundaries

The two public Go entrypoints choose fixed 100 or 256 capacity; one private compiler owns every
other step. The shared positional `maxImpactPaths` remains 100. Index construction retains its
separate existing resource limits; this extension adds no worker, process, cache, network request
or alternate Git reader. Copy sources and caller-authored authorities are resolved over the entire
range, never a subset. A new profile identity does not turn caller assertions into authority.

Delete/type/mode changes, symlinks, submodules, binary content, unsafe paths, excluded or malformed
Go sources, ancestry failures and snapshot drift stay refused. A 257-member range stays unsupported.
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

Boundary evidence includes 0, 1, 100, 101, 140, 256 and 257 members; mixed 102-Go/38-non-Go
coverage; SHA-1/SHA-256 semantic parity; same-change ADR/ledger withholding; unsupported members
beyond the original cap; and snapshot drift. A test name here identifies evidence, not a passing
run. Actual results, exclusions and independent review belong in `../BUILD-LOG.md`.

## Non-goals, rollout and rollback

No default range widening, dynamic capacity, batching, new membership digests, broader language
admission, reverse-import/test closure, accepted root, native FULL qualification or latency claim.
The prototype coordinator uses the explicit experimental profile, while ordinary public `impact
--base` remains default 100. Existing frozen enrollment plans and failed receipts are not rewritten.

Operator acceptance and promotion remain unresolved. The prototype may proceed through focused
tests and independent review, then final integrated CEM/OCM and required gates; no source boolean
or passing local check promotes it. Kill the extension if it changes default semantics, weakens
global authority/refusal behavior, or cannot handle the actual bounded input within existing limits.
Rollback removes the opt-in profile and coordinator argument, restoring the visible default refusal
without erasing its historical evidence or claiming that rollback reaches full support.
