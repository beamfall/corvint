<!-- SPDX-License-Identifier: Apache-2.0 -->
# mbrescue merge-forward to cb403735

The owner requested merging mainline `cb403735903b1e54a8469373489960c0b2b89d99`
into lane head `e4337ee6f2fda656b0f88986c08ab208acdc8ccf`, previously based on
`538e823f4a1d220a3fb5b12c0529248f18924823`. Merge `3bde27236` preserves both parents.
The conflicted merge-forward route in docs/DOGFOOD.md governs the whole-range
rebind and reseal against the merged mainline, overriding the ordinary incremental
reseal base. Existing bind and seal commits remain in ancestry.

Conflicted paths and resolutions:

- `.corvint/shell-foreign-callers.json`: retain mainline's github-guard.sh mention
  and explanation for pin-lag.sh, plus the lane's callers, mentions and reasons
  elsewhere. Compact string arrays retain the registry's existing bounded size
  (65,059 bytes of the 65,536-byte limit); no caller is dropped.
- `docs/specs/REQUIREMENTS.tsv`: take mainline at merge, then regenerate from the
  merged staged specs using `make spec-requirements`. That owning target also
  regenerates `docs/specs/INDEX.json`; both generated outputs start from mainline.
- `docs/specs/affected-plan-selection-v0.md`: keep lane AFP-V0-920 and its intent,
  acceptance and rollback alongside mainline AFP-V0-2700..2702 and 2720..2726.
  Mainline does not define AFP-V0-920. No requirement renumbering is needed.
- `internal/companionrelease/exported_current_test.go`: retain the lane's move
  into `internal/companionrelease/currentproof/currentproof_test.go` and public
  proof functions in current_proof.go. Move mainline's RequireLiveTier guard and
  testsupport import to the relocated archive-build test, keeping both behaviours.

The spec README merged automatically and retains both sides' entries. The
Makefile has no separate index target: spec-requirements owns all three spec
projections. No generated Go file changed. Focused verification is limited by
owner instruction to companionrelease and currentproof (one heavy.sh command
per package), spec-requirements-check and line-citations-check. The affected plan
is retained outside the tree; its wider selection is NOT_RUN here because the
host verifier and batch CI own subsequent coverage. No doc-gates, corvint test,
gate-affected, task-store writes or publication is part of this round.

The external handback records exact bind/seal commits, check return codes,
retained NOT_PRODUCED reasons and review limits. No broader outcome is asserted.

## Focused verification found parent VCS discovery

The required TMPDIR is below `/home/russell/lanes`, whose read-only `.git`
marker makes Go discover a VCS root although Git cannot use it. The relocated
currentproof package fails both its smoke generator and archive build with
`error obtaining VCS status: exit status 128`. Its first-statement live-tier
check runs by default, as mainline requires; no skip variable was set.

The proof's positive and negative archive builds, and the queue-policy smoke
fixture generator, now pass `-buildvcs=false`, as runGoBuild already does under
PUB-V0-003. The exported source commit/tree and archive digest remain the proof's
identity. This fixes scratch-location dependence without changing assertions,
export contents or the diagnostic-dependency negative control. The existing
currentproof tests provide the failing regression under the mandated TMPDIR.
The two affected packages are rerun; broader verification stays with the host.
