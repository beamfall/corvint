# Proposed stable CEM 1.0 public contract

This is a scratch technical proposal for exact `cem/1.0`. Owner scope is accepted;
this contract remains PROPOSED / EXPERIMENTAL. It does not implement, release or
promote stable CEM. The frozen `cem/1.0-experimental.1` candidate is unchanged.
Future public destination: `protocol/cem-1.0/stable/`, under Apache-2.0. New
permissive implementation files outside `protocol/**` are not authorized; portable
code can extend its existing licensed files after accepted design/review.

- `ALGORITHMS.md`: exact wire, complete 0.3 preservation, reference closure,
  sidecar lifecycle, result authority boundaries and qualification requirements.
- `stable.schema.json`: closed ten-key stable map; cross-field and byte rules
  require the algorithms as well as schema validation.
- `OPERATION.json` and `RESULT.schema.json`: distinct native/portable stable
  operation and closed result, with integrity separated from native authority.
- `MIGRATION.json`: precise decisions for 19 consumer families and two later
  additions; first wire/verify/CLI vertical precedes broad migration.
- `fixtures/manifest.json`: 50 independently constructed proposed SHA1/SHA256
  positive/adversarial cells and 54 literal Git objects. Git patch/sidecar readback
  is observed; all stable implementation outcomes remain NOT_RUN.
- `inherited/`: unchanged pinned public algorithm/schema inputs. These supply
  pure algorithms; their profile-specific admissions are not stable aliases.

Fixture Tasks/runner artifacts are manufactured opaque examples, including failed
and skipped observations. They are not valid native authority or execution
records. A reference-integrity success cannot authenticate their content. The
new sixth snapshot-head artifact role and distinct raw snapshot digest are proposed
stable decisions for Gate A; native receipt identity and raw-file hash remain separate;
no candidate bytes or meanings change.

Gate A and independent contract review are required before implementation. Full
runtime/repository support, all actual runner/platform/lifecycle qualifications,
external authored consumer conformance, matched external outcomes and governing
release gates remain mandatory OPEN. Existing linked/config admission gaps and
source/execution/semantic authority gaps remain visible. No Tasks store or source
checkout is changed by this packet.

`build_contract.py`, `build_fixtures.py` and `build_migration.py` are scratch
construction provenance, not product implementations or executable manifest
instructions. Do not regenerate a frozen reviewed packet in place. A later
revision must preserve the frozen manifest and bytes first.

Materialized `fixtures/repositories/**` Git administrative directories and
`construction-history/**` are scratch-only, excluded from the proposed public
payload. Public literal object bytes and hashes live in `fixtures/git-objects/`;
no Git-generated hook/config code is promoted under this packet license.

R1 transport: all fixtures/ and expected-results/ paths above are logical pack
paths reconstructed from FIXTURES.pack.json, outside tracked source. PACKING.md
defines bounded reconstruction. Original construction scripts remain only in
the unchanged predecessor proposal. R1-EXPECTED.json is declarative, NOT_RUN.
