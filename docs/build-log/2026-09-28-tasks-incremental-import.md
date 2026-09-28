# CAL-V0-018 incremental import prototype

The accepted S6 contract requires one full audit followed by work proportional to the records
written under that same lock/session. The prototype carries an opaque IMPORT_APPLY sequence
that owns freshly decoded canonical records and physical metadata. Public Model remains the
untrusted-input oracle. Whole-export importer graph/reference validation and physical apply,
CAS, journal, request identity and redo remain on the existing paths. No wire state changes.

Capacity uses independently measured archive entry contributions and checked totals for every
existing publication prefix, including the abort/fresh UNPAUSE reserve. Size-only digest width
arithmetic never becomes content evidence. Capacity forks temporarily mutate owned metadata
with bounded undo; only successful publication advances the sequence. Returned batch bytes
are detached and cannot expose a mutable full inventory or generic Plan.

The approved practical plan requires small parity, ownership and physical-failure checks before
one frozen complete-command comparison. These checks passed locally; promotion is pending the
predeclared >=20% complete-command wall-time benefit and the remaining differential/invalidation
proofs, frozen selected checks and independent review. No speedup or proportional-work completion
is claimed at this prototype checkpoint. Recovered historical export identity and fresh controlled
measurements will be retained separately from the previous loaded/profiled nonqualifying run.

Original Corvint query, initial dogfood NOT_PRODUCED reasons, affected advice, metrics preflight,
small tests and interruption cleanup proof are retained under
`/tmp/corvint-import-scaling-20260928`. Query/context, affected and enrolled CEM/OCM are applicable;
retrieval-learning evaluations and provider/UI/runtime routes do not apply to this import-only
change. The root coordinator owns native task state, integration and completion. Rollback restores
per-batch public Model without migrating durable state.
