# Local Observability Dashboard V0

Owner: Russell Lewis
Requested: 2026-08-23
Intent status: proposed (owner requested the product outcome)
Delivery status: experimental
Authoritative inputs: `AGENTS.md`, `docs/PRODUCT.md`, `docs/DOGFOOD.md`,
`docs/specs/live-proof-carrying-verification-v0.md`,
`docs/specs/pulse-snapshot-lease-v0.md`,
`docs/specs/go-live-test-provider-v0.md`,
`docs/specs/agent-harness-integration-v0.md`, and each admitted artifact's exact owning specification

## Agent digest
- Claim: A local read-only dashboard may present independent evidence axes without becoming repository, execution, or promotion authority.
- Status: proposed (owner requested the product outcome)/experimental
- Exists: `internal/dashboard` and an experimental P0 CLI snapshot; no qualified graphical dashboard exists.
- Blocked on: owner acceptance, a UI shape brief, policy amendments, and a frozen browser/platform matrix for P1.
- Read next: User and measurable job; Verified current state and gap; Independent truth axes.

## User and measurable job

An engineer or coding agent operating Corvint should be able to open one local surface and determine:

1. what Corvint data exists, which revision and authority it represents, and what is stale or absent;
2. which retained artifacts show Corvint use and which total-activity denominators were never observed;
3. whether queries, evidence compilation, CEM/OCM closure, Pulse, live verification, harness adapters,
   and Beamfall shadow dogfood have admitted evidence at their actual delivery stage; and
4. which exact artifact and verifier explains every aggregate before the operator acts on it.

The dashboard is an inspector, not an authority. It is useful only if an operator can reach a cited,
revision-bound fact faster than by opening individual receipts. It fails its job if an attractive
chart hides an unavailable denominator, stale or mixed worktree state, rejected evidence, or a
repository-owned gate.

The owner has selected a local dashboard as product direction. This document freezes the P0 data and
proof contract and the proposed P1 local-server contract. P0 is an experimental CLI snapshot and does
not add a graphical interface. P1 frontend work remains blocked until all of the following are true:

1. the owner explicitly accepts this capability contract;
2. the owner confirms a separate UI shape brief;
3. `AGENTS.md` and `docs/PRODUCT.md` receive a scoped owner-approved amendment permitting an optional
   local read-only dashboard while retaining their hosted UI, telemetry, and automatic-collection
   prohibitions; and
4. the supported browser and operating-system qualification matrix is frozen.

## Verified current state and gap

- `internal/dashboard/` contains 76 Go files: an experimental implementation exists, but this
  proposed contract's qualification and outcome gates remain unproven.

- The only production-validated durable usage source today is
  `.context-corvint/traces/<revision>.jsonl`. It records explicit outcomes and paths against revisions,
  but has no event time, latency, or complete denominator of all Corvint activity. Dashboard output may
  expose counts and outcomes, never task text, commands, or paths.
- Stored query and impact envelopes contain useful bounded fields but have no strict stored-envelope
  verifier, observation timestamp, latency, or full-call denominator. Until that verifier exists they
  are unsupported input, not usage receipts.
- CEM and OCM are declared only as one explicit profiled bundle with independently caller-supplied
  expected base and target. The V0 adapter is not delivered and reads no bytes; future checking must
  preserve that binding. Default-path discovery would hide pairing authority.
- Pulse, harness, Go live-test, and Change Frontier have no durable, dashboard-qualified receipt
  source at this base. Protocol transcripts, command stdout retained without an accepted storage
  profile, and conformance fixtures are not usage data.
- The Markdown specification index has no strict parser. Beamfall artifacts are advisory experiments
  and its canonical gate ledger remains exclusively Beamfall-owned.
- There is no accepted dashboard snapshot, browser UI, usage journal, or dashboard conformance suite.
  There is no hosted telemetry, account, repository upload, background collection, database, or
  dashboard authority claim.

## Independent truth axes

No single field may collapse validity, epistemic class, authority, completeness, currency, or
delivery. Every source and metric carries the applicable axes below.

| Axis | Closed values | Meaning |
|---|---|---|
| `validity` | `VALID`, `INVALID`, `NOT_PRESENT`, `INACCESSIBLE`, `UNSUPPORTED`, `DISABLED`, `EXPIRED` | Whether admitted bytes exist and pass their exact verifier and bounds. |
| `epistemicClass` | `OBSERVED`, `DECLARED`, `ADVISORY`, `NOT_OBSERVED` | Whether the value was measured, copied from a declaration, derived from non-authoritative evidence, or unavailable. |
| `authorityClass` | `REPOSITORY_ACCEPTED`, `OWNING_VERIFIER`, `PROVIDER_QUALIFIED`, `ADAPTER_QUALIFIED`, `CALLER_REPORTED`, `ADVISORY`, `NONE` | The strongest authority that actually supports the value. |
| `completeness` | `COMPLETE`, `PARTIAL`, `UNKNOWN` | Whether the stated denominator is closed over the metric's exact cohort and window. |
| `currency` | `VALIDATED_AT`, `HISTORICAL`, `STALE`, `MIXED`, `UNKNOWN` | Temporal relation to the recorded observation boundary. `CURRENT` is not a dashboard value. |
| `deliveryStage` | `ACCEPTED`, `VALIDATED`, `IMPLEMENTED`, `EXPERIMENTAL`, `NOT_STARTED`, `FAILED`, `UNSUPPORTED` | Exact stage from an accepted contract or the compiled adapter registry. |

Authority strength is the table order from strongest to weakest. A derived metric uses the weakest
contributing authority. `DECLARED` does not imply acceptance: a proposed repository declaration is
`DECLARED` with authority `ADVISORY`. A failed owning verifier makes the source `INVALID`; it cannot
contribute to a success numerator. A valid partial count may exclude an invalid source only when the
exclusion is visible and `completeness` is `PARTIAL`.

`0` is measured only when the admitted source set, cohort, and denominator are `COMPLETE`. Missing,
invalid, inaccessible, unsupported, disabled, expired, or unretained input produces `NOT_OBSERVED`,
`PARTIAL`, or `INVALID`, never zero or success. A dashboard snapshot is historical immediately after
its `VALIDATED_AT` scan boundary. It never creates `PASSED`, `SAFE`, `READY_TO_MERGE`, whole-program
correctness, or market validation.

## Staged product surface

P0 is the Go snapshot compiler only:

```text
corvint-dashboard-snapshot snapshot --root ROOT \
  [--source ADAPTER_ID=RELATIVE_PATH]... \
  [--cem-ocm-profile cem/0.1+ocm/0.1|cem/0.2+ocm/0.1 \
   --cem RELATIVE_PATH --ocm RELATIVE_PATH \
   --expected-base OBJECT_ID --target OBJECT_ID]
```

The five CEM/OCM flags are all-or-none, each may occur exactly once, and their relative CLI order is
irrelevant. P0 writes one canonical snapshot to stdout and starts no server. Ordinary invocation uses
the process clock for acquisition boundaries, reads `generatedAt` once after the scan, and records
`clockSource: PROCESS`. The only deterministic black-box conformance invocation is:

```text
corvint-dashboard-snapshot snapshot --conformance \
  --generated-at RFC3339_NANO_UTC --root ROOT \
  [--source ADAPTER_ID=RELATIVE_PATH]... \
  [--cem-ocm-profile cem/0.1+ocm/0.1|cem/0.2+ocm/0.1 \
   --cem RELATIVE_PATH --ocm RELATIVE_PATH \
   --expected-base OBJECT_ID --target OBJECT_ID]
```

`--conformance` and `--generated-at` are required together. Either flag supplied without the other
is a usage error that emits no snapshot; ordinary invocation rejects both flags individually and
does not admit a caller-supplied clock. The conformance invocation records `clockSource: CALLER`;
its fixed clock supplies every scan/acquisition timestamp sample, so all such samples equal the
supplied value. That time has no authority beyond presentation and acquisition-boundary evidence and
cannot create source currency. There is no
environment variable, build tag, test-process detection, hidden flag, config file, or other ambient
bypass for clock injection. P1 exposes neither flag and uses its process clock.

### CLI argument and path grammar

Every P0 path-bearing argument is bounded before any filesystem or repository operation. `ROOT`,
each `RELATIVE_PATH` extracted from `--source ADAPTER_ID=RELATIVE_PATH`, `--cem`, and `--ocm` MUST be
a nonempty valid UTF-8 string of 1 through 4,096 bytes inclusive and contain no NUL or Unicode
control character. The bound is the Go byte length of the argument value itself: 4,096 is accepted
and 4,097 is `DASHBOARD_INVALID_ARGUMENT`. The adapter ID and `=` in a `--source` value do not count
toward the relative-path bound; `ADAPTER_ID` is independently limited to 128 ASCII bytes and exactly
`[a-z0-9]+(?:-[a-z0-9]+)*`. The `--cem-ocm-profile` value is bounded to 4,096 bytes before any
filesystem or repository operation and MUST then equal exactly `cem/0.1+ocm/0.1` or
`cem/0.2+ocm/0.1`. `--expected-base`, `--target`, and `--generated-at` use their fixed grammars above
and are not path arguments. Any of the five CEM/OCM flags without all other four, a duplicate, or a
profile outside that closed set is `DASHBOARD_INVALID_ARGUMENT` before any path open.

`ROOT` MUST already be an absolute, lexically clean native path: under the target's Go 1.27.0
`path/filepath` semantics, `IsAbs(ROOT)` is true and `Clean(ROOT)` is byte-for-byte `ROOT`. It is
never silently made absolute or cleaned. Every other path value above is a byte-exact valid-UTF-8,
repository-relative POSIX path using `/`; it has no native or POSIX absolute/volume prefix,
backslash, empty component, `.` component, `..` component, or lexical-cleaning difference. On
POSIX, a non-UTF-8 argv byte
sequence is rejected. On Windows, the bound is measured after Go's command-line conversion to the
UTF-8 string and native absoluteness/cleanliness is evaluated with Windows `path/filepath` rules.
Only `darwin`, `linux`, and `windows` can be qualified in V0; another target is `UNSUPPORTED` until
its argument, rooted-filesystem, file-identity, and process-containment vectors pass.

The independent trace-conformance command applies the same exact `ROOT` rule to `--root` and the
same 1-through-4,096-byte valid-UTF-8/no-control bound to `--manifest` and `--snapshot`; those two
values MUST also be absolute, lexically clean native paths. Argument-bound vectors perform no path
open and prove exact acceptance at 4,096 and rejection at 4,097 on every qualified platform.

P1 may add:

```text
corvint dashboard serve --root ROOT [the same source flags except --generated-at] [--open]
```

P1 is an explicitly launched, terminating loopback process. It never starts at login, in the
background, from a Corvint read command, or from a harness event. `--open` is the only permission to
ask the operating system to open a browser. The proposed information architecture is `Overview`,
`Evidence`, `Queries`, `Verification`, `Frontier`, `Beamfall`, and `Privacy`; no frontend file may be
implemented before the P1 gates above pass.

## Compiled adapter registry and exact admission order

The binary contains canonical `corvint-dashboard-adapter-registry/0`. Each entry contains exactly
`adapterId`, `acceptedProfiles`, `defaultLocation`, `deliveryStage`, `issueCodes`, `maxBytes`,
`sourceKind`, and `verifierId`. Entries sort by `adapterId`; the registry's canonical SHA-256 is in
every snapshot. Artifact bytes cannot supply a verifier command, schema, authority, or derivation.
A source-scoped issue's `code` MUST be one of its source adapter's `issueCodes`; the compiler and the
snapshot-only verifier both reject any other code, even one in the global issue-code table.
`adapterRegistrySha256` is `sha256:` plus
`SHA-256(UTF8("corvint-dashboard-adapter-registry/0") || 0x00 || canonical(registry))`.

Implementation and admission order is fixed. A later adapter cannot bypass an earlier acquisition or
identity failure:

1. validate the adapter ID and delivery state without opening its optional path, then build one
   bounded stable-read envelope for each configured supported path;
2. validate the durable local trace store;
3. compile an explicitly declared CEM/OCM bundle as `UNSUPPORTED` without opening either path;
4. admit stored query/impact envelopes only after their strict verifier is implemented;
5. admit a strict specification index only from the exact verified HEAD blob after its parser is
   accepted; and
6. admit Beamfall only through a separately accepted advisory validator that cannot read or affect
   its canonical gate ledger.

The compiled registry is:

| Adapter ID | Input/profile and verifier | Location/admission | Base-state computation |
|---|---|---|---|
| `stable-read-v0` | internal `dashboard-stable-read/0` / `go-stable-read-v0` | wraps every configured source | byte count/digest only after safe complete read; never a usage source |
| `local-trace-v1` | trace schema v1 / `go-local-trace-v1` | default exact worktree directory `.context-corvint/traces` | production-validated durable input: retained row count, revision, explicit outcome only |
| `cem-ocm-bundle-v0` | caller-declared `cem/0.1+ocm/0.1|cem/0.2+ocm/0.1` / `go-cem-ocm-bundle-v0` | explicit five-flag bundle only; no default paths or reads | `NOT_STARTED`; `UNSUPPORTED` source and `NOT_OBSERVED` metrics |
| `query-envelope-v1` | no accepted stored-envelope profile / `unsupported` | explicit only; no defaults | `UNSUPPORTED`; every metric `NOT_OBSERVED` until strict verifier delivery |
| `impact-envelope-v1` | no accepted stored-envelope profile / `unsupported` | explicit only; no defaults | `UNSUPPORTED`; every metric `NOT_OBSERVED` until strict verifier delivery |
| `head-spec-index-v0` | no accepted canonical parser / `unsupported` | exact verified HEAD blob only, never worktree Markdown | `UNSUPPORTED`; no row counting or stage inference |
| `beamfall-shadow-v0` | no accepted advisory receipt profile / `unsupported` | explicit only after cross-repository contract | `UNSUPPORTED`; every metric `NOT_OBSERVED` |
| `pulse-dogfood-v0` | no delivered durable receipt profile / `unsupported` | none | `UNSUPPORTED`; protocol transcripts rejected as usage |
| `harness-usage-v0` | no delivered durable receipt profile / `unsupported` | none | `UNSUPPORTED`; retained stdout files are not inferred receipts |
| `go-live-usage-v0` | no accepted durable storage profile / `unsupported` | none | `UNSUPPORTED`; ephemeral/streamed results are not usage |
| `frontier-usage-v0` | no accepted durable storage profile / `unsupported` | none | `UNSUPPORTED`; command output is not discovered by filename |

The table is explanatory. The exact initial canonical registry body is the JSON array below. Object
keys and rows are already in canonical order; issue-code arrays are sorted. `maxBytes: "0"` means the
adapter cannot admit bytes in this registry version: its source carries neither `byteCount` nor
`contentSha256`, and the compiler and the snapshot-only verifier both reject either field, even
`"0"`.

```json
[
  {"acceptedProfiles":[],"adapterId":"beamfall-shadow-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"BEAMFALL_SHADOW","verifierId":"unsupported"},
  {"acceptedProfiles":["cem/0.1+ocm/0.1","cem/0.2+ocm/0.1"],"adapterId":"cem-ocm-bundle-v0","defaultLocation":null,"deliveryStage":"NOT_STARTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"CEM_OCM_BUNDLE","verifierId":"go-cem-ocm-bundle-v0"},
  {"acceptedProfiles":[],"adapterId":"frontier-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"FRONTIER_RECEIPT","verifierId":"unsupported"},
  {"acceptedProfiles":[],"adapterId":"go-live-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"GO_LIVE_RECEIPT","verifierId":"unsupported"},
  {"acceptedProfiles":[],"adapterId":"harness-usage-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"HARNESS_RECEIPT","verifierId":"unsupported"},
  {"acceptedProfiles":[],"adapterId":"head-spec-index-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"HEAD_SPEC_INDEX","verifierId":"unsupported"},
  {"acceptedProfiles":[],"adapterId":"impact-envelope-v1","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"IMPACT_ENVELOPE","verifierId":"unsupported"},
  {"acceptedProfiles":["corvint-local-trace/1"],"adapterId":"local-trace-v1","defaultLocation":".context-corvint/traces","deliveryStage":"NOT_STARTED","issueCodes":["OBSERVATION_TIME_UNKNOWN","REPOSITORY_OBJECT_UNAVAILABLE","SOURCE_CHANGED_DURING_READ","SOURCE_INACCESSIBLE","SOURCE_INVALID_IDENTITY","SOURCE_INVALID_SCHEMA","SOURCE_MULTILINK_UNQUALIFIED","SOURCE_NOT_PRESENT","SOURCE_OVERSIZED","SOURCE_SPECIAL_FILE","SOURCE_SYMLINK","STORE_CHANGED","TRACE_ANCESTRY_BOUND","TRACE_STORE_BOUND","UNSUPPORTED_OBJECT_ALTERNATES","VERIFIER_REJECTED"],"maxBytes":"16777216","sourceKind":"LOCAL_TRACE_STORE","verifierId":"go-local-trace-v1"},
  {"acceptedProfiles":[],"adapterId":"pulse-dogfood-v0","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"PULSE_RECEIPT","verifierId":"unsupported"},
  {"acceptedProfiles":[],"adapterId":"query-envelope-v1","defaultLocation":null,"deliveryStage":"UNSUPPORTED","issueCodes":["SOURCE_UNSUPPORTED"],"maxBytes":"0","sourceKind":"QUERY_ENVELOPE","verifierId":"unsupported"},
  {"acceptedProfiles":["dashboard-stable-read/0"],"adapterId":"stable-read-v0","defaultLocation":null,"deliveryStage":"NOT_STARTED","issueCodes":["SOURCE_CHANGED_DURING_READ","SOURCE_INACCESSIBLE","SOURCE_MULTILINK_UNQUALIFIED","SOURCE_NOT_PRESENT","SOURCE_OVERSIZED","SOURCE_SPECIAL_FILE","SOURCE_SYMLINK"],"maxBytes":"16777216","sourceKind":"INTERNAL","verifierId":"go-stable-read-v0"}
]
```

The registry hash preimage is the whitespace-free canonical encoding of the parsed JSON array above,
without a trailing LF. No prose table participates. A registry stage/profile/verifier/limit change
changes the profile version and conformance vectors before code.

At this base, only `local-trace-v1` can yield observed durable product data. Registry rows for later
adapters MUST remain present so their unavailable metrics render `NOT_OBSERVED`, but their
`deliveryStage` is `NOT_STARTED` or `UNSUPPORTED`. An implementation cannot advertise P0 support for
an adapter until this spec and its traceability row name the passing verifier evidence.

Default worktree and Git-dir locations are resolved only after the frozen repository authority proves
the reciprocal worktree/Git-dir relationship. A missing trace directory produces a `NOT_PRESENT`
source row. Repeated `--source` values add exact files for a named supported adapter and sort by
adapter ID then relative-path bytes before paths are discarded. Duplicate adapter/path pairs fail
arguments. Directories are forbidden except `local-trace-v1`, which enumerates only immediate regular
files whose names match the owning exact revision-filename grammar; it never recurses.

Adding a profile, location, field, or derivation changes the registry version or this spec first.
Unrecognized profiles are `UNSUPPORTED`; similar filenames or JSON shapes never select an adapter.

### Stable-read envelope

`stable-read-v0` produces one internal immutable object containing exactly:

```text
adapterId, byteCount, configuredOrdinal, contentSha256, end,
fileIdentityBefore, fileIdentityAfter, start, validity
```

It contains no path and is not serialized as product usage. The adapter verifier receives only this
object, its immutable bounded byte slice, and the independently probed repository authority described
below.
`fileIdentityBefore` and `fileIdentityAfter` are exactly
`{mode,modTimeNanoseconds,platformIdentity,size}`. POSIX `platformIdentity` is exactly
`{device,inode,linkCount}`; Windows is exactly `{fileIndex,linkCount,volumeSerialNumber}`. Every value
except `modTimeNanoseconds` is a canonical unsigned decimal string; `modTimeNanoseconds` is a
canonical signed decimal string. Other platforms report `UNSUPPORTED` until an equivalent identity
profile and adversarial corpus are accepted. `start` is sampled from the snapshot clock immediately
before the first descriptor `fstat`; `end` is sampled from it immediately after the successful
second read's final `fstat`. They are UTC RFC3339 strings with exactly nine fractional digits;
black-box conformance mode's fixed clock makes both equal the supplied `--generated-at` value. A
successful retry discards every
byte, identity, and time from the failed attempt. `configuredOrdinal` is the product source ordinal;
all member reads of the default trace store therefore carry `"0"` and remain distinguishable only
inside the aggregate body below.

### Local trace-store aggregate

The default `local-trace-v1` input is one configured aggregate source, never an opaque directory and
never one product source per filename. Its no-follow directory descriptor is enumerated before any
member read and again after all member reads. Each enumeration is limited to 1,000 immediate entries
and forms a canonical array of exact `{fileIdentity,name}` objects sorted by raw UTF-8 `name` bytes;
`fileIdentity` has the platform shape above and is obtained without following the entry. Every
immediate entry participates in the stability set even when its name is not a trace candidate.
Addition, removal, rename, or identity replacement makes the sets differ. The implementation retries
the entire directory enumeration and all member acquisitions once; a second difference makes the
aggregate `INVALID`, contributes no trace metric, and emits `STORE_CHANGED`.
The store directory's descriptor identity and the rooted path-to-identity binding are also captured
before and after the scan; either changing is the same `STORE_CHANGED` failure. Enumeration, entry
identity, and member opens are descriptor-relative to that directory and never follow a name through
a symlink.

A trace candidate name is exactly `<revision>.jsonl`. For repository object format `sha1`,
`revision` is exactly 40 lowercase hexadecimal characters; for `sha256`, it is exactly 64. It
semantically names the Git commit object against which every row in that file was recorded: it is
not a tree ID, current-HEAD alias, branch, map-derived value, or filename label. The repository
authority MUST prove that exact commit object and every row revision. Non-candidate immediate names
are ignored after participating in the bounded stability set; such a name may hold any byte the
platform permits in one name component, such as `\` on POSIX, and is never resolved as a path. A
name holding the platform separator, `/`, NUL, or invalid UTF-8 is not an immediate name and fails
the store. A candidate that is not a qualified
single-link regular file is rejected under the stable-read rules.

After stable reads and verification, the aggregate body is the canonical JSON array of exact rows:

```text
{revision,contentSha256,byteCount}
```

Rows sort by raw revision bytes. `contentSha256` is the admitted member's stable-read digest and
`byteCount` is its canonical unsigned decimal byte count. The aggregate source's `members` field is
this exact array, its `byteCount` is the checked sum of all member `byteCount` values, and its
`contentSha256` is `sha256:` plus:

```text
SHA-256(UTF8("trace-store/0") || 0x00 || canonical(members))
```

The aggregate source has `configuredOrdinal: "0"`, `profile: "corvint-local-trace/1"`, and
`observationTime: null`; acquisition time is not source-owned event time. Its `cohortIds` is the
sorted unique set of revision-cohort IDs for admitted members. Each such cohort uses that member's
revision, successful stable-read `start` as `sourceObservationStart`, successful stable-read `end` as
`sourceObservationEnd`, and authority-qualified tree/object fields. The aggregate observation
interval is the minimum admitted stable-read `start` and maximum admitted stable-read `end`; both are
null for an empty admitted set. Its `repositoryReadsSha256` binds the exact semantic witness set
below for every admitted member. These aggregate values, public `members`, and public
cohorts make the source ID and every per-revision artifact metric independently recomputable.

If every candidate is admitted, the aggregate is `VALID`/`COMPLETE`, including an unchanged empty
store. If at least one candidate is admitted and another is rejected, it is `VALID`/`PARTIAL`, the
rejected member contributes no success numerator, and the compiled issue ID appears in `exclusions`.
If candidates exist but none is admitted, the aggregate is `INVALID`/`PARTIAL` with no cohorts or
trace metrics. A store-set change is always `INVALID`, not a partial aggregate. No failed member's
bytes or path enters `members`, an ID, an issue, or a metric.
For a stable enumeration, compiled acquisition/verifier precedence assigns each rejected candidate
exactly one member-terminal issue. A member-terminal issue is recognizable from public bytes only
when its `code` belongs to this closed set and `observed` is a nonzero decimal string:

```text
REPOSITORY_OBJECT_UNAVAILABLE, SOURCE_CHANGED_DURING_READ, SOURCE_INACCESSIBLE,
SOURCE_INVALID_IDENTITY, SOURCE_INVALID_SCHEMA, SOURCE_MULTILINK_UNQUALIFIED, SOURCE_OVERSIZED,
SOURCE_SPECIAL_FILE, SOURCE_SYMLINK, TRACE_ANCESTRY_BOUND, VERIFIER_REJECTED
```

For each code present, exactly one issue has `severity: ERROR`, the aggregate trace source ID as
`sourceId`, the number of candidates assigned that terminal code as `observed`, and `limit: null`.
Those issue IDs, and only those issue IDs, enter the aggregate source's `exclusions` and the invalid
`RETAINED_MEMBER` count row. Summing their `observed` values exactly recomputes the rejected-member
count without revealing names. A code outside this set MUST NOT contribute to that count even when
some other issue family uses a numeric `observed` field.

The closed non-member trace controls are:

| Code | Exact role and public fields |
|---|---|
| `SOURCE_NOT_PRESENT` | missing aggregate store only; `WARNING`, `observed: null`, `limit: null` |
| `SOURCE_INACCESSIBLE`, `SOURCE_SPECIAL_FILE`, `SOURCE_SYMLINK`, `SOURCE_MULTILINK_UNQUALIFIED` | aggregate store-root failure only when `observed: null`; `ERROR`, `limit: null`; the same code is member-terminal only with nonzero `observed` |
| `TRACE_STORE_BOUND` | aggregate entry/row/row-byte/store-byte bound; `ERROR`, `observed: null`, and `limit` equal to the first crossed bound (`1000`, `262144`, or `16777216`) |
| `UNSUPPORTED_OBJECT_ALTERNATES` | authority-initialization control only; never member-terminal and never serialized in a successfully materialized V0 snapshot because failure precedes a qualified initial repository identity |
| `STORE_CHANGED` | unstable candidate universe; `ERROR`, `observed: null`, `limit: null` |
| `OBSERVATION_TIME_UNKNOWN` | exactly one `INFO` issue for a stable `VALID` or `PARTIAL` trace aggregate, because V1 traces own no event time; `observed: null`, `limit: null`; it changes no validity or completeness axis |

Every row in the table uses the aggregate trace source ID when a snapshot source exists. Candidate
disappearance never becomes `SOURCE_NOT_PRESENT`: the post-enumeration mismatch becomes
`STORE_CHANGED`. A pre-qualification repository failure, including alternates, is the fatal
`DASHBOARD_REPOSITORY_UNAVAILABLE` and emits no snapshot issue.
For a stable `VALID`/`PARTIAL` aggregate, `source.exclusions` is exactly the sorted terminal issue-ID
set; `OBSERVATION_TIME_UNKNOWN` is an issue but not an exclusion. For an aggregate-root or aggregate-
override failure, `source.exclusions` contains exactly its one winning error issue ID. A missing
store uses its one `SOURCE_NOT_PRESENT` issue ID. No non-winning or merely provisional issue is
serialized.

Precedence is total and fail-fast. Within one candidate the first applicable step below wins and no
later step is evaluated for classification:

1. classify the no-follow entry as symlink, then non-regular/special, then unqualified multi-link;
2. open/read classification is inaccessible, declared/observed oversize, then two-read instability;
3. enforce the aggregate 1,000-row, 256-KiB-row, and 16-MiB-store limits; line framing retains at most 1,001 row descriptors and stops at the first row-count crossing;
   any crossing is non-member `TRACE_STORE_BOUND` and discards all provisional state, while descriptor memory never scales with excess rows;
4. parse candidate files by revision-byte order and rows by file order; within a row check UTF-8/JSON,
   the exact closed field set and schema version, revision, task, `opened_paths`, `changed_paths`,
   `verification`, `outcome`, then `trace_id`; malformed shape/schema is `SOURCE_INVALID_SCHEMA`, a
   revision or trace-ID contradiction is `SOURCE_INVALID_IDENTITY`, and a secret/forbidden value,
   invalid byte-exact path/command value, invalid outcome, or duplicate trace ID is
   `VERIFIER_REJECTED`;
5. qualify the exact revision commit/tree identity, then ancestry, historical terminal path entries,
   and the terminal-blob existence batch below; identity contradiction is
   `SOURCE_INVALID_IDENTITY`, non-ancestry/distance is `TRACE_ANCESTRY_BOUND`, an absent/non-regular/
   over-bound terminal blob is `VERIFIER_REJECTED`, and a missing/promised object or failed exact
   batch response is member-terminal `REPOSITORY_OBJECT_UNAVAILABLE`; and
6. a batch-wide grammar, truncation, extra-output, process, containment, output, or deadline failure
   assigns `REPOSITORY_OBJECT_UNAVAILABLE` once to every candidate that references at least one OID
   in that failed batch; each candidate is still counted once.

After provisional candidate processing, aggregate override priority is `STORE_CHANGED`, then
`TRACE_STORE_BOUND`; the winning override discards all
provisional members, cohorts, terminal counts, and trace metrics and is the only aggregate-error
issue. If no override wins, member-terminal counts are retained and
`OBSERVATION_TIME_UNKNOWN` is added last. Initial repository failure and final repository-identity
drift retain their separately frozen fatal/retry behavior and outrank this entire adapter sequence.

### Verifier execution contract

P0 verifier IDs are compiled in-process Go dispatch entries and pass the owning public conformance
vectors. A verifier cannot open an arbitrary path, consult ambient environment or configuration,
spawn a process, write, fetch, or return free-form error text. Its result is a closed normalized
structure or one compiled issue code. Its sole non-value capability is the compiled, bounded,
read-only repository-authority interface below. The verifier invokes that Go interface in-process;
it never selects an executable, constructs an argument, observes a repository path, or receives Git
stderr.

The repository-authority implementation is the sole V0 subprocess exception. It MUST use the
upstream Git CLI under the exact profile below because Git is the repository object, layout, and
reachability authority; this does not make Git a verifier subprocess. V0 MUST NOT contain or fall
back to a second loose-object, pack/index, commit/tree, historical-path, or reachability parser in Go
or another language. No adapter, registry row, artifact, manifest, environment value, or browser
request can select or extend the Git operations.

#### Repository-authority Git execution profile

The profile is exactly `corvint-dashboard-git-authority/0`. Before the first repository probe, Corvint
inspects the process startup environment. Using the platform's environment-name comparison
(case-sensitive on Unix and case-insensitive on Windows), any startup entry named
`GIT_OBJECT_DIRECTORY` or `GIT_ALTERNATE_OBJECT_DIRECTORIES` whose value has nonzero byte length
makes the repository authority unavailable before executable or repository discovery. An absent or
empty entry is accepted, but neither inherited entry is copied to a child.

Before the first repository probe, Corvint
resolves the literal executable name `git` exactly once with Go `exec.LookPath` against the parent
process's startup `PATH`, makes a relative result absolute against the process startup directory,
resolves its symlink chain, and thereafter executes only that absolute result. Resolution failure,
an unresolved link, a non-regular executable, a file larger than 256 MiB, or unsupported executable
file identity makes the repository unavailable. An executable qualification probe opens that
resolved path without following its final component, keeps the descriptor open, and performs this
exact sequence: `fstat`; read 65,536-byte chunks through EOF with a 268,435,457-byte overflow guard
while computing byte count and SHA-256; `fstat`; rewind the same descriptor; `fstat`; repeat the
complete bounded chunked read and digest; final `fstat`. All four identities, both byte counts, and
both digests MUST match, the file MUST be regular and no larger than 268,435,456 bytes, and its path-
to-descriptor binding MUST still match. The private baseline is exactly resolved path, identity,
byte count, and digest; executable bytes are not retained or serialized.

The complete qualification probe runs before the first repository operation and as the final
operation within Finish.
Immediately before each child `Start` and immediately after its leader `Wait`, Corvint `fstat`s the
held descriptor and reopens/re-identifies the resolved path without following the final component;
both MUST match the baseline. The reopen walks every ancestor directory with the directory-binding
sequence of `corvint-dashboard-git-layout/0` below, so an ancestor is re-identified by directory
identity only: its modification time, size, and link count are not drift evidence, because creating
or removing a sibling entry changes them without changing what the path resolves to (decision
0175). The executable file itself keeps the complete baseline identity and byte-count comparison,
so replacement by rename or rewrite is still drift. Any executable drift invalidates the whole authority, never one
candidate. Replacement by a malicious same-UID actor between the last pre-start probe and the
kernel's executable open remains outside the declared threat boundary.

Every non-cat-file child has closed stdin; cat-file receives only its exact batch above. Every child
uses the resolved executable path as `argv[0]`. Except for existing
values of `SystemRoot`, `TMPDIR`, `TEMP`, `TMP`, and `USERPROFILE`, omitted when unset, its environment
contains exactly:

```text
LANG=C
LC_ALL=C
GIT_CONFIG_NOSYSTEM=1
GIT_CONFIG_GLOBAL=<the platform os.DevNull path>
GIT_CONFIG_SYSTEM=<the platform os.DevNull path>
GIT_TERMINAL_PROMPT=0
GIT_OPTIONAL_LOCKS=0
GIT_NO_LAZY_FETCH=1
GIT_NO_REPLACE_OBJECTS=1
GIT_GRAFT_FILE=<the platform os.DevNull path>
GIT_ALTERNATE_OBJECT_DIRECTORIES=
GIT_PROTOCOL_FROM_USER=0
GCM_INTERACTIVE=never
GIT_ASKPASS=
SSH_ASKPASS=
```

The inherited OS-bootstrap names appear first in the order shown and the fixed rows follow in the
order shown. `GIT_OBJECT_DIRECTORY` is always omitted rather than exported with an empty value;
`GIT_ALTERNATE_OBJECT_DIRECTORIES` is always the fixed empty row shown. `PATH`, HOME, XDG, SSH,
proxy, credential, Git-dir/worktree, namespace, replace-ref, alternate-object, and config environment
variables are not inherited. The fixed `GIT_GRAFT_FILE` row disables `info/grafts` parent grafts,
which `GIT_NO_REPLACE_OBJECTS` does not, so a graft cannot splice a commit into HEAD ancestry.
Repository-local structural
configuration may be read by Git only to discover the object format and linked-worktree layout; it
is not accepted as product authority until the reciprocal checks below succeed. User, system,
global, command-selected, include-selected, and ambient configuration is excluded. Every invocation
has this exact common argument prefix after `argv[0]`:

```text
--no-optional-locks
--literal-pathspecs
-c core.fsmonitor=false
-c core.untrackedCache=false
-c core.excludesFile=
-c credential.helper=
-c submodule.recurse=false
-c fetch.recurseSubmodules=false
-c protocol.allow=never
-c protocol.file.allow=never
-c advice.graftFileDeprecated=false
```

Before the first Git child, Corvint performs `corvint-dashboard-git-layout/0` using only descriptor-
relative filesystem operations. It opens the clean absolute `ROOT` component by component without
following symlinks, opens the worktree directory descriptor, and records the exact platform identity
shape already frozen for stable reads. For every directory binding the sequence is parent-relative
no-follow metadata, no-follow open, descriptor `fstat`, then a second parent-relative no-follow
metadata probe; all three MUST name the same file and have equal directory identity: the platform
file identity (device and inode, or volume serial and file index), file type and permission bits,
and on POSIX owner and group. Directory modification time, size, and link count are excluded from
that comparison (decision 0175). The binding records the descriptor `fstat`'s full identity, and a
later retained-binding re-identification compares that full identity. Directories, pointer files, configuration files, and the
objects directory may not be symlinks or special files. Every file open in this layout and in the
executable probe is non-blocking, so a FIFO or device swapped in after the no-follow metadata probe
fails the descriptor regular-file check instead of blocking the open.

The immediate `.git` entry is either that qualified directory or one single-link regular file of at
most 4,096 bytes whose stable bytes are exactly `gitdir: PATH LF`. `PATH` is a nonempty valid-UTF-8,
lexically clean native absolute path or a native path relative to the pointer file's parent; it has
no NUL/control byte. Corvint resolves it component-by-component without following links and opens the
per-worktree Git-directory descriptor. An optional immediate `commondir` file in that directory has
the same stable-read/file/path grammar except its body is exactly `PATH LF`; absent means the common
Git directory is the per-worktree Git directory. Corvint opens the resulting common directory and its
immediate `objects` directory with the same binding sequence. For a `.git` pointer file, the per-
worktree Git directory MUST contain an immediate single-link regular `gitdir` file with exact stable
body `PATH LF` that resolves back to the original `.git` file identity. No pointer path or directory
may be inferred from Git output.

Before any Git child, Corvint also stable-reads the required immediate common `config` and every
existing immediate `config.worktree` under the common and per-worktree Git directories, deduplicated
by file identity. Each is single-link regular, at most 1,048,576 bytes, valid UTF-8, empty or LF-
terminated, has lines of at most 4,096 bytes, and contains no CR, NUL, backslash, or control other
than tab/LF. The deliberately narrow accepted grammar permits only blank lines; whole-line `#` or
`;` comments; `[SECTION]` or `[SECTION "SUBSECTION"]` headers with ASCII
`[A-Za-z][A-Za-z0-9.-]*` section names and an unescaped printable subsection; and, inside a section,
ASCII `[A-Za-z][A-Za-z0-9-]*` variables optionally followed by `=` and an opaque printable value.
Spaces/tabs may surround tokens; inline comments, escapes, continuations, trailing section text, and
every other form are rejected as ambiguous. ASCII-case-folded section names `include`, `includeIf`,
or either name followed by `.` are rejected before Git starts. Thus no repository-local include or
includeIf target is ever opened. An external-include canary MUST prove rejection before the first Git
child and zero opens of the planted target.

Any entry at relative `objects/info/alternates` beneath the per-worktree Git directory or common Git
directory, even empty or non-regular, is rejected by no-follow metadata before Git starts. All layout/config files use the same two-read/four-identity
stable-read algorithm; their identities, counts, and digests are private authority state.

Discovery's working directory is the caller-supplied repository root after the exact clean-absolute
argument validation above; Corvint performs no additional normalization. Its command tail is exactly
`rev-parse --path-format=absolute --show-toplevel --absolute-git-dir --git-common-dir`. Its three
path fields MUST be byte-exact valid-UTF-8, lexically clean native absolute paths which, when opened
with the same no-follow binding sequence, match the already qualified worktree, per-worktree Git,
and common Git directory identities. The command is repeated with the validated worktree as cwd and
again with the validated per-worktree Git directory as cwd plus the two arguments `--git-dir=.` and
`--work-tree=<validated-worktree>` inserted before the command tail. Both results MUST match those
same descriptor identities. The exact tail
`rev-parse --path-format=absolute --git-path objects` returns one byte-exact valid-UTF-8, lexically
clean native absolute path that MUST match the prequalified common objects descriptor. Any nonempty
alternate-object environment, different directory identity, or failed reciprocal binding is
`UNSUPPORTED_OBJECT_ALTERNATES`; Git is never allowed to follow an alternate.

After reciprocal qualification, every child cwd is the validated worktree root. Only these command
tails exist in V0, where `REVISION`, `HEAD`, and object IDs are already-qualified lowercase full IDs
of the repository's declared object format and `PATH...` is one byte-exact qualified trace-path chunk:

```text
rev-parse --show-object-format HEAD^{commit} HEAD^{tree}
status --porcelain=v1 -z --untracked-files=all --ignore-submodules=none
rev-parse --verify REVISION^{commit}
rev-parse --verify REVISION^{tree}
merge-base --is-ancestor REVISION HEAD
rev-list --ancestry-path --count --max-count=10001 REVISION..HEAD
ls-tree -z --full-tree REVISION -- PATH...
cat-file '--batch-check=%(objectname) %(objecttype) %(objectsize)'
```

The apostrophes on the final documentation line delimit one argument and are not argument bytes.
That command tail is exactly two Go `Args` elements: `cat-file` and the literal
`--batch-check=%(objectname) %(objecttype) %(objectsize)`. No other `cat-file` mode or option is
present.

Every non-`-z` successful tail returns one field per line, with exactly one LF after the last field. Before allocating line/field slices, Corvint requires the exact expected LF count;
any mismatch, CR, NUL, blank field, prefix, suffix, progress, or extra byte fails closed. Discovery and `--git-path`
fields use the byte-exact valid-UTF-8 native-path grammar above and may be non-ASCII; a `café` ROOT is
a required positive vector. Object format, object IDs, counts, types, sizes, and fixed status words
are ASCII. Discovery returns exactly three fields in the option order shown; reciprocal discovery returns the same three;
`--git-path objects`, each single-object `rev-parse --verify`, and `rev-list` return exactly one;
`rev-parse --show-object-format HEAD^{commit} HEAD^{tree}` returns exactly three. The `rev-list`
field is canonical unsigned decimal. `status -z` and `ls-tree -z` use their exact NUL-delimited
grammars, have no LF terminator, and may be empty only where the command semantics permit an empty
set. Any other stdout grammar is a closed authority failure; stderr is never used to repair it.

The first tail must return exactly object format, HEAD commit ID, and HEAD tree ID. Each ordinary
porcelain-v1 `status -z` record is exactly `XY SP PATH NUL`; a rename/copy record is exactly
`XY SP DESTINATION NUL SOURCE NUL`, and both destination and source enter the dirty set. Each status
byte is from the closed ASCII alphabet ` MTADRCU?`. `??` is the only form containing `?`; `!` is
forbidden; and `XY` cannot be two spaces. `R` or `C` in either status byte requires exactly the
second NUL-terminated source path, while every other form requires exactly one path. A missing,
surplus, empty, or impossible path frame is repository unavailable. Each status
path must be a byte-exact valid-UTF-8 repository-relative POSIX path satisfying the same lexical,
control, volume, and component restrictions as the trace-path profile, but it is not secret-screened.
Any other status byte/status/path grammar is repository unavailable. The parser deduplicates and
sorts by raw UTF-8 bytes, admits at most 100,000 unique paths, and derives only dirty count and the
frozen dirty-set digest; raw paths are discarded. Exit
`0` from `merge-base` means ancestor, exit `1` means not ancestor, and every other result is
unavailable. After ancestry succeeds, the decimal `rev-list` result is the number of commits on
ancestry paths from the revision (exclusive) to HEAD (inclusive); `10001` or more is
`TRACE_ANCESTRY_BOUND`. The commit verification must return `REVISION` exactly, so a tag ID cannot
stand in for a commit. Its tree result becomes the cohort tree revision.

Trace paths already satisfy `corvint-dashboard-trace-path-witness/0`. They are sorted and deduplicated,
then charged to a cumulative historical-path ledger keyed by `(revision, raw UTF-8 path bytes)`.
The first request for a key consumes one of 100,000 reservations even when the path/member is later
rejected; repeated use and caching cannot evade the charge. Within one attempt each key is looked up
once and its closed result may serve dependent candidates. A retry re-executes the lookup because
failed-attempt evidence is discarded, but the reservation remains charged.
Paths are
then split on every platform into the longest sorted prefix containing at most 64 paths and at most
8,192 raw UTF-8 bytes across the path arguments; neither bound is platform-dependent. Before every
Windows child start, Corvint applies Go 1.27's exact `os/exec` argument quoting, encodes the resulting
command line, resolved executable path, and validated cwd/worktree root as UTF-16, adds one
terminating code unit to each, and requires their combined count to be at most 32,000 UTF-16 code
units. A crossed chunk or Windows preflight bound is closed repository-authority unavailability;
Corvint never attempts an over-bound `CreateProcess`. `ls-tree` must
return exactly one NUL-delimited row of
`MODE SP TYPE SP OBJECT_ID TAB PATH NUL` for each requested path and no other row. Each row's returned
path must byte-match the request, its mode must be `100644` or `100755`, type must be `blob`, and
object ID must match the repository format. Thus intermediate components must resolve as trees and
the terminal entry names an exact historical regular blob. Missing/deleted paths, duplicate output,
trees, symlinks, gitlinks, tags, and malformed output reject that member. `ls-tree` metadata alone
does not prove that the terminal blob object is locally available, so its full object ID is only
provisional until the following batch check succeeds.

The cumulative cat-file ledger contains the snapshot HEAD commit ID, every provisional trace
revision commit ID, and every provisional terminal blob ID. Its key is the full lowercase object ID
paired with the repository's one qualified object format. Each key has exactly one expected type:
`commit` for HEAD/trace revisions and `blob` for terminal paths. A later use of one key with another
expected type is `SOURCE_INVALID_IDENTITY`. The first request consumes one of 100,000 reservations,
including rejected work; repeated use/caching cannot evade the charge. Within each attempt Corvint
checks each key once, deduplicates, and sorts by raw object-ID bytes. A retry reruns the check because
failed-attempt evidence is discarded, while the cumulative reservation remains charged.

The sorted ledger is divided into the longest prefixes containing at most 50,000 IDs and at most
4,194,304 stdin bytes. A batch stdin is exactly one full ID plus LF per row, in that order, with no
blank row, CR, NUL, abbreviation, expression, path, prefix, or suffix; stdin closes immediately after
the final LF. Across both scan attempts there are at most 100,000 cat-file-ledger reservations
and 8,388,608 cat-file stdin bytes, including rejected and discarded work.

For every input, `cat-file` MUST return in the same order exactly one line:

```text
OBJECT_ID SP OBJECT_TYPE SP OBJECT_SIZE LF
```

`OBJECT_ID` must byte-equal the input and `OBJECT_TYPE` must byte-equal that ledger key's expected
`commit` or `blob`. `OBJECT_SIZE` is canonical unsigned decimal and at most `16777216`; the object
body is never requested or returned. An ID/type contradiction rejects every dependent candidate as
`SOURCE_INVALID_IDENTITY`; an over-bound object rejects them as `VERIFIER_REJECTED`. Failure of the
HEAD commit key is instead pre-output `DASHBOARD_REPOSITORY_UNAVAILABLE`, because no initial snapshot
repository identity exists. Exact
`OBJECT_ID SP missing LF`, `OBJECT_ID SP ambiguous LF`, any other status, malformed or non-ASCII
field, noncanonical size, reordering, truncation, missing line, extra line/byte, unexpected exit, or
crossed bound rejects every candidate referencing an ID in that failed batch under the terminal
`REPOSITORY_OBJECT_UNAVAILABLE` precedence above. There is no partial trust in a failed batch.

Each cat-file child has at most 4,194,304 stdout bytes; all cat-file children across both attempts
together have at most 8,388,608 stdout bytes. They also consume the common child-count, aggregate Git-stdout, ten-second
child, 30-second authority, stderr, process-group/Job-Object, termination, force-kill, and reap
limits. `GIT_NO_LAZY_FETCH=1` remains mandatory. `--batch`, `--batch-command`,
`--batch-all-objects`, `--buffer`, `--filters`, `--textconv`, `--use-mailmap`, and
`--follow-symlinks` are absent; no body, filter, text conversion, attribute lookup, or lazy fetch is
permitted. The validated terminal blob ID and type remain the semantic witness; object size and the
cat-file transcript do not add a witness field or enter `repositoryReadsSha256`. A future need for
object bodies requires a new accepted profile.

Each child has a ten-second deadline. The whole authority has one 30-second monotonic deadline across
both attempts. Stdout is bounded to 8 MiB per child and 128 MiB cumulatively across attempts; stderr
is bounded to 64 KiB per child and is never parsed into or copied to output. The cumulative ledgers
admit at most 100,000 historical-path keys, 100,000 cat-file keys, 4,096 Git children, and 128 MiB of
Git stdout; rejected work and a discarded first attempt consume these budgets. A crossed output,
ledger, process, or deadline bound fails closed. No counter, reservation, or deadline resets on retry.

Corvint starts every Git child in a new process group on POSIX or kill-on-close Job Object on Windows,
drains bounded stdout/stderr concurrently, and always reaps the leader. After a normal leader exit it
closes pipes/stdin and proves the whole group/job quiescent before accepting output; surviving
descendants are a containment failure. Parent cancellation or SIGINT/SIGTERM stops stdin, sends TERM
to the entire group/job, waits one monotonic 250 ms grace, force-kills the whole group/job if needed,
then waits for proven quiescence and leader reap before returning pre-output
`DASHBOARD_INTERRUPTED`. A child timeout, parse/output/ledger failure, or surviving descendant uses
the same 250 ms TERM-then-force-kill sequence but returns the applicable closed repository-
unavailable result, not `DASHBOARD_INTERRUPTED`. No authority call returns while a descendant may
remain; a platform unable to prove group/job quiescence is unsupported.

The fixed operations perform no checkout, index refresh, optional
lock, hook, filter, credential lookup, fetch, lazy fetch, protocol access, DNS, or socket operation.
The profile is an allowlist, not a shell template: no shell is invoked, no response file is expanded,
and every argument is one direct `exec.Cmd.Args` element.
Filesystem access-time suppression remains outside the portable claim.

The repository authority's Start completes layout/config/executable qualification, reciprocal Git
discovery, the identity tail, status tail, and HEAD cat-file check immediately before acquisition.
After the final adapter callback and capability close, Finish performs exactly: full rooted layout/
config reciprocal discovery with fresh descriptor reopens and comparisons; the identity tail; the
status tail; then the executable's complete second two-read qualification. It compares root,
worktree, `.git` entry, per-worktree Git, common Git, objects, layout/config file identities/digests,
object format, HEAD, HEAD tree, dirty count/set digest, and executable identity/count/digest. No
repository-authority operation occurs after Finish returns.

Any Start/Finish, executable, descriptor, config, HEAD/tree, or status drift discards all acquired
values and retries the entire scan once. All clocks, child/stdout/input, both ledger, source-count,
and source-byte reservations remain monotonic and cumulative. Evidence from the failed attempt is
discarded. A second drift or crossed cumulative repository-authority retry budget is the pre-output fatal
`DASHBOARD_REPOSITORY_UNAVAILABLE`, never a serialized `INVALID` source/snapshot. The repository
authority is immutable for each adapter callback and cannot outlive that callback.

Closed translation is exact: alternate/object-directory rejection during initialization has the
closed internal reason `UNSUPPORTED_OBJECT_ALTERNATES` and then becomes the fatal
`DASHBOARD_REPOSITORY_UNAVAILABLE`; it is never serialized as a snapshot issue. Non-ancestor or
ancestry distance above 10,000 is
`TRACE_ANCESTRY_BOUND`; absent/deleted or non-regular historical paths are `VERIFIER_REJECTED`;
object-format, full-ID, commit/tree identity contradictions are `SOURCE_INVALID_IDENTITY`. A member-
scoped missing/promised object, malformed response, unexpected exit, timeout, containment failure,
or child bound is terminal `REPOSITORY_OBJECT_UNAVAILABLE` for exactly the dependent candidates.
The same failure in Start/Finish/HEAD qualification, any cumulative authority bound, or any failure
before a qualified initial repository identity is fatal `DASHBOARD_REPOSITORY_UNAVAILABLE`;
parent cancellation/SIGINT/SIGTERM remains `DASHBOARD_INTERRUPTED`. Git stderr,
paths, executable identity, exit text, and free-form errors never enter a source, issue, metric,
stderr object, or hash.

`local-trace-v1` uses this service to prove that each trace revision is reachable within the exact
10,000-commit rule and to qualify its historical tracked path set. Other adapters receive no
repository capability unless an accepted registry row names it. A member-scoped unavailable object
or authority bound rejects every exactly dependent member under the frozen precedence; an initial,
HEAD, drift, or cumulative authority failure is pre-output fatal. The dashboard never degrades to
unverified JSON parsing. The
`repositoryReadsSha256` body remains only the semantic witness rows frozen above: executable,
process, pack, index, parent, intermediate-tree, status, and incidental traversal reads never enter
product identity.

`go-cem-ocm-bundle-v0` is `NOT_STARTED`, has `maxBytes: "0"`, and reads neither CEM nor OCM bytes.
The mandatory `--cem-ocm-profile` is caller-declared only: it selects the configured source's exact
registry profile but cannot upgrade delivery, authority, validity, or a metric from `NOT_OBSERVED`.
No filename or file body may infer it. When the adapter is delivered under a later accepted contract,
it MUST receive independent expected-base and target arguments, verify both maps and their pairing
against those arguments, and reject a map-derived revision. The dashboard never repairs, rewrites,
or implicitly pairs CEM/OCM. Current CEM/OCM bytes remain unread and absent from aggregates.

A subprocess verifier remains forbidden. The bounded repository-authority Git process above is the
sole exception and is not extensible into a general subprocess-verifier facility. Any other child
process or any new Git command/profile requires an accepted amendment freezing the same executable,
argv, cwd, environment, offline, timeout, output, containment, cleanup, and closed-result properties.
Artifact bytes can never choose an executable, command, argument, environment value, or working
directory.

## Safe source acquisition

Every configured root is opened once per attempt and traversed descriptor-relative. The implementation MUST use
the Go rooted-filesystem facility or an independently reviewed equivalent and MUST:

1. reject absolute source arguments, `..`, empty components, NUL, symlinks in any component, final
   symlinks, non-regular source files, devices, sockets, and FIFOs; the trace-store root is opened as
   a directory, but every enumerated child must be a regular file;
2. require one link for admitted artifact files; platforms that cannot qualify link identity report
   `UNSUPPORTED` rather than claiming external hardlink containment;
3. open the source without blocking, so a FIFO or device swapped in after the component preflight
   fails the descriptor check rather than hanging the open, then
   `fstat` the opened descriptor before reading, reject a declared size above the per-source bound,
   read at most `maxBytes+1`, rewind the same descriptor, read the same bound a second time, and
   `fstat` before and after each read;
4. retain bytes only when both complete reads are byte-identical and every identity, size, and
   modification fact is unchanged; retry the full acquisition once on difference, then emit
   `SOURCE_CHANGED_DURING_READ`;
5. compute `contentSha256` only after a complete bounded read; oversized, inaccessible, special, or
   unstable sources use `contentSha256: null`; and
6. never use file modification or access time as an event timestamp, freshness proof, or usage age.

Reads deliberately perform no write, create, lock, rename, cache, trace, index, Git, or repository
mutation. Portable filesystem access-time suppression is not claimed. Bind mounts and malicious
same-UID replacement outside the qualified rooted-filesystem model remain visible threat-boundary
limits.

The repository authority's Start runs immediately before first acquisition and Finish runs after the
last verifier/capability close in the exact order above. Head, tree, object format, worktree/layout/
config identity, dirty count/set digest, and executable identity MUST match. One difference retries
the entire scan; a second is fatal `DASHBOARD_REPOSITORY_UNAVAILABLE` with no snapshot.
`dirtyPathsSha256` is `sha256:` plus
`SHA-256(UTF8("corvint-dashboard-dirty-paths/0") || 0x00 || canonical(sorted byte-exact qualified
repository-relative dirty paths))`; paths themselves never enter the snapshot.

The global per-artifact limit is 16 MiB. `local-trace-v1` additionally inherits its owning limits:
at most 1,000 immediate entries, 1,000 retained rows, 256 KiB per row, and 16 MiB across the complete
store. A cumulative source-reservation ledger keyed by `(adapterId, configuredOrdinal)` admits at
most 10,000 keys and 256 MiB of logical source bytes across both attempts; first reservation charges
the key/declared bound and retry never resets or double-charges that same key. Physical two-read work
is counted monotonically and is at most 1,073,741,824 bytes across both possible attempts, plus
overflow-detection bytes. Only the overflow-probe allowance resets for each actual whole-scan
attempt; the logical reservation and physical-byte ledgers remain invocation-wide. A singular
configured artifact spends at most one overflow byte per acquisition attempt, two for its
`(adapterId, configuredOrdinal)` key per whole-scan attempt and four across the invocation. A trace
member's allowance is keyed by `(configuredOrdinal, member name, store attempt)` within the current
whole-scan attempt, not shared by the store: each member acquisition attempt spends at most one
overflow byte, two per member per store attempt, so an unstable member ends as its own
member-terminal `SOURCE_CHANGED_DURING_READ` and cannot refuse another member's read. One store
attempt reads at most the 1,000-member cap, so it spends at most 1,000 x 2 = 2,000 overflow bytes;
with two store attempts per scan attempt and two scan attempts, a configured trace store spends at
most 8,000 overflow bytes per invocation (decisions 0276 and 0284). A per-source or trace-store bound
follows its
typed source/aggregate issue above; a global source-count/logical-byte/physical-read bound is fatal
`DASHBOARD_RESOURCE_EXHAUSTED` with no snapshot. No bound returns a prefix. Two equal reads establish stable retained bytes only during those observation
windows, never atomic filesystem currency after the second read.

## Observation cohorts

Every admitted singular artifact receives one cohort ID; an aggregate source receives the sorted
set of its admitted member cohort IDs as specified by its adapter:

```text
dashboard-cohort:sha256:
  SHA-256(UTF8("corvint-dashboard-cohort/0") || 0x00 ||
         canonical(cohort object without cohortId))
```

The cohort object contains exactly:

```text
adapterId, cohortId, dirtyPathsSha256, producerIdentity, profile,
repositoryObjectFormat, sourceObservationEnd, sourceObservationStart,
sourceRevision, sourceTreeRevision
```

Every preimage field is therefore public in the snapshot. `producerIdentity` is the compiled
`verifierId` for in-process P0 adapters; future provider receipts use only their owning verified
producer identity. Unknown values are null. `cohorts` sorts by `cohortId`.

Unknown members are JSON `null`, not inferred from file time or current HEAD. A health, percentile,
closure, or outcome metric has exactly one cohort ID. Inventory metrics may span cohorts only with
`scopeClass: MULTI_COHORT_INVENTORY`; they cannot carry a health conclusion. Historical cohorts are
displayed separately. Source bytes from different revisions, dirty digests, provider identities, or
observation windows are never joined into one coherent health metric.

Age is computed only from an owning-profile observation timestamp against `generatedAt`. The exact
buckets are `LT_1H`, `H1_TO_24H`, `D1_TO_7D`, `GE_7D`, and `UNKNOWN`. A future timestamp is `INVALID`.
Absent source-owned time is `UNKNOWN`, never the file's modification time. Current local traces have
age `UNKNOWN`.

## Field computability and usage honesty

The following table is binding. “Retained” means verified durable artifacts supplied to this scan,
not all calls that occurred.

| Displayed concept | Computation | Required state or prohibition |
|---|---|---|
| configured inventory and bytes | exact valid, invalid, missing, inaccessible, unsupported, and excluded configured sources; bytes only for complete reads | complete only over the configured registry; no claim about unconfigured files |
| local trace use | exact retained verified rows by revision and explicit outcome | label `retained traces`; advisory; no task text/path/command display, event time, latency, causal attribution, or total-task denominator |
| query/impact use | none at base | `NOT_OBSERVED`; raw files and packet bytes cannot establish calls or usage until the stored-envelope verifier exists |
| query/impact latency | none at base | `NOT_OBSERVED`; packet bytes are never latency |
| CEM/OCM closure | none at base; later exact explicit-bundle verification | `NOT_OBSERVED` until adapter delivery; empty denominator is `EMPTY_UNIVERSE`, never 100%; structural closure is not test/gate success |
| capability delivery | compiled registry only | exact adapter stage; Markdown rows and filenames do not upgrade delivery |
| Pulse, harness, live verification, Frontier | none at base | every usage/run/latency/status field `NOT_OBSERVED`; protocol/conformance/ephemeral output is not durable usage |
| Beamfall shadow | none at base | `NOT_OBSERVED`; any future value remains `ADVISORY`; canonical gate state is never imported or rewritten |
| latest verifier boundary | this scan's verifier end, source digest, verifier ID, and cohort | label `VALIDATED_AT`; never reuse as currentness or original execution time |

A complete usage denominator requires a future accepted, bounded, sequence-complete journal with a
declared start/end window, zero sequence gaps, retention state, and secret-screening result. Until
then, no percentage of all Corvint activity, success rate, calls per time, or adoption trend is emitted.

## Canonical snapshot wire

P0 emits `corvint-dashboard-snapshot/0` canonical JSON. JSON is UTF-8, all emitted strings are NFC, no
insignificant whitespace, object keys sorted by raw UTF-8 bytes, and one trailing LF. Duplicate or unknown input
fields, non-UTF-8, invalid Unicode, floats, exponent notation, negative zero, and JSON numbers are
rejected. Counts, sizes, ordinals, and durations are canonical unsigned decimal strings with no
leading zero except `0`. Timestamps are UTC RFC3339 with exactly nine fractional digits. Arrays use
the order stated below; sets sort by canonical element bytes.

The root contains exactly:

```text
schema, generatedAt, observation, repository, cohorts, sources,
data, usage, verification, frontier, harnesses, beamfall,
privacy, issues, snapshotSha256
```

`schema` is `corvint-dashboard-snapshot/0`. The grouped fields `data`, `usage`, `verification`,
`frontier`, `harnesses`, and `beamfall` are arrays of metric objects sorted by `(name, dimensions,
cohortIds)`. Empty groups are `[]` and required unavailable metrics remain present with null values.
The top-level `cohorts` array contains the exact cohort objects defined above and sorts by `cohortId`.

`observation` contains exactly:

```text
adapterRegistrySha256, configuredSourceSetSha256, end, limitsProfile,
scanState = COMPLETE|PARTIAL|INVALID, start, clockSource = PROCESS|CALLER
```

`COMPLETE` means every configured path was classified without an unstable read, verifier rejection,
or crossed limit; a classified `NOT_PRESENT`, `DISABLED`, or registry-level `UNSUPPORTED` source does
not make the scan mechanically incomplete. `PARTIAL` means the snapshot is safe to inspect but at
least one configured source is inaccessible, unstable, expired, excluded, or invalid. `INVALID`
means an internal adapter/cohort/reference result contradicted its closed schema; every health metric
is then invalid or not observed, and every `usage` metric, including `usage.trace.retained`, is the
at-base unavailable row, so a snapshot carrying an observed usage row under `INVALID` is rejected. Repository or executable drift follows the pre-output fatal retry
rule instead. A fatal repository probe,
argument, interruption, aggregate resource, or output-bound failure emits no snapshot.

`configuredSourceSetSha256` is `sha256:` plus
`SHA-256(UTF8("corvint-dashboard-configured-sources/0") || 0x00 || canonical(sorted rows of
{adapterId,configuredOrdinal,contentSha256}))`. It never hashes paths. `repository` contains exactly:

```text
dirtyPathCount, dirtyPathsSha256, headRevision, objectFormat,
treeRevision, worktreeState = CLEAN|MIXED|UNKNOWN
```

No root, remote, branch, user, email, or repository name enters the wire. Unknown values are null.

Each `sources` element contains exactly:

```text
adapterId, authorityClass, byteCount, cohortIds, completeness, configuredOrdinal,
contentSha256, currency, deliveryStage, displayLabel, epistemicClass,
exclusions, id, members, observationEnd, observationStart, observationTime,
profile, repositoryReadsSha256, validity, verifierId
```

`id` is `dashboard-source:sha256:` plus
`SHA-256(UTF8("corvint-dashboard-source/0") || 0x00 ||
canonical({adapterId,configuredOrdinal,contentSha256,profile,repositoryReadsSha256}))`; it never
hashes or reveals a path. `repositoryReadsSha256` is null when the adapter has no repository-object
capability. For `local-trace-v1` it is `sha256:` plus:

```text
SHA-256(UTF8("corvint-dashboard-repository-reads/0") || 0x00 ||
        canonical(sorted unique witness rows))
```

Each witness row contains exactly, in canonical object-key order:

```text
kind, objectFormat, objectId, objectType, revision
```

The only rows are:

- `SNAPSHOT_HEAD`: `objectId` is the snapshot repository `headRevision`, `objectType` is `commit`,
  and `revision` is null;
- `TRACE_REVISION`: `objectId` and `revision` are the exact admitted trace revision and
  `objectType` is `commit`; and
- `TRACE_PATH_OBJECT`: `revision` is the exact admitted trace revision, `objectId` is the terminal
  object at one validated trace path in that commit, and `objectType` is `blob`.

`objectFormat` is exactly the repository `sha1` or `sha256` format in every row. Its lowercase
hexadecimal length is therefore 40 or 64. Rows sort by canonical row bytes and exact duplicate rows
collapse. If multiple validated paths resolve to the same blob at one revision, their identical row
appears once: this is a semantic object witness set, not a path-count receipt. It contains no path,
path hash, tree-walk object, commit-parent traversal object, pack/index object, or implementation-
specific read. Thus independent executions of the frozen Git authority compute identical bytes and
source identity does not acquire a hidden path commitment.

The exact trace path validation profile is `corvint-dashboard-trace-path-witness/0`. Each path is a
byte-exact valid-UTF-8 repository-relative POSIX path of at most 4,096 bytes; is nonempty; uses `/`;
has no control, NUL, backslash, absolute/volume prefix, empty, `.`, or `..` component; and equals its
slash-based lexical clean form. Corvint performs no Unicode normalization or case folding. The owning
trace secret/forbidden-path screen must accept it. The trace producer and stored-row reader refuse any path outside this profile (decision 0235, `LTPM-V0-011`). Paths are deduplicated and sorted by raw UTF-8
bytes across `opened_paths` and `changed_paths` before lookup. They are private repository identities
and never enter a public ID, snapshot string, or semantic witness row; the emitted-string NFC rule
therefore does not apply to them. At the named revision, the revision object MUST be a commit, its
root and every intermediate component MUST be a
tree, and the terminal entry MUST be a regular blob with mode `100644` or `100755`. Absent, deleted,
tree-terminal, symlink (`120000`), gitlink (`160000`), tag, promised, or malformed objects reject that
trace member; they never become null or a witness row.

Reachability has the exact semantics of `git merge-base --is-ancestor REVISION HEAD`, independent of
the command: `REVISION == HEAD` or `REVISION` is reachable from `HEAD` by following zero or more commit
parent edges. Each admitted revision must satisfy it. Parent commits and tree nodes traversed to prove
reachability/lookups are bounded internal work but are not witnesses. No semantic witness row is
admitted until its exact cat-file ledger check succeeds. The empty valid store witness body contains
only `SNAPSHOT_HEAD`. A stable partial store contains HEAD plus witnesses from admitted members only.
An invalid store with no admitted members, or any changed identity, has
`repositoryReadsSha256: null` and contributes no trace metric. `displayLabel`
is exactly ASCII `adapterId#configuredOrdinal`. `configuredOrdinal` is a canonical decimal string.
The default trace store has ordinal `0`; additional explicit inputs for one adapter sort by raw UTF-8
relative-path bytes and receive consecutive ordinals after any default. The five-flag CEM/OCM bundle
is ordinal `0` and uses its exact caller-declared profile even while `NOT_STARTED`/`UNSUPPORTED`.
Paths determine ordering only and never enter an ID or output.
Canonical snapshots never contain source paths. `cohortIds` and `exclusions` are sorted unique
arrays. `members` is the exact trace-store aggregate body for `local-trace-v1` and null for every
other V0 adapter. `observationStart` and `observationEnd` expose the successful stable-read interval
for a singular source and the defined min/max interval for an aggregate; unavailable or empty inputs
use null. `observationTime` is only a source-owned event time, never an acquisition or file time.
Nullable fields use JSON null.

Every SHA-256 field is `sha256:` plus 64 lowercase hexadecimal characters. Random bootstrap values
use unpadded base64url. Git object IDs retain the owning object format and are lowercase hexadecimal.

Every metric contains exactly:

```text
authorityClass, cohortIds, completeness, currency, denominator,
deliveryStage, dimensions, epistemicClass, exclusions, name,
numerator, scopeClass, sourceIds, unit, validity, value, window
```

`dimensions` is a sorted array of `{name,value}` pairs whose names and values are fixed below; it
cannot contain a path or free-form artifact text. `value`, `numerator`, and
`denominator` are canonical decimal strings, an owning closed enum, an exact fraction `n/d`, or null
as fixed per metric name. `window` is null or exactly `{end,start}` from source-owned times.
`sourceIds`, `cohortIds`, and `exclusions` are sorted unique arrays.
Metrics have no separate ID in V0. The unique metric key is the tuple
`(name, canonical(dimensions), canonical(cohortIds))`; duplicate keys make the snapshot noncanonical.

The exact P0 metric names and representations are:

| Name | Value and unit |
|---|---|
| `data.artifact.count`, `data.artifact.bytes` | separate configured-source and retained-member inventory; exact rules below |
| `usage.trace.retained` | decimal retained row count by revision and explicit outcome |
| `usage.query.retained`, `usage.impact.retained`, `usage.harness.retained` | null until their exact durable adapters are delivered |
| `usage.response.bytes`, `usage.latency.p50`, `usage.latency.p95` | null at base; latency unit nanoseconds after an accepted exact-linked source exists |
| `verification.cem.hunks`, `verification.ocm.obligations`, `verification.closure` | null until explicit-bundle adapter delivery; closure is exact fraction `n/d`, null with `universe=EMPTY` when `d=0` |
| `verification.live.runs`, `verification.pulse.runs`, `frontier.items`, `beamfall.shadow.runs` | null at base |

`unit` is exactly `COUNT`, `BYTES`, `NANOSECONDS`, `RATIO`, or `STATE` as implied by the table.
`scopeClass` is exactly `SINGLE_COHORT`, `MULTI_COHORT_INVENTORY`, or `UNAVAILABLE`. For
`data.artifact.count|bytes`, dimensions are exactly `adapterId`, `ageBucket`, `authorityClass`,
`artifactKind`, `deliveryStage`, `revision`, and `validity`. `artifactKind` is exactly
`CONFIGURED_SOURCE` or `RETAINED_MEMBER`; `revision` is null or a validated Git object ID and all
other values are registry enums. Every artifact inventory metric uses
`scopeClass: MULTI_COHORT_INVENTORY` and can never carry a health conclusion.

`CONFIGURED_SOURCE` counts each configured aggregate or singular source exactly once, including
missing, unsupported, partial, and invalid sources. Its `data.artifact.count` numerator/value is the
grouped source count. Its byte numerator/value is the sum of the grouped checked admitted
`byteCount`s, or null with `epistemicClass: NOT_OBSERVED` when any grouped source has no complete
bytes; a known part is never reported as the group's bytes. `sourceIds` contains the grouped product source IDs and `cohortIds` is the
sorted union of those sources' cohort IDs. When the complete configured-source universe is closed,
count denominator is its total source count and byte denominator is its total complete-read byte
count; otherwise both denominators are null and completeness is `PARTIAL` or `UNKNOWN`. A partial
trace aggregate still contributes configured count `1`, uses the aggregate source validity and
`completeness: PARTIAL`, and never receives a closed denominator.

`RETAINED_MEMBER` is emitted only for `local-trace-v1`. Each admitted member contributes count `1`
and its exact bytes in its validated revision group, with the aggregate product source ID and that
member's one cohort ID. A stable rejected-candidate universe emits an additional `validity: INVALID`
count row whose value/numerator is the sum of terminal-issue `observed` counts, byte value/numerator
is null, source ID is the aggregate ID, cohort IDs are empty, and exclusions are those terminal issue
IDs. On a complete store, member count denominator is the total admitted member count and member byte
denominator is the total admitted member bytes. On a partial/invalid store every retained-member
denominator is null and completeness is `PARTIAL`. `STORE_CHANGED` instead makes both member rows
null `NOT_OBSERVED` with `completeness: UNKNOWN`, because even the candidate count is not observed
(decision 0263).

An unchanged empty valid trace store emits the configured-source count row with value/numerator
`"1"`, its byte row with `"0"`, and one retained-member count/byte pair with value/numerator and
denominator all `"0"`. Those rows are `VALID`/`COMPLETE`, name the aggregate source ID, use empty
`cohortIds` and public `members: []`, and use `revision: null`, `ageBucket: UNKNOWN`. Thus empty is a
measured zero while absent, unstable, and unobserved stores never become zero.

For `usage.trace.retained`,
dimensions are exactly `outcome` and `revision`; numerator/value is the grouped retained row count and
denominator is all valid retained trace rows in that one revision cohort. At-base unavailable metrics
have `dimensions: []`, null value/numerator/denominator, `scopeClass: UNAVAILABLE`,
`epistemicClass: NOT_OBSERVED`, and `completeness: UNKNOWN`. Delivery of any future metric changes
this table first to freeze its dimensions and arithmetic.

Future percentiles require at least one exact finite non-negative sample and use nearest rank over
sorted integer nanoseconds: rank `ceil(p*n)`, one-indexed. They expose sample count as `denominator`.
Distinct counts use exact bounded sets, never sketches. Rates are absent from V0.

`privacy` contains exactly:

```text
collection = DISABLED, outboundNetwork = NONE, pathDisclosure = NONE,
rawBodies = EXCLUDED, threatBoundary = LOCAL_ACCOUNT_NOT_DEFENDED
```

Every `issues` element contains exactly:

```text
id, code, severity = INFO|WARNING|ERROR, sourceId, observed, limit
```

`observed` and `limit` are nullable decimal strings. `sourceId` is nullable. `id` is
`dashboard-issue:sha256:` plus
`SHA-256(UTF8("corvint-dashboard-issue/0") || 0x00 || canonical(issue without id))`. Issue codes are
limited to:

```text
SOURCE_NOT_PRESENT, SOURCE_INACCESSIBLE, SOURCE_UNSUPPORTED, SOURCE_DISABLED,
SOURCE_EXPIRED, SOURCE_OVERSIZED, SOURCE_SPECIAL_FILE, SOURCE_SYMLINK,
SOURCE_MULTILINK_UNQUALIFIED, SOURCE_CHANGED_DURING_READ, SOURCE_INVALID_SCHEMA,
SOURCE_INVALID_IDENTITY, SOURCE_WRONG_COHORT, VERIFIER_REJECTED,
LIMIT_ARTIFACTS, LIMIT_INPUT_BYTES, LIMIT_SAMPLES, LIMIT_SNAPSHOT_BYTES,
OBSERVATION_TIME_UNKNOWN, MIXED_COHORT_EXCLUDED,
REPOSITORY_OBJECT_UNAVAILABLE, UNSUPPORTED_OBJECT_ALTERNATES,
STORE_CHANGED, TRACE_ANCESTRY_BOUND, TRACE_STORE_BOUND
```

No issue contains free-form errors, paths, URLs, environment values, source strings, or verifier text.
`sources` and `issues` sort by `id`.

`snapshotSha256` is `sha256:` plus:

```text
SHA-256(UTF8("corvint-dashboard-snapshot/0") || 0x00 ||
        canonical(snapshot with snapshotSha256 set to JSON null, without trailing LF))
```

The final document replaces null with that value and adds one LF. Hash verification repeats the same
replacement. For identical frozen source bytes, source arguments, registry, limits, repository probe,
and caller-supplied `generatedAt`, independent implementations and repeated CLI runs produce identical
bytes.

### Two independent verifier layers

`corvint-dashboard-snapshot-verifier/0` consumes only the exact snapshot bytes. It verifies canonical
JSON and LF, the closed schema/registry/metric types, sorting and uniqueness, every hash whose
preimage is public in the snapshot, source-member/cohort/interval references, limits, and internal
numerator/denominator arithmetic. It verifies the trace aggregate digest from public `members`, but
it has no trace bodies and therefore MUST NOT claim to revalidate trace rows, outcomes, historical
tracked paths, commit reachability, repository-object authority, or any other source semantic. A
passing snapshot-only verifier proves structural and arithmetic self-consistency only.

Trace semantics require the separate independent `corvint-dashboard-trace-adapter-conformance/0`
verifier. Its exact black-box invocation is:

```text
corvint-dashboard-trace-conformance verify --root ROOT \
  --manifest MANIFEST --snapshot SNAPSHOT
```

`ROOT` is a planted bounded repository corpus containing the exact trace artifacts and Git objects;
`SNAPSHOT` is the exact canonical snapshot file. `MANIFEST` is canonical JSON plus LF whose root
contains exactly, in canonical key order:

```text
configuredSources, generatedAt, schema
```

`schema` is exactly `corvint-dashboard-trace-conformance-manifest/0`; `generatedAt` is the exact
conformance timestamp. `configuredSources` is sorted by `(adapterId, configuredOrdinal)` and each row contains exactly
`adapterId, configuredOrdinal, relativePath`. Relative paths are conformance-only inputs, obey the
production path grammar, and never enter the snapshot. The manifest rejects duplicates, unknown
fields, unsupported adapters with paths, absolute/traversing paths, noncanonical ordinals/times, and
any corpus that crosses the production artifact, byte, row, object, ancestry, or output limits.

The trace adapter verifier is implementation-independent: it MUST NOT import or call production
`internal/dashboard` code or share generated expected metrics. It independently performs rooted
stable acquisition, public trace-row/schema validation, filename/revision and historical Git-object
qualification, exact `corvint-dashboard-trace-path-witness/0` lookup and semantic-witness construction,
aggregation, cohort/source/issue/hash construction, outcome arithmetic, and every
configured-source/member denominator. Using `generatedAt`, registry, manifest, and planted corpus, it
constructs the full expected snapshot and requires byte-for-byte equality with `SNAPSHOT`. It runs
offline without mutation. Repository qualification independently drives the exact
`corvint-dashboard-git-authority/0` upstream-Git profile; implementation independence does not permit
a second object parser or a different Git command/environment profile. Positive, empty, partial,
malformed-row, wrong-revision, unreachable-
commit, changed-store, limit, and hostile-string vectors are required. Neither verifier layer may be
reported as the other: snapshot-only PASS never proves trace source semantics, and adapter-semantic
PASS does not replace canonical snapshot verification.

The compiler first materializes and verifies one complete canonical snapshot of at most 4 MiB before
the first stdout write. Every validation, acquisition, compilation, interruption, bound, and internal
failure before output begins writes zero stdout, exits `2`, and writes one canonical JSON object plus
LF to stderr:

```text
{"code":"CODE","profile":"corvint-dashboard-error/0"}
```

`CODE` is exactly `DASHBOARD_INVALID_ARGUMENT`, `DASHBOARD_REPOSITORY_UNAVAILABLE`,
`DASHBOARD_RESOURCE_EXHAUSTED`, `DASHBOARD_INTERRUPTED`, or `DASHBOARD_INTERNAL_ERROR`. No diagnostic
text or path is emitted by the machine command. A final snapshot that would exceed 4 MiB is
`DASHBOARD_RESOURCE_EXHAUSTED`, not a truncated or oversized snapshot.

Stdout transport is a distinct portable failure boundary; V0 does not claim an atomic 4 MiB pipe or
file write. The writer loops until all materialized bytes are written, accepting a benign short
write only when it makes positive progress and returns no error. A zero-byte write, negative or
over-remaining byte count, write error, or recoverable panic from the stdout writer exits `2` and
emits exactly this stable redacted stderr object plus LF:

```text
{"code":"OUTPUT_WRITE_FAILED","profile":"corvint-dashboard-error/0"}
```

The byte count returned with an error is retained before failure classification. A writer may also
retain bytes and then panic. If failure occurs before any byte is retained, stdout remains empty. If
it occurs after `N > 0` bytes, stdout may contain exactly those `N` bytes; the process makes no
atomicity claim, does not retry from byte zero, and emits no other stdout. In particular, a writer
may retain all snapshot bytes and then return `n == len(remaining), err != nil` or panic. Stdout is
then a complete canonical and hash-valid snapshot, but the command still emits `OUTPUT_WRITE_FAILED`
and exits `2`.

Consumer acceptance is the conjunction of all three conditions: stdout is the complete
LF-terminated canonical snapshot and its hash verifies; the process exits `0`; and stderr is exactly
empty. Success produces that triple. A prefix fails the first condition, while a full-length
error/panic result fails the second and third; canonical/hash verification alone is never sufficient.
No transport error, byte count, panic value, writer text, destination, or path enters stderr.

## P1 loopback HTTP and browser contract

P1 is specified for security review but remains blocked by the owner gates above.

### Listener and bootstrap

- Bind exactly `tcp4 127.0.0.1:0`; V0 has no IPv6, hostname, wildcard, LAN, proxy, or remote mode.
- Record the port and require exact Host `127.0.0.1:PORT`. Reject absolute-form targets,
  forwarded-host headers, any other Host, and non-origin-form request targets.
- Generate independent random 256-bit `BOOTSTRAP_TOKEN`, `SESSION_ID`, and `URL_PREFIX`. Print exactly
  once to stdout: `http://127.0.0.1:PORT/d/URL_PREFIX/#BOOTSTRAP_TOKEN`. Corvint writes it nowhere else.
  Terminal scrollback, browser extensions/session restoration, and malicious same-UID processes are
  outside the claimed boundary and are stated in `Privacy`.
- Unauthenticated access is limited to exact GET/HEAD bootstrap HTML and hashed static assets beneath
  `/d/URL_PREFIX/`. The external script reads the fragment into memory, immediately calls
  `history.replaceState`, then POSTs it in header `Corvint-Fragment-Token` to exact
  `/d/URL_PREFIX/session`. The server compares in constant time, consumes the bootstrap token only on
  first successful exchange, and rejects every replay.
- The session endpoint is the sole mutating HTTP endpoint and mutates only ephemeral in-memory auth
  state. It returns no token body and sets a distinct opaque session ID in cookie `corvint_session`,
  `HttpOnly; SameSite=Strict; Path=/d/URL_PREFIX/`, with no Domain, Expires, or Max-Age. The session is
  process-local and useless after shutdown. P1 does not claim prevention of browser-managed session
  restoration; restored values have no matching server state.

The complete endpoint set is:

| Method and path | Authentication | Behavior |
|---|---|---|
| `GET|HEAD /d/URL_PREFIX/` | none | bootstrap HTML only |
| `GET|HEAD /d/URL_PREFIX/assets/SHA256.js` | none | exact manifest-bound script |
| `GET|HEAD /d/URL_PREFIX/assets/SHA256.css` | none | exact manifest-bound stylesheet |
| `POST /d/URL_PREFIX/session` | fragment header plus exact Host, Origin, and `Sec-Fetch-Site: same-origin` | one-shot ephemeral session exchange; zero-length body |
| `GET /d/URL_PREFIX/api/snapshot` | session plus same-origin request contract | exact P0 snapshot bytes |
| `GET /d/URL_PREFIX/api/events` | session plus same-origin request contract | P2 full-snapshot SSE only |

Every other method or path is unlisted and rejected. Asset SHA-256 is the unprefixed 64-character
lowercase digest recorded in the binary's exact asset manifest.

### Request and response policy

Authenticated API and SSE require the exact cookie, exact Host, an origin-form target, and
`Sec-Fetch-Site: same-origin`. If `Origin` is present it must equal
`http://127.0.0.1:PORT`; any other value fails. Browsers unable to supply this contract are
unsupported. No CORS header is emitted. `OPTIONS`, TRACE, CONNECT, unexpected bodies, and every
unlisted method/path return a constant bounded error without reflection.

HTML responses use exactly:

```text
Content-Security-Policy: default-src 'none'; base-uri 'none'; form-action 'none';
  frame-ancestors 'none'; script-src 'self'; style-src 'self'; img-src 'self';
  connect-src 'self'; font-src 'none'; object-src 'none'; manifest-src 'none'; worker-src 'none'
Cache-Control: no-store
Referrer-Policy: no-referrer
X-Content-Type-Options: nosniff
Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()
```

Each displayed multi-line value above is transmitted as one header value with single ASCII spaces
between directives; line wrapping is documentation-only.

API/SSE also use `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, and
`X-Content-Type-Options: nosniff`. Assets contain no inline script/style/event handler, remote
resource, `eval`, service worker, Web Storage, IndexedDB, beacon, source map, or runtime package
download. Dynamic response compression is disabled. Displayed values use text nodes only, never HTML
insertion.

P1 permits authenticated loopback HTTP/SSE traffic. The dashboard process performs no outbound
socket, DNS lookup, fetch, remote asset request, listener other than the exact loopback socket, or
repository upload. Browser and operating-system networking is outside this process claim; network
conformance omits `--open` and scopes spies to the dashboard process tree.

### Lifetime and cleanup

The server has a 15-minute idle timeout and eight-hour hard lifetime. An authenticated snapshot/API
request resets idle time; an open SSE connection, heartbeat, asset request, failed auth, or bootstrap
probe does not. Default interactive mode ignores stdin. `--exit-on-stdin-eof` is an explicit ownership
mode. `--owner-pid PID` is accepted only on platforms with a qualified parent-death observer;
otherwise startup fails `UNSUPPORTED`.

SIGINT, SIGTERM, idle expiry, hard lifetime, explicit stdin EOF, or qualified parent death stops
admission, cancels scans, closes SSE and the listener, clears tokens/sessions/buffers, waits at most
five seconds, then exits. Recoverable HTTP-handler panics are caught and trigger the same shutdown;
process abort, runtime fatal error, kernel failure, and power loss are not claimed to execute cleanup.
No dashboard operation launches a verifier child in V0.

### SSE P2

V0 SSE sends full snapshots only, never deltas. Each `snapshot` event data object contains exactly:

```text
kind = FULL, sequence, serverGeneration, snapshot, snapshotSha256
```

`serverGeneration` is a fresh random 256-bit process value. `sequence` starts at `0` and increases by
one per completed scan. `snapshot` is the exact canonical object represented by the named digest. A
reconnect, `Last-Event-ID`, client gap, queue overflow, or generation mismatch always receives the
latest full snapshot; no replay or delta attachment occurs. Each client has one pending-frame slot.
A slow client is disconnected, does not block scans, and reconnects for a full snapshot. Heartbeats
contain no state and do not advance sequence or idle time. Four clients and one singleflight scan per
second are hard maxima.

## Requirements

### Data and proof

- `LOD-V0-001`: Every displayed metric and source-derived value MUST derive through the compiled
  adapter and verifier registry and expose source IDs, cohort IDs, truth axes, denominator, exclusions,
  delivery stage, and scan boundary. Structural snapshot verification and independent adapter-
  semantic conformance MUST remain separate proof layers.
- `LOD-V0-002`: Missing, disabled, invalid, inaccessible, unsupported, expired, and unretained
  configured inputs MUST remain distinct and MUST never become measured zero or success. Unconfigured
  artifacts are outside the declared scan universe and MUST NOT be represented as absent.
- `LOD-V0-003`: P0 MUST produce byte-identical CLI snapshots for identical frozen inputs and the
  supplied time only through the exact black-box `corvint-dashboard-snapshot snapshot --conformance
  --generated-at RFC3339_NANO_UTC ...` invocation. The two flags are inseparable and no ambient
  bypass exists. P1 MUST serve those exact bytes, including LF, from its authenticated snapshot
  endpoint without reserialization. P0 output follows the exact pre-output versus transport-failure
  boundary above and makes no atomic stdout-write claim. Consumers MUST require the complete
  canonical/hash-valid stdout bytes, exit `0`, and empty stderr together.
- `LOD-V0-004`: Aggregation MUST perform no deliberate repository, Git, trace, receipt, index, cache,
  or authoritative-system mutation. Filesystem atime suppression is outside the portable claim.
- `LOD-V0-005`: Acquisition MUST follow the rooted, bounded, descriptor-relative contract. Mixed
  cohorts MUST be separated and mid-read mutation retried once then rejected. Trace-store identity
  MUST use the exact bounded two-enumeration `trace-store/0` aggregate contract. Repository objects
  MUST be qualified only through the one frozen upstream-Git authority exception with repository and
  ambient graft files disabled; no second Git parser or object authority exists in V0.
- `LOD-V0-006`: Owning status axes and stages MUST remain separate. Query `READY`, a passing test,
  empty declared frontier, valid CEM/OCM structure, or Beamfall shadow match never becomes correctness,
  completeness, merge authority, or market validation.

### Data and usage statistics

- `LOD-V0-007`: P0 MUST emit every exact metric in the registry, including invalid and missing source
  inventory, with unavailable values null and honestly typed.
- `LOD-V0-008`: Trace metrics MUST say `retained`; total tasks, calls, rates, event times, and latency
  remain `NOT_OBSERVED`. Query, impact, and harness metrics remain `NOT_OBSERVED` until their exact
  durable adapters are delivered.
- `LOD-V0-009`: CEM/OCM metrics require the explicit independently revision-bound bundle. Empty
  universes are labelled empty and never 100% closed. No implicit current-path pairing is allowed.
- `LOD-V0-010`: Pulse, live verification, harness, and Frontier remain `NOT_OBSERVED` until accepted
  durable receipt profiles exist. Protocol, conformance, and ephemeral stream bytes are not usage.
- `LOD-V0-011`: Output MUST exclude trace task text, paths, commands, prompts, tool arguments/output,
  source bodies, environment values, and causal inference.
- `LOD-V0-012`: Beamfall remains `NOT_OBSERVED` until an accepted advisory adapter exists. Corvint MUST
  NOT read, write, or reinterpret Beamfall's canonical gate ledger.
- `LOD-V0-013`: Every view MUST expose scan boundary, cohort/revision, dirty/mixed state, unavailable
  denominator, invalid count, and latest dashboard verifier boundary without relying on color.

### Local server, privacy, and security

- `LOD-V0-014`: P1 MUST implement the exact loopback, Host, origin/fetch-metadata, endpoint, method,
  header, no-egress, no-DNS, and bounded-lifetime contract. V0 has no remote mode.
- `LOD-V0-015`: P1 MUST implement the one-shot fragment bootstrap and distinct ephemeral session
  exactly, with no Corvint-controlled persistence or token reflection and visible boundary limits.
- `LOD-V0-016`: P1 API/SSE MUST require the session and same-origin request contract. Only the session
  exchange may mutate ephemeral auth state; no endpoint mutates Corvint or repository data.
- `LOD-V0-017`: Canonical/API output MUST exclude paths, roots, secrets, credentials, source bodies,
  prompts, raw outputs, users, emails, remote URLs, and free-form errors. V0 has no path-reveal flag.
- `LOD-V0-018`: Privacy MUST state that malicious same-UID processes, browser extensions/restoration,
  compromised local accounts, bind mounts, and unqualified platform behavior are not defended claims.

### Bounds, lifecycle, accessibility, and compatibility

- `LOD-V0-019`: V0 admits at most 10,000 configured artifacts, 16 MiB per artifact, 256 MiB aggregate
  input, 100,000 metric samples, 4 MiB final snapshot bytes, four SSE clients, and one scan per second.
  Repository authority additionally admits the cumulative 100,000 historical-path keys, 100,000
  cat-file keys, 4,096 children, 128 MiB stdout, and exact per-child/chunk/Windows bounds above.
  Retry resets only the overflow-probe allowance for an actual whole-scan attempt; it never resets a
  monotonic deadline, source reservation, logical or physical byte ledger, or process counter.
  Every CLI root/path value has the exact inclusive 4,096-byte platform-qualified bound above.
  An individual rejected source makes a bounded snapshot `PARTIAL` only when all derived metrics
  exclude it visibly. An aggregate input/sample/artifact/output limit emits the fatal bounded resource
  error and no partial snapshot.
- `LOD-V0-020`: P1 MUST satisfy the qualified lifetime/cleanup contract. Claims exclude unrecoverable
  process/runtime/kernel/power failure. No artifact verifier subprocess exists in V0; the exact
  repository-authority Git child is the sole process exception and is not verifier-extensible.
- `LOD-V0-021`: P2 MUST use the full-snapshot-only SSE contract; no V0 delta exists.
- `LOD-V0-022`: After owner shape confirmation, the UI MUST be keyboard operable; meet WCAG 2.2 AA
  contrast/focus; support 200% zoom, reduced motion, narrow viewports, accessible tables and text
  equivalents for charts; and never encode status by color alone. The confirmed brief freezes
  system-adaptive versus single-theme behavior before implementation.
- `LOD-V0-023`: Embedded assets MUST build from repository source without runtime download. A
  versioned manifest and hashes bind assets to the Go binary. Unsupported browsers receive only the
  CLI command/download path, never a partial visualization.
- `LOD-V0-024`: Dashboard absence, crash, invalid input, or rollback MUST leave every existing Corvint
  command, verifier, provider, Beamfall gate, and repository workflow unchanged.

### Collection and rollout

- `LOD-V0-025`: Historical usage collection requires a separate accepted profile and explicit opt-in
  writer with sequence completeness, retention, redaction, secret-screening, bounds, and disablement.
  Dashboard reads never activate it.
- `LOD-V0-026`: Existing durable artifacts are the baseline. Missing fields remain `NOT_OBSERVED`;
  broad automatic developer-activity collection MUST NOT be added to populate a chart.
- `LOD-V0-027`: Corvint self-dogfood MUST precede Beamfall dogfood. Both stay shadow-only, preserve all
  repository authority, record failed/unavailable outcomes, and compare P0 CLI bytes with P1 HTTP
  bytes only after P1 exists.
- `LOD-V0-028`: P0 experimental promotion requires deterministic conformance and independent proof
  review. P1 additionally requires accepted intent, scoped contract amendments, confirmed shape,
  frozen browser/OS matrix, and independent security/accessibility review. The outcome gate below is
  required before validation.

### Roadmap join (experimental)

The `roadmap` CLI subcommand of `cmd/corvint-dashboard-snapshot` and the `internal/dashboard/roadmap`
package join task-manager roadmap tickets to source evidence, test receipts, and generated-doc state.
It is additive to, and does not extend, the `corvint-dashboard-snapshot/0` P0 schema: it emits its own
`corvint-dashboard-roadmap/0` wire schema so the hash-validated P0 snapshot compiler and its acceptance
tests stay untouched. It is experimental and unaccepted; the receipts-directory and docs-state file
schemas below are proposed contracts, not repository conventions.

- `LOD-V0-029`: The roadmap join MUST read the task manager only through its own read-only `roadmap`
  and `ticket show` verbs, executed via `internal/procgroup.Run` with a bounded timeout and output
  cap. It MUST NOT invoke, expose, or depend on any task-manager mutation verb.
- `LOD-V0-030`: A non-zero `atm` exit, a malformed envelope, an envelope profile other than
  `taskman-command-result/0`, or a non-`OK` envelope outcome MUST produce a whole-snapshot
  `NOT_OBSERVED` outcome carrying the specific reason. It MUST NOT be treated as an empty success.
  As in LAC-V0-022, stderr is read as the reply only when stdout is empty and the exit is non-zero,
  and only a refusal envelope is admitted alongside a non-zero exit or a signal; an `OK` envelope
  with either is a failure.
  A `ticket show` failure for one ticket MUST NOT suppress or invalidate any other ticket's data; it
  MUST surface as a `NOT_OBSERVED`-coded blocker scoped to that one ticket.
  Parent cancellation observed before output is not an `atm` failure: the `roadmap` subcommand MUST
  end it as pre-output `DASHBOARD_INTERRUPTED` with exit 2, never a `NOT_OBSERVED` snapshot.
  `internal/dashboard/roadmap/compile_test.go`'s `TestCompileJoinsRequirementsAndIsolatesPerTicketFailure`
  and the five `TestCompileWholeSnapshot...` cases are the acceptance evidence, with
  `cmd/corvint-dashboard-snapshot/roadmap_cmd_test.go`'s `TestRoadmapCancelledDuringCompileIsInterrupted`
  for cancellation.
- `LOD-V0-031`: Each ticket's `requirementRefs` MUST resolve against `docs/specs/REQUIREMENTS.tsv`
  rows by exact requirement ID; an ID absent from that table MUST render `resolved: false`
  ("unresolved") rather than inventing a source location. The table is opened without blocking and
  read only when the open descriptor is a regular file of at most 8 MiB; a FIFO, device, or oversize
  table is a load error, never an empty or partial table. When the table cannot be loaded, every ref
  MUST render `resolved: false` with reason `requirements table not observed`, never as an ID the
  table lacks; `internal/dashboard/roadmap/compile_test.go`'s
  `TestCompileEvidenceNotObservedWhenRequirementsTableUnreadable` covers it.
  `internal/dashboard/roadmap/requirements_test.go` and
  `internal/dashboard/roadmap/readfile_unix_test.go` are the acceptance evidence.
- `LOD-V0-032`: A test receipt is a proposed one-file-per-requirement-ID JSON document under a
  configured receipts directory, holding `inputIdentity` (an opaque tree-identity string) and a
  `projection` (`internal/testvalidity.Projection`). The current tree digest is computed from
  `git rev-parse HEAD^{tree}` and so reflects only the last committed tree; whether that digest still
  describes everything on disk is checked separately via
  `git status --porcelain=v1 --untracked-files=all --ignored=no` (no filters, no hooks). Both Git
  reads run under a sanitized environment: no inherited `GIT_*` variable, user or system
  configuration (so no user `core.excludesFile`), or replace object applies. When that check reports the worktree clean, a receipt whose `inputIdentity` does not match the current tree
  digest MUST render `STALE`, not `CURRENT`, and a matching `inputIdentity` MUST render `CURRENT`.
  When the worktree is dirty, or the dirty check itself could not be observed, every receipt
  classification MUST render `NOT_OBSERVED` with reason `worktree-dirty` instead of `CURRENT` or
  `STALE` — a dirty or unverifiable worktree is missing evidence, not a documented gap to render
  around (AGENTS.md invariant 2). A missing receipts directory, missing file, unparseable file, or
  unavailable current tree digest MUST render `NOT_OBSERVED` with its own specific reason; never a
  default pass. The dashboard's roadmap output MUST carry the observed worktree-dirty flag so a
  consumer can see why receipts went `NOT_OBSERVED`. A receipt is opened beneath the receipts
  directory through `os.Root`, without blocking, and read only when the open descriptor is a regular
  file of at most 8 MiB; a requirement ID or symlink that escapes the directory, a non-regular file,
  or an oversize file MUST render `NOT_OBSERVED`, never a receipt read from elsewhere.
  `internal/dashboard/roadmap/receipts_test.go`, `internal/dashboard/roadmap/readfile_unix_test.go`,
  and `internal/dashboard/roadmap/atm_test.go` (including `TestTreeChecksIgnoreAmbientGitEnvironment`)
  are the acceptance evidence (amended 2026-09-13).
- `LOD-V0-033`: Generated-doc state is a proposed docs-state JSON document, a map keyed by ticket
  local ID to `{path, sha256, state}`, read from a configured path. A missing path, unreadable or
  unparseable file, or an entry absent for the ticket's local ID MUST render `NOT_OBSERVED` with the
  specific reason. The file is opened without blocking and read only when the open descriptor is a
  regular file of at most 8 MiB; a FIFO, device, or oversize file is unreadable.
  `internal/dashboard/roadmap/docsstate_test.go` and
  `internal/dashboard/roadmap/readfile_unix_test.go` are the acceptance evidence.
- `LOD-V0-034`: Given identical `atm` output, `REQUIREMENTS.tsv`, receipts, docs-state, and tree
  digest, two compiles MUST produce byte-identical output.
  `internal/dashboard/roadmap/compile_test.go`'s `TestCompileIsDeterministic` is the acceptance
  evidence.

## Staged deterministic acceptance

### P0 snapshot compiler

1. `LOD-AT-P0-001` (`001..006`): two independent decoders invoke the compiled binary exactly as
   `corvint-dashboard-snapshot snapshot --conformance --generated-at RFC3339_NANO_UTC ...` and
   reproduce exact canonical bytes and hash. The snapshot-only
   `corvint-dashboard-snapshot-verifier/0` independently proves public-preimage hashes, member/cohort/
   interval links, registry/types, and internal arithmetic while explicitly declining trace-semantic
   proof. Black-box negative vectors prove either flag alone is a usage error and that environment
   variables, config, build tags, and test-process identity cannot inject time. Golden valid/invalid/
   missing sources prove compiled dispatch, truth-axis separation, mixed-cohort rejection, and
   missing-versus-zero behavior. Black-box writers cover positive-progress short writes, zero
   progress, invalid counts, error before byte zero, error after every selected `N`, full-length
   `n,err`, and recoverable panic before/after retaining selected prefixes and the full length. They
   require the acceptance triple: canonical/hash-valid complete bytes, exit `0`, and empty stderr.
   Every error/panic yields exit `2` plus exact `OUTPUT_WRITE_FAILED`; a complete stdout retained
   before failure is still rejected.
2. `LOD-AT-P0-002` (`001`, `005`, `007..013`): the independent
   `corvint-dashboard-trace-conformance verify --root ROOT --manifest MANIFEST --snapshot SNAPSHOT`
   implementation consumes the exact planted repository/trace corpus, configured-source manifest,
   and snapshot; independently drives the same frozen upstream-Git authority profile; and reproduces
   every source, cohort, semantic witness row, issue role/precedence, count, byte total, outcome,
   exclusion, denominator, hash, and final snapshot byte.
   Empty, partial, invalid-member, and changed-store vectors prove exact `CONFIGURED_SOURCE` versus
   `RETAINED_MEMBER` behavior and missing-versus-zero separation. Query/impact, CEM/OCM, Pulse,
   harness, live, Frontier, spec-index, and Beamfall remain explicitly `NOT_OBSERVED`/unavailable.
3. `LOD-AT-P0-003` (`004..005`, `017..019`, `024..026`): argument vectors accept exactly 4,096
   bytes and reject 4,097 before opening a path under each qualified OS grammar. Descriptor/path
   spies cover symlinked components, FIFO/device/socket, hardlink qualification, traversal,
   grow/shrink, same-size rewrite,
   oversized input, aggregate overflow, hostile trace strings, and trace-directory add/remove/rename/
   replacement across enumeration. Independent code recomputes the sorted `trace-store/0` members,
   checked byte sum, aggregate digest, min/max observation interval, revision cohorts, and exact
   repository semantic-witness set. A non-ASCII `café` root, status-XY matrix, 64-path/8,192-byte
   chunk edges, and Windows 32,000-UTF-16-unit preflight prove the portable argv/path grammar.
   Nonempty inherited `GIT_OBJECT_DIRECTORY` and `GIT_ALTERNATE_OBJECT_DIRECTORIES`, plus spies that
   require omission of the former and the fixed empty child row for the latter, prove the exact
   startup-environment boundary. External include/includeIf targets, malformed config, linked-worktree
   reciprocal pointers,
   alternates, executable replacement between every lifecycle boundary, output/timeout/process-tree
   failure, retry, and cumulative-budget vectors prove the exact descriptor, executable, argv, cwd,
   environment, limit, Finish order, cleanup, and redaction profile; the include target is never
   opened. Cat-file canaries cover valid HEAD/revision/blob keys, shared OIDs, expected-type conflict,
   missing/promised/ambiguous objects, over-bound size, wrong type/ID, reordering, malformed/truncated/
   extra output, lazy-fetch spies, and absence of body/filter/textconv execution. Independent
   executions produce the same `repositoryReadsSha256`,
   incidental internal Git reads cannot change it, and a non-Git object parser cannot satisfy the
   profile. Spies confirm no writes, fetch, DNS, raw text, or implicit collection.
4. `LOD-AT-P0-004` (`009`): each exact declared CEM/OCM profile with all five flags produces the
   corresponding ordinal-`0` `UNSUPPORTED` source and only `NOT_OBSERVED` metrics without opening
   either path. Every missing/duplicate flag, over-4,096-byte or unknown profile, and profile supplied
   without the remaining required flags fails before a path open; flag order does not change bytes. Filename,
   map-derived revision, current local pairing, and default-path discovery never select a profile.
5. `LOD-AT-P0-005` (`024`): every pre-existing Corvint CLI golden and public verifier result is
   byte/decision identical with the dashboard package present and absent.

### P1 server and UI

6. `LOD-AT-P1-001` (`003`, `014..016`, `019..020`): HTTP body equals P0 bytes; wildcard/LAN/IPv6
   bind, poisoned Host/Origin/fetch metadata, token replay, cookie omission, cross-origin request,
   unexpected methods/bodies, path traversal, idle/hard expiry, slow client, signals, explicit EOF,
   and recoverable handler panic fail closed without listener/session/residue.
7. `LOD-AT-P1-002` (`016..018`, `022..023`): the frozen browser/OS matrix passes exact CSP/header and
   dependency audits, hostile-string injection, keyboard-only navigation, screen-reader equivalents,
   contrast/focus, reduced motion, 200% zoom, confirmed theme behavior, and narrow view.
8. `LOD-AT-P2-001` (`019..021`): restart, reconnect, forged Last-Event-ID, sequence gap, one-slot
   overflow, four-client pressure, and slow client always deliver a correct full snapshot or
   disconnect; no old generation attaches.

## Failure and degradation

| Failure | Required result |
|---|---|
| no trace directory or configured source | source row `NOT_PRESENT`; every usage/health metric `NOT_OBSERVED` |
| invalid or oversized source | typed issue, null content digest when unread, excluded source ID, no success contribution |
| mixed revision/cohort | separate cohorts; incompatible source excluded from single-cohort metrics |
| missing source-owned time | age `UNKNOWN`; file time ignored |
| unsupported adapter/profile | `UNSUPPORTED` source and null metrics; no inferred parser |
| CEM/OCM not explicitly and independently bound | bundle rejected; current local OCM pairing remains untouched and uncounted |
| historical collection disabled | retained inventory remains; total-use/rate/history `NOT_OBSERVED` |
| server/session failure | terminate or require fresh process/token; never anonymous fallback |
| unsupported browser | preserve P0 CLI snapshot; render no partial browser interpretation |
| Beamfall source before profile acceptance | reject `UNSUPPORTED`; canonical gate remains Beamfall-owned |

## Rollout, rollback, and frozen outcome trial

P0 lands only as additive `dashboard snapshot`. P1 adds an optional explicitly launched server and
embedded assets after its gates. P2 adds full-snapshot SSE. P3 may add a separately accepted bounded
usage journal only if the artifact-only baseline leaves a measured unanswered need.

Rollback is source-level removal of the isolated command registration/package for P0 or termination
and deletion/replacement of the optional P1 binary. No database, migration, service registration,
repository-installed state, remote account, or owned receipt is changed. Rollback acceptance builds
the candidate with and without dashboard registration and proves all pre-existing CLI goldens,
verifier decisions, files, and Beamfall workflows identical; P1 additionally proves no listener or
usable session remains after shutdown.

Before the outcome trial, register and hash 20 Corvint tasks, five Beamfall tasks, their expected source
artifacts, and these five questions:

1. What repository revision/tree and dirty/cohort state does this view represent?
2. Which configured sources are invalid, absent, unsupported, or excluded, and which denominators are
   unavailable?
3. What retained trace evidence exists, and what total-usage claims are prohibited?
4. What exact verification status, currency, scope, qualification, and containment were observed?
5. What Frontier or Beamfall fact is repository-authoritative, advisory, empty only over a declared
   universe, or not observed?

Use the same agent model, harness, tool permissions, and frozen answer key in a randomized crossover:
half of each repository's tasks use the receipt/CLI baseline first and half use the dashboard first.
Timing starts when frozen artifacts become available and ends when the agent submits all five answers
with source IDs. A task is useful only when at least four answers are fully correct, every material
qualifier is preserved, and no critical interpretation error occurs. A critical error is exactly:
stale/mixed shown current; missing/invalid shown zero or success; selected test shown whole-program
safety; structural/empty Frontier shown merge authority; Corvint shadow shown a Beamfall gate; or an
answer citing the wrong cohort.

Browser overhead is P1 authenticated snapshot-ready wall time minus P0 snapshot-ready wall time on the
same frozen inputs and machine, alternated for cache order. Keep the dashboard only with at least 40%
lower median correct-answer time than baseline, zero critical errors, zero hidden invalid denominator,
zero deliberate repository mutation or outbound/non-loopback disclosure, at least 80% useful tasks,
and median browser overhead no greater than two seconds. Any safety count kills P1 and retains P0 only;
failure of speed/usefulness kills or narrows the whole dashboard to the CLI snapshot.

## Non-goals and simpler baseline

The simpler baseline is existing bounded CLI envelopes and owning verifier reports. It remains fully
supported and authoritative.

V0 does not include hosted telemetry, accounts, repository upload, LAN/remote serving, a database,
embeddings, graph store, source browser, code editor, arbitrary query builder, raw log/prompt viewer,
path viewer, default developer-activity collection, team surveillance, billing, entitlement, policy
mutation, test execution, merge authorization, gate replacement, SSE deltas, or a claim that
visualization itself is a Corvint breakthrough.

## Traceability

| Requirement group | Delivery | First authoritative evidence |
|---|---:|---|
| `LOD-V0-001..006` P0 proof snapshot | not-started | independent canonical vectors, hash preimage, rooted-source adversarial corpus; `TestVerifyCanonicalRejectsSelfRehashedZeroMaxBytesObservation`; `TestScanTraceStoreIgnoresBackslashNonCandidateName`; `TestSourceIssueCodeMustBelongToAdapterRegistry`; `TestQualifyTraceIgnoresRepositoryGrafts`; `TestAccuracyDashboardCorpusIgnoresRepositoryGrafts`; `TestAuthorityCapsMalformedGitTranscriptAllocation` |
| `LOD-V0-007..013` P0 metrics | not-started | planted trace corpus with independent field-by-field recomputation and unavailable-source vectors; `TestStoreChangedEmitsUnobservedMemberUniverse`; `TestConfiguredBucketWithoutCompleteBytesHasNullBytes`; `TestInvalidScanRejectsObservedTraceUsage` |
| `LOD-V0-014..018` P1 local security/privacy | blocked | owner gates, exact HTTP adversarial suite, no-egress/disclosure audit |
| `LOD-V0-019..024` bounds/lifecycle/UI | P0 not-started; P1 blocked | P0 bounds/nonmutation, then qualified lifecycle/browser/accessibility matrix; `TestOverflowProbeAllowanceIsKeyedAndCappedPerWholeScanAttempt`; `TestScanTraceStoreUnstableMemberDoesNotSpendOtherMembersOverflowProbes`; `TestCompileSnapshotRetriesWithAttemptScopedProbesAndCumulativeSourceLedger`; `TestScanReportsExactTraceStoreBounds`; `TestScanCapsTraceRowAllocationBeforeParsing` |
| `LOD-V0-025..028` collection/rollout | not-started | Corvint-first then Beamfall-second preregistered trial |
| `LOD-V0-029..034` roadmap join (experimental) | experimental, unaccepted | `internal/dashboard/roadmap` unit tests and the real seeded-planning-store integration test |

## Unresolved decisions and hard blocks

- Owner acceptance of this full capability contract and scoped `AGENTS.md`/`docs/PRODUCT.md`
  amendment. P0 may remain experimental without the UI amendment; P1 cannot start.
- Owner confirmation of the separate engineering-control-room shape brief, including single-theme or
  system-adaptive behavior. P1 cannot start.
- Supported browser/operating-system qualification matrix. P1 cannot start.
- Whether P3 receives a separately accepted bounded local usage-journal profile. No P0/P1 code may
  create it implicitly.
