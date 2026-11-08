# Selwide FULL completeness repair

The owner's self-review repair names three soundness findings under V1-5018 /
AFP-V0-9012: symlink-only packages omitted from FULL discovery, unsharded execution
using a stale discovery after source drift, and explicit FULL receipts proving only
their own argv. This repair preserves exported fields, diagnostics, exit codes,
goldens and frozen outputs. V1-4941 / AFP-V0-9013 and V1-4710 / AFP-V0-9014 are unchanged.

## Decision and closed classes

Execution discovery uses the existing bounded source traversal and module ownership,
with an explicit file-kind policy. The allowlist is regular files and symlinks that
resolve to regular files inside the repository. Broken links, external targets,
links to directories and unknown kinds (tested with a FIFO) refuse discovery.
Graph reads retain their existing conservative link handling; their regular-file
walk is unchanged. Existing strict output declarations and walk bounds remain.
The regression puts the only failing test behind a symlink in an otherwise empty
package, with its target under testdata; FULL must execute that failure.

Every execution binds the enumerated universe to HEAD and its tree, checks clean
source after enumeration, and checks it again immediately before executing. This
uses the existing source-drift refusal for one shard, multiple shards, narrowed
selection and FULL fallback. The drift regression adds a failing package after
listing and exercises both one and four shards; an unrecognized checkout status
also cannot certify independent receipt discovery.

Explicit FULL container receipts require the frozen test prefix, a sorted valid
package list, and exact equality with a separate discovery from the trusted driver
in the container's requested target checkout. Independent checks require the target,
tree and clean status before and after discovery. Receipt argv/outcomes never supply
the expected universe. The verifier has its own cold scratch paths on /work because
the execution runtime is removed at driver exit. Legacy wildcard receipts retain
their frozen validation. Unknown package argument forms, unavailable discovery and
mismatched snapshots fail closed. Tests reject a successful receipt omitting b from
both argv and outcomes in a real a/b repository, accept its complete counterpart,
and reject unknown forms and binding drift.

Acceptance witnesses: `TestExecutionSourceKinds_AFPV09012`,
`TestSymlinkOnlyFullExecution_AFPV09012`, `TestShardedSourceDrift_AFPV0022`,
`TestContainerRunResult`, and `TestDiscoverRunUniverseBinding_AFPV09012`.

## Limits and rollback

Raw logs stay in `~/lanes/selwide-evidence/` with prefix `self-review-8-`.
The affected plan selected all tested packages, retained UNKNOWN with 71 unknowns,
and warned of mutable worktree input. No independent review or live Docker
qualification is claimed: those remain host-owned. Repository-wide gates,
publication, Tasks writes, pre-change recapture and sealing are prohibited in this
round. Native ticket completion remains pending. Rollback reverts this repair's
source/spec/test commit; it restores known incomplete FULL behavior and cannot be
promoted as qualified. No new wire contract or authority is introduced.

Friction: an initial whole-package invocation was refused for missing
HEAVY_WHOLE_REASON. The rerun supplies the owner's whole-package requirement and
the retained affected UNKNOWN; no semaphore or selection override was used.
Citation repin retains 180 unmatched basename citations. Linux runs provide no
Darwin-only implementation evidence. Final command results and log digests follow.

The first affected-package suite exposed cancellation tests whose injected context
matches the existing walk function's stack frame. Moving the walker body changed
that frame. The final repair keeps the body in its original function and adds an
optional execution-only argument; no existing test assertion changed. The focused
cancellation and source-kind checks passed after this correction. An initial PR
suite also reported a confined campaign selected-set miss; the exact existing
campaign test passed on focused recheck without a production or test change. The
failed run remains evidence, not a passing gate.

## Focused verification results

Go 1.27.3 verified. Final whole-package commands use `script/heavy.sh go test
-count=1 -timeout 30m` with each of `./internal/liveverify/affected`,
`./internal/liveverify/affected/golang`, `./tools/corvint-pr-tests`, and
`./tools/ci-forward-row` separately. Final package results all PASS.
`make -s spec-requirements-check line-citations-check` passed; citation repin and
`make -s line-citations-check` were repeated after the final source placement fix.
No exhaustive gate or independent review was run.

- `self-review-8-affected.log`: exit 0; UNKNOWN, selected=431, excluded=468, unknown=71; self-binary-stale
  SHA-256 `f77755c92071a9dcf6e804123a082f59cfe228c8caf9abf9888d27d6e14fe2da`; 191355 bytes.
- `self-review-8-focused.log`: ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	32.375s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=34.1s rc=0
  SHA-256 `aa35b2a2b4db9e14040967a49368d173fd44a61f9e9f6590ba663209ba0568d3`; 598 bytes.
- `self-review-8-whole-affected.log`: heavy.sh: REFUSED whole-reason-missing (exit 2)
  SHA-256 `c9d9e41b2a2afb244831fe25a377b58a47b4168ba625396c46a46136f42d5671`; 1740 bytes.
- `self-review-8-whole-affected-final.log`: FAIL	github.com/Beamfall/corvint/internal/liveverify/affected	105.760s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=109.0s rc=1
  SHA-256 `93ec1eef1d4d75f3cc8082a1c40f6c805c01517ba0842afe5344edf80a945566`; 4959 bytes.
- `self-review-8-focused-cancellation.log`: ok  	github.com/Beamfall/corvint/internal/liveverify/affected	0.092s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=3.0s rc=0
  SHA-256 `211f3976986fba73adc7c52e27ec1abd83dcee56f6ec99f165cf6638f6b780e6`; 795 bytes.
- `self-review-8-confine-recheck.log`: ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	12.324s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=14.1s rc=0
  SHA-256 `f74e0b8c6e942e39048d1510e5ebc4b036c97ffe380d1cebb16fe435e1c76817`; 598 bytes.
- `self-review-8-whole-pr.log`: FAIL	github.com/Beamfall/corvint/tools/corvint-pr-tests	159.788s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=161.0s rc=1
  SHA-256 `5d9454ec94b5b16a9ce7dda0cc6b8134dc68fd3afe7aa2beb6327fb7d2f73694`; 8007 bytes.
- `self-review-8-whole-affected-repaired.log`: ok  	github.com/Beamfall/corvint/internal/liveverify/affected	106.559s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=106.9s rc=0
  SHA-256 `167bcd7bc8bc7378c3285a8cf2a77cfef8116c26517b344b0a5a28af447521e2`; 799 bytes.
- `self-review-8-whole-golang-final.log`: ok  	github.com/Beamfall/corvint/internal/liveverify/affected/golang	8.354s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=9.9s rc=0
  SHA-256 `3ebdfd4e1f115a4d2c7259a8186a9f02d096ffb0a072ee4b5cd704e7c56fbb59`; 609 bytes.
- `self-review-8-whole-pr-final.log`: ok  	github.com/Beamfall/corvint/tools/corvint-pr-tests	180.025s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=180.3s rc=0
  SHA-256 `52784f1b032a322d3361109772d21b7904670c71a6ed49434574f5f2450241ad`; 600 bytes.
- `self-review-8-whole-workflow-final.log`: ok  	github.com/Beamfall/corvint/tools/ci-forward-row	51.043s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=52.1s rc=0
  SHA-256 `228436dc980639c2ad2754199a852a3cef3dc802f714312b7f1f5a18134b92d8`; 596 bytes.
- `self-review-8-spec.log`: ok  	github.com/Beamfall/corvint/internal/specindex	0.943s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=2.2s rc=0; ok  	github.com/Beamfall/corvint/internal/lrfrepo	0.077s; heavy.sh: done queue=0.0s (load=0s slot=0.0s) run=0.5s rc=0
  SHA-256 `d2f0b2b75a6cbc6d9fd27431cbbf7677ce331ff658014e91b96b0252d77a0b92`; 1717 bytes.
- `self-review-8-repin-final.log`: exit 0; line citations: --repin found no citation whose content moved unchanged; line citations: note: skipped 180 citation(s) whose basename is not tracked at repository root (--verbose lists them)
  SHA-256 `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`; 190 bytes.
- `self-review-8-citations-final.log`: exit 0; line citations: note: skipped 180 citation(s) whose basename is not tracked at repository root (--verbose lists them)
  SHA-256 `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba`; 118 bytes.
