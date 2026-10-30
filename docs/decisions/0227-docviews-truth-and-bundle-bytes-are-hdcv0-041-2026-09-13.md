# Decision 0227 — docviews truth and bundle bytes are HDCV0-041 canonical JSON

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`internal/docviews` encoded the truth corpus and the bundle with `encoding/json` and
`SetEscapeHTML(false)`. That encoder writes members in Go declaration order and escapes U+2028 and
U+2029 as `\u2028` and `\u2029`. The `CATN` profile said only "lexical member declaration" and
named no escape set, so two conforming encoders could produce different truth digests for the same
corpus. The HDC plan re-binding in the same package already hashed `doccompiler.CanonicalJSON`
(`HDCV0-041`) bytes.

The call:

(a) The truth-digest input and the canonical bundle bytes are exactly `HDCV0-041` canonical JSON:
members sorted by UTF-8 bytes, the `HDCV0-041` escape set (no HTML escaping; U+2028 and U+2029
literal), and one trailing LF. The truth digest stays
`SHA-256("corvint-cross-audience-truth/0\0" || truth bytes without their one trailing LF)`; the bundle
digest hashes the bytes with the LF. This adds `CATN-V0-015` and amends the profile's closed-model
text.

(b) The bytes come from the single `doccompiler` emitter. `doccompiler` exports
`CanonicalJSONUnbounded`, the same encoder without the 8 MiB bound. `CanonicalJSON` is now that
function plus the bound. The unbounded entry exists because `HDCV0-041` bounds a plan or receipt,
while `CATN-V0-012` bounds a bundle at 64 MiB and `Verify` must still measure an oversized bundle and
report `LIMIT_EXCEEDED`. The docviews-private `canonicalJSON` encoder is removed.

(c) Clean break, no compatibility reader. The two encoders differ only on U+2028 and U+2029 in
strings: member order already matched, and `<`, `>`, `&`, control characters, and invalid UTF-8
encoded identically. A truth or bundle digest computed before this change does not verify when its
content contains U+2028 or U+2029. No bundle has been released. The existing fixed vectors contain
neither code point and are unchanged. A new vector with `<`, `&`, and U+2028 is pinned.

(d) `proposedNavDigest` (`internal/doccompiler/plan.go`) is not migrated. It is not an HDC plan,
receipt, or CATN byte string. `internal/doccompiler/build.go` compares it (`nav-candidate-mismatch`)
with the `nav_sha256` that the MkDocs authority probe computes in Python
(`json.dumps(sort_keys=True, separators=(",",":"))`, default `ensure_ascii`, no trailing LF). The
digest's contract is equality with that probe. `HDCV0-041` bytes include a trailing LF and literal
non-ASCII, so they would never match. The existing Go `json.Marshal` spelling also disagrees with the
probe for non-ASCII, `<`, `>`, or `&` titles. That defect is filed in `docs/agent-memory/bugs.md`
and is not fixed here.

Consequences: `TestTruthDigestAndBundleHashHDCV0041BytesWithoutHTMLOrLineSeparatorEscapes`
(`internal/docviews/compile_test.go`) pins the new vector and would fail under the removed encoder,
which wrote `\u2028`.

Rollback: revert the commit. That restores the docviews-private encoder, the single-function
`CanonicalJSON` body, and the pre-amendment `CATN` text. Digests over content containing U+2028 or
U+2029 revert with it.
