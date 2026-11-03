# CI admits a well-formed binary consumer (V1-4361)

Date: 2026-11-03. Ticket: V1-4361. Requirements: AFP-V0-028 (amended), AFP-V0-982, STR-V0-750.

## Problem

After the ci-forward-bar verdict was MET (V1-4360), PR and merge-queue runs that touched a Go
package reaching `cmd/corvint-web-flows` still recorded FULL. The plan named
`UNSELECTED_BINARY_CONSUMER tools/web-flows/test/e2e.mjs` (AFP-V0-762). The deciding driver's
`unknownNarrowing` did not attribute it, and `corvint test --profile ci` kept it outside the
AFP-V0-028 allowlist. Because AFP-V0-098 narrows only when the driver's selection covers the
shadow's, either side alone forced every shard FULL.

FULL bought no coverage. No `.github` workflow runs `script/web-flows-gate`, so a FULL Go run never
executed `e2e.mjs` or `navigation-e2e.mjs`.

## Decision

The 2026-11-02 aff-w42 entry took option (b): admit the consumer on the local profile only. This
change takes option (a) for CI, which the owner approved in session on 2026-11-03 ("Yes, fix it"):

- `gateselect.BinaryConsumer` is the one parser both sides share. It is AFP-V0-982's third named
  rule, beside the deleted-source rule and `UnobservedModule`.
- The driver admits a well-formed consumer.
- `corvint test` names it as a still-required check on every profile.
- A malformed detail (absolute, escaping, unclean, empty, backslash or control character) still
  forces FULL on both sides.
- `AllowedUnknown` is unchanged.

## Evidence

Tests were run through `script/heavy.sh` on WSL at ea09e3db5 plus this diff:

- `./internal/gateselect -run CINV0860`: three new cases ran. A consumer beside a frontier narrows;
  an escaping consumer and a backslashed consumer fall back.
- `./internal/sessiontest -run 'STRV0750|AFPV0982|STRV0003|STRV0064|STRV0012'`. The parity table
  now requires the driver to admit a well-formed consumer that the allowlist does not.
- Both packages were also run whole: rc=0.

Still open: a merge-queue run recording NARROWED for such a change. That is acceptance criterion 2
of V1-4361, and it is observed only after this change merges and the pinned tools move to it.

## Rollback

Revert the commit. The driver and the `ci` profile then force FULL on a consumer again, together.
