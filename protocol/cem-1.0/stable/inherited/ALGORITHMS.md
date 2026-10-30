# Portable CEM 0.3 algorithms — P0 scratch draft

Status: proposed contract preparation only. No portable implementation, qualification, frozen
packet or public release is asserted. Intended destination: protocol/cem-0.3/ (Apache-2.0).
This prose is independently authored from the public 0.1/0.2 contracts and the requirements
CEM-SM-001..010 and TCQ-V0-052/056. It copies no native implementation or native test fixture.

## Exact wire and inherited mechanics

Dispatch only exact `cem/0.3` in `verify-03`. Keep the six canonical map keys `spec`,
`baseRevision`, `patchSha256`, `excludedPath`, `evidence`, `hunks`; each is required and no
extension key is allowed. Exclusion is exactly `.corvint/change.cem.json`. Evidence and hunk
identities use the canonical identity JSON formulas of the unchanged public 0.1 algorithm;
coverage/discriminates are not included in the hunk identity. Do not re-label another profile.
All inherited disposition, evidence reachability, relation, hunk mapping, raw-byte span,
Git object identity, patch simulation, same-path drift and ordering obligations remain.

Read a private immutable copy of the input map with a 4 MiB raw limit. Reject invalid UTF-8,
unpaired decoded surrogates, duplicate keys at every depth, unknown/missing keys, trailing
JSON and a non-object root. Integers use unsigned decimal integer lexemes, not booleans,
floats, exponent forms or negative zero; mathematical range is 0..9007199254740991.
Witness arithmetic must be checked without host overflow. Do not reject a valid wire-range
start/count pair early merely because its computed endpoint exceeds the wire bound: native
range-start MAX/count 1 reaches derived-hunk verification, and a wrong patch digest takes
precedence. Native depth probe fixtures at nesting 64/65 reject invalid-json; 62/63 reach
unknown-field. Freeze the exact container-count convention with those bytes, not an invented
portable-only operational reinterpretation. Schema cannot detect lexical,
duplicate-key, UTF-8-byte-length or cross-field constraints; successful schema validation is
never sufficient. Null is not an absent optional witness.

Inherited resource bounds: map 4 MiB; patch 8 MiB; one blob 64 MiB; distinct blob bytes 128 MiB;
4096 evidence items; 2048 hunks; 32 bases per hunk; path 512 UTF-8 bytes; reason 64 UTF-8 bytes;
262144 LF-tokenizer records per input; 1024 Git operations; 10 seconds per Git operation;
30 minutes cumulative. Tree reads are bounded at 4 MiB and walks at 128 levels. Exceeding a
separately declared implementation ceiling is operational unsupported, never protocol invalid.
Wire-bound violations retain their inherited error classification.

## Canonical authority and sidecar distinction

Require independently supplied full lowercase base and target commit OIDs in the repository's
SHA-1 or SHA-256 width. Map base is compared with expected base, never adopted as authority.
Resolve and hash immutable commit/tree/blob objects under the inherited CEM-CB boundary. Read
no source from a checkout. Disable ambient redirection/configuration, replacement objects,
grafts, text conversion, hooks, filters, prompts and network fetch. Validate primary and
reciprocal linked worktrees; reject alternates. Missing objects stay unavailable.

Derive the whole-repository canonical patch with the exact observed WP1 diff recipe in
WP1-OBSERVED.json, substituting only authenticated administrative directory/base/target and
the repository-format empty-tree OID; the captured SHA-1 empty-tree literal must not be
used for SHA-256 repositories. Validate object-format counterpart before qualification. Excluding
only the literal root sidecar. Neither caller patch nor map path may select another exclusion.
Hash that exact patch, require map patch digest equality, parse every hunk and prove its base
simulation. Retain all evidence drift records in ascending evidence ID order. Stable/relocated
accept; stale/ambiguous/deleted refuse. Unknown hunks are valid mapping states; this operation
has no max-unknown/max-mechanical policy switch.

At base, sidecar absence or regular mode 100644 is allowed; historical bytes need not match.
Other base kinds/modes fail `excluded-path-not-file`. At target, absence is allowed. A present
100644 blob must equal the ORIGINAL RAW input map bytes, including whitespace, else
`excluded-artifact-mismatch`. Exact matching committed 0.3 bytes ACCEPT under the existing
native verifier. An inherited 0.2 sidecar, re-encoded equivalent map, executable sidecar,
symlink, tree or gitlink refuses. Never re-encode before this comparison.
This says nothing about producers: mark/cover/discriminate still refuse replacing their 0.2
input or writing the Core sidecar path. P0 does not add or widen a producer.

## Structural proof

For a structural claim require an existing regular base file whose contents parse as Go, a modification group with
both sides present, and a changed image. Refuse create/delete, absent/nonregular base,
invalid Go syntax, invalid scan/parse/format or identical image with `unproven-mechanical`.
For rename, import-reorder and formatter-only, apply exactly the claimed hunk to immutable
base bytes; siblings cannot prove or taint that image. For move, apply the whole file group.
Token comparisons include comments and automatic semicolons by kind/literal, never positions.

* Rename: all noncomment token changes must be one consistent unexported identifier pair;
  destination is absent from base and source is declared locally, including a named method receiver or statement label. Reject source use as a
  selector, composite key, field, interface/concrete method or package name. Comments may
  change only by whole-word substitution; go/line/export directives remain byte-identical.
  This is FILE-LOCAL: sibling-file references may break compilation and are not inspected.
* Move: top-level declaration token multiset is identical but order differs; outside tokens
  are identical; var/init declarations preserve their relative order. Const/type order may
  change. Whole-group semantic siblings must cause refusal.
* Import reorder: ordered specs differ, name/path multiset and comments are identical, and
  all tokens outside import declarations are identical. Alias changes are not reordering.
* Formatter only: go/format output for both complete images is identical. This deliberately
  overlaps import reorder where gofmt sorts imports. No semantic-equivalence claim follows.

These predicates have a toolchain dependency. P0 initially permits structural proof only when
runtime.Version is exactly `go1.27.1`. The scratch prototype emits
`EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS`; admission as qualified still requires that tuple
to pass the frozen new corpus. Other
runtime versions—including devel labels, suffix variants and Go 1.24—return operational
`unsupported-structural-runtime` when a structural proof is required. No downgrade to a byte
proof is allowed. Nonstructural 0.3 and historical/candidate commands retain their existing
runtime behavior and the module's Go1.24 source floor. Qualification must compare 1.24 and
1.27.1 parsers/scanners/formatters over the entire corpus plus version-sensitive syntax,
comments, directive placement and formatting cases IN P0 before implementation exit. Corpus
agreement alone does not prove every possible Go source agrees or qualify future toolchains.
Changing the runtime allowlist later needs new measured evidence and review, not a version guess.

## Witness retention and authority

Coverage has exactly profileSha256/testRun/mode/state/covered. Digest is lowercase SHA-256;
testRun is 1..256 UTF-8 bytes of caller text; mode is set/count/atomic. Text checks reject
U+0000..U+001F and U+007F only: U+0085 and U+2028 remain admitted. Do not apply Unicode
IsPrint or reject all non-ASCII/control-category characters. Covered ranges are
one-based positive counts, ascending, nonoverlapping and nonadjacent, wholly within newRange;
covered state iff nonempty, uncovered iff empty. Absence differs from uncovered. The validator
does NOT replay a coverprofile or prove ranges are actually added/executed lines. Preserve a
well-shaped caller range on a context line if native admits it; do not strengthen the wire.

Discriminates has exactly treeRevision/selectionSha256/mutants/killed/survived/survivors/bounds/
state/detail. Git OID and SHA-256 syntax are inherited. Counts are nonnegative with
killed+survived <= mutants. Survivor list length equals survived; each operator is 1..64 UTF-8 bytes,
line is inside newRange, description is 1..512 UTF-8 bytes excluding C0 and DEL. Bounds have positive
maxHunks/maxMutants/wallTimeSeconds. Detail is 0..512 UTF-8 bytes excluding C0 and DEL; operator has that same character rule.
State discriminates requires killed>=1 and survived=0; survived requires survived>=1;
not-run requires mutants=0. Thus 2/1/0 is allowed (one uncompiled residual), but 2/0/0 fits no
state and refuses. Missing and not-run are different. Do not require killed+survived=mutants,
mutants<=maxMutants, treeRevision=current target or selectionSha256=replayed selection without
an existing native obligation. Well-shaped changed identities are retained, not authenticated.

Result witness copies preserve all validated values, ordering and absence by hunk ID; omitted
optional members remain omitted. Never synthesize empty/zero witnesses for absent members.
No witness result establishes test execution, Tasks validity, current applicability, criterion
adequacy, semantic correctness or completion authority. Execution/rerun are NOT_OBSERVED.

## Freeze and conformance

Before portable code, materialize independently authored fixtures into exact SHA-1/SHA-256
Git commits, canonical patch bytes, map bytes and expected IDs. Pin every artifact and raw
manifest. Verify exact absent/matching/mismatching sidecar fixtures natively first, retaining
actual stage/code/drift/witness behavior. Literal fixtures are supplied in this draft; their
canonical patches and commit IDs are NOT_PRODUCED until that construction step.

The vector inventory defines the closed P0 test scope; no manifest-driven executable recipes.
Freeze old 0.1, 0.2 and all 31 candidate packet cases/bytes/results. No candidate artifact
roots, references, role admissions or assurance dimensions enter this operation. Shared pure
strict JSON/Git primitives may be factored from permissive code; native AGPL implementation
and tests must not be translated/copied. A native/portable mismatch blocks P0; do not change
native behavior to fit the draft. External adoption remains OPEN.

## Result-stage correction from native black-box probes

Native exit 2 includes wire-contract errors; it does not mean every such input was operationally
unsupported. Preserve top-level error `code` separately from verifier `issueCodes`. A malformed
wire map returns no valid/assurance result; a verifier refusal can have structural-only or
canonical assurance. Canonical assurance reports derived patch authority, not successful map
validation: wrong patch digest, unproven mechanical and unsafe drift can retain it. The new
operation distinguishes protocol REJECT from operational UNSUPPORTED while retaining native
exit class and code/issue placement for qualified cases. Never convert canonical assurance
on a failed result into acceptance. See OPERATION.json and native-observed-correspondence.json.

Native probes used installed build163 (vcs b967f6bb..., Go1.27.1), not the clean follow-up
source commit05d66c36. Retain the full binary pin in the boundary report. The separate baseline
passes bind that clean source; neither evidence set substitutes for future changed-source checks.

## Prototype process boundary

The current scratch operation has the bounded helper-free Git scope in OPERATION.json.
Cancellation targets the owned direct child through CommandContext until Wait reaps it.
It never signals retained process-group IDs, including after reap. Fixed Git calls disable
hooks, pager and transports and use the narrow sanitized configuration envelope. Output
writes are bounded without promoted io.ReaderFrom methods. Descendant retirement is
NOT_QUALIFIED; this prototype does not establish safety for arbitrary helper-capable Git.

The file extension is not a structural-proof gate: native accepts valid Go syntax in
`sample.txt` (structural-non-go vector, clean 05d66c36). The earlier `.go` restriction was
a draft error, preserved in repair1-pre-extension; no native behavior was changed.

Every new03 drift record retains validated `baseBlobOid`, including deleted evidence;
historical portable result shapes stay unchanged. Canonical diff process failures use
`git-diff-failed`; ordinary object reads retain `git-read-failed`, and resource/cancellation
failures retain their operational codes at both boundaries.
