# V1-5621 AC2: commit identity guard

Date: 2026-11-18. Host: `PC` (Linux). Lane: `m8-authguard`.
Base: `84a977dc5721a840159c369f1ab4fce91ce2a2d1`.
Implementation: `c7e7d7f7aa21bf843013cb10107e3a98bd695d09`.
Requirements: `ASD-V0-560` and `CIN-V0-006`; existing trailer behavior remains `ASD-V0-831`.
Only AC2 is delivered. AC1 (history disposition) and AC3 (GitHub contributors) remain open.

## Decision and boundary

Extend the existing trailer guard rather than add a second identity implementation. Compare raw
Git author/committer name and email fields case insensitively: exact `Claude`, or any mailbox at
`anthropic.com` or its subdomains. Mailmap presentation cannot mask these raw fields. Trailer,
message-cleanup and strip matching remain unchanged; no history is rewritten.

The existing installed pre-push wrapper calls `script/pre-push-windows-vet.sh`; that script now
runs the shared guard before vet, including on documentation-only pushes. Read the destination
location supplied by Git. Exclude ancestry of its published branch/tag refs and replaced refs,
not cached tracking refs or an unrelated origin. Exclude server-internal PR refs from publication
history: a normal fetch does not supply them. Missing published objects require a fetch; missing
or shallow history and failed enumeration refuse instead of claiming a clean range. Deleted refs
introduce no commits. The guard never fetches or installs hooks during a check.

CI already calls the shared checker through `make ai-coauthor-trailers-check`. With a PR event
file, the target validates its actual head SHA and checks base..head, excluding the synthetic
checkout merge. Existing local invocation, skip and missing-base behavior stays intact. No
`.github/` edit or follow-up patch is needed. Existing workflow REUSE admission is unchanged.

The self-development spec already has 256 obligations. Amend its owning requirement, and the
existing CI requirement, without adding IDs or increasing capacity. The test's new Makefile read
is declared in `.corvint/shell-read-scopes.json`. Rollback is a revert of the implementation
commit: keep the prior trailer guard, restore the CI target and pre-push vet-only path.

## Before and after evidence

The same first identity regression is run on host `PC`, with the guard and Makefile copied from
the base above and the candidate fixture suite copied beside them. Command:

```sh
TMPDIR=/home/russell/lanes/m8-authguard.tmp \
  bash /home/russell/lanes/m8-authguard.tmp/baseline/script/check-ai-coauthor-trailers_test.sh
```

Before: exit 1, because the guard accepted the first planted `cLaUdE` author name: 0/1 required
refusals. This is the expected regression failure, not a passing baseline.

After: the implementation above, tested as its identical staged source before commit, passes
`script/check-ai-coauthor-trailers_test.sh` with exit 0. All 12 identity fixtures are refused on
each of the direct range, CI target and actual local pre-push paths: 36/36 required refusals.
The clean commit passes all three paths. The existing trailer assertions are unchanged.

Additional cases cover historical identities, existing/new refs, annotated tags, deletion,
multiple updates, stale tracking refs, another empty destination, unknown published history,
internal PR refs, replaced custom refs, mailmap, shallow clones and invalid/missing PR events.
The synthetic merge case passes only the actual clean PR head through the CI target.

A read-only enumeration of the pinned `origin/main` finds 11 historical prohibited identities.
A temporary bare mirror uses that exact base as main and borrows immutable local objects. Feeding
the new commit as a new ref to `script/pre-push-windows-vet.sh origin MIRROR` exits 0, with all
11 historical commits excluded. No repository push or history rewrite occurs.

## Verification and limits

- `corvint affected --base 84a977dc5721a840159c369f1ab4fce91ce2a2d1 --format json`: exit 0.
  It reports UNKNOWN: 328 selected units, 4 unknowns and 25 unresolved packages. Read-scope
  configuration requests FULL; the lane charter prohibits that expansion. Retain language/build
  variants, foreign shell invocation, changed shell scopes and a nonselectable shell unit as
  unknowns. This is not exhaustive verification.
- Whole `tools/ci-reuse-plan` package through `script/heavy.sh`, Go 1.27.4, `-count=1 -timeout 30m`:
  PASS, 39.352 seconds. This package is selected for the changed Makefile. Its `go vet` also passes
  through `script/heavy.sh`; no Go package source was edited.
- Both existing shell suites run in full. The vet-hook suite runs through `heavy.sh --admit-only`.
- `make spec-requirements`: PASS, generated files unchanged. `make spec-capacity-check BASE=...`:
  PASS, no new IDs. `make changed-checks BASE=...`: PASS.
- Staged citation repin: no moved anchors. Final required spec/citation/OpenSpec gates: PASS.
  OpenSpec retains 65 existing baseline findings in 29 subjects, with 0 new failures.
- Independent review, hosted CI and contributors observation: NOT_RUN; orchestrator/owner scope.
  OpenSpec commit rules OSC-V1-003/004: NOT_RUN without an opt-in base, as the check reports.
  Whole-repository tests, doc-gates and selection-baseline: NOT_RUN per lane charter.
  CEM, bind, seal, push, PR and task-store completion: NOT_PRODUCED per the explicit override.

## Friction

1. Host load queued the heavy runner without overrides. The CI package test waited 35 seconds,
   its vet waited 25 seconds, and the final vet-hook run queued 193 seconds. Reproduce by running
   the commands in the lane's `verify.txt` while load exceeds configured `HEAVY_MAX_LOAD=24`.
2. The first repin reported the unstaged Makefile was not checked. Stage it and rerun
   `script/check-line-citations.sh --repin`; the final staged repin reports no moved anchors.
3. The first new clean-push fixture lacked `origin/main`, so the existing vet fallback attempted
   `make windows-vet` in a fixture without that target. Reproduce by omitting the fixture's
   `refs/remotes/origin/main` setup. The fixture now establishes that base; assertions stay intact.
4. The implementation commit printed a read-only-filesystem refusal for
   `/home/russell/projects/corvint/.git/packed-refs.lock` during Git maintenance. The commit exited
   0 and its ref and requested author/committer identity read back correctly. Repro: a normal
   `git commit` in this sandbox when Git maintenance tries to lock the shared packed refs.

## Retained raw evidence

Artifacts stay in `/home/russell/lanes/m8-authguard-evidence/`. The hashes below bind the measured
outcomes; no raw log or historical identity dump is committed.

| Artifact | SHA-256 |
|---|---|
| `before-final.log` | `28f5eec58b6092dcd54e2e6abbf6eb82d753b122328bb882995845b08123e17a` |
| `after-final.log` | `b7635351cb03a43a7946683b7c94ea9bda407a5b74a41ddc5b7eaba6ea3e2b7c` |
| `historical-identities.tsv` | `b109b1b942e47d9caa6514f3f43763a5347a3b5127e49b74307b4af4690a9ca7` |
| `historical-main-push.log` | `8e947882fd7dda8f1b8c3a2a5ea6eb2c1345ebe143ca460806efd347b7b0e8b6` |
| `affected.json` | `2373fbba09bb16253adf082f3887c0721a41948fd73c4f4d86bb738ddd27b4ba` |
| `ci-reuse-plan-test.log` | `a31189d8daa3f4d31f2cf886df6707da83d73aec42915bd465462130061e1241` |
| `ci-reuse-plan-vet.log` | `8a5c5943724e38c33e0156fd779659817bf12d58d0645d002f5b32837a26e1d4` |
| `pre-push-vet-final.log` | `ac345948dc248f35c56b5ad85f751b4e93b8e118848815ad51118d75818c6d28` |
| `final-gates.log` | `2b9754f626e246c5eb294913f169259188acffab213559c89f09656d9e78044a` |
| `changed-checks.log` | `21ad1056d1a408109f5e92874fa2fced92524c8d8af70f31075c88db16357e6b` |
| `spec-requirements-final.log` | `574000ff552503675798e941126791a2bd9028c1ae10acfa0799c446b1be1e19` |
| `repin-final.log` | `dce4b7fe3eadc26103692787d6775a7701ed62b33f01ccc227f7182433b8ea7a` |
