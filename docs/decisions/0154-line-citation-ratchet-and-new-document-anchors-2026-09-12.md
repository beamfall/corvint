# Decision 0154 — Ratchet unpinned citations and pin every new document

Date: 2026-09-12. Status: accepted. Authority: repository owner call via delegated coordinator,
2026-09-12.

Content anchors detect the common drift shape that structural line checks cannot: a citation can
still land on a non-blank line after the intended content moves. Requiring anchors across the
legacy corpus immediately would invite mechanical repinning, which would assert content without
reading it and violate the existing drift rule. Leaving anchors wholly optional would let that
legacy debt grow.

The owner call adopts candidates (a) and (c). The gate commits an initial ceiling of 941, equal to
the measured number of checked citations without content anchors in the acceptance index. A count
at or below the ceiling passes; a count above it fails. Lowering the ceiling needs no further
decision. Raising it requires a visible contract change and owner review.

Every checked citation in a scanned document added after this decision must carry an `@<hex>`
content anchor. `script/line-citation-legacy-documents.txt` is the closed allowlist of scanned
documents present at acceptance. Both that allowlist and the document corpus are read from the Git
index, so the classification needs no base revision and an unstaged edit cannot affect it. The
allowlist must not acquire later documents: absence from it is the durable definition of new.

The fixture test proves the ratchet's failing polarity and both new-document polarities, including
that an unstaged allowlist edit does not grandfather a new document. Existing citations are not
bulk-repinned; they remain structural debt bounded by the ceiling and are pinned only after their
cited lines are read.

Rollback: remove DCG-V0-017 and DCG-V0-018, the closed allowlist, and the two enforcement branches.
DCG-V0-014's prepended-target rule remains, while other anchors return to opt-in behavior.
