## 2026-10-04 V1-0750: corvint-tasks canonicalizes mutation payloads at the CLI boundary

Human-owned intent: native ticket V1-0750 (BUG, P3). With an otherwise valid `ticket create`
payload, corvint-tasks refused (a) pretty-printed JSON with `byte 1: expected object key`,
(b) default `json.dumps` separators with `insignificant whitespace is not canonical`, (c) reversed
keys with `non-canonical encoding (key order, whitespace or escape form)`, and (d) unsorted
labels with `array is not canonical-byte sorted at index 1`. Every agent rebuilt the same
sort-and-compact recipe, and a saved memory existed only to carry it.

Requirement IDs: none. No spec in `docs/specs` owns the mutation CLI yet; the task-store contract
recovery (V1-0310) is still open, as V1-0329 recorded. This entry records the behavior until that
contract exists. The §3.3 wire rule is unchanged: envelopes, stored records, receipts and journal
bytes stay canonical, and `mutation.Decode` still parses them strictly.

Decision: canonicalize only at the CLI input boundary, before the envelope is built.

- `wire.ParseInput` reads a caller's payload under the same value rules as `wire.Parse`
  (decode bounds, duplicate keys refused, no JSON numbers, valid UTF-8, no lone surrogates
  or hostile code points, no BOM). It also admits JSON whitespace between tokens, any key
  order, any valid escape form and an optional final LF. It never reorders an array.
  `readPayload` uses it for ticket mutations and release mutations. The release path
  previously compacted whitespace only.
- `mutation.CanonicalPayload` runs the operation's own closed-table decoder over a
  `wire.NewSetSortingReader`. Every array that decoder reads as a set (`Array` with
  `semantic` false: labels, requirementRefs, capabilities, requiredGates, touchPaths,
  resources, evidence, scope) is sorted in place by canonical bytes. A duplicate element
  still refuses, with its path. Ordered arrays (`acceptanceCriteria`, `dependencies`) are not
  touched, so set knowledge comes from the one schema table that already enforces it.
  No mutation payload has a set nested inside a set element, so in-place sorting is final,
  and the strict decode in `store.Mutate` re-checks the result.
- Strict `wire.Parse` refusals for stored documents now name the defect. Whitespace at a
  structural position reports whitespace rather than `expected object key`. A re-encode
  mismatch reports either the JSON pointer of the first object with unsorted keys or the
  canonical escape rule. An unsorted set reports its path and says to sort its elements by
  canonical bytes.

Failure modes considered: a pretty-printed payload whose request digest differed from its
canonical form would break idempotent retries. The tests prove equality through replay. Sorting
an ordered array would change meaning. Only decoder-declared sets are sorted, and a test proves
that sorting `acceptanceCriteria` gives a different request. Silently deduplicating a set would
hide a caller error, so duplicates refuse. A widened release payload parse cannot weaken the
release wire rule, because the store still decodes the composed envelope strictly. Release set
arrays are not sorted at the boundary; their refusal now names the path and the fix.

Evidence: `TestV10750_MutationPayloadsAreCanonicalizedBeforeTheDigest` covers six create
variants plus one refine variant: pretty-printed with newlines, folded to spaces, default
separators, reversed keys, unsorted set arrays, and all combined. In each case a canonical
retry under the same request ID and issuedAt replays, so the digests are equal. All seven
fail against the previous `mutate.go`.
`TestV10750_OrderedArraysKeepTheirOrderAndSetDuplicatesRefuse` covers recorded criterion
order, the non-replay of a sorted-criteria retry, and the `/payload/labels` duplicate refusal.
`internal/tasks/wire/input_test.go` covers `ParseInput` canonicalization and its retained value
rules, the strict-parse messages (no `expected object key` for whitespace), and set-only
sorting with argv order preserved.

Rollback: revert the change. CLI payloads again must be canonical bytes, and refusals return to
the previous messages. Stored data, receipts and digests need no migration in either direction.
