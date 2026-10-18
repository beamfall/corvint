# V1-1675: e2e-safe selection across a separate test repository (issue 1078)

Ticket V1-1675, GitHub issue 1078. Requirements AFU-V1-067 to AFU-V1-072 in
[application-flow-understanding-v1](../specs/application-flow-understanding-v1.md), accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). This is not a new owner
decision; it applies that standing acceptance to this lane's requirements.

## Decisions

- **The test repository is the root.** The suite, provider, flows, discovery and inventory stay in
  the repository `affected` runs in (T). Application repositories (A) contribute changed paths and
  evidence only, so discovery, inventory and the impact graph keep their single-repository
  contracts.
- **Authority is the provider's declaration.** A repository counts only when the committed provider
  declares its ID and parentless `origin` commit. An operator `--repository` binding the provider
  does not declare, a declared ID left unbound, a checkout whose HEAD does not reach the origin, a
  subdirectory of a checkout, or a checkout that moves while it is read is
  `e2e-repository-unbound` and the full relevant suite. Nothing narrows on an unpinned repository.
- **Explicit base per repository.** `--repository-base ID=FULL_COMMIT` is required for each bound
  ID. There is no merge-base guess. The A changed set is base..HEAD plus the worktree status, and
  the result binds base, revision (HEAD), tree and the uncommitted path count.
- **Qualified paths, no new graph.** A changes join the changed set as `ID:PATH`. They have no
  impact-graph edge, so only a coverage record or a reviewed source link naming the path explains
  one; anything else is `e2e-unmapped-change`. A root path that looks qualified is refused the same
  way, so the two namespaces cannot alias.
- **Evidence pins per repository.** A coverage record names the A commit its run observed
  (`repositories`), and the record is stale when any covered A path, A global path or qualified link
  target changed between that commit and the A base. A reviewed qualified link names the A commit
  the review saw (`reviewed_at_repository`) and is reviewed only when its target content is equal at
  that commit and at the A base. Outside this profile such a link is `target-unpinned`.
- **Additive schema.** `e2e-safe-selection/1` gains the optional `repositories` member;
  `ProofLink.reviewed_at_repository` is omitted when empty, so existing proof digests and the frozen
  corpus report are byte-identical.

## Review findings (Codex, PR 1102)

- **Grafted ancestry.** `rev-list` and `merge-base` honor `info/grafts` even with
  `GIT_NO_REPLACE_OBJECTS`, so a foreign checkout could graft its HEAD onto the declared origin and
  bind. A checkout with `info/grafts` in its common Git directory, or a shallow one, now does not
  bind (AFU-V1-068), checked before the ancestry reads and again after the changed-path reads.
- **Unrepresentable application paths.** An A change `x:y.ts` became `app:x:y.ts`, which does not
  parse as qualified and so took root impact-graph ownership; a T file of that name could explain
  it. Such a path now unbinds the repository, and any path in a declared `ID:` namespace never
  takes root ownership or reach (AFU-V1-068, AFU-V1-069).
- **Coverage tier paths.** Tier paths were not validated, so `other:src/search.rb` or
  `app:../src/search.rb` could stand as coverage and miss the change they name. A tier path must be
  a canonical root path or a canonical qualified path of a declared ID (AFU-V1-070); a root path
  check is included because the same miss applies to a non-canonical root path.

## Dogfood

`make dogfood-change BASE=2ab0e74421147cb16d2da183419e1ad2d991fa52` at lane start returned rc=2
(no CEM map yet, `outcome-input-not-provided`), as expected before the bind.

## Limits

No planning snapshot per repository and no impact graph over A: a change in an A file that only
reaches a linked file through imports is not explained and falls back to the full suite. `flows
export` omits links with qualified targets, and docs maintenance records a qualified target as an
absent root input. Live qualification against a real two-repository
project is `NOT_RUN`; the evidence is the deterministic two-repository fixtures named in the spec.
