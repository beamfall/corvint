# Decision 0560 — Blind-v7: preregistered repository pool and committed coin

Date: 2026-10-25. Status: accepted (owner RUN and COMPLETE grants on V1-0835 at ticket revision 2,
2026-10-25). Ticket: V1-0835. Governs: the blind-v7 commission that decision 0427 item 4(b) names
under GPK-V0-073. Supersedes nothing; decision 0307's record of v4, v5 and v6 stands.

## Context

No sealed retrieval partition remains (decision 0307). The V1-0835 authoring guide picks five
repositories from a preregistered pool of about fifteen that Corvint has never indexed, by a
committed coin: the hash of a named future upstream tag. The coin has to be committed before the
tag exists, so this record and `benchmarks/blind-v7/pool.json` are committed before any pool
repository is cloned and before any case is written.

## Decision

1. **Pool.** Fifteen public repositories, five per language group, each pinned at the default-branch
   head that `git ls-remote --symref` advertised at 2026-10-25T17:44:34Z:
   Go `alecthomas/kong`, `sirupsen/logrus`, `BurntSushi/toml`, `go-resty/resty` (branch `v3`),
   `rs/zerolog`; Python `marshmallow-code/marshmallow`, `Delgan/loguru`, `arrow-py/arrow`,
   `msiemens/tinydb`, `jd/tenacity`; JavaScript/TypeScript `pinojs/pino`, `yargs/yargs`,
   `sindresorhus/got`, `koajs/koa`, `immerjs/immer`. The pins are in the pool file and stay fixed.
2. **Prior-use check.** A bounded search at `origin/main` `866a7fd52` found none of the fifteen in
   any tracked path, in the history of `benchmarks`, `docs`, `conformance`, `tools`, `.corvint` or
   `README.md`, in the primary checkout's `.corvint` ledgers, or in the tracked dependency
   manifests. Four drafted candidates (commander.js, attrs, typer, rich) were dropped before
   pinning because tracked lock files install them, and httpx because its name occurs in retained
   ARB result paths. The limits are recorded in the pool file.
3. **Coin.** The commit id of `refs/tags/go1.27.4` in `https://github.com/golang/go.git` (peeled if
   annotated). It is the next patch release of the toolchain this repository pins (decision 0557)
   and was not published when this record was committed. No other tag, date or source substitutes
   for it; until it is published the selection waits.
4. **Selection.** Rank every pool id by the SHA-256 of `go1.27.4:COIN:ID`; take the lowest-ranked id
   of each group, then the two lowest-ranked remaining ids. The exact command is in the pool file.
5. **Bars, fixed now.** On the single run: top-5 task success at least 0.10 above the runner's
   exact-search baseline; zero must-exclude hits over every emitted result; abstention precision at
   least 0.8 and recall at least 0.7, where an abstention is an `OUT_OF_SCOPE` answer with zero
   results; and each repository's evaluation latency divided by its case count at most 5 s. PASS
   iff all five hold.
6. **What follows the coin.** Cases are authored on the Windows host's WSL2 in a new empty
   `~/bench/v0835/`, where the corvint binary is never installed or invoked, from upstream source,
   `git log` and `git grep` only: about thirty positive cases with one to three reasoned decoys
   each and ten within-repository near-miss out-of-scope probes, following the ticket's guide. A
   preregistration file (case digest, these bars, pins, runner digest, one command) is then
   committed and its digest recorded in a new decision before exactly one run. The appeal route
   (a second person voids one decoy before seeing other output) is available; no agent exercises
   or simulates it.

## Consequences

V1-0835 stays open and waits on the go1.27.4 tag. Nothing here is a measurement. The pool file's
SHA-256 at this commit is `837948018892341849ad52b4fed024df6eb0dc60c5458ecd86eee791763acc0e`.

## Rollback

Withdraw the commission before the coin resolves by a later decision that keeps this record and
the pool file. After the coin resolves, the selection cannot be redrawn; a withdrawn v7 never
makes any pool repository held out for another partition without a fresh prior-use check.
