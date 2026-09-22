# Decision 0121 — the refusal snapshot admits exactly the ignored ledger, and `dogfood-record` records

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls
and record them (orchestration thread, 2026-09-12).

Commit `cd3cd86b` routed `lrf` through the `SOL-V0-007` command boundary. The
`lrf-ocm-python` parity fixture ignores `.corvint/`, so its `unsupported-ocm-python-claims` refusal
appends `.corvint/self-observations.jsonl`, and the `conformance/cli-parity-v0` refusal check, which
hashes ignored files, failed `TestGPKV0002ManifestReplay` with "refusal mutated repository".
Separately, `dogfood-record` refused with a typed `unsupported-verify-syntax` code but was neither
a `SOL-V0-007` recording command nor a listed exclusion.

Call 1: the refusal nonmutation check admits exactly the gitignored ledger. A refusal passes when
Git status is unchanged, `.corvint/self-observations.jsonl` is an owner-only regular file afterwards,
and every other worktree and Git-directory entry, with its mode, is byte-identical. Any other
change still fails, including creating `.corvint/` itself and a ledger the fixture does not ignore
(which changes status). `GPK-V0-007` gains the ledger exception `GPK-V0-008` already stated, and
`GPK-V0-008` states the conformance measurement. No frozen expectation is re-captured
(`GOC-V0-002`); refusal rows carry no snapshot digests.

Alternative set aside: excluding `lrf` from `SOL-V0-007`. No owning spec forbids an `lrf` ledger
write, AGENTS.md invariant 4 grants the append to every non-`query` command, and `GPK-V0-008`
already excepts the row. Excluding `lrf` would drop a real observation to satisfy a test harness
that disagreed with the accepted rule.

Call 2: `dogfood-record` records. It is `record`'s explicit coordinator surface, refuses with the
same `unsupported-verify-syntax` code through one JSON stderr envelope, no owning spec forbids its
ledger write, and `script/dogfood-change.sh` already writes the same ledger through
`dogfood-observe` (`SOL-V0-008`). The host-adapter exclusion reasons (`ESV-V0-003`, pre-root exits,
the two-second hook bound) do not apply. `SOL-V0-007` lists it among the recording and
single-envelope commands.

Consequences:

- `TestGPKV0002ManifestReplay` passes on the unchanged manifest.
- A `dogfood-record` verification-syntax refusal in a repository that ignores the ledger appends one
  row; exit status, stdout, stderr bytes, and trace-store bytes are unchanged.

Rollback: revert this decision's commit. That restores the digest-only refusal check, removes
`dogfood-record` from the recording boundary, and restores both backlog entries.
