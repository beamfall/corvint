# Editor context analyzer-input audit repair

The full selected contextindex run for candidate `b2532dae846790783278d80bb068684a10507cba`
failed `TestAnalyzerSchemaInputs/IDX-SNAP-V0-017`; the original receipt is retained at
`/tmp/lsp-editor-context-evidence/focused-final.log`. The builder incorrectly reported all selected
checks passed by conflating the successful scoped race log with the full-package log. No later
full-contextindex pass existed at that handoff. The coordinator caught the error before final
binding/publication, froze the candidate, and explicitly authorized this surgical repair.

The audit intentionally covers all contextindex production sources and extraction dependencies,
including consumer-only process helpers and the new gitstatus worker policy. Following its
existing contract, analyzerSchemaID and auditedSchema move from `/96` to `/97`; the audited SHA-256
is `3586823cd8223f1abc9ff1dfd67101c7c89776c400816f1b8e9b7e0fc79cc9d3`. This conservatively
invalidates old analyzer-keyed packs. It changes no extraction/ranking semantics or worker process
lifecycle, and requires no format migration. The accompanying original build-log claim is corrected.

The independent reviewer reported no additional findings in the unchanged context/worker feature
source and requested a readback of this exact schema/docs delta. The older CPU sampler proof is
retained only for its unchanged inherited-group and real lexical CPU cancellation path: its
instrumented source predates the 256 KiB input bound, executable digest and stricter final cleanup/
result validation. It is not evidence of those final validation changes. Current native parity and
editor cancellation/EOF/retry probes separately exercise the replacement build.

## Verification

The pre-test affected receipt is `affected-schema-repair.json`. The formerly failing focused
schema audit passed (`schema-focused-repair.log`). The complete contextindex package then passed
with exit 0 in 168.928 seconds (`contextindex-schema-repair-full.log`); the actual process exit
was collected before this repair was frozen. Native READY and ABSTAINED exact-object parity and
fixture immutability passed (`schema-repair-native-parity.log`). The schema97 binary passed READY
→ CANCELLED → healthy READY and EOF during an observed native worker, with all previously observed
PIDs absent (`editor-proof-schema97.json`, 201 ms diagnostic EOF-to-exit observation). The scoped
documentation checks passed (`schema-repair-docs.log`). Prior unchanged helper/race/source-review
evidence retains its original binding and scope. No repository-wide gate, CEM binding, publication,
profile promotion or ticket completion is claimed.
