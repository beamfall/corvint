# AUTOMATION and GLOSSARY citations scanned and pinned (DCG-V0-042)

Date: 2026-10-20. Tickets: V1-0997, V1-1034, V1-0225.

Decision: the line-citation gate now scans `docs/AUTOMATION.md` and `docs/GLOSSARY.md`, but neither
joins the legacy allowlist. When these files were first scanned, 153 citations failed: wrong lines, blank lines, past-end spans and
changed pins. No citation was pinned mechanically. Each one was handled in one of three ways:

- relocated through history: the document line's own commit was mapped to the index with an equal-line diff;
- re-read and moved to the line that now states the cited claim;
- for the one file never tracked (`docs/CORVINT-TECHNOLOGY-REVIEW-2026-08-29.md`), turned into prose.

Both documents now carry only anchored citations. The allowlist exception (decision 0176 style) and
the unpinned inventory are therefore unused.

Limit: `docs/specs/README.md` rows move often. The GLOSSARY `NOT_RUN` citation of README rows will
fail on unrelated row inserts. DCG-V0-040 relocation reports such a move and `--repin` fixes it.
