# End-of-line policy gate V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `EPG-V0`
Intent status: accepted (decision 0150, owner instruction 2026-09-12)
Delivery status: implemented
Authoritative inputs: `../../AGENTS.md` invariants 1, 2 and 8,
`../SPEC-DRIVEN-DEVELOPMENT.md` "Required capability-spec shape",
`../decisions/0061-windows-is-a-deferred-target-and-gitattributes-is-repository-wide-2026-09-05.md`,
`../decisions/0150-eol-policy-gate-spec-2026-09-12.md`, and the root `.gitattributes`.

## Agent digest
- Claim: Every tracked path keeps the repository-wide `-text` attribute, and only two named fixtures may commit CRLF or mixed bytes.
- Status: accepted (decision 0150, owner instruction 2026-09-12) / implemented
- Exists: `script/check-eol-policy.sh`, `script/check-eol-policy_test.sh`, and the `eol-policy-check` and `eol-policy-test` targets in `make gate`.
- Blocked on: nothing for this gate; behaviors without a dedicated fixture assertion remain identified as `NO TEST` below.
- Read next: Requirements; Trust boundary, limits, and failure modes; Traceability.

## Human intent and scope

Decision 0061 made the repository-wide Git attribute `* -text`. Git therefore preserves committed
bytes instead of normalising line endings, which is required for byte-exact fixtures but also means
a contributor can commit CRLF without Git correcting it. The end-of-line policy gate makes the
chosen attribute and the permitted non-LF set mechanically checkable.

Affected user: an agent or engineer staging repository content on any platform. Measurable job: for
the complete tracked-path listing reported by Git, fail when any path does not report the exact
`attr/-text` value, or when any non-exempt index entry reports CRLF or mixed endings; otherwise
report the tracked and exempt-path counts.

The owner instruction of 2026-09-07 placed repository gate tooling in scope for AGENTS.md
invariant 8. Decision 0150 accepts this specification under the owner's 2026-09-12 delegation.
This document owns the existing gate only; it does not change the script, `.gitattributes`, or the
two fixtures.

## Verified current state

At `1c951c53`, `script/check-eol-policy.sh` exits 0 over the live repository and prints
`eol policy: 3064 tracked paths clean, 2 exempt fixture(s)`. Its fixture test exits 0 and prints
`check-eol-policy_test: 4 cases passed`. The test covers one clean tracked set, one narrower
attribute override, one committed CRLF path, and the two exact non-LF fixture paths. The script is
unchanged by this specification.

## Requirements

- `EPG-V0-001`: The gate MUST resolve the repository root from its own location, change to that
  root, and read the complete NUL-delimited output of `git ls-files --eol`. Each non-empty record in
  that output MUST count as one tracked path; the gate MUST NOT enumerate paths with a filesystem
  walk.
- `EPG-V0-002`: The gate MUST split each record at its first tab into the EOL fields and path, fail
  on a record without that separator, take the first space-delimited field as the index EOL and the
  third as the attribute value, trim trailing whitespace from that attribute, and treat a missing
  attribute field as the empty string.
- `EPG-V0-003`: Every tracked path MUST report the exact attribute value `attr/-text`. Any other
  value, including the empty value, MUST be collected as attribute drift. The two ending-exempt
  paths are not exempt from this attribute check.
- `EPG-V0-004`: For every path not named by EPG-V0-005, an index EOL of exactly `i/crlf` or
  `i/mixed` MUST be collected as ending drift. The gate MUST NOT reject another index EOL value
  through this ending check.
- `EPG-V0-005`: The ending-drift exemption MUST be the closed set
  `interop/cem-0.1/repository/base/src/ending.txt` and
  `interop/cem-0.1/patches/line-ending.patch`. When either exact tracked path is encountered, the
  gate MUST count it as exempt and skip only its ending check, regardless of the index EOL value.
  No basename, directory, pattern, or third path MUST inherit the exemption.
- `EPG-V0-006`: The gate MUST scan the whole listing before reporting policy drift. It MUST report
  every attribute-drift path with its observed attribute value and every ending-drift path with its
  observed index EOL, under separate diagnostic headings and repair guidance, and MUST emit both
  groups when both exist. Any policy drift MUST exit 1.
- `EPG-V0-007`: Failure to start or successfully close `git ls-files --eol`, and an unparsable
  listing record, MUST fail non-zero rather than producing a clean verdict. The gate itself MUST
  write no repository or trace state.
- `EPG-V0-008`: A drift-free run MUST exit 0 and print exactly one summary line containing the
  number of tracked records and the number of exact exempt paths encountered.
- `EPG-V0-009`: `make gate` MUST invoke both `eol-policy-check`, which runs the policy script, and
  `eol-policy-test`, which runs its fixture test. Both targets MUST be declared phony.

## Non-goals and simpler baseline

The simpler baseline is the root `* -text` rule alone. That rule preserves bytes but does not
prevent a CRLF-committing checkout from publishing those bytes, which is the gap this gate closes.

The gate does not rewrite, normalise, stage, or remove content. It does not inspect untracked paths,
working-tree EOL values, encodings, lone carriage returns, or any index classification other than
the exact `i/crlf` and `i/mixed` values. It does not parse `.gitattributes` itself or require that
its root line have a particular textual spelling; it enforces the effective `attr/-text` value Git
reports for every tracked path. It does not discover exemptions from fixture content, and it grants
no third path an exemption automatically.

## Trust boundary, limits, and failure modes

Git's `ls-files --eol` output is the sole policy input. The tracked set and index EOL classification
come from Git's index view; the effective attribute field is whatever Git reports for the current
attribute configuration. The shell wrapper resolves the repository root and launches one Perl
interpreter, which launches Git once. The gate reads that stream and writes only diagnostics.

| Failure | Behavior |
|---|---|
| A tracked path reports an attribute other than exact `attr/-text` | fail after the full scan, naming the path and observed attribute |
| A non-exempt tracked path reports `i/crlf` or `i/mixed` | fail after the full scan, naming the path and observed index EOL |
| An ending-exempt path reports an attribute other than `attr/-text` | fail as attribute drift; the exemption skips only the ending check |
| Both drift classes occur | fail with both diagnostic groups and all collected paths |
| Git cannot start or exits non-zero, or a record has no tab separator | fail non-zero; no clean summary is emitted |
| A tracked path has another index EOL classification | pass the ending check; its attribute must still be `attr/-text` |
| No policy drift exists | print the tracked and exact-exemption counts, exit 0 |

## Acceptance criteria and testing matrix

Evidence is the four-case fixture in `script/check-eol-policy_test.sh`, the live corpus invocation,
and explicit source inspection where the current test has no assertion.

| Requirement | Evidence |
|---|---|
| EPG-V0-001 | `script/check-eol-policy.sh:22-40`; fixture cases operate over tracked paths, but there is `NO TEST` for invocation from a different working directory or exclusion of an untracked path |
| EPG-V0-002 | `script/check-eol-policy.sh:40-48`; `NO TEST` supplies a malformed `git ls-files --eol` record or missing attribute field |
| EPG-V0-003 | `script/check-eol-policy_test.sh:40-51` makes a tracked Markdown path report `attr/text`, requires failure, and requires that path in the diagnostic; `NO TEST` checks attribute drift on an ending-exempt path |
| EPG-V0-004 | `script/check-eol-policy_test.sh:53-67` commits CRLF, requires failure naming that path, and rejects a diagnostic naming the LF-clean path; `NO TEST` separately exercises `i/mixed` or another index EOL value |
| EPG-V0-005 | `script/check-eol-policy_test.sh:69-78` commits CRLF and mixed bytes at the two exact exempt paths and requires a passing count of two; the closed-set path literals are in `script/check-eol-policy.sh:29-32` |
| EPG-V0-006 | cases 2 and 3 require one named failing path; `NO TEST` covers multiple paths, simultaneous drift groups, the exact headings, or exit status 1 rather than merely non-zero |
| EPG-V0-007 | source inspection of `script/check-eol-policy.sh:34-59`; `NO TEST` forces Git start/close failure, an unparsable record, or observes repository writes |
| EPG-V0-008 | `script/check-eol-policy_test.sh:31-38` requires the clean summary and case 4 requires the exempt count; the live gate passed at `1c951c53` with 3064 tracked paths and 2 exemptions |
| EPG-V0-009 | `Makefile:16-23` declares the targets phony and enrolls both in `gate`; `Makefile:87-95` invokes the two scripts |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| EPG-V0-001 | root resolution and NUL-delimited tracked listing in `script/check-eol-policy.sh:22-40` | `script/check-eol-policy_test.sh`; `NO TEST` for different-cwd invocation or an untracked-path fixture |
| EPG-V0-002 | record parsing and attribute normalization in `script/check-eol-policy.sh:40-48` | `NO TEST`; code inspection only |
| EPG-V0-003 | attribute comparison in `script/check-eol-policy.sh:50` | `script/check-eol-policy_test.sh:40-51`; `NO TEST` for exempt-path attribute drift |
| EPG-V0-004 | exact index-EOL comparison in `script/check-eol-policy.sh:52-56` | `script/check-eol-policy_test.sh:53-67` for CRLF; `NO TEST` for mixed or another index EOL value |
| EPG-V0-005 | closed exemption map and ending-only skip in `script/check-eol-policy.sh:29-32` and `script/check-eol-policy.sh:52-55` | `script/check-eol-policy_test.sh:69-78` |
| EPG-V0-006 | accumulated diagnostics and exit 1 in `script/check-eol-policy.sh:61-74` | `script/check-eol-policy_test.sh:40-65` for one path per drift class; `NO TEST` for multiple/simultaneous drift or exact status 1 |
| EPG-V0-007 | Git open/close and parse failures in `script/check-eol-policy.sh:34-59` | `NO TEST`; code inspection only |
| EPG-V0-008 | success summary in `script/check-eol-policy.sh:76` | `script/check-eol-policy_test.sh:31-38` and `script/check-eol-policy_test.sh:69-78`; live policy check at `1c951c53` |
| EPG-V0-009 | `eol-policy-check` and `eol-policy-test` recipes and gate membership in `Makefile:16-23` and `Makefile:87-95` | `script/check-eol-policy_test.sh`; Makefile inspection |

## Rollout, rollback, and drift

This change records ownership and acceptance for an already-enforced gate. It does not change
runtime behavior. Rollback removes this spec, decision 0150, and their index rows, restoring the
documentation gap while leaving decision 0061, `.gitattributes`, the script, its test, and gate
membership unchanged.

Drift rule: the exemption remains a closed set of two exact paths and bypasses only the ending
classification. Any change to the tracked-set source, accepted attribute value, rejected EOL
classifications, exemption paths, diagnostics, success summary, or Makefile membership changes
EPG-V0 behavior and must amend this spec with corresponding evidence in the same commit.

## Unresolved

The behaviors marked `NO TEST` above are source-inspected but not independently exercised by the
four-case fixture. They are disclosed acceptance-evidence gaps, not inferred passes. This decision
does not require broadening the test to accept the current contract, and a future test-only change
may close them without changing the requirements.
