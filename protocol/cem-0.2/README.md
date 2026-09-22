# CEM 0.2 candidate portable vectors

This Apache-2.0 packet supplements the existing CEM contracts. It changes no wire and freezes no
new minimum profile. V1-0013 still depends on verified daily-loop evidence from V1-0010 and owner
acceptance. The historical `interop/cem-0.1/manifest.json` and its 32-case external matrix are unchanged.

`manifest.json` pins 21 raw artifacts, one SHA-1 base and six complete target commits. Each target
contains `.corvint/change.cem.json` byte-for-byte equal to its declared `map`. The manifest itself
is SHA-256 `9389102480c910ddb1366d385702bcf44a72dadbbece9e68bce683420f64983a`, pinned separately
by both test consumers. Expected identities and drift records were calculated from the published
identity formulas, literal fixture bytes and Git objects, without using a Corvint producer or
capturing verifier output. Consumers must check the manifest and every artifact before execution.

To reconstruct a case, initialize an empty SHA-1 Git repository on `main`, copy the three files in
`repository/base`, and commit with `baseMessage`. Use the manifest's author as both author and
committer and its timestamp for both dates. Require the resulting `baseRevision`. Apply the case's
exact patch, copy its `map` to `.corvint/change.cem.json`, stage all changes, and commit with the
case name and the same identity/date. Require the exact `targetRevision`. Disable ambient Git
configuration, attributes, filters, hooks and line-ending conversion. These fixtures contain no
executable source or Git configuration.

| Case | Required observation |
|---|---|
| stable | Both evidence records retain their original blob and byte span. |
| relocated | `a.txt` has one match at bytes 7..13; `b.txt` remains stable. |
| stale | `a.txt` has no match; verification rejects and retains both records. |
| ambiguous | `a.txt` has two matches; verification rejects without selecting one. |
| deleted | `a.txt` is absent; verification rejects with null target blob/span. |
| unknown | The changed hunk retains `unknown` / `no-evidence` with no evidence or basis; structural validity does not satisfy a zero-unknown policy. |

Every nonempty evidence array is in descending ID order. The manifest's expected drift array is
in ascending ID order and pins every ID, path, status, target blob and target span. Mixed-status
cases prevent a consumer from satisfying the packet by repeating one status for every record.
The same evidence/relation pair may support multiple hunks; uniqueness is scoped to each hunk's
`basis` array under the existing supported-hunk contract. The mixed cases exercise this reuse;
the historical consumer test separately rejects repetition within one hunk. This interpretation
clarifies the frozen algorithm packet's shorthand without changing its bytes or wire rules.

Each `legacyMap` is a separately pinned `cem/0.1` exact-patch artifact. A legacy consumer MUST reject
the 0.2 map, then reach the declared result for its 0.1 artifact and explicit patch. Rejection codes
are intentionally unspecified. These paired artifacts demonstrate historical readability of the
shared evidence/hunk contract; removing `excludedPath` is **not** a safe migration of canonical
assurance. A 0.1 result never inherits independent canonical derivation or target-artifact binding.

The native `TestPortableCanonicalVectors` also requires independent base/target authority, refuses
an external patch for 0.2, rejects a copied map differing only in whitespace from the committed
sidecar, and preserves unknown-policy failure. The separate standard-library/Git-only consumer's
`TestPortableProfileCompatibility` exercises the same raw packet through its existing process ABI.
It imports no native parser or verifier.

This packet supplies reference portability evidence, not independently authored 0.2 interoperability,
sealed anti-copying qualification, a SHA-256 repository matrix, OCM/frontier compatibility, or a
product-value result. The independent 0.2 reader and producer path remain unbuilt. Existing 0.1
SHA-256 and hostile-input vectors remain in their original suite. OCM and frontier retain their
existing experimental status and licensing boundaries; no implementation or specification is
relicensed by this packet.
