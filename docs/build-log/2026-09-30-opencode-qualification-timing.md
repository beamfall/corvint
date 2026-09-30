# OpenCode qualification timing: issue 387

Human-owned intent: [GitHub #387](https://github.com/beamfall/corvint/issues/387), native ticket V1-0534, owner parallel dispatch on 2026-09-30. Existing AHI-012, AHI-018 and AHI-032 govern this bounded repair; it grants no execution authority or integration promotion.

The source slice uses a genuine user-prompt query for CRB-V0-012 transport success within the existing 4000 ms query budget. A separate delayed-start fixture proves automatic session-start still returns timeout with deadlineMs 2000. Transport success is not an AHI-012 p95 measurement. An independent proposal review rejected forcing session-start onto the query budget as semantic masking.

The descendant observer excludes same-generation zombie state from the final live check: an exited process can await its parent's reap without executing work. PID/start ownership and signal revalidation remain intact. The passive qualification witness observes only captured identities for at most 500 ms before supervisor rescue; missing/reused/zombie identities count as exited, snapshot failures remain errors, and persistent live descendants cannot pass.

Requirements and evidence:

- AHI-032: TestObservedExitedZombieIsNotSurvivor exercises an actual unreaped zombie and the final observer loop. It failed before the fix; the scratch candidate passed three repetitions.
- AHI-032: TestPassiveWitnessSettle distinguishes transient exit from a persistent live sleep process; the transient case failed before the fix. TestGateInterruptionWitness retains its broken-cleanup negative control before rescue.
- AHI-012/AHI-018: TestHostAdapterJavaScriptHosts retains production ceiling/notice checks and adds genuine delayed query success versus automatic timeout. Existing interruption and V1-0371 same-group descendant regressions remain required.

Corvint query, path impact and affected were used with original receipts retained under /tmp/corvint-opencode-387. Query omissions/test candidate withholding and non-Go impact limits remain visible. The ordinary-load baseline passed internal/procgroup, internal/opencodequalification and the JavaScript host suite (43.877 seconds). Deterministic delayed transport and zombie regressions reproduce failure mechanisms; the natural full qualification flake was not reproduced and its original final cleanup JSON was unavailable.

Delivery is incomplete. This admitted source slice excludes the shared harness spec, requirements index and CEM/changes paths owned by another live attempt. Their update, native widen, CEM/OCM binding/seal, final selected-check receipts, capability campaign, independent delivered-diff review, draft PR, integration and native completion remain open until evidenced. make dogfood-change at change start is NOT_RUN because its tracked CEM write was outside admitted scope; private enrollment remains active with the saved key. No full make gate was requested; exhaustive coverage is NOT_RUN, not equivalent to focused coverage. Billed tokens/cache usage are NOT_OBSERVED.

Rollback is to revert only this issue's eventual source/test commits; preserve qualification records, unrelated work and native receipts. No package version, production deadline, authority or qualification record is changed by this slice.

## Verified source-slice outcome

After implementation, focused procgroup/opencodequalification packages passed (8.494/4.962 seconds), the JavaScript host and interruption selection passed (46.966 seconds), and the declared focused-docs command passed. The stock OpenCode native-only campaign with the candidate collector and Core binary returned PASS; raw report and cleanup evidence remain in /tmp/corvint-opencode-387/native-candidate. This is native campaign evidence only, not a promoted installation record or final immutable CEM/OCM-bound acceptance. One independent Sol/low reviewer found no blocking defect; its only non-blocking concern was residual fixture sensitivity under machine saturation beyond the ordinary-load evidence. Final shared spec/evidence integration and native completion remain open.
