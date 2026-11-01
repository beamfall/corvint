# Stable CEM 1.0 public contract

Status: the stable `cem/1.0` wire is implemented and produced on main. The Core
producer writes it, and both the native verifier (`corvint cem verify-stable`)
and the portable consumer (`interop/cem01-go`) verify it. Owner scope is
accepted. The technical contract in this directory is proposed: it is not
accepted and not promoted, and no stable release is made by it. Qualification
is bounded (`ALGORITHMS.md`, CEM-V1-025..028). The frozen
`cem/1.0-experimental.1` candidate one directory up is unchanged. This packet is
Apache-2.0. New permissive implementation files outside `protocol/**` are not
authorized.

- `ALGORITHMS.md`: exact wire, complete 0.3 preservation, reference closure,
  sidecar lifecycle, result authority boundaries, qualification requirements,
  the citation block and stop list (CEM-V1-108), result serialization and the
  verdict digest (CEM-V1-109) and the canonical diff recipe (CEM-V1-110).
- `stable.schema.json`: closed stable map with ten required keys and the optional
  `provenance` and `role` members (CEM-V1-093, CEM-V1-095); cross-field and byte
  rules require the algorithms as well as schema validation.
- `OPERATION.json` and `RESULT.schema.json`: distinct native/portable stable
  operation and closed result, with integrity separated from native authority.
- `MIGRATION.json`: precise decisions for 19 consumer families and two later
  additions; first wire/verify/CLI vertical precedes broad migration.
- `fixtures/manifest.json`: 50 independently constructed proposed SHA1/SHA256
  positive/adversarial cells and 54 literal Git objects, constructed before the
  implementation. They run through `repository-envelope-packet/`, which carries
  the same pack bytes.
- `tamper-matrix/`, `citation-vectors/`, `neutral-criterion/`,
  `receipt-authentication/` and `repository-envelope-packet/`: sealed suites that
  the native verifier and the portable consumer both run with pinned verdicts.
  `citation-vectors/` (CEM-V1-111) pins anchored hunks, nonzero specificity, a
  same-path deletion and the exact citation block bytes. Its `results/` and
  `neutral-criterion/results/` pin whole result lines, nested member order
  included (CEM-V1-113). `reference-precedence/` (CEM-V1-177) pins the stage,
  code and whole result line of six graph mutations whose precedence
  `ALGORITHMS.md` fixes (CEM-V1-176).
- `inherited/`: unchanged pinned public algorithm/schema inputs, now historical
  provenance only. `ALGORITHMS.md` restates their pure algorithms, the 0.3 hunk
  vocabulary and structural reasons as the 1.0-normative text; their
  profile-specific admissions are not stable aliases.

Fixture Tasks/runner artifacts are manufactured opaque examples, including failed
and skipped observations. They are not valid native authority or execution
records. A reference-integrity success cannot authenticate their content. The
sixth snapshot-head artifact role and distinct raw snapshot digest are
implemented and await technical acceptance; native receipt identity and raw-file
hash remain separate; no candidate bytes or meanings change.

Technical acceptance and independent contract review remain open. Full
runtime/repository support, all actual runner/platform/lifecycle qualifications,
external authored consumer conformance, matched external outcomes and governing
release gates remain mandatory OPEN. Existing linked/config admission gaps and
source/execution/semantic authority gaps remain visible. No Tasks store or source
checkout is changed by this packet.

`build_contract.py`, `build_fixtures.py` and `build_migration.py` are scratch
construction provenance, not product implementations or executable manifest
instructions. Do not regenerate a frozen reviewed packet in place. A later
revision must preserve the frozen manifest and bytes first.

Revision r2 (V1-3443, CEM-V1-936) amends `ALGORITHMS.md` items 6 and 7 to admit
Git's canonical full-index binary section beside a textual hunk. The r1
`ALGORITHMS.md` and `manifest.json` bytes are preserved unchanged under
`revisions/r1/`, and this manifest lists them. The shared parity case is
`interop/cem01-go/testdata/binary-section/`.

Materialized `fixtures/repositories/**` Git administrative directories and
`construction-history/**` are scratch-only, excluded from the proposed public
payload. Public literal object bytes and hashes live in `fixtures/git-objects/`;
no Git-generated hook/config code is promoted under this packet license.

R1 transport: all fixtures/ and expected-results/ paths above are logical pack
paths reconstructed from FIXTURES.pack.json, outside tracked source. PACKING.md
defines bounded reconstruction. Original construction scripts remain only in
the unchanged predecessor proposal. R1-EXPECTED.json is a declarative
expectation set that no test reads or executes; it is construction history, not a
conformance claim.
