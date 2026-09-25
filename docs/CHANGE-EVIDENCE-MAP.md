# Change Evidence Map 0.1

Status: experimental interchange contract.

> **Pre-release security erratum — 2026-08-23.** The current
> [`interop/cem-0.1/ALGORITHMS.md`](../interop/cem-0.1/ALGORITHMS.md) and vector manifest replace
> every earlier `cem/0.1` draft. Conformance requires the hardened line-boundary, Git mode-binding,
> and LF-tokenizer limits plus the exact negative-vector rejection codes; earlier 0.1 behavior is
> not an accepted compatibility profile.

A Change Evidence Map (CEM) is a sidecar association map for agent edits. It records, for every
textual unified-diff hunk, immutable repository evidence selected by the producer, an explicit
unknown, or a mechanically verified exception.
An editor can emit the JSON sidecar and CI can verify it without Corvint, an LLM, a network service,
or a repository upload.

CEM is **source-content-free** in the narrow wire-format sense: it does not contain source excerpts,
diff bodies, prompts, commands, credentials, or ticket text. Paths and content-derived digests can
still disclose or permit guessing of project information, so maps are local by default and must not
be published without repository-owner approval. Source-content-free does not mean anonymous or
safe to publish.

The normative shape is [`cem-0.1.schema.json`](cem-0.1.schema.json). Repository conformance also
requires the deterministic checks in this document; schema validity alone is insufficient. The key
words MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT, and MAY are interpreted as in RFC 2119 and RFC
8174.

## Positioning and non-claims

Evidence-carrying changes, proof-carrying coding, assurance cases, and agent authorship provenance
already exist as ideas or products. CEM does not claim to invent those categories and is not a
formal proof of correctness. Its narrower proposal is a vendor-neutral interchange for:

- mapping each patch hunk to exact, immutable Git evidence;
- making unsupported edits explicit rather than silently confident;
- verifying patch, hunk, blob, and span identity deterministically; and
- detecting evidence drift without a shared model, index, editor, or hosted service.

Verification proves structural integrity and mapping completeness. It does not prove that cited
evidence semantically supports an edit, influenced its generation, states the producer's actual
reasoning, makes a test adequate, or makes the change safe.

Relevant prior art and adjacent work includes George C. Necula's 1997 formal
[Proof-Carrying Code](https://dl.acm.org/doi/10.1145/263699.263712), Max's July 2026
[Evidence-Carrying Changes](https://rmax.ai/notes/evidence-carrying-changes/), and Philip
Schenk-Hana's July 2026 [Proof-Carrying Coding](https://realitygraph.dev/proof-carrying-coding).
CEM borrows the producer-attaches/consumer-verifies asymmetry while deliberately making a smaller
claim than formal proof.

## Document

A `cem/0.1` document has five required fields:

| Field | Meaning |
|---|---|
| `spec` | Exact value `cem/0.1` |
| `baseRevision` | Full 40- or 64-hex Git commit object ID |
| `patchSha256` | SHA-256 of the exact patch bytes |
| `evidence` | Immutable Git blob spans; may be empty |
| `hunks` | Every textual patch hunk, exactly once |

Unknown fields are invalid. Exact patch bytes are obtained out of band and are never embedded in
the map. No newline, path, encoding, or whitespace normalization is applied before hashing them.

The map deliberately omits producer identity, test attestations, signatures, semantic scores, and
a redundant whole-map digest. Those are useful envelope concerns, but including them in 0.1 would
make the first interoperability target larger without strengthening its core hunk-to-evidence
claim.

## Patch profile

CEM 0.1 accepts bounded Git-style unified text patches:

- file headers use `--- a/PATH` and `+++ b/PATH`, with `/dev/null` for creates or deletes;
- hunk headers use standard `@@ -START,COUNT +START,COUNT @@` ranges;
- paths are UTF-8, repository-relative, `/`-separated, and normalized;
- creates, deletes, and modified renames are supported; and
- binary patches, path traversal, non-UTF-8 paths, empty patches, and indistinguishable duplicate
  hunks are rejected.

Pure renames with no textual hunk have nothing to map. A patch containing any unsupported binary
form is not `cem/0.1` conformant.

For each hunk, the verifier reconstructs the old-side bytes and confirms they exactly match the
declared range in `baseRevision`. This prevents a valid-looking patch from being paired with the
wrong base commit.

## Immutable evidence

Each evidence item contains:

- `id`: a content-derived `evidence:sha256:…` identifier;
- `blobOid`: the 40- or 64-hex Git blob object ID;
- `path`: its exact path at `baseRevision`;
- `span`: zero-based, half-open raw-byte offsets `{start, end}`; and
- `spanSha256`: SHA-256 of exactly those bytes.

The path MUST resolve to the declared blob at `baseRevision`. A span MUST be non-empty, lie within
that blob, and match its digest. Evidence is read through Git object identity, never through an
unchecked worktree path or symlink.

An evidence ID is derived from this canonical object, serialized as UTF-8 JSON with keys sorted,
no insignificant spaces, and non-ASCII characters unescaped:

```json
{"blobOid":"OID","path":"PATH","span":{"end":END,"start":START},"spanSha256":"DIGEST"}
```

The ID is `evidence:sha256:` followed by SHA-256 of those bytes. Evidence arrays need not be sorted;
duplicate IDs are invalid.

## Hunks and dispositions

Every parsed hunk MUST appear once, identified by:

- `id`: a content-derived `hunk:sha256:…` identifier;
- `path`: the post-image path, or the pre-image path for a delete;
- `oldRange` and `newRange`: `{start, count}` copied from the hunk header;
- `disposition`: `supported`, `unknown`, or `mechanical`;
- `reason`: one disposition-specific enumerated code;
- `basis`: evidence references for supported hunks; and
- `coverage` (`cem/0.3` only, optional): one patch coverage witness written by `cem cover`
  (`TCQ-V0-052`): `{profileSha256, testRun, mode, state, covered}`, where `state` is `covered`
  or `uncovered` and `covered` lists the ascending, non-adjacent added-line ranges inside
  `newRange` that the named coverprofile reached. `cem/0.1` and `cem/0.2` reject the key as
  `unknown-field`; and
- `discriminates` (`cem/0.3` only, optional): one mutation discrimination witness written by
  `cem discriminate` (`TCQ-V0-056`): `{treeRevision, selectionSha256, mutants, killed, survived,
  survivors, bounds, state, detail}`, where `state` is `discriminates` (every compiled mutant
  killed), `survived` (each survivor listed as `{operator, line, description}`), or `not-run`
  (zero counts and a `detail` naming why). `cem/0.1` and `cem/0.2` reject the key as
  `unknown-field`.

The hunk body digest is SHA-256 of the exact diff body bytes after the `@@` header. The hunk ID is
SHA-256 of canonical UTF-8 JSON containing `contentSha256`, `oldPath`, `newPath`, `oldRange`, and
`newRange`, prefixed with `hunk:sha256:`. Because old and new paths participate in the ID, creates,
deletes, and renames cannot collide even though the compact wire record exposes one display path.
The verifier recomputes the ID from the patch.

### `supported`

A supported hunk MUST have at least one basis. Every basis names an evidence ID in the same map and
one typed relation:

| Relation | Evidence role |
|---|---|
| `specification` | Requirement or accepted contract |
| `decision` | Architectural or product decision |
| `test-claim` | Executable claim or test source |
| `implementation` | Existing implementation behavior |
| `call-site` | Caller or consumer constraint |
| `dependency` | Dependency behavior or boundary |
| `incident` | Incident evidence relevant to the change |

`supported` means only that the producer attached at least one structurally valid evidence
reference. Its `reason` MUST be `evidence-backed`. It does not mean the verifier established
semantic entailment, causality, correctness, or model-internal use. Basis pairs must be unique.

### `unknown`

Unknown hunks MUST have an empty basis and use exactly one reason code:

- `no-evidence`
- `insufficient-evidence`
- `conflicting-evidence`

Unknown is a valid, explicit result. Policy may reject it, request widening, or require human review;
the verifier does not silently convert it into support.

### `mechanical`

Mechanical hunks MUST have an empty basis and use exactly one reason code:

- `whitespace-only`
- `line-ending-only`

The verifier proves the selected transformation directly from removed and added bytes. Generated
output, lockfile updates, file moves, and formatter claims are intentionally excluded from 0.1
because their correctness needs external commands or repository-specific policy.

The experimental `cem/0.3` profile ([`specs/cem-0.3-structural-mechanical.md`](specs/cem-0.3-structural-mechanical.md))
adds four Go-only structural reasons, `rename`, `move`, `import-reorder`, and `formatter-only`,
that the verifier recomputes from the base blob and the patch with the Go standard library. A
0.1 or 0.2 map carrying one of them is invalid, and a claim the verifier cannot reproduce fails
`unproven-mechanical`. The same profile admits the optional per-hunk `coverage` witness
(`TCQ-V0-051..054`, [`specs/test-claim-qualification-v0.md`](specs/test-claim-qualification-v0.md)):
`cem cover` intersects one operator-named local Go coverprofile with each hunk's added lines and
records a `covered` or `uncovered` witness on every hunk, and `cem report` downgrades a
`test-claim` basis whose hunk has no covering witness with a visible reason. The same profile
admits the optional per-hunk `discriminates` witness (`TCQ-V0-055..058`): `cem discriminate`
runs the bounded `prove --mutate` runner on the map's changed Go hunks against the `_test.go`
files their test claims cite, records `discriminates`, `survived`, or `not-run` on every hunk
pinned to the target tree and the test selection digest, and `cem report` downgrades a hunk
with survivors visibly without ever failing the build. `cem/0.3` is experimental: `mark`, `cover`
and `discriminate` write it only to an `--output` path that is neither their `cem/0.2` input nor
`.corvint/change.cem.json`, which frontier and OCM read as `cem/0.2` (`CEM-SM-006`). Keep that
copy uncommitted; it verifies against a target that commits no sidecar.

## Verification and drift

A repository-conformant verifier MUST:

1. reject malformed, oversized, duplicate-key, or unknown-field input;
2. resolve `baseRevision` to that exact full commit ID;
3. match the exact patch SHA-256 and validate its old bytes against the base;
4. recompute every hunk ID and require every parsed hunk exactly once;
5. resolve every evidence path and blob through Git;
6. recompute every evidence ID and span digest;
7. enforce disposition, basis, unknown, and mechanical rules; and
8. exit non-zero on any failure.

An optional target-revision check reports each evidence item as:

- `stable`: the same path resolves to the same blob;
- `relocated`: the blob at the same path changed, but the exact cited bytes occur exactly once;
- `ambiguous`: the exact cited bytes occur more than once in the changed target;
- `stale`: the path exists but the exact cited bytes no longer occur; or
- `deleted`: the path does not exist at the target.

For `stable` and `relocated`, `targetSpan` reports the exact target byte range. The report includes
the target resolved to an exact full commit OID as `targetRevision`. Any `stale`, `ambiguous`, or
`deleted` item makes verification fail and the reference CLI exits non-zero.

CEM 0.1 never normalizes whitespace or uses fuzzy similarity. Whitespace added before or after an
evidence span therefore does not create false staleness: the byte-identical span is relocated only
when it has one exact match. Whitespace changed inside the cited span is stale, and multiple exact
matches are ambiguous. This preserves harmless movement without turning lexical similarity into
claimed provenance.

## Security, privacy, and bounds

- Map and patch inputs are untrusted; consumers MUST NOT execute any value from them.
- Maps are local-only by default. Digests do not anonymize private repository identities.
- The reference CLI writes maps atomically with owner-only permissions, refuses a symlink as the
  final output path, and never follows a map-input symlink for implicit in-place replacement.
- Absolute paths, `..`, empty segments, backslashes, NUL, and control characters are invalid.
- A map is at most 4 MiB; a patch 8 MiB; an individual blob 64 MiB; all evidence blobs read in one
  verification 128 MiB; a path 512 UTF-8 bytes; and a reason 64 UTF-8 bytes. CEM 0.1 reasons are
  enumerated and are currently shorter than this bound.
- A map contains at most 4,096 evidence items, 2,048 hunks, and 32 bases per hunk.
- Any patch or blob passed through the LF tokenizer contains at most 262,144 lines. For a patch,
  this count includes diff metadata, file and hunk headers, and every hunk-body line.
- Wire integers are at most 9,007,199,254,740,991 so JSON implementations can preserve them
  exactly.
- The reference verifier permits at most 1,024 Git operations, 10 seconds for one Git operation,
  and 30 minutes for its Git verification budget. The cumulative deadline remains a finite
  hostile-input resource bound, but it is a hang detector rather than a performance threshold:
  the operation-count limit bounds fan-out, the per-operation timeout bounds one stuck Git child,
  and the cumulative deadline bounds many near-timeout children. The former 20-second deadline
  could instead reject a valid verification solely because a loaded host delayed otherwise bounded
  Git operations. This changes only an operational refusal boundary; no CEM wire value or frozen
  conformance vector changes.
- Git object alternates are unsupported: ambient alternate-object environment is ignored and a
  repository-local `objects/info/alternates` file causes verification to fail before object reads.
- The local Git executable, object database, repository metadata, and same-user OS integrity are
  roots of trust and MUST remain stable during one verifier call. CEM 0.1 is not a same-UID sandbox.
- CEM is not a signature format. Authenticity requires an external signed commit, CI artifact, or
  signature envelope that binds the map and exact patch.

Implementations MAY enforce lower documented operational limits but MUST distinguish those from
protocol invalidity.

## Canonical committed-change profile

Fresh `corvint cem prepare` runs emit the separate experimental `cem/0.2` profile specified in
[`specs/cem-0.2-canonical-binding.md`](specs/cem-0.2-canonical-binding.md) and shaped by
[`cem-0.2.schema.json`](cem-0.2.schema.json). It adds only the fixed
`"excludedPath":".corvint/change.cem.json"` field. `status`, `verify`, and `report` require an
independent expected base and target, derive the WP1-pinned patch themselves, and reject `--patch`.
`cem/0.3` keeps that shape and adds only the wider mechanical reason vocabulary and the optional
hunk `coverage` and `discriminates` witnesses; every canonical rule in this section applies to
both profiles.
If the sidecar exists in the target tree, its mode must be `100644` and its raw blob bytes must equal
the verified input. `prepare` is the preceding candidate phase: it ignores inherited target-side
sidecar bytes, because committing the generated map creates the final revision. Its returned
verification action uses literal `HEAD` and must be run after that commit. This closes producer-patch
and unrecorded-exclusion authority gaps without changing the frozen five-field `cem/0.1` interchange.
The reusable structural verifier labels every 0.2 result `"assurance":"structural-only"`; only the
repository workflow labels a result `"assurance":"canonical"` after independently resolving the
base and target, deriving the patch, and applying target-side artifact binding. CEM 0.1 result
envelopes remain unchanged and omit this field.

**Frozen minimum proof wire (2026-09-22).** The minimum portable proof wire is frozen as of
2026-09-22 (decision 0357): `cem/0.2` with its N-1 `cem/0.1` reader, pinned by
[`../protocol/cem-0.2/manifest.json`](../protocol/cem-0.2/manifest.json) (`CEM-CB-025`);
`ocm/0.1-experimental`, pinned by `conformance/ocm-v0/` (`OCM-V0-014`, `OCM-V0-015`); and
`frontier/0` with `frontier-error/0`, pinned by `conformance/frontier-v0/` (`CF-V0-034`). Each
packet pins its vector files by SHA-256 and pins the stable, relocated, stale, ambiguous, deleted
and unknown states of this section's drift rules. A frozen vector never changes: a wire change takes
a new exact `spec` identifier, whose reader must still read the frozen N-1 vectors. `cem/0.3` is not
part of the frozen minimum and keeps its experimental status. The freeze fixes conformance bytes; it
does not promote any profile.

## Minimal integration

The complete producer/verifier surface is deliberately four operations:

```text
begin(base revision, exact patch) -> hunk identities
cite(hunk, blob path, byte span, typed relation) -> updated map
mark(hunk, unknown|mechanical, enumerated reason) -> updated map
verify(map, exact patch, repository, optional target revision) -> report
```

Corvint is a reference producer and verifier, not a required retriever or runtime. An agent may use
native search, an IDE, another graph, or human-selected evidence and still emit the same map.
The recommended committed-change workflow derives the normative patch itself and returns a numbered
worklist. `--lines` compiles a one-based inclusive line range into the normative byte span:

```console
corvint cem prepare --base BASE_SHA --target HEAD
corvint cem cite --map .corvint/change.cem.json --hunk 1 \
  --evidence-path docs/decision.md --lines 12:18 --relation decision
# Commit .corvint/change.cem.json before verification.
corvint cem status --map .corvint/change.cem.json \
  --expected-base BASE_SHA --target HEAD --max-unknown 0 --max-mechanical 0
corvint cem report --map .corvint/change.cem.json \
  --expected-base BASE_SHA --target HEAD --max-unknown 0 --max-mechanical 0
```

`prepare` resumes an identical valid candidate and refuses an outdated one unless `--replace` is explicit.
`cite` and `mark` accept either the full derived hunk ID or the canonical one-based worklist ordinal.
When a citation selects base bytes from a path modified by the candidate patch, `cite` obtains the
exact candidate patch from a `baseRevision`-to-`HEAD` canonical derivation or the per-worktree cache,
validates it against `patchSha256`, applies that path's complete hunk group to the base blob with the
verifier's patch simulation, and accepts the span when the simulated blob is byte-identical to the
base (`stable`) or its exact bytes occur once in the changed result (`relocated`). No occurrence
(`stale`) or multiple occurrences (`ambiguous`) in a changed result fail with
`cite-span-not-stable`; the former blanket same-path refusal no longer applies. This is a producer
precheck and does not change the verifier's frozen drift rules.
For the default patch path, later commands resolve the repository's real Git directory, including a
linked worktree whose `.git` is a gitfile; no hardcoded worktree path is required.
Strict `status` is the local completion check, `report` is the optional human view, and standalone
`verify` is the equivalent machine/CI surface; running all three locally is unnecessary.
The lower-level `begin` command accepts exact patch bytes supplied out of band. Use `mark` when a
hunk must remain explicitly unknown or when the producer requests one of the byte-verifiable
or, for Go files, structurally verifiable mechanical classifications (a structural reason
upgrades the map to `cem/0.3`):

```console
corvint cem mark --map .corvint/change.cem.json --hunk 1 \
  --disposition unknown --reason insufficient-evidence
```

Use `cover` after running the tests yourself to attach one local coverprofile as a per-hunk
witness; Corvint neither runs tests nor finds profiles, and the command writes a `cem/0.3` copy
of a canonical map to `--output`:

```console
go test -coverprofile=cover.out ./pkg/...
corvint cem cover --map .corvint/change.cem.json \
  --coverprofile cover.out --test-run 'go test -coverprofile=cover.out ./pkg/...' \
  --output .corvint/witness.cem.json
```

Use `discriminate` to ask whether the cited tests notice when a changed hunk is mutated. It
runs the sandboxed `prove --mutate` runner on at most `--max-hunks` changed Go hunks with at most
`--max-mutants` mutants each inside one `--wall-time` budget (defaults 8, 8, `10m`), pins the
run to `--target` and to the digest of the selected test files, and records a witness on every
hunk; survivors downgrade the report visibly and never fail the build:

```console
corvint cem discriminate --map .corvint/change.cem.json --target HEAD \
  --max-hunks 4 --max-mutants 8 --wall-time 5m --output .corvint/witness.cem.json
```

## Example shape

Digest values are illustrative, not a conformance vector:

```json
{
  "spec": "cem/0.1",
  "baseRevision": "1111111111111111111111111111111111111111",
  "patchSha256": "2222222222222222222222222222222222222222222222222222222222222222",
  "evidence": [{
    "id": "evidence:sha256:3333333333333333333333333333333333333333333333333333333333333333",
    "blobOid": "4444444444444444444444444444444444444444",
    "path": "docs/decision/accepted.md",
    "span": {"start": 120, "end": 188},
    "spanSha256": "5555555555555555555555555555555555555555555555555555555555555555"
  }],
  "hunks": [{
    "id": "hunk:sha256:6666666666666666666666666666666666666666666666666666666666666666",
    "path": "src/session.ts",
    "oldRange": {"start": 41, "count": 3},
    "newRange": {"start": 41, "count": 5},
    "disposition": "supported",
    "reason": "evidence-backed",
    "basis": [{
      "evidenceId": "evidence:sha256:3333333333333333333333333333333333333333333333333333333333333333",
      "relation": "specification"
    }]
  }]
}
```

## Thirty-day decision

### Proposed owner-required solo alternative

This amendment is proposed and requires owner acceptance. Replace the externally dependent adoption
condition with 30 owner-run change lanes, each independently reviewer-blind for missed-evidence
findings, using a sealed task/counting protocol. Promotion or kill may use only the resulting
reviewer-blind missed-evidence count; independent producer/consumer adoption remains
`EXTERNAL_DEPENDENT` and cannot make this local decision unfireable.

Run 30 Corvint-assisted agent-change lanes and 30 matched controls using a frozen task set and
published counting rules.

Success requires all of:

- at least 80% of participating editors emit a valid map after a one-page integration;
- median producer overhead below two seconds, excluding evidence retrieval;
- at least 70% of material hunks are legitimately `supported`;
- reviewer-found missed-evidence findings fall at least 20% versus controls;
- fewer than 5% of maps fail merge-time verification for unexplained drift; and
- at least one independent non-Corvint producer and two independent consumers pass the same
  conformance suite within one engineer-day each.

Kill or redesign 0.1 if any of these occurs:

- more than 30% of material hunks are legitimately `unknown`;
- mechanical false classification exceeds 20% under reviewer audit;
- any required independent implementation needs more than one engineer-day;
- the artifact adds more than 10% median end-to-end change latency; or
- missed-evidence reduction is below 20% after the preregistered trial.

The protocol earns expansion only after this wedge works. Test attestations, signatures,
multi-repository evidence, issue receipts, semantic entailment, and outcome sharing remain outside
0.1.
