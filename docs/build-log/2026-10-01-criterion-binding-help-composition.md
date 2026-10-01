# Criterion-binding command help after composition

The owner requested closing the known composition gap between PR416 and PR424 in V1-0604.
Public main `cf4c8a39e171d72374e8baadeb744d4105116eda` contains both changes. Its Tasks CLI,
criterion-binding and wire inputs are byte-identical to `c4de689a`, where the existing
`TestCALV0047_AllCommandHelpIsReadOnly` failed: `criterion-binding capture --help` reached the
execution parser and returned `capture requires unique --ticket and --attempt`. Both leaf verbs
were in `ReadVerbs` but absent from the usage map. The parent-family help already worked.

CAL-V0-047 already governs every public leaf and requires help before store or stdin activity;
no new authority, wire contract or spec requirement is needed. The repair adds the two leaf
usages, states capture's ticket/attempt flags and result-envelope distinction, and describes
verify's canonical capture stdin contract. The inventory test checks their exact flag sets and
stdin descriptions while retaining panic-on-stdin, filesystem readback and both help aliases.
The execution parser and criterion-binding codecs are unchanged. Reverting these help entries
and assertions restores the prior behavior without changing any stored data.

The complete CLI, criterionbinding and wire package tests and focused vet passed. Existing native
capture tests retain the original claim across a body-only edit and verify strict offline framing.
The compiled candidate passed the all-command help test on empty and initialized roots. It also
captured and verified a disposable native queue binding, then released that fixture attempt and
passed receipt audit. That fixture uses synthetic cutover parser input, as the existing native
capture test does; it is not deployment qualification. A separate read-only capture of the live
development queue refused with `SNAPSHOT_MOVED` and `queue policy blocked or changed`; that refusal
is retained, not promoted to a successful live capture. Compiled offline verification of the
committed canonical capture fixture passed outside a repository.

Evidence is retained under `/tmp/corvint-parallel-dispatch/criterion-help-composition/`. The old
PR424 receipts remain bound to their original targets and are not new composition test evidence.
Corvint prechange context and affected receipts retain omission and static-selection limits.
The owner selects focused verification, independent review and the native required gate rather
than `make gate`. No queue policy, installed executable or unrelated CI source is changed.
Source review, final CEM/OCM binding and seal, publication, integration and native completion have
separate evidence bindings; this build note does not claim those steps are already complete.
