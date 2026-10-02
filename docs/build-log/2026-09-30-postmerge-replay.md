# Historical replay harness and retained workflow limits

Human-owned issue #395 requires historical merge replay through the whole post-merge loop,
recorded outward requests and basis-labelled mismatches. The admitted source slice on public base
`9afd8313edad658d6c40b1abe454fe7417eb6cad` is a partial experimental companion under PMR-V0.
It does not declare #395 or its #388 parent complete.

The independent plan review found no remaining HIGH concern after two repairs: applicability
belongs to fixed harness policy rather than adapter assertions, and compared outcomes must join
to the actual connector input. Human labels need a separate pinned registry; provenance in the
fixture cannot authenticate itself. Pinned execution, private scratch and minimal environment
also do not establish semantic correctness or OS isolation.

The harness invokes the existing `postmergeconnector.Build`, `ValidatePlan` and `Record` APIs.
It runs exact adapter bytes twice, validates source/fixture/runtime bindings and required stages,
records twice per run to exercise actual idempotency, and compares outcomes plus canonical ledger
bytes. Follow-up expectation is compared to the actual plan. Recording is retained in the report.

Author scope and trusted validation are not integrated. Docs/tests work and all drafts block before
recording; a stage digest cannot self-certify them. All reports retain whole-workflow qualification
`NOT_OBSERVED`. Synthetic protocol adapters exercise the harness, not accepted historical labels
or real child-stage execution. Real-stage integrations, host qualification, historical expectations,
final change evidence, landing and native completion remain explicit closeout obligations.

Focused conformance and actual CLI checks passed after repair (23.092 seconds); focused vet
passed. Cancellation retired a real child process on this Darwin host. Independent code review
found three MED issues, all resolved in one repair cycle: approvals now bind full immutable
identity, JSON keys require exact casing recursively, and product root normalization precedes
adapter execution. The bounded re-review found no remaining HIGH or MED findings.

Corvint's affected plan selected the new unit directly and 141 additional literal/unbounded
readers, retaining language frontiers and new documentation ownership unknowns. The owner's
scoped-issue policy selects focused companion checks; repository-wide gate and those broad
reader suites are NOT_RUN. Initial dogfood evidence was NOT_PRODUCED (no diff/CEM and intent
scope drift). Final CEM/OCM, frozen docs checks and seal wait for the shared registry/evidence
reservation to be released; passing focused checks does not waive them.
