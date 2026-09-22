# Decision 0136 — Extensionless root files are line-citation targets

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12.

The documentation citation scanner recognized slash-qualified files with dotted extensions and
root basenames with dotted extensions. It did not recognize a backticked root `Makefile:N` or
`LICENSE:N`, so such a token bypassed every existence, range, trivial-line, and content-anchor
check. Treating all bare basenames as implicit paths would instead make an ambiguous token such as
`prove.go:767` depend on surrounding prose, which DCG-V0-001 deliberately refuses to guess.

The owner call: an extensionless token is a citation when the Git index tracks that exact path.
In particular, an extensionless basename tracked at the repository root names one unambiguous
path and anchors its paragraph like an explicit full path. An untracked lookalike remains prose and
does not change the current paragraph anchor. Slash-qualified tracked extensionless paths retain
the same exact-index rule. DCG-V0-015 owns this additive grammar instead of silently widening
DCG-V0-001.

The fixture test proves both polarities for `Makefile`: a citation to a real line passes, while a
citation to a blank line is refused. The repository check exposed the decision-number gate spec's
historical stale `Makefile` line 14 pin with digest `3d917a6f`; reading the Makefile showed that the
fresh-clone gate contract was stated on line 18, so the citation was repinned there.

Rollback: remove DCG-V0-015 and the extensionless scanner alternative. Root `Makefile:N` and
`LICENSE:N` tokens then become unchecked prose again; dotted and slash-qualified dotted citations
remain governed by DCG-V0-001.
