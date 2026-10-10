# Decision 0475 — AngularJS injected enums and barrels (AMAP-V0-024..026) accepted

Date: 2026-10-09. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-09
("AMAP-V0-024..026 accepted"), then "accept the amended wording too" for the AMAP-V0-025/026 text
amended at 0939aeb7, and "confirmed, that reading is fine" that the round-9 and round-10
fail-closed lexer and import checks are covered by the accepted AMAP-V0-025 wording with no spec
text change. This covers native ticket V1-1061 (GitHub #705).

## Context

V1-1061 extends the AngularJS injected state-name reader of decision 0460 (AMAP-V0-021..023). The
proposed requirements are `AMAP-V0-024..026` in `docs/specs/application-map-v0.md`: an injected
TypeScript string enum read only when every member is a literal string, one level of `export ...
from` behind the registering file's import of the table, and one `di-constant` unknown naming why
the one in-scope registration did not resolve. AGENTS.md invariant 8 keeps acceptance human-owned.

## Decision

The owner accepts `AMAP-V0-024..026` as written. The requirement text is unchanged since 0939aeb7.
Review rounds 11 to 23 changed only the reader's fail-closed rules (an injected table stays
`UNKNOWN` / `not-read-whole` in more cases); reading them as covered by the accepted AMAP-V0-025
wording is the lane's, by the same reasoning the owner confirmed for rounds 9 and 10, and is not a
separate owner confirmation. The spec's trust-boundary section records, outside the
requirement text, the coordinator's scope decision that the reader assumes non-adversarial
application source: code that deliberately rebuilds the `angular` global's name or reaches it
through other host objects or string construction can still leave a stale literal, and is out of
scope. Acceptance evidence is the `TestAMAPV0024`, `TestAMAPV0025` and `TestAMAPV0026` tests in
`internal/appmap` (`docs/build-log/2026-10-09-v1-1061-appmap-enum-barrel.md`).

## Limits

This decision settles intent. Delivery stays experimental: the evidence is synthetic repositories
on Darwin, with no adopter-scale qualification; `make gate` is `NOT_RUN`. The fail-closed rules make
many ordinary closures read `UNKNOWN` (see the build log's Limits). V1-1061 is not completed by this
decision.

## Rollback

Revert this decision and the V1-1061 change: the injected-enum, re-export and `di-constant` reader
in `internal/appmap` (`stateconst.go`, `stateinject.go` and their tests), the AMAP-V0-024..026
text and the trust-boundary limit, then regenerate `docs/specs/REQUIREMENTS.tsv`. Injected tables
then again resolve only as AMAP-V0-021..023 describe. No store or wire format changes beyond the
additive `di-constant` unknown kind.
