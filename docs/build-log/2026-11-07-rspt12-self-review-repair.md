# Self-review repair: retained trace dependencies

Scope: the three repair findings in rspt12, under RSP-V0-007, RSP-V0-009,
RSP-V0-021 and RSP-V0-054. No wire fields, error codes, exit codes, stderr,
goldens or core-freeze outputs change.

Rename resolves ancestor links but keeps each final operand as a directory entry.
An exclusive temporary renamed over dest -> data therefore creates dest, never
data. TestRSPV0007RenameOverSymlink covers rename, renameat and renameat2 followed
by a read of the unchanged target; it checks the replacement path and target
content as well as the retained read with the component present.

DropRead conservatively retains an entire read whose lexical and resolved paths
differ. A creation proof for generated content cannot establish the provenance of
a tracked link or symlinked ancestor. This chooses conservative retention within
the existing schema over introducing separate link evidence. The new
TestRSPV0007WriteProofSymlink covers create/replace proofs and matching write
aliases. The existing plain-file proof assertions are unchanged.

Modeled build attribution supplements the record with BuildLoaded and retains its
trace components on every test. TestRSPV0009Attribution now checks both records in
full, including reads, listings, lookups, execs, writes and external contacts, and
retains incomplete read state/reason despite qualified single-test markers. Its
former expectation that build reads disappear contradicted RSP-V0-009 and is
corrected as explicitly requested. Other existing assertions remain unchanged.

## Verification and boundaries

The new symlink and modeled-build assertions failed against the original
implementation. After repair, whole-package tests passed through script/heavy.sh,
one package per command, with no -run filter: internal/testevidence in 0.596 s and
internal/proctrace in 0.610 s. These are shared-host package times, not performance
claims. The Linux/amd64 proctrace test binary cross-compiles successfully; its
ptrace execution is NOT_RUN on this Darwin/arm64 host and remains a host-verifier
obligation. No remote host was enabled and no lock directory was overridden.

Affected selection used 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 and selected both
packages whole. The inherited range includes other lanes: 507 selected units,
81 unknowns, scope UNKNOWN. The installed build-127 binary is stale relative to
the integrated source; no completeness or qualification claim follows from it.
Line-citation repin found no moved unchanged citations; line-citations-check and
changed-checks passed. Raw logs remain outside the repository under
/Users/russelllewis/lanes/rspt12-evidence/; SHA-256 digests:

- self-review-testevidence-before.log: 4cdcc272e6c4a7636c167d6e2538141af9c0bcc1f1edd404494d1745f8c17e01
- self-review-testevidence-after.log: be6382d0b9b5ab17c01f1b1dfc8dc40c88c8a9db269e0d755ba337f645d3ff77
- self-review-proctrace-darwin.log: 97ad3ec0941b285ea10b4e8118be5eabae3407b63b4788730a03200e51f4479d
- self-review-proctrace-linux-compile.log: d45aa4760712b556b8a61300146bc0cb6d60a3c4bce5f8e5914fb7fc0fa7f56d
- self-review-changed-checks.log: 16490952eff646a240e0da98f0ad66fcef122700c633eddc19e9cc1a1cf6c001

The pre-change capture is reused. Rebinding/checking follows this repair; sealing,
publication, Tasks writes, model-backed review and the repository-wide gate are
NOT_RUN by instruction. Independent review belongs to the orchestrator. No new
qualification is asserted. Rollback is a revert of the repair, without migration;
it would restore the reported dependency-loss bugs.
