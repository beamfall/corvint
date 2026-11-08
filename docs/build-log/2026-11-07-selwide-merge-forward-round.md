# Selwide merge-forward round

Merged mainline `e19823d919cfa653ee0ed05708d03f5fd12ca929` into lane head
`0681753385719489c7b5887481dabaf2eaf8bcf2` with a two-parent merge, preserving
both histories. DWC-V0-7601 and DWC-V0-7602 govern the whole-range rebind from
that merged mainline commit. This lane was unsealed; this round stops at a
passing dogfood-check and leaves sealing to its owner.

Conflicted paths and resolutions:

- `.corvint/unbounded-readers.json`: union of both sorted unit sets and reasons;
  the lane's hidden CI helper floors and mainline's additional readers remain.
- `docs/specs/README.md`: mainline's updated selection summary and all entries,
  with the lane's AFP-V0-9012..9014 range retained.
- `docs/specs/REQUIREMENTS.tsv`: mainline's generated version at merge; regenerated
  afterward with `make spec-requirements`, which owns REQUIREMENTS.tsv,
  INDEX.json and the README header projection.
- `docs/specs/falsifiable-packet-v0.md`: mainline's symbol-based NormalizePaths
  citation preserves the same digest construction without a fragile line span.
- `internal/liveverify/affected/golang/golang.go`: lane's shared package source
  discovery, complete generic-exclusion opt-in and explicit package enumeration;
  mainline's HiddenSourcePaths is retained and uses the shared filtered walk,
  preserving declared-output exclusion before filename validation (AFP-V0-9012).
- `internal/liveverify/affected/hidden_unrepresentable_test.go`: both sides' common
  hidden-source regression plus all lane embed, disk/immutable parity and strict
  declaration cases.
- `internal/sessiontest/run.go`: mainline's declared-suite confirmation, nested
  module diagnostics and returned hidden patterns, retaining the lane's explicit
  indexed dot/underscore package arguments for root discovery (AZU-V0-9923).

No requirement collision: AFP-V0-9012, AFP-V0-9013 and AFP-V0-9014 are absent
from the merged mainline's requirement index; no renumbering is needed.

Verification is scoped by the owner's merge-round instruction to the three
conflict-resolved Go packages and spec-requirements/line-citations checks.
Repository-wide gates, whole affected selections and live qualifications are
NOT_RUN in this round; host verification and batch CI own those observations.
The local CEM retains its individual NOT_PRODUCED and NOT_OBSERVED reasons.
