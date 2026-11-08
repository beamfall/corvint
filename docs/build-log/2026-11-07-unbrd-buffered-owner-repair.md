# unbrd: retain unknown buffered-program targets (2026-11-07)

V1-4283, AFP-V0-2956. The owner's self-review repair identifies a generated script that tests
`reader` while `go list` lists `lib`. Rejecting its format from build-only classification still
linked only `lib`, so a runtime configuration edit could omit the Makefile target.

## Decision and class

Uncertified listing output has uncertified execution targets. The closed format allowlist remains
exactly default package paths, `{{.Dir}}`, and `{{.ImportPath}}`, with only the options already
listed in AFP-V0-2956. Every other template, encoding, option or incomplete option value retains
execution edges to every owner and marks the invocation computed. This uses existing graph and
frontier mechanisms; it does not attempt to evaluate arbitrary templates or trace temporary files.
The conservative behavior applies even when the consumer is absent from the scanned text.

An initial edges-only implementation failed the new integration regression: unbounded runtime
reads that are not root locators do not propagate through package edges. Retaining the computed
invocation frontier covers those reads and targets outside the owner graph without changing the
graph's propagation contracts. The final implementation retains both mechanisms.

The AFP-V0-2956 amendment follows the owner's explicit repair request and standing lane acceptance.
No wire fields, error codes, exit codes, diagnostic text, goldens, core-freeze outputs or existing
assertions are weakened or changed to accommodate output. Selection intentionally widens for
uncertified formats. Non-goals: template evaluation, graph propagation changes, unrelated readers.
Rollback must retain uncertainty for uncertified output; never restore listed-package-only safety.

## Acceptance evidence

`TestBufferedListingOtherPackageSelection_AFPV02956` tests the exact reported concatenating
`printf` template and an unknown `-future-format` option against a distinct unbounded reader's
runtime configuration. Both omitted the driver before repair; both now select it. Default,
directory and import-path controls preserve precision. `TestDriverListOutputAllowlist_AFPV02956`
adds the reported template and asserts all-owner execution edges plus the computed frontier for
every uncertified form, retaining all prior assertions.

Raw evidence is external under `/home/russell/lanes/unbrd-evidence/`; the final handback records
package checks, spec/citation checks and the post-commit checked bind. The affected plan selected
both touched packages, 497 selected / 71 unknown, scope UNKNOWN, with `self-binary-stale` retained.
No precision equivalence is claimed for that installed binary. Pre-change evidence is reused by
instruction. Independent review, wider host validation and native ticket completion belong to
the orchestrator; no Tasks write, publication or seal is authorized in this repair.

| Evidence | SHA-256 | Observation |
| --- | --- | --- |
| `repair11-before.log` | `8b0b3c80067f986067c8e1feef6209d81946c52c96f218de1aadc7be5e24b695` | Before fix: two omissions; exit 1 |
| `repair11-golang.log` | `321e66f9f1392166a8e25a639c1b48aeab42642b5dd9fb1efa1d3b0ccdea157b` | Edges-only attempt: two omissions; exit 1 |
| `repair11-focused.log` | `15557fbb32933db22a27407d2c3723f86bb8226950729c794d2609dc3b43af46` | Final focused regression passes; 0.019s |
| `repair11-golang-final.log` | `4fb7cf592f362e9a58b0d140a9d1ac78c13dd38a51d211e305e5ba15cb9ed4c2` | Full golang package passes; 8.697s |
| `repair11-affected-final.log` | `41002e750794c4c3787969411decf31a10c2930d8e0109fbf827c3d713baf2ef` | Full affected package passes; exit 0 |
| `repair11-affected.json` | `b60379985a55e351e2556584886af264dd4d5e0de7f16c4cf66e4b53a0d09b8a` | 497 selected; 71 unknown; scope UNKNOWN |
