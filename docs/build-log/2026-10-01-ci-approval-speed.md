# CI approval latency and complete partition execution

Owner intent: reduce GitHub merge approval time and use Corvint to run only necessary PR tests.
Scope: V1-0616, V1-0617 and V1-0618; base `adf8358220769b8d6724ad27d27625602b8a7c62`.
Builder uses the current Codex runtime; independent reviewer uses Astra/high because partition
coverage and trusted build authority require reasoning across mixed fallback paths. Live builder
model/effort controls and billed-token attribution are NOT_OBSERVED. The owner authorized credits.

## Original evidence and earliest proof

[Successful run 36878582999](https://github.com/beamfall/corvint/actions/runs/36878582999)
took 49m55s; root race shard steps took 1530/872/2915/640 seconds. Its control-plane follow-up
took 10 seconds. The approval wait is dominated by CI execution, including a 508-second archive
step, rather than the status-posting stage. Advisory costs retain the 253 passing package terminal
elapsed values from that exact tested merge (`da6b049552d6caf828cfe9a7c29cf78bef31a914`).
The four greedy bins in an offline replay were about 1379.546 seconds each; this is a simulation,
not a hosted measurement, and excludes setup, compilation and contention.

The original 32 MiB receipt rejection subtest was run locally with Go 1.27.1, `-race -count=1`,
the unchanged `TestBBFV0013ReceiptApprovalAndBounds/BBF-V0-013_encoded_evidence_respects_document_bound`
selector, and the same cache paths. Its terminal elapsed changed from 737.87 seconds to 0.01
seconds. The same host was Darwin 25.6.0/arm64 with Apple clang 21.0.0. The Go binary SHA256 was
`548608a910c46de32c65a3934f461b1787acf6ddd371044826068d8503b8509b`; compiler SHA256 was
`b8763cf250e607a778bb4603cecb5b90338814d0a3dfcba0d57b1de242f610e9`. Host/tool identity
was observed in the same session after the paired measurements and retained in
`/private/tmp/corvint-ci-receipt-host.json`; it is local evidence, not execution attestation.
The package run includes parent setup and took 743.44 seconds before and 5.196 seconds
after. Original JSON logs are retained in `/private/tmp/corvint-ci-bounds-baseline.jsonl` and
`/private/tmp/corvint-ci-bounds-after-original.jsonl`; these local paths are not portable receipts.
The new raw-field, aggregate, metadata and small-secret regressions pass. Bounds now take
precedence over secret screening for rejected oversized envelopes; admitted bytes retain every
existing secret check. No rejected raw or encoded bytes are returned.

## Design and independent findings

Gate A found two high risks, both repaired before implementation: repartitioning a selected
subset can omit packages when other shards fall back, and the tested `go.mod` can redirect an
import used by a hidden helper. The implementation partitions the complete runtime universe
first, then intersects each shard with the selected set. Every mixed selected/full combination
retains the selected union without duplication. The fallback helper builds from explicit protected
files in an isolated synthesized module with closed offline Go settings. Embedded implementation
and cost bytes produce the same partition digest in the helper and trusted driver; a mismatch
fails the required check. Tests cover malicious module redirection, unknown packages, invalid
costs, disjoint complete partitions, actual selected/fallback failure propagation, empty shards,
and full historical execution for the new frozen shard profile. Final review found a stale
universe after source drift; selected and already-full paths now recheck the enumeration HEAD,
tree and clean status before assignment, and any drift fails. A proxy-tool regression inserts
a new package after the initial enumeration check and proves refusal in both paths. Empty shards
also preserve cancellation as a nonzero audited outcome. Exact encoded-limit minus/at/plus-one
and adjacent base64 sizes retain size-versus-secret precedence; these repair regressions pass.

AFP-V0-022 defines this bounded experimental execution profile. The hidden helper has explicit
race tests, formatter, vet and Windows build coverage in CI because root `./...` excludes hidden
directories. Main/release remain complete, mandatory static/build/interop checks and read-only
permissions remain, and empty shards never invoke an implicit current-package Go test.

Independent final diff review and its first repair re-review PASS with no remaining HIGH findings.
Ticket acceptance is PARTIAL overall until genuine hosted measurements, integration and promotion.

## Corvint use and retained limits

Installed official Core is 1.0.0-rc.1 build 163; Tasks is the standalone developer build 202.
Official update check found no newer applicable component. Publisher authentication and local
Tasks cutover qualification remain NOT_VERIFIED/NOT_OBSERVED. Context, affected-plan and original
CI replay evidence are retained under `/private/tmp/corvint-ci-*`; initial dirty-tree dogfood
refusals are preserved and final clean binding uses the same base. The native ticket store owns
claims and closeout; no competing queue was created.

The existing broad unbounded-reader floor is retained: a one-file receipt-test replay selected
171 of 284 packages with 159 unresolved readers. V1-0246/V1-0081 own the missing read bounds.
The PR416 event base differs from the tested merge's first parent, so exact-topology admission
correctly remains FULL. No base substitution or uncertainty waiver is introduced.

AFP-V0-014 still requires 200 genuine frozen rows, exact host/tool/environment identities and
reviewed artifact pins. The main history prerequisite is met, and the qualification workflow
now freezes the four-shard PR identity while each row still runs one complete invocation.
Generic CI pins remain empty. Qualification and hosted speedup are NOT_RUN until actual workflow
outputs establish them. Tickets remain open until integration and their required evidence.

## Rollback and verification boundary

Revert this task's partition/workflow/receipt changes and clear selection pins to restore the
previous complete placement and receipt error order. The dirty primary checkout is preserved.
Focused checks, independent diff review, documentation checks and clean CEM/OCM binding govern
this scoped change; repository-wide `make gate` is NOT_RUN under the owner's focused-check policy.
Merge, owner control-plane acceptance for `.github` changes, and selective promotion are separate
unfinished integration boundaries; publishing the reviewed ready PR is authorized.
