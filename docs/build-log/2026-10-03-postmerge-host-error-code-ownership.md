# Postmerge host emitted-code ownership: observed CI failure and proposed documentation repair

Status: documentation-only correction applied; staged-index ownership check passed.
Terminal binding and updated hosted CI remain pending.

At ready PR #492 head `58f3b9f2a5e49576f450b60d26ea64fac3ebfa34`, CI run 37132571546,
completed doc-gates job 111230445898 failed `error-code-ownership-check` with 33 launcher held-reason
or raw transcript-operation names absent from every owning spec. The actual tested merge is
`4db95b0339b348335dc462420356e9f6dc162c9e`, tree
`01ab0fc92d24753fb001bacb34a692a09ede76ea`, with parents current main
`32aa5a3f2b7f3fe658d208049fb009e2c9e24c13` and the exact published task head. The preceding five
documentation checks passed; the subsequent use-case receipt step was skipped. Its result is unknown.

The repair inventories the existing codes in `docs/specs/postmerge-ci-host-v0.md`, separating
held event reasons from raw transcript operation names. It changes no implementation, numbered
requirement, schema, release boundary, or qualification claim. The original six frozen selected
checks did not include the mandatory CI ownership check; retain that coverage limit without changing
the original enrollment. The focused ownership check reproduced the same 33-code RED at the original sealed head. An
initial run after applying the prose still failed with the same codes because the gate reads the
Git index, not unstaged worktree bytes. That failed invocation is retained. After staging only the
reviewed spec and log and verifying index/worktree equality, the distinct ownership run passed
(exit 0; owned process joined and group absent). The actual updated hosted doc-gates result is still
pending. Native bug `V1-0690` retains the documentation defect separately from whole #398 acceptance.

Original source review, seed, CEM binding, pure seal and test receipts remain under their immutable
bindings. Continuation must preserve the original key/plan and source bytes, reopen only this
branch's own CEM through the supported repair path, derive the new citation map from the actual
changed hunks, and record new binding-specific checks/reports before an ordinary same-PR update.
No reset, new intent seed, automatic full local gate, or wholesale source review follows from this
prose correction. No code change was made. All 347 original-base CEM archives remain byte-identical; the original
348-map sealed snapshot and its own CEM remain preserved in immutable Git history and private
evidence. The own-seal inverse is one appended R100 commit, not a history rewrite.

Full #398 acceptance remains OPEN: physical engine/image execution, the positive paired consumer,
upstream delta/connector obligations and hosted full replay are unqualified. OCM's original 14
unassessed obligations remain explicit. A passing code-ownership inventory cannot close them.

Private raw job log SHA-256: `c5b84fc59c663235ecf6aa542281bbafa685b6824d916bfa647c6e95b6a128d8`.
Revert these two documentation additions through a new commit if the inventory is wrong; preserve
the actual failure, original source and historical proof artifacts.

Corvint affected evidence retained scope `UNKNOWN`, 175 selected units and 31 unknowns for the
original full source range. This repair uses the owner-scoped original six frozen checks plus the
missing ownership gate; it does not claim equivalent coverage for the broader reader set.

The accepted ownership contract `ECO-V0-006` also requires the owning row to cite the emitting
`path:line`. Before final binding, the inventory gained only those 33 exact source sites from the
verified immutable-source mapping. The earlier indexed GREEN remains bound to its original staged
inputs; final-target checks must assess the complete cited inventory. No emitter or code meaning
changed.
