# Propose the `corvint-plugin/1` envelope (V1-4819)

Date: 2026-11-11. Lane: relhyg. Status of the result: proposed / experimental; acceptance stays
with the owner. No owner acceptance row was written.

## Decision

Write `docs/specs/plugin-envelope-v0.md` (prefix `CPE-V0`, `CPE-V0-001..017`) as the envelope that
decision 0591 items 1 and 4 and `CMS-V0-010` name but no spec defined. The prefix was checked free
(`make requirement-ids-free PREFIX=CPE-V0 RANGE=001..040`: free at origin/main `b55635262788`; no
`CPE-` row in `REQUIREMENTS.tsv` or `INDEX.json`).

Design choices, each a proposal for the owner:

- Generalize `corvint-runner/1` instead of inventing a second protocol. Its framing (strict
  newline-delimited JSON over stdio), `hello` negotiation, one process per run, limits, digest
  pinning outside the repository and fail-to-FULL rule (`RSP-V0-026` to `RSP-V0-031`, `RSP-V0-047`)
  become the envelope's. A profile owns only its methods, shapes and conservative fallback.
  `RSP-V0-032` already says the runner protocol becomes this envelope's profile with its IDs kept.
- `hello` carries `profile: "NAME/MAJOR"`, so a plugin serves exactly one profile per process and a
  profile versions apart from the envelope.
- One outstanding request at a time (`CPE-V0-003`): failure attribution stays exact; batching is per
  method, which is what `CMS-V0-010`'s "batched calls served by one process" needs.
- Every failure ends the session and discards all its results; the consumer reports UNKNOWN or
  abstains for every item the plugin was to answer (`CPE-V0-010`, `CPE-V0-011`). A profile with no
  conservative fallback cannot be admitted.
- 23 failure vectors, each with the core code and the consumer observation, are the acceptance tests
  for the future host and conformance kit (V1-4817). Vector names are plain numbers, because a
  `PREFIX-XX-NN` cell in a spec table can be read as a requirement ID.
- `repository-mutated` is detected after the session, not prevented; network use is not observable.
  Stronger containment is recorded as NOT_PROVIDED instead of implied.

## Evidence

- `make spec-requirements`: regenerated `REQUIREMENTS.tsv` (17 `CPE-V0` rows, mode `unrecorded`),
  `INDEX.json` and the README row; a second run printed `no changes`.
- Implementation and tests: NOT_PRODUCED. No code implements the envelope; the spec's acceptance
  table says so.

## Open for the owner

Acceptance of the spec, and the four questions in its last section: the single-request rule, one
plugin store versus one per profile, whether `test-provider` becomes a profile before V1-4824, and
the promotion and kill criteria.
