# Post-merge /2 contract registration on current main

Date: 2026-10-04. Owning ticket: V1-0542 (#395), still OPEN.

## Decision

Register the owner-accepted `/2` contract seed (`postmerge-runtime-v2.md`, its frozen
`conformance/postmerge-runtime-v2/` data and the 2026-10-03 decision entry) unchanged on public
main `4b10a02144faed4a77b36eba4b0d1cbc315706d3`, instead of at its original seed base
`dd8cc0ca918a80faaa041c98a783de11550b2bcc`. The seed commit
`823eff14982c669cece9f304d34dc818e6dbe873` never completed its CEM citation, check or seal, so
it could not serve as an implementation base; this change is that ordinary bound seed.

## Revalidation at the new base

- All eleven producer source files pinned by `schema-source-inventory.json` have the same SHA-256
  at `4b10a021` as at `dd8cc0ca`. No `/0` or `/1` spec, `internal/postmergeworkflow`,
  `internal/postmergeconnector` or `internal/intake` file changed between the two bases.
- The seed's `INDEX.json` edit re-escaped eight unrelated non-ASCII titles and intents. This change
  inserts only the new `postmerge-runtime-v2.md` entry; the other rows keep their base bytes.
  `REQUIREMENTS.tsv` is regenerated from the index.
- Conformance files and the 2026-10-03 entry are byte-identical to the seed. The spec header lacked
  the `Intent status:`/`Delivery status:` lines and the `## Agent digest` block that
  `internal/specindex` requires, and its INDEX claim exceeded 160 characters. This change adds that
  metadata and renames the `Authority and agent digest` heading to `Authority`. Requirement,
  profile, trust and acceptance text is unchanged.

## Positive replay remains blocked

No `/2` runtime code ships here. A deterministic positive replay under PMR-V2-009/010 needs
producer and stage evidence that does not exist on main:

- #389 `corvint delta` is OPEN and absent, so the `/1` refusal `actual-delta-unavailable` still holds.
- #394 has no producer decision on main (`internal/testacceptance` has no decision emitter). The
  prepared patch `da35448e…` at `e5d777cb…` is an unapplied proposal.
- The process-proof profile admits only an exact-tuple-qualified Linux procfs host. This delivery
  ran on Darwin, so process qualification was not attempted.
- Real author, scope, validation, metrics and corpus stages, two fresh complete historical runs
  and local or CI qualification all remain `NOT_PRODUCED`.

If `/2` code were built before those producers exist, it would either always refuse, duplicating
`/1`, or compare attachments that nothing verified. PMR-V2-004 forbids the second. #388 is the
consumer umbrella and does not establish producer success.

## Rollback

Revert this commit and its evidence commits. `/0`, `/1`, their archives and the native store are
untouched.
