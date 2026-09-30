# Experimental post-merge connector contract

Human issue [392](https://github.com/beamfall/corvint/issues/392) requests vendor-neutral tracker
and forge reads, deterministic fixed-body writes, idempotency and credential separation.
The proposed PMC-V0 contract keeps those effects in a separate companion under decision 0373.
The two reference kinds are a file tracker and a file forge; no vendor transport is qualified.

Native Tasks attempt generation47 admitted a disjoint source slice on public base
`25730eaa0e9ce21ef6771ddf8be18f13cff1ad91`. Shared spec/CEM paths were deferred until successful
atomic widening receipt1345, after the previous owner's release. Capacity and peer scopes were
preserved. The reviewed source was retained at `82428f33`; current main had a README integration conflict.
The lane released its own generation47 as unpublished (receipt1346), then atomically reclaimed
generation50 (receipt1347) on current public base `8f4b866a4ed201a52d8c63196050a9623a449dfe`.
A new clean worktree retained only this lane's source and regenerated shared registry data without
editing upstream source. The old worktree/enrollment remains retained and unsatisfied.
Receipt structure/projection agree; authentication, liveness, runtime qualification and
historical acceptance remain `NOT_OBSERVED`; semantic coverage remains `UNKNOWN`.

The companion checks independent policy identity pins against immutable Git commits and changed
files, walks a bounded hierarchy, and rederives every generated request before effects. Security
classification is trusted input; unknown classification blocks. Local recording and dry-run have
identical canonical bytes; stable-key upserts reconcile partial writer failures. Credentials and
external writes are absent from the local references. An embedding host must separately isolate
the author and deterministic writer; the process cannot attest sibling environments.

One bounded native read-only reviewer assessed the plan and delivered source, with no nested
delegation. Plan review tightened identity framing and static file safety. Source review found
relative author-root containment and product/Git metadata protection gaps; regression tests cover
those repairs and subdirectory and linked-worktree boundaries. At most two repair rounds were used.
Trusted private single-writer directories and prior generated state remain prerequisites; hostile
directory races, concurrent writers, power-loss durability and remote authentication are unqualified.

The focused package and CLI conformance suite uses actual disposable Git commits and local files,
tests idempotency, fixed rendering, security routing, malformed input, partial failure and unchanged
protected destinations. The first run failed because the bounded process runner requires an
absolute executable; resolving Git through the trusted host PATH repaired the local setup.
Focused tests passed; vet passed before destination repairs and must bind the final candidate.
Required focused-docs and CEM/OCM closeout retain their actual final receipts rather than inferring
success from package tests. Integration and native ticket completion remain pending.

Corvint pre-change query and affected receipts were retained in the lane checkpoint. Native hook
index staleness was observed; snapshot refresh is a separate derived-state operation, not evidence
of behavior. Core build163 and Tasks build202 match observed official rc1/developer channels;
publisher identity is `NOT_VERIFIED` and broad release qualification is not inferred. Source tests
use installed Go1.27.1. Full repository gate, remote live writes and independent adopter runs are
`NOT_RUN`; no paired baseline exists, and token/cache/cost measurement remains `NOT_OBSERVED`.

Rollback stops invoking the optional command and reverts task commits, preserving private recorded
state. There is no Core registration, store migration, installed-path change or automation authority.
