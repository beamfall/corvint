# clicontract scope trim — 2026-11-04

Owner request: retain passing V1-2685 and V1-3738 work, withdraw only behavior that breaks
existing native contracts, and preserve all existing assertions and goldens. Incoming revision:
`a6a73050a898bcd07da696d9ee9867682de5f612`; base:
`d436dff76de1b8e75c427069ae7f80003ae99bc2`.

Decision: preserve native compatibility at the process boundary. Restore the Rust entrypoint
byte-for-byte from base, removing both shared imports so its candidate closure stays isolated.
The native bridge full-package check also rejected the same wrappers in C/JNI, Java and
Objective-C (NJB-005 ambient stderr capability and the source-size ceiling). Restore those three
entrypoints byte-for-byte too; no analyzer implementation or assertion changes.
In the shared helper, leave corvint-test's existing authored help to native dispatch. The four
folded compatibility shims also keep their original plain help; their generated offline JSON/tree
catalogs remain. Analyzer --version argv reaches the native parser, retaining descriptor bytes
and native refusals for extra arguments. Other common version handling, offline inventories,
installer behavior, Tasks help and the four previously MET tickets remain unchanged.

CAP-V0-7700 is partially withdrawn for Rust/native-bridge process interception and the five test/folded help
paths. CAP-V0-7702 is partially withdrawn for common analyzer versions and corvint-test's native
entrypoint. Reasons are recorded in the requirement rows and generated index. TCL-V0-7703 is
retained. The universal shipped-help conformance assertion conflicts with the withdrawn behavior;
it remains unchanged and cannot establish delivery of those paths. V1-2685 and V1-3738 remain
PARTIAL against their original universal acceptance. V1-3736 retains its prior PARTIAL scope.

The smallest end-to-end check passed: whole-package Rust tests, including the import boundary,
and whole-package corvint-test tests, including unchanged nested-help and folded goldens. Tests
run through script/heavy.sh with the affected plan retained outside the repository. No new test
or golden is needed: the original assertions are the acceptance witnesses for this restoration.
Independent review belongs to the orchestrator. No publication, task mutation or seal is authorized.

Raw command output and the exact command/result matrix are retained under
`/home/russell/lanes/clicontract-evidence/scope-trim/`; the handback records final verification and
CEM status. Structural dogfood closure does not assert universal requirement acceptance.
Rollback: revert this scope-trim commit to reinstate the incoming behavior and its known failures.
