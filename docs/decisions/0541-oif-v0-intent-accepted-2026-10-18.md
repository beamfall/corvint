# Decision 0541 — OIF-V0 intent-form registry intent accepted, 2026-10-18

Date: 2026-10-18. Status: accepted. Authority: owner (chat, 2026-10-18).

## Context

`docs/specs/ocm-intent-forms-v0.md` (`OIF-V0`) has been proposed since 2026-09-25. It is the
`ocm prepare --intent-form` registry through which ADR decisions, roadmap Acceptance tickets and,
since decision 0540, the `STA-V0` OpenSpec and Spec Kit forms enter OCM. `STA-V0` (accepted by
decision 0540) recorded owner acceptance of `OIF-V0` as one of its two remaining blockers; the
other is a passing `spec-toolchain-conformance` run (`STA-V0-011`).

On 2026-10-18 the owner wrote: "I accept the OIF-V0 intent-form registry intent; mark STA
delivered once STA-V0-011 passes."

## Decisions

### 1. The OIF-V0 intent is accepted

`OIF-V0`'s header reads `Intent status: accepted (owner, 2026-10-18, decision 0541)`. This accepts
the intent, the registry and the requirements `OIF-V0-001` to `OIF-V0-011` as written.

### 2. Delivery stays experimental

Acceptance of intent does not promote delivery. `OIF-V0` keeps `Delivery status: experimental`
and `OIF-V0-011` still forbids advertising the flag as delivered. Its own promotion evidence, one
real change dogfooded with a declared form and an independent review, remains `NOT_RUN`.

### 3. The open decisions stay open

`OIF-V0` open decisions 1, 2, 4 and 5 (tickets without Acceptance, single-item selection, LRF over
declared forms, and passing a form through the dogfood intents file) are not answered by this
acceptance. They remain recorded limits of the accepted intent. Decision 3 was already resolved.

### 4. STA-V0 promotion is conditional on STA-V0-011

The owner's acceptance removes `OIF-V0` acceptance from `STA-V0`'s blockers. `STA-V0` delivery
stays experimental until a `spec-toolchain-conformance` CI run passes (`STA-V0-011`, V1-1697). That
run is the only remaining promotion condition the owner named; the promotion itself is a separate
change that cites the passing run.
