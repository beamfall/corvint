# Decision 0265 — LRF requirement-ID redaction: amend the frozen algorithm text, keep the code

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The native LRF tokenizer (`internal/lrf/terms.go`) redacts requirement-ID-shaped tokens in every
term source after content-ID redaction, as the original Python `src/context_corvint_lrf.py` did. The
frozen identifier algorithm in `docs/specs/lexical-relevance-floor-v0.md` step 4 listed only
content IDs, so the text and the implementation disagreed.

No `conformance/lrf-v0` vector pins either reading: the corpus passes with the redaction removed.
`LRF-V0-002` already states that a requirement ID MUST NOT satisfy proximity alone, and removing
the redaction would let the letters of an added `WIDGET-001` match a `widget` evidence span, which
weakens the floor. The call is to keep the code and amend step 4 to describe it exactly: at the
input start or after a byte outside `[A-Za-z0-9_]` (non-ASCII included), take the longest match of
the requirement-ID grammar `[A-Z][A-Z0-9-]{2,31}-[0-9]{3}` from `docs/lrf-0.schema.json`; replace
it with one space only when the input ends after it or the next byte is outside `[A-Za-z0-9_]`,
otherwise move one byte on without trying a shorter match. Pinned by
`TestRequirementIDInAddedBytesSuppliesNoTerm`. No requirement IDs added; the corpus manifest and
registration hashes are unchanged.
