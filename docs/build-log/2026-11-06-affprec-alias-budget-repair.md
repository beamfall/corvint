# affprec self-review repair: package dependency budget

Date: 2026-11-06
Owner direction: repair only the P2 source-alias dependency overflow finding.
Ticket: V1-4036. Requirements: AFP-V0-9000, AFP-V0-9001.

## Finding and decision

The alias traversal's capacity check does not account for the scanner's two
file-kind globs or other package globs. The package observer could suppress a
declared scanner's lexical reasons, then discard its dependency union at the
path-token bound. A change to a non-Go alias target consequently omitted the
scanner while reporting an UNKNOWN frontier. AFP-V0-9000 already requires the
lexical fallback when dependency bounds are exhausted.

Retain declared source scanners' lexical reasons separately until the final
package bound check. Restore them whenever that check discards dependencies.
The check sees the deduplicated union, including literals from files observed
after the scanner, and both glob and token bounds. Preserve the existing
leaf-pattern exception and its working-directory condition. Fixture-only
declarations do not require source dependencies and retain their existing
semantics. No wire member, diagnostic, golden, assertion or bound changes.
Rollback removes this deferred fallback; it restores the demonstrated omission.

## Evidence and limits

`TestReadKindsDependencyOverflow_AFPV09000` lowers the path-token bound and
creates a `.go` symlink to `data/imports.txt` outside any source package. Only
the target changes from valid source to a forbidden Tasks import. An exact-bound
control retains bounded selection. Three overflow controls require unbounded
scanner selection: the file-kind globs alone exhaust capacity, or an additional
literal glob in an earlier or later file exhausts it. The original observer,
restored with an external Go overlay, fails all three overflow controls; the
repaired observer passes the focused regression. Existing assertions are intact.

Raw evidence is outside the repository at
`/home/russell/lanes/affprec-evidence/alias-overflow/`. The final handback retains
the whole-package command, results, digests and final CEM/check outcome.
The affected route selected the changed Go package before tests; its installed
binary reported self-binary-stale and UNKNOWN scope, retained without claiming
complete coverage. Existing pre-change receipts are reused as directed, with
their stale bindings visible. No new capture ran.

Independent review and broader qualification are deferred to orchestration.
No Tasks writes, publication, repository-wide selection/gates or seal run in
this repair. Optional analyzer, learning, provider and benchmark routes are not
applicable to this observer fallback; no performance or external validation is
claimed. Structural CEM acceptance cannot establish semantic test adequacy.
