# Decision 0176 — The line-citation gate scans `docs/decisions/*.md`

Date: 2026-09-13. Status: accepted, delegated coordinator call. Authority: repository owner call
via delegated coordinator, 2026-09-13, on the `fixes.md` entry "`check-line-citations.sh` never
scans `docs/decisions/*.md`".

## Outcome

`scanned_doc()` in `script/check-line-citations.sh` now matches `docs/decisions/[^/]+\.md`. Every
decision record is a checked document: its `path:line` citations are resolved against the Git
index, and DCG-V0-017/018 (content anchors, the closed legacy allowlist) apply to it exactly as to
any other scanned document.

## Historical citations are converted, not exempted

A citation a decision explicitly frames as historical — reviewed once against a named commit, not
asserted to hold at HEAD — must not fail structurally when its target drifts. Decision 0105 is the
concrete case: its `OACS-V0-008` bullet says the script was "reviewed clause by clause against its
script at `3c41cad3`". Its `Makefile` citation at lines 129-136 is converted to the prose form
`` `Makefile@3c41cad3` as reviewed above (lines may have moved since) ``, dropping the `:N` line
suffix so the gate's citation-token regex (every alternative requires a trailing `:digit`) does not
match it at all; a bare `path@hash` with no line number is inert prose to the gate, by inspection of
the regex. No new exemption mechanism is introduced for this case: it needed none.

## Every other stale citation the scan surfaced

Widening the scanned set surfaced 40 pre-existing unpinned `path:line` citations across 16
other decision files (0004, 0005, 0006, 0038, 0039, 0051, 0082, 0093, 0094, 0098, 0104, 0132, 0136,
0142, 0161, 0171), none of them off-limits. Each was read against its cited lines and resolved
without any new mechanism:

- **Repinned in place** when the cited content is unchanged: append `@<hex>` from
  `script/check-line-citations.sh --hash`.
- **Relocated and repinned** when the content moved but is still findable by its own text (for
  example decision 0093's `go-production-kernel-migration-v0.md` citation moved from lines 990-1011
  to lines 1052-1071; decision 0098's `cem.go` citation moved from line 985 to line 986).
- **Converted to prose** (drop the `:N` suffix) when the cited file is no longer tracked in the Git
  index at all (decision 0004's `context_corvint_trace.py`, decision 0005's three
  `context_corvint.py` citations, decision 0006's `context_corvint_index.py`, decision 0039's
  `context_corvint_index.py` and `conformance/perf-v0/manifest.go`).

None of these needed the legacy allowlist or the ceiling: each had a findable, readable target.

## Decision 0158: legacy-listed as a one-time scope-widening admission

Decision 0158 is off-limits to this change (a different in-flight lane owns it) and cannot be
repinned. It carries 30 unpinned `path:line` citations, plus 3 of those whose cited line has since
shifted by one row (`internal/worksource/git.go` line 191, `internal/dashboard/roadmap/atm.go` line
131, `internal/localcompletion/storage.go` line 429, now landing on a blank or bracket-only line).

Decision 0154 closes `script/line-citation-legacy-documents.txt` against new documents: "The
allowlist must not acquire later documents: absence from it is the durable definition of new."
Read literally against a document's authorship date, admitting 0158 (numbered and dated after
0154) would violate that rule. But 0154's allowlist was computed once, "the closed allowlist of
scanned documents present at acceptance" — over the scanned-document set as `scanned_doc()` defined
it that day, which excluded `docs/decisions` entirely. Decision 0158 was never a scanned document
at 0154's acceptance; it enters scanning for the first time today, by this decision, which changes
`scanned_doc()` itself rather than adding an entry against an unchanged definition. That is a
distinct policy lever from the one 0154 closed, and closing it was never stated. This decision
exercises it, as the same kind of one-time bulk admission 0154 itself performed for the
pre-existing spec/agent-memory corpus when DCG-V0-017/018 were adopted (ceiling seeded at "the
measured number of checked citations without content anchors in the acceptance index").

Accordingly: `docs/decisions/0158-local-excludes-remain-status-policy-2026-09-12.md` is added to
`script/line-citation-legacy-documents.txt`, admitting its 30 pre-existing unpinned citations as of
this scope widening. This is the narrow, single-document exception the scope change requires, not
a reopening of the corpus generally: no other decision file is added, because every other stale
citation this scan surfaced was resolved above without it. The 3 citations whose line has drifted
are further named in `script/line-citation-drift-waivers.txt` (existing DCG-V0-020 mechanism, see
below), because legacy-listing only exempts the missing-anchor check, not the unconditional
blank/bracket-only-line check.

**Ceiling.** The total unpinned citation count after this admission is 927, against the committed
ceiling of 941 (14 of headroom; the ceiling itself is unchanged). No ceiling raise is needed or
made. On merge into the integration branch, backlog hunt entries in `docs/agent-memory/ideas.md` had added 29 unpinned citations; 17 were read and pinned, leaving 939 under the same ceiling.

## DCG-V0-020: the drift waiver, extended

`script/line-citation-drift-waivers.txt` (referenced in code as DCG-V0-020) already named two
pinned citations in decision 0170 whose cited lines drifted, exempting only the anchor-mismatch
check, because `internal/localcompletion` is owned by another in-flight lane this change may not
touch. That citation is extended, not replaced: the same mechanism now also exempts the
blank/bracket-only-line check for a named citation, needed because decision 0158's 3 drifted
citations are unpinned (so the anchor-mismatch check never applies to them) but still trip the
unconditional trivial-line check. The file-existence and line-range checks are never exempted by
this mechanism, pinned or not. This reuses the existing mechanism at its narrowest extension rather
than inventing a fourth exemption path.

## Consequences

`./script/check-line-citations.sh` exits 0. The `fixes.md` entry this decision resolves is removed
in the same commit. `docs/specs/documentation-citation-gate-v0.md` gains DCG-V0-019 (decisions are
scanned; the historical-citation conversion rule) and documents DCG-V0-020's broadened scope; its
"Rollout, rollback, and drift" paragraph on decision 0154 gains a sentence pointing to this
decision's scope-widening admission of 0158.

Rollback: revert this decision's commit. That reverts `scanned_doc()` to exclude `docs/decisions`,
restores every decision-file citation this change touched, removes 0158 from the legacy allowlist,
and removes its 3 drift-waiver rows. Decision 0170's original two drift-waiver rows are unaffected.
