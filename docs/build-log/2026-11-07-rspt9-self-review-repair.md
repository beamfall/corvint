# rspt9 self-review repair

The three P1 findings in the supplied independent review concern V1-5111 and
RSP-V0-002/003/005: partial native coverage promoted to whole-line exclusion,
malformed report siblings erasing failures, and discovery using different path
semantics from execution. This repair changes only those boundaries and their
regressions. Existing wire shapes, errors, goldens and original assertions remain.

Decision: certify browser whole-line coverage only from exact direct UTF-16 unit
arrays whose every unit is 0 or 1 in every observation. Partial, mapped,
truncated, mixed or future representations make native evidence unavailable.
This deliberately widens to FULL, including when another observation had useful
coverage: omitting unknown lines alone can lose positive body-execution evidence
and unsafely exclude. Complete executed/unexecuted controls still reach the
shadow selector's existing rules. No legacy display adapter or selector changed.

Decision: parse the strict JSON envelope once into independent raw records, and
validate aggregates separately. Only known terminal assertion statuses certify
results; unknown records retain an error without discarding known failures before
or after them. Jest file units retain a validated failed child even when siblings
or aggregate metadata are invalid. A passing FULL retry cannot erase the failure.

Decision: discovery uses the existing execution semantics: Jest --runTestsByPath
and Playwright escaped, anchored absolute paths. Other explicit filter forms that
the command builder cannot certify refuse. Existing inventory decoding stays
compatible with already captured reports; no runner/configuration is qualified.

Acceptance: the repair tests named in the spec exercise adapter-to-selector,
parser-to-SelectionSession.Run, unknown forms, and literal-path command parity.
Whole-package runs use the affected plan and the existing heavy semaphore, one
package per command. The initial implementation exposed the existing Vitest
captured-report contract, the Playwright absolute-path detail, and a new fixture
that placed an edit on a region boundary; these were corrected without changing
any original assertion. A transient new-fixture syntax error also failed and was
corrected. Final command results and limitations are retained in the lane handback.

Self-development routes: used `affected` from the required base; its installed
binary reports self-binary-stale and scope UNKNOWN (95 unknown entries), so this
is focused package selection, not exhaustive coverage. Pre-change capture is
reused by instruction. Final CEM binding/checking is attempted once after all
edits; independent review and host gates belong to the orchestrator. Live runner,
qualification, replay, and performance evidence remain NOT_RUN/NOT_OBSERVED.

Rollback: revert this repair to restore the previous internal adapters; keep
runner qualification absent and production narrowing disabled. Do not alter the
frozen CLI, wire contracts, goldens, or qualification records to mask a failure.
