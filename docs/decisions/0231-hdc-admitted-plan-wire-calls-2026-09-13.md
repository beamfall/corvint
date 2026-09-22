# Decision 0231 — HDC admitted plan wire calls

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`HDCV0-027` names the plan members but not their spelling, and `HDCV0-028/029` name operation
contents but not the proposal patch bytes. This decision makes those calls for the pure plan-only
compiler in `internal/doccompiler/admittedplan.go` (`CompileAdmittedPlan`, `VerifyAdmittedPlan`,
`StaleOperations`). It builds on the clause admission of decision 0229.

The call:

(a) Scope: plan-only. The environment pin is the `HDCV0-CAP-001` plan-only variant
`{"kind":"plan-only"}`, and the configuration snapshot is `{"status":"NOT_RUN"}`, because a plan-only
request starts no process and so no pinned MkDocs loader runs. Any other pin kind is
`plan-environment-unsupported` and any other snapshot status is `plan-snapshot-unsupported`. An
execution pin (`HDCV0-001..010`, `HDCV0-CAP-001..004`) and a real snapshot are not implemented.

(b) Members. The closed top-level members are `profile`, `source_identity` (`revision`, `sources`
of `path`, `mode`, `blob`, `sha256`), `environment_pin`, `configuration_snapshot`, `clauses` (the
decision 0229 admitted clauses, anchors always an array), `operations`, `candidate_patch_sha256`,
`uncertainty`, `limits_consumed` (`anchors`, `clauses`, `operations`, `source_bytes`,
`source_files`), and `exclusions` (`code`, `path`). The bytes are `HDCV0-041` canonical JSON. No
build, offline, or artifact member exists. Every array is present, never `null`.

(c) Source identity. The consumed sources are every anchor path, of any admitted state, whose pinned
bytes are valid text hashing to the pinned Git blob, plus every present target. An anchor path that
is not consumed that way is an `anchor-source-unverified` exclusion. `sha256` is over the pinned bytes.

(d) Order. `sources` are strictly increasing by path, `clauses` by ID, `operations` by target, and
each `clause_ids` by ID. A repeated or decreasing member is `unordered-set`; a repeated clause or
operation ID is `duplicate-clause` or `duplicate-operation`; an unknown member is
`plan-unknown-field`. The experimental `PatchPlan` bytes fail as `plan-unknown-field`.

(e) Operations. `create_file` has `target_state` `ABSENT`, empty `target_blob` and `target_sha256`,
and bytes `0..0`. `insert_after` has `PRESENT`, the pinned blob and SHA-256, and `start_byte` equal to
`end_byte` equal to the target length: it appends after the last byte, and a request cannot choose
another position. The inserted bytes are one LF (two when the target lacks a final LF, none for an
empty target) and then the `corvint-hdc-prose-templates/0` rendering of the operation's clauses.
`replacement_sha256` is over exactly the inserted or created bytes. `edit_nav` is refused as
`nav-authority-required` until `HDCV0-030`.

(f) Targets. A target is a clean repository-relative `.md` path of bytes 0x21..0x7E other than `"`
and `\`, so the diff header needs no Git quoting. One operation per target; a second is
`overlapping-operation`. A pinned target must be a regular (`100644`/`100755`) text blob whose bytes
hash to its blob (`target-unverified`) and at most 1 MiB. A tracked but unpinned or non-blob target is
`unpinned-target`. Absence is proven only from the index's tracked tree: without one it is
`absence-unproven`, and a target under a tracked file or over tracked paths is `target-conflict`. A
reason is non-empty UTF-8 of at most 4,096 bytes with no control character; an operation ID uses
the clause ID grammar; an operation names at least one admitted clause (`unknown-clause`).

(g) Patch. `proposal.patch` is a `git apply` unified diff, one file section per operation in plan
order, with up to three context lines; an unterminated last line is spelled as removed and re-added
with `\ No newline at end of file`. `candidate_patch_sha256` is its SHA-256.

(h) Verification. `VerifyAdmittedPlan` checks canonical bytes, closed decode, identity, kinds,
uniqueness, and order, then refuses a plan whose revision is not the index revision as
`stale-source`, then recompiles the clauses and operations and refuses anything not byte-equal in
plan and patch as `plan-not-reproducible`. `StaleOperations` lists, in plan order, operations whose
precondition fails at another index; it changes nothing and never retargets.

(i) Uncertainty and exclusions. `uncertainty` is exactly `configuration-snapshot-not-run` and
`environment-plan-only`. `exclusions` always carries `edit_nav-requires-configuration-authority`.

(j) `PatchPlan` stays experimental. It still carries the profile string, and it is still what
`internal/docviews` binds (`CATN-V0-001`), with truth and bundle digest pins over its bytes. It is not
`HDCV0-027` bytes. Moving docviews to the admitted plan changes the CATN wire, so it is filed in
`docs/agent-memory/fixes.md` instead of made here. `internal/docmaintain`, `cmd/corvint` docs and the
MCP docs bridge use `DraftSources`/`ConsumeDraft`, not `PatchPlan`, so the earlier four-consumer note
was wrong.

Delivery: `HDCV0-027` for the plan-only environment variant; `HDCV0-028` for `create_file` and
`insert_after`; `HDCV0-029`. `HDCV0-030..032` stay undelivered. No requirement IDs are added.

Rollback: revert the commit. No other package calls the new functions, and no emitted wire changes.
