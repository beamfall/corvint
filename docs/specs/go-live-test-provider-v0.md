# Go Live-Test Provider V0

**Owner:** Russell Lewis  
**Date:** 2026-08-23  
**Intent status:** accepted (decision 0052)
**Delivery status:** experimental
**Profiles:** `go-live-capability/0`, `go-live-discovery/0`, `go-live-plan/0`,
`go-live-preview/0`, `go-live-event/0`, `go-live-run/0`, `go-live-error/0`,
`go-live-coverage-observation/0` (later gate only)

revision 12 (round 11 Claude + Codex applied)

## Agent digest
- Claim: An experimental Go provider exposes bounded discovery, planning, preview, run, and observation receipts without qualification authority.
- Status: accepted (decision 0052)/experimental
- Exists: `internal/liveverify` and retained source package `cmd/corvint-go-test-provider`, built standalone as `corvint-go-test-provider`, as experimental implementation surfaces.
- Blocked on: LPCV operation coverage, containment, network denial, independent provider qualification broader than the AT-14 fixtures below, the real frozen 30-minute `Execute` deadline (GLTP-V0-039 clause (b)), and broader Linux/external/full qualification and shadow/frontier/exclusion gates (NOT_RUN; see section 13). AT-14's GLTP-V0-038..043 qualification fixtures and the disclosed production changes — the parent `errorCode` preserved as a typed sentinel across seven edit sites (`cmd/corvint-go-test-provider/main.go:425-426`, `:430`, `:437`, `:440`, `:443`, `internal/liveverify/provider/provider.go:352`, and `:369-370`) and the receipt-composer STALE/TIMEOUT/INFRASTRUCTURE/CANCELLATION precedence with timeout-only `cancelled`-bit suppression (`internal/liveverify/provider/receipt.go:773-792`) — are delivered; the conformance suite and provider tests pass on the unsandboxed Darwin host, and a sandboxed `ps` denial is INCONCLUSIVE, never a silent pass. GLTP-V0-042's pre-launch refusal half and GLTP-V0-043's shadow-mode antecedent remain unevidenced and outside promotion, having no P0 host.
- Read next: Decision and boundary; Frozen V0 operation; Requirements.

## 1. Decision and boundary

`internal/liveverify/` contains 76 Go files and `cmd/corvint-go-test-provider` exists. This is
experimental implementation evidence only; it does not qualify this proposed provider.

Corvint may add one local experimental Go 1.27 live-test transcript producer. Its first slice executes
explicit package patterns with the pinned Go toolchain and normalizes `go test -json`/`test2json`
events. It is a dependency-free Go implementation and closed local wire format; it is not an LPCV
runtime provider, does not replace `go test` or repository gates, and does not change Go's own
meaning for test and coverage results.

This specification is a pre-qualification experiment beneath
[`live-proof-carrying-verification-v0.md`](live-proof-carrying-verification-v0.md), not an
implementation of all of that parent's requirements. A conflict fails closed in favor of the
parent. In particular, V0 has no affected-test selector, exclusion certificate, safe-omission
claim, repository-pass claim, or Wallaby parity claim. Every V0 run has scope `UNKNOWN`; passing
selected packages is useful observation, never proof that omitted tests, configurations, platforms,
packages, nested modules, or project gates are safe.

The first implementation may emit live **preview** frames. A preview is visibly provisional,
non-persistent, and never policy evidence. Canonical evidence events are emitted only after the
complete Workspace Execution Identity (WEI) is known and replay the bounded normalized facts under
that WEI. This preserves the parent's rule that evidence events carry a complete WEI without
pretending that dynamically observed test identities were known before execution.

This explicit-package slice is not a qualified LPCV provider. It intentionally lacks the parent's
complete operation set, parent-suite and source-anchor facts, qualified resource/process
containment, and default network denial. Missing Go runner facts remain explicit unknowns. It cannot
satisfy `LPCV-V0-021..022`, `LPCV-V0-038..039`, or `LPCV-V0-043..046`; no result from this profile is
current LPCV evidence or policy input. A later profile may claim LPCV compatibility only after those
requirements and their independent qualification gates pass.

## 2. Frozen V0 operation

The provider accepts an independently verified source materialization identity and a closed plan
produced from one direct `go list` acquisition. Its cwd is the verified source materialization root;
the fresh provider-owned HOME/cache/temp tree is separate from that cwd. It executes the logical
command:

```text
<pinned-go> test -json -count=1 -vet=off <package-pattern>...
```

No test-name filter, benchmark, fuzz target, profile, arbitrary Go flag, test-binary flag, shell,
command template, environment inheritance, watch daemon, affected-test inference, or parallel Corvint
run is in V0. Package concurrency performed internally by `go test` is preserved as a Go runner
fact. The repository's ordinary command remains the recovery path and its canonical gate remains
authoritative.

## Requirements

- **GLTP-V0-001.** Each V0 document is one UTF-8 canonical JSON object followed by exactly one LF.
Objects have only their profile's listed keys, sorted by raw UTF-8 key bytes. Arrays preserve order
only where this specification says it is semantic; otherwise they sort by canonical element bytes
and contain no duplicates. Values are strings, booleans, `null`, arrays, and objects only. Counts,
ordinals, byte sizes, durations, exit codes, and timestamps are strings. The codec rejects numbers,
whitespace outside strings, optional escapes, duplicate keys, invalid UTF-8, BOM, lone surrogates,
noncharacters, unknown keys, unknown enum values, depth over 16, or a document over 16 MiB before
materializing an unbounded value. Strings use the Change Frontier dependency-free escape rules.
Code blocks containing `A|B`, `decimal`, `value`, `string`, `...`, or `null` inside a quoted value
are closed schema notation, not example canonical bytes; a wire document contains exactly one
enumerated value with the type and bounds stated beside that block.

Every `decimal` value is canonical unsigned decimal: exactly `0` or `[1-9][0-9]*`, with no sign or
leading zero, and its parsed value must fit `uint64`. Counts, ordinals, byte sizes, durations, and
resource observations are additionally bounded by their plan limit or the named field's bound.
`exitCode` is `null` or canonical unsigned decimal in `0..4294967295`; signal termination uses the
separate `signal` field. A decoder checks length before numeric conversion and rejects overflow.

- **GLTP-V0-002.** Raw `go test -json` is runner input, not canonical Corvint JSON. Its decoder accepts
only one complete JSON object per LF-delimited line, rejects duplicate keys and invalid UTF-8, and
parses `Elapsed` from its decimal token without binary floating-point. Unknown fields or actions are
`UNSUPPORTED_RUNNER_EVENT`; they are never ignored. Go 1.27 `OutputType` is retained when present;
absence is valid only for a documented Go 1.27 action that omits it.

- **GLTP-V0-003.** `H(kind, profile, body)` is lowercase SHA-256 of
`u32be(len(kind)) || UTF8(kind) || u32be(len(profile)) || UTF8(profile) ||
u64be(len(body)) || body`, where `body` is canonical JSON without the terminal LF. A prefixed ID is
`<kind>:sha256:<digest>`. No host path, integer, or concatenated field is hashed without this exact
framing.

- **GLTP-V0-004.** The following identities are frozen:

| Field | Kind | Profile | Canonical body |
|---|---|---|---|
| capability `id` | `go-live-capability` | `go-live-capability/0` | complete capability without `id` |
| discovery `id` | `go-live-discovery` | `go-live-discovery/0` | complete discovery without `id` |
| plan `id` | `go-live-plan` | `go-live-plan/0` | complete plan without `id` |
| `runId` | `go-live-attempt` | `go-live-attempt/0` | `{nonce,planId}` |
| preview `id` | `go-live-preview` | `go-live-preview/0` | complete preview without `id` |
| event `id` | `go-live-event` | `go-live-event/0` | complete event without `id` |
| final `wei` | `workspace-execution` | `go-live-wei/0` | `{actualEnvironmentSha256,capabilityId,discoveredTestSetSha256,discoveryId,planId,toolchainId}` |
| run `id` | `go-live-run` | `go-live-run/0` | complete run without `id` |

`go-package` uses profile `go-package/0` and body exactly
`{depOnly,dirPathSha256,forTest,importPath,inputFilesSha256,kind,match,moduleSha256,name}`.
`match` is the sorted unique `Match` array and `moduleSha256` is `null` or the bare digest defined
in GLTP-V0-010. `inputFilesSha256` and
`dependencyMaterializationSha256` are bare digest portions of
`H("go-input-set","go-input-set/0",sorted [{mode,pathSha256,rawSha256}])`. `mode` is exactly four
lowercase octal permission digits for a verified regular file. `goenvSha256` is the bare digest portion of
`H("go-tool-output-set","go-tool-output-set/0",sorted [{argv,exitCode,stderrRawSha256,stdoutRawSha256}])`.

Path commitments are domain-separated and P0 accepts UTF-8 POSIX paths only. A native absolute path
digest is the bare digest of
`H("go-native-path","go-native-path/0",{"goos":<host-goos>,"path":<clean-absolute-path>})`.
The body is GLTP-V0-001 canonical JSON wherever it is computed, so the provider's `cwdPathSha256`
equals the parent's for every UTF-8 path, including one with a non-ASCII or DEL character.
A materialization-relative path digest is the bare digest of
`H("go-logical-path","go-logical-path/0",{"namespace":"SOURCE|DEPENDENCY|TOOLCHAIN|RUN","path":<clean-slash-relative-path>})`;
the path is `.` or non-empty segments without `.`, `..`, empty segments, backslash, control, or a
leading slash. `cwdPathSha256`, `rootPathSha256`, and toolchain `pathSha256` use the native form;
package directories and ordinary input files use the logical form in their verified namespace.
Run-owned artifact paths use the `RUN` logical form and never commit the random host prefix.

The sole generated-input exception is a Go 1.27 synthetic `TEST_MAIN` GoFile beneath run-owned
`GOCACHE`. Its `pathSha256` is instead the bare digest of
`H("go-generated-testmain","go-generated-testmain/0",{"importPath":<test-main-import-path>,"mode":<mode>,"rawSha256":<raw-digest>,"role":"TEST_MAIN_GOFILE"})`.
Its native absolute cache path is never committed. Any other run-generated discovery input is
`DISCOVERY_INPUT`.

`providerBuildSha256` is the bare digest of
`H("go-provider-build","go-provider-build/0",{"executableRawSha256":<raw-digest>,"goVersion":"go1.27.1"})`.
A bounded tree walk is the bare digest of
`H("go-tree-walk","go-tree-walk/0",sorted [{mode,pathSha256,rawSha256}])`, with logical
`TOOLCHAIN` paths. `gorootSha256` and `toolDirSha256` use that exact preimage. The bare
`invokedToolsSha256` digest uses
`H("go-invoked-tool-set","go-invoked-tool-set/0",sorted [{mode,pathSha256,rawSha256}])`.

The 32-hex `nonce` comes from the operating-system cryptographic random source and is generated
before launch. `eventRootSha256` is the bare digest of
`H("go-event-root","go-event-root/0",{"events":[{"digest":"64-lowercase-hex","sequence":"decimal"}]})`,
where the event array is in consecutive sequence order. Exact-byte SHA-256 fields hash every named raw byte,
including a terminal LF where one existed.

### 4. Closed capability and plan wire

- **GLTP-V0-005.** The capability document contains exactly:

```json
{"goProtocol":"go1.27/test2json","id":"go-live-capability:sha256:64-lowercase-hex","operations":["execute"],"profile":"go-live-capability/0","providerBuildSha256":"64-lowercase-hex","providerVersion":"semver","supportedContainment":["PROCESS_GROUP_BEST_EFFORT"],"supportedCoverage":["NONE"],"supportedNetwork":["UNKNOWN"]}
```

The operation and coverage arrays are frozen in the displayed order. `operations:["execute"]` is
the complete local-experiment surface and explicitly does not satisfy LPCV-V0-021. V0 admits only
the exact `go1.27.1` toolchain tuple named in the plan; this is format conformance, not LPCV
qualification. A later Go release needs a new conformance tuple even when its output appears
compatible.

This exact capability is emitted only on the experimental POSIX host set `aix|darwin|dragonfly|freebsd|linux|netbsd|openbsd|solaris`.
`PROCESS_GROUP_BEST_EFFORT` maps exactly to a new process group plus group-directed termination and
to runner result `PROCESS_GROUP_BEST_EFFORT`; it is always unqualified. Windows and every unlisted
host emit pre-launch `UNSUPPORTED_PLATFORM`, do not emit this execute capability, and remain
unsupported until a separately specified kill-on-close Job Object or equivalent exists.

- **GLTP-V0-006.** A plan contains exactly:

```json
{
  "capabilityId":"go-live-capability:sha256:64-lowercase-hex",
  "coverage":{"mode":"NONE","packagePatterns":[]},
  "discoveryId":"go-live-discovery:sha256:64-lowercase-hex",
  "environment":[{"name":"NAME","valueSha256":"64-lowercase-hex"}],
  "id":"go-live-plan:sha256:64-lowercase-hex",
  "invocation":{"argv":["@PINNED_GO@","test","-json","-count=1","-vet=off"],"cwdPathSha256":"64-lowercase-hex","packagePatterns":["./..."]},
  "limits":{"coverageBytes":"268435456","coverageFiles":"4096","cpuMilliseconds":"1800000","eventBytes":"16777216","events":"100000","lineBytes":"1048576","memoryBytes":"4294967296","openFiles":"4096","outputBytes":"8388608","packages":"4096","processes":"1024","runMilliseconds":"1800000","tests":"100000"},
  "limitsEnforced":["EVENT_BYTES","EVENTS","LINE_BYTES","OUTPUT_BYTES","PACKAGES","RUN_TIME","TESTS"],
  "network":{"mechanism":null,"mode":"UNKNOWN"},
  "profile":"go-live-plan/0",
  "scope":{"conclusion":"UNKNOWN","excluded":[],"requestedPackagePatterns":["./..."],"unknownReasons":["BUILD_CONSTRAINT_VARIANTS","CROSS_PLATFORM_VARIANTS","DISCOVERY_INCOMPLETE","EXTERNAL_MODULE_FRONTIER","FUZZ_BENCHMARK_FRONTIER","MANDATORY_GATE_OUTSIDE_RUN","NESTED_MODULE_FRONTIER","NETWORK_STATE_UNKNOWN","NON_GO_TEST_FRONTIER","NO_AFFECTED_SELECTION_PROOF","PACKAGE_PATTERN_SEMANTICS","PARENT_TEST_UNAVAILABLE","SOURCE_ANCHOR_UNAVAILABLE","UNOBSERVED_DYNAMIC_SUBTESTS"]},
  "source":{"dependencyMaterializationSha256":"64-lowercase-hex","materializationSha256":"64-lowercase-hex","moduleMode":"MODULE|WORKSPACE|VENDOR","rootPathSha256":"64-lowercase-hex","wsi":"workspace-source:sha256:64-lowercase-hex"},
  "toolchain":{"cgoEnabled":"0","goarch":"value","goenvSha256":"64-lowercase-hex","goexeSha256":"64-lowercase-hex","goos":"value","gorootSha256":"64-lowercase-hex","goversion":"go1.27.1","id":"go-toolchain:sha256:64-lowercase-hex","invokedToolsSha256":"64-lowercase-hex","pathSha256":"64-lowercase-hex","toolDirSha256":"64-lowercase-hex"}
}
```

`coverage.packagePatterns` is empty and coverage mode is `NONE` in executable P0. The actual executable is the regular file identified by
`toolchain.pathSha256` and `goexeSha256`, never a PATH lookup after planning.

Package patterns are either the lone literal `./...` or one to 4,091 exact import paths from the
discovery manifest. Exact paths are 1..4096-byte normalized UTF-8 strings with no leading `-`,
`@`, comma, whitespace, control, backslash, empty segment, `.`, `..`, glob, or ellipsis segment. They
sort by UTF-8 bytes and contain no duplicates. `./...` cannot be combined with another pattern. The
4,091 bound leaves room for the executable and four frozen Go arguments within the 4,096-element
argv bound.
Future coverage patterns obey the exact-import-path grammar, so comma joining cannot change semantics.

The `limitsEnforced` array is frozen in the displayed order. A limit not present in it is a recorded
target ceiling, not an enforcement or safety claim, and prevents LPCV qualification.

- **GLTP-V0-007.** The environment array is the complete child environment for registered profile
`GO127_CGO0_OFFLINE_POSIX_0`. Names sort by UTF-8 bytes and are exactly `CGO_ENABLED`, `GOARCH`,
`GOCACHE`, `GOENV`, `GOFLAGS`, `GOMODCACHE`, `GONOPROXY`, `GONOSUMDB`, `GOOS`, `GOPRIVATE`,
`GOPROXY`, `GOROOT`, `GOSUMDB`, `GOTOOLCHAIN`, `GOTMPDIR`, `GOVCS`, `GOWORK`, `HOME`, `TEMP`, `TMP`,
and `TMPDIR`; no `PATH`, ambient
variable, dynamic-loader variable, credential variable, or caller addition is present. Values are
represented only by exact-byte digests in the wire. The executor retains the secret-screened values
ephemerally and re-hashes the complete `name=value` array immediately before child exec. The parent
checks the exact names, fixed values, and run-root paths at discovery, and its canonical plan binding
independently refuses an environment array whose names are not exactly this set or whose digests
differ from the fixed values below; it holds no run root, so the six run-root paths are not rechecked there.

The fixed values are `CGO_ENABLED=0`, `GOENV=off`, `GONOPROXY=`, `GONOSUMDB=`, `GOPRIVATE=`,
`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, and `GOVCS=*:off`; the fresh empty HOME makes
Go's default netrc authentication source empty. `GOOS`, `GOARCH`, and `GOROOT` equal the pinned toolchain
tuple. `GOFLAGS` is exactly `-mod=vendor` in `VENDOR` mode and `-mod=readonly` otherwise. `GOWORK` is
`off` in `MODULE|VENDOR` mode and the verified absolute `go.work` path in `WORKSPACE` mode.
`GOMODCACHE` is the separately verified read-only dependency materialization, never a writable
run-owned cache.

Before listing, the provider creates one absent-name run root outside the source materialization as
a mode-`0700` non-symlink directory. It creates fresh empty mode-`0700` children `home`,
`go-build-cache`, `tmp`, and `go-tmp`; `HOME`, `GOCACHE`, `TEMP|TMP|TMPDIR`, and `GOTMPDIR` respectively name those
exact children. It rejects any pre-existing component, link, non-directory, ownership mismatch, path
escape, or mode mismatch and revalidates every final component before list, before test exec, and
before deletion. Listing and execution share this same run root, so list may populate only the
run-owned writable cache. Provider-owned writable state is deleted after post-identities; the
caller-owned verified dependency materialization is not. Network state remains `UNKNOWN` because
test code has no independently qualified OS denial even though Go module/proxy/VCS acquisition is
disabled.

- **GLTP-V0-008.** `toolchain.id` is `H("go-toolchain","go-toolchain/0",body)` over the complete
toolchain object without `id`. `gorootSha256` and `toolDirSha256` use the exact bounded walk above;
`invokedToolsSha256` commits the compiler, assembler, linker, and cgo-disabled platform helpers
selected by the list/test action graph. Because the frozen argv contains `-vet=off` and coverage is
`NONE`, vet, cover, and cgo tools are excluded; observing one is `TOOLCHAIN_UNKNOWN`.
Corvint resolves the native absolute Go executable, rejects links in the admitted final components,
hashes all toolchain inputs immediately before and after execution, and binds raw output from direct
`go version` and `go env -json GOARCH CGO_ENABLED GOEXE GOOS GOROOT GOTOOLDIR GOVERSION` acquisitions
in `goenvSha256`. An executable, interpreter, dynamic-library, or environment identity Corvint cannot
pin produces `TOOLCHAIN_UNKNOWN`, not a run. The first executable tuple requires
`CGO_ENABLED=0`; a CGO-enabled tuple needs a registered compiler, linker, dynamic-library, and
environment identity plus separate conformance and containment evidence. Experimental P0 does not
claim race-free pathname execution against a malicious concurrent actor with the same UID: such an
actor, untrusted repository, or known concurrent tool/source mutator prevents launch. Mandatory
pre/post equality detects ordinary drift but is not promoted into a stronger race-prevention claim.

- **GLTP-V0-009.** The source WSI and materialization come from the parent verifier, not this provider.
They cover exact tracked, untracked, ignored, generated, nested-module/workspace, vendor, overlay,
configuration, and build-tag frontiers or enumerate them as unknown. The provider re-verifies the
materialization before package listing, immediately before launch, after the runner's process-group
cleanup and bounded pipe drain, and
after coverage finalization. It never writes generated files, coverage, cache, or temp data inside
the repository. The dependency materialization binds every consumed `go.mod`, `go.sum`, `go.work`,
`go.work.sum`, vendor manifest, local replacement, module-cache module, and standard-library source
byte plus mode and normalized path. It is complete, read-only, offline, and rechecked pre/post; a
missing or externally mutable input prevents launch. The parent lease ID also binds the native path
digests of the module-cache directory and the selected `go.work` (`null` outside workspace mode), so
revalidation under another path with identical content is drift, not the same lease.
P0 does not call process-group cleanup proof that all descendants exited. A detected escaped process
forces source/toolchain currency `UNKNOWN` and execution `INCOMPLETE`; an undetected daemon escape is
an explicit residual reason this transcript is not LPCV evidence.

### 5. Package acquisition and scope truth

- **GLTP-V0-010.** Planning runs the pinned Go executable directly with fixed logical argv
`go list -deps -test -json=Dir,ImportPath,Name,ForTest,Match,DepOnly,Module,GoFiles,CgoFiles,CFiles,CXXFiles,MFiles,HFiles,FFiles,SFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles,TestGoFiles,XTestGoFiles,TestEmbedFiles,XTestEmbedFiles,Imports,TestImports,XTestImports,Error,DepsErrors <package-pattern>...`
under the same source, cwd, toolchain, environment,
offline module cache, and recorded network state as execution. It emits one closed discovery document:

```json
{
  "argv":["@PINNED_GO@","list","-deps","-test","-json=closed-field-list","./..."],
  "dependencyMaterializationSha256":"64-lowercase-hex",
  "environmentSha256":"64-lowercase-hex",
  "exitCode":"0",
  "id":"go-live-discovery:sha256:64-lowercase-hex",
  "inputsSha256":"64-lowercase-hex",
  "moduleMode":"MODULE|WORKSPACE|VENDOR",
  "packages":[{"depOnly":false,"dirPathSha256":"64-lowercase-hex","forTest":"import/path|null","id":"go-package:sha256:64-lowercase-hex","importPath":"import/path","inputFiles":[{"mode":"0644","pathSha256":"64-lowercase-hex","rawSha256":"64-lowercase-hex"}],"inputFilesSha256":"64-lowercase-hex","imports":["import/path"],"kind":"DEPENDENCY|REQUESTED|TEST_MAIN|TEST_VARIANT","match":["./..."],"module":"normalized-module-body|null","moduleSha256":"64-lowercase-hex|null","name":"package-name","testImports":["import/path"],"xTestImports":["import/path"]}],
  "profile":"go-live-discovery/0",
  "rawStderrSha256":"64-lowercase-hex",
  "rawStdoutSha256":"64-lowercase-hex",
  "requestedRunnerPackages":["import/path"],
  "sourceWsi":"workspace-source:sha256:64-lowercase-hex",
  "toolchainId":"go-toolchain:sha256:64-lowercase-hex"
}
```

The literal `closed-field-list` in the schema expands to the exact comma-separated field order in
the command above. The bounded streaming decoder accepts only those Go 1.27 selected fields and
their documented omission rules, rejects duplicate object keys, and normalizes every optional field
to its closed commitment form. It records module/workspace/vendor mode; import path, the derived
`go-package` ID (never a nonexistent raw `Package.ID` field), `Name`, `ForTest`, `Match`, `DepOnly`,
normalized `Module`, and directory-path digest; `GoFiles`,
`CgoFiles`, `CFiles`, `CXXFiles`, `MFiles`, `HFiles`, `FFiles`,
`SFiles`, `SwigFiles`, `SwigCXXFiles`, `SysoFiles`, `EmbedFiles`, `TestGoFiles`, `XTestGoFiles`,
`TestEmbedFiles`, and `XTestEmbedFiles`;
imports and test imports; module and replacement identities; build errors; and exact output digests.
The published `inputFiles` array is their sorted unique normalized union and makes
`inputFilesSha256` recomputable; the generated-testmain rule above replaces only its native cache
path identity. The published `module` body makes `moduleSha256` recomputable. `inputsSha256` commits
the sorted package inputs plus module/workspace/vendor/dependency materialization. Only original
requested packages enter `requestedRunnerPackages`; synthetic test-main, test variant, and dependency
packages never require runner terminal events. Any unrecognized consumed input, decode error,
duplicate package identity, path escape, unresolved package, non-empty stderr, nonzero exit, bound
crossing, or source/toolchain/dependency drift prevents execution.

For this exact invocation, raw `Module` is `null` or contains only `Path`, `Version`, `Replace`,
`Time`, `Main`, `Indirect`, `Dir`, `GoMod`, `GoVersion`, `Sum`, and `GoModSum`; empty optional fields
may be omitted by Go. `Replace` is `null` or one object with the same keys and a null/omitted
`Replace`. Fields produced only by other `go list` modes (`Query`, `Versions`, `Update`, `Retracted`,
`Deprecated`, `Error`, `Origin`, and `Reuse`) are unsupported here. The normalized module body is
exactly
`{dirPathSha256,goModPathSha256,goModSum,goVersion,indirect,main,path,replace,sum,timeRaw,version}`;
nullable raw strings become strings or `null`, paths become contained normalized-path digests, and
`replace` recursively uses the same body with `replace:null`. `moduleSha256` is the bare digest of
`H("go-module","go-module/0",body)`.

Normalization order is non-circular: normalize the raw module and input-file union; compute
`moduleSha256`, `inputFilesSha256`, and classification; compute the derived `go-package` ID from the
GLTP-V0-004 body; then compute package-input bodies, `inputsSha256`, and finally the discovery ID.
`environmentSha256` is the bare digest of
`H("go-environment","go-environment/0",sorted-plan-environment-array)`. For each acquired package,
the package-input body is exactly
`{depOnly,forTest,id,imports,match,moduleSha256,name,testImports,xTestImports}` with nullable strings
made explicit and each string array sorted unique by UTF-8 bytes. `inputsSha256` is the bare digest
of `H("go-discovery-inputs","go-discovery-inputs/0",body)` where `body` is exactly
`{dependencyMaterializationSha256,moduleMode,packages,sourceWsi}` and `packages` is the package-input
body array sorted by canonical element bytes. These definitions, the exact raw output digests, and
the discovery body in GLTP-V0-004 are the complete discovery-ID preimage.

Classification is closed: `REQUESTED` has non-empty `Match`, empty `ForTest`, and `DepOnly:false`;
`TEST_VARIANT` has non-empty `ForTest`; `TEST_MAIN` has empty `ForTest`, empty `Match`,
`DepOnly:false`, and `Name:"main"`; every remaining object is `DEPENDENCY`. Any object matching more
than one rule or a non-dependency matching none is `DISCOVERY_DECODE`. `requestedRunnerPackages` is
exactly the sorted unique `ImportPath` set of `REQUESTED` objects. This set, not suffix inspection or
dependency inference, determines which runner packages require terminal events.

- **GLTP-V0-011.** Requested patterns are caller scope, not discovered package identities. The final
run separately enumerates requested patterns, listed packages, packages with runner events, tests
observed, tests executed, tests skipped, and package/test terminal states. A test identity is
`H("go-test","go-test/0",{"name":<entire Test value>,"package":<exact import path>})`; Corvint does
not split slash-delimited subtest names or infer parents.

- **GLTP-V0-012.** V0 `excluded` is always `[]`; P0 performs no negative-proof procedure for frontier
absence, so `unknownReasons` is always the exact full sorted array shown in GLTP-V0-006
and GLTP-V0-021. It does not conditionally omit build-constraint, cross-platform, external-module,
fuzz/benchmark, mandatory-gate, nested-module, non-Go-test, or package-pattern frontiers.
No coverage edge, successful list, zero exit, or package pass removes one.

- **GLTP-V0-013.** A package with no tests, a skipped test, a cached marker, and an unobserved test are
distinct. `-count=1` is mandatory. Any runner indication that cached output was reused makes the run
`INCOMPLETE`. A passing package is `PASSED_IN_EXPLICIT_SCOPE`; it is never a repository, module,
gate, affected-test, or change-safety conclusion.

### 6. Runner normalization

- **GLTP-V0-014.** Go 1.27 `go test -json` interleaves two closed input shapes. A TestEvent has only
`Time`, `Action`, `Package`, `Test`, `Elapsed`, `Output`, `OutputType`, `FailedBuild`, `Key`, `Value`,
and `Path`; its actions are `start`, `run`, `pause`, `cont`, `output`, `pass`, `bench`, `fail`,
`skip`, `attr`, and `artifacts`. A BuildEvent has only `ImportPath`, `Action`, and `Output`; its
actions are `build-output` and `build-fail`. Action distinguishes the shapes before any optional
field is interpreted. The exact action is retained as `rawAction`; normalization never upgrades it.

- **GLTP-V0-015.** A normalized fact payload contains exactly:

```json
{"artifactPathSha256":null,"attribute":null,"durationNanoseconds":null,"factKind":"TEST","failedBuild":null,"output":{"byteCount":"0","outputType":null,"sha256":"64-lowercase-hex","text":null,"truncated":false},"package":"import/path","parentTestId":null,"rawAction":"start","sourceAnchor":null,"test":null,"timeRaw":"2026-08-23T12:00:00-04:00","timeUtc":"2026-08-23T16:00:00Z"}
```

A normalized BuildEvent fact contains exactly:

```json
{"factKind":"BUILD","importPath":"package-id","output":{"byteCount":"0","sha256":"64-lowercase-hex","text":null,"truncated":false},"rawAction":"build-fail"}
```

`Elapsed` must be a non-negative base-10 seconds token with at most nine fractional digits and is
converted exactly to nanoseconds. `Time` accepts RFC3339Nano with an explicit `Z` or numeric offset;
the exact input is retained and its instant is normalized to UTC. The frozen `go test -json`
invocation requires non-empty `Package` and `Time` on every TestEvent; their absence is a runner
matrix failure, not a cached-result exception. `OutputType` is absent/empty only
for `REGULAR`, otherwise exactly `frame`, `error`, or `error-continue`. `attr` requires `Test`; raw
`Key` and `Value` are strings, and omission means the empty string because Go 1.27 encodes those
fields with `omitempty`. Its normalized `attribute` is exactly
`{"keySha256":"64-lowercase-hex","valueSha256":"64-lowercase-hex"}`, hashing the exact UTF-8
bytes including empty values; `attribute` is `null` for every other action. `artifacts` requires
`Test` and non-empty `Path`; `artifactPathSha256` is its contained normalized-path digest and is
`null` otherwise. `Key`, `Value`, and `Path` are forbidden on all other actions.
`start|run|pause|cont|output|attr|artifacts` forbid `Elapsed`; `pass|fail` require it; `skip|bench`
permit it because Go 1.27's timestamped converter emits elapsed values for timed skip/benchmark
reports. `output` alone permits `Output` and `OutputType`; all non-output facts use the empty-byte
digest, byte count `0`, null type/text, and `truncated:false`. `FailedBuild` occurs only on a
package-level `fail`. Package-level events omit `Test`; test-level terminal events require it.
Go documentation does not provide parent-suite or source-anchor facts, so both remain `null` and
`PARENT_TEST_UNAVAILABLE` plus `SOURCE_ANCHOR_UNAVAILABLE` enter the unknown frontier. `Output` is
the post-test2json UTF-8 string (invalid original bytes have already become replacement characters),
not the test process's raw bytes. Its UTF-8 bytes are hashed and `text` remains `null` in the first
implementation. `Key`, `Value`, and `Path` are inert; values are digest-only and artifact paths are
accepted only after containment validation.

- **GLTP-V0-016.** The provider validates separate build, requested-runner-package, and whole-test-name
state machines. A package is `ABSENT -> STARTED -> PASSED|FAILED|SKIPPED`; a test is
`ABSENT -> RUNNING <-> PAUSED -> PASSED|FAILED|SKIPPED|BENCH`; output/attr/artifacts do not change
state. A build identity is `ABSENT -> OUTPUT* -> FAILED` or remains nonterminal when no build failure
was reported. Only `requestedRunnerPackages` require package terminals. Parallel interleaving is
valid; arrival order is observation order, not a Go semantic total order. Because the frozen plan
does not enable benchmarks, a decoded `bench` action makes this P0 run `INCOMPLETE` while remaining
available to the exact runner-decoder corpus. At EOF every test that entered `RUNNING` or `PAUSED`
must have exactly one terminal, and every requested package must have exactly one terminal; any
Test-bearing output/attr/artifacts action requires an existing nonterminal test state, and any
remaining nonterminal state makes execution `INCOMPLETE`. Duplicate/conflicting
terminals, action after terminal, continue without pause, pause without run, TestEvent package absent
from discovery, failed-build mismatch, build failure without failed package/process failure, or
event/exit disagreement makes execution `INCOMPLETE`.

- **GLTP-V0-017.** Runner output, package names, test names, file paths, failure text, and timestamps
are inert evidence. They are never executed, interpolated, used as a path without independent
canonicalization, or decoded into terminal control sequences. Machine renderers preserve JSON
escaping and label all such content `UNTRUSTED_RUNNER_OUTPUT`.

### 7. Preview, evidence stream, and terminal closure

- **GLTP-V0-018.** A preview contains exactly
`{id,kind,payload,planId,profile,runId,sequence}`. `kind` is `STARTED|RUNNER_FACT|LIMIT|RETRACTED`.
The payload is respectively exactly `{"started":true}`, one GLTP-V0-015 TEST or BUILD fact,
`{"limit":"COVERAGE_BYTES|COVERAGE_FILES|CPU|EVENT_BYTES|EVENTS|LINE_BYTES|MEMORY|OPEN_FILES|OUTPUT_BYTES|PACKAGES|PROCESSES|RUN_TIME|TESTS"}`,
or `{"reason":"CANCELLED|CLIENT_DISCONNECT|DECODE|LIMIT|NEWER_WSI|PROVIDER_FAILURE|SOURCE_DRIFT|TOOLCHAIN_DRIFT"}`.
Sequence is a canonical decimal string starting at `0` and increasing by one. Preview frames are
memory-only, visibly provisional, cannot be queried as current evidence, and are retracted
immediately on a newer WSI, cancellation, malformed input, limit, or provider failure.

- **GLTP-V0-019.** After execution, coverage finalization, and the post-source/toolchain checks, Corvint
computes the discovered-test-set digest and final WEI. It then emits canonical evidence events,
each containing exactly `{fact,id,profile,runId,sequence,wei}`, where `fact` is a normalized fact.
Sequence starts at `0`, is consecutive, and preserves original observation order. The terminal run
document is emitted as the final line after all events; no byte may follow it.

- **GLTP-V0-020.** `actualEnvironmentSha256` is the bare digest portion of
`H("go-environment","go-environment/0",body)` over
the sorted exact plan environment array after re-hashing the retained values at child exec.
`discoveredTestSetSha256` is the bare digest portion of
`H("go-discovered-tests","go-discovered-tests/0",body)` over exactly
`{"completeness":"INCOMPLETE","packages":[sorted requested runner package strings],"tests":[sorted
go-test IDs]}`; V0 completeness is always `INCOMPLETE` because dynamic subtests not reached are not
discoverable. The final WEI uses the exact preimage row in GLTP-V0-004 with these two bare digests.
Each event ID is `H("go-live-event","go-live-event/0",event-without-id)`; its 32-byte digest is the
decoded suffix of that ID. Absence of a test event is not proof a test does not exist. Events with a
wrong WEI, sequence gap/duplicate, malformed line, duplicate terminal, bytes after terminal, replay
under another run, or event-root mismatch are unusable evidence.

- **GLTP-V0-021.** The terminal `go-live-run/0` object contains exactly:

```json
{
  "coverage":{"artifactSha256":null,"completeness":"ABSENT","mode":"NONE","packages":[],"rawRootSha256":null},
  "ephemeralDeletion":"COMPLETE|INCOMPLETE",
  "eventCount":"decimal",
  "eventRootSha256":"64-lowercase-hex",
  "execution":{"cancelled":false,"containment":{"mechanism":"PROCESS_GROUP_BEST_EFFORT","qualified":false},"durationNanoseconds":"decimal","exitCode":"decimal|null","failureClass":"ASSERTION_OR_TEST|BUILD|DISCOVERY|INFRASTRUCTURE|TIMEOUT|CANCELLATION|CRASH|STALE|UNKNOWN|null","limit":"string|null","resources":{"cpuMilliseconds":"decimal|null","memoryPeakBytes":"decimal|null","openFilesPeak":"decimal|null","processesPeak":"decimal|null"},"signal":"SIGINT|SIGTERM|SIGKILL|OTHER|null","status":"PASSED|FAILED|INCOMPLETE","timedOut":false},
  "id":"go-live-run:sha256:64-lowercase-hex",
  "io":{"decoder":"COMPLETE|REJECTED|TRUNCATED","stderrBytes":"decimal","stderrDrained":true,"stderrRawSha256":"64-lowercase-hex","stdoutBytes":"decimal","stdoutDrained":true,"stdoutRawSha256":"64-lowercase-hex"},
  "persistence":"NONE",
  "planId":"go-live-plan:sha256:64-lowercase-hex",
  "profile":"go-live-run/0",
  "qualification":"EXPERIMENTAL_TRANSCRIPT",
  "runId":"go-live-attempt:sha256:64-lowercase-hex",
  "scope":{"buildTerminals":[{"importPath":"string","status":"FAILED"}],"conclusion":"UNKNOWN","excluded":[],"executedTests":["go-test:sha256:..."],"listedPackages":["go-package:sha256:..."],"observedTests":["go-test:sha256:..."],"packageTerminals":[{"package":"import/path","status":"PASSED|FAILED|SKIPPED"}],"requestedPackagePatterns":["./..."],"skippedTests":["go-test:sha256:..."],"testTerminals":[{"status":"PASSED|FAILED|SKIPPED|BENCH","test":"go-test:sha256:..."}],"unknownReasons":["BUILD_CONSTRAINT_VARIANTS","CROSS_PLATFORM_VARIANTS","DISCOVERY_INCOMPLETE","EXTERNAL_MODULE_FRONTIER","FUZZ_BENCHMARK_FRONTIER","MANDATORY_GATE_OUTSIDE_RUN","NESTED_MODULE_FRONTIER","NETWORK_STATE_UNKNOWN","NON_GO_TEST_FRONTIER","NO_AFFECTED_SELECTION_PROOF","PACKAGE_PATTERN_SEMANTICS","PARENT_TEST_UNAVAILABLE","SOURCE_ANCHOR_UNAVAILABLE","UNOBSERVED_DYNAMIC_SUBTESTS"]},
  "sourceCurrency":"CURRENT|STALE|UNKNOWN",
  "sourcePostSha256":"64-lowercase-hex|null",
  "sourcePreSha256":"64-lowercase-hex",
  "toolchainCurrency":"CURRENT|STALE|UNKNOWN",
  "toolchainPostSha256":"64-lowercase-hex|null",
  "toolchainPreSha256":"64-lowercase-hex",
  "wei":"workspace-execution:sha256:64-lowercase-hex"
}
```

All identity arrays contain the element shapes shown and are sorted unique; package/build/test
terminal arrays sort by canonical element bytes. `listedPackages` is exactly every discovery
package ID. `observedTests` is exactly every test ID named by any TestEvent; `executedTests` is
exactly every test ID that entered `RUNNING`; `skippedTests` is exactly every test ID with a
`SKIPPED` terminal; and `testTerminals` contains exactly one element for every terminal test state.
`packageTerminals` contains exactly one element for every requested runner package, and
`buildTerminals` contains exactly every failed build identity. `listedPackages` elements are derived
`go-package` IDs, never raw Go package IDs. Vertical bars and descriptive placeholders in this schema
block denote mutually exclusive allowed values, never literal wire strings. `PASSED` requires exit zero, every requested runner
package terminal pass, every entered test terminal, complete event decode and pipe drain, current
source and toolchain observations, complete ephemeral deletion, no cache marker, no semantic bound
crossing, and a terminal document. A requested runner package whose terminal is `skip` (for
example, a package with no test files) is not a pass; the provider transcript's execution
classification applies the same rule and never reports `PASSED` for a run whose receipt cannot. `FAILED` requires one observed product/test assertion or build
failure consistent with the runner and process exit plus current source/toolchain observations and
complete ephemeral deletion;
discovery/infrastructure/timeout/cancellation/crash are `INCOMPLETE` with their own failure class.
For a current-currency receipt, `cancelled:true` requires `failureClass:"CANCELLATION"` except when
a post-launch provider failure coexists: the truthful cancellation bit remains true and the
provider-failure record is `failureClass:"INFRASTRUCTURE"`. Every other current-currency
`cancelled:true` receipt with a non-`CANCELLATION` class is rejected. Source or toolchain drift
continues to take the `STALE` precedence described below.
`STALE` is never collapsed into product failure. A run does not require coverage `COMPLETE`. A coverage failure keeps
an otherwise valid test status but leaves overall scope `UNKNOWN` and coverage `PARTIAL|REJECTED`.
Source or toolchain drift sets its currency axis to `STALE` or `UNKNOWN`, forces execution
`INCOMPLETE` with failure class `STALE`, and prevents current display; it is never `PASSED` or
`FAILED`.
`sourcePreSha256`/`sourcePostSha256` are the bare digest portions of the parent WSI recomputation
immediately before exec and after cleanup; `toolchainPreSha256`/`toolchainPostSha256` are the bare
digest portions of `H("go-toolchain-observation","go-toolchain-observation/0",body)` over the exact
toolchain file/input commitment and direct version/env outputs at those points. Equality yields
`CURRENT`; inequality yields `STALE`; an absent post-observation yields `UNKNOWN`. These fields make
currency recomputable rather than asserted.

- **GLTP-V0-022.** A coordinator crash may prevent a terminal document. Consumers then synthesize no
run receipt: they discard previews and treat retained bytes as an incomplete diagnostic transcript.
No prior pass is reused. Late frames from an older run can never attach to a newer WSI or WEI.

### 8. Later coverage gates (not executable P0)

- **GLTP-V0-023.** A later Go unit-coverage slice MUST use only Go's public `-coverprofile` interface. The
provider creates an absent target path beneath a fresh, empty, mode-`0700`, run-owned directory
outside the repository, passes that exact path in direct argv, opens the result without following
links, and accepts only one bounded regular file created during this run. It validates the coverage
profile grammar, exact mode, package/file mapping, counts, source containment, and terminal process
state before canonicalizing it. A missing, pre-existing, linked, replaced, malformed, mixed-mode,
escaping, oversized, or post-source-mutated profile is `PARTIAL|REJECTED`. Corvint MUST NOT scrape
private `go test -work` directories or rely on cmd/go's undocumented temporary layout.

- **GLTP-V0-036.** The ordinary unit-coverage form in GLTP-V0-021 MUST use mode
`UNIT_COVERPROFILE_SET`, `UNIT_COVERPROFILE_COUNT`, or `UNIT_COVERPROFILE_ATOMIC` for a validated
profile whose Go mode is respectively `set`, `count`, or `atomic`; `NONE` remains reserved for
absent coverage. For an accepted profile, `rawRootSha256` MUST be lowercase hexadecimal SHA-256 of
the complete raw regular-file byte sequence exactly as read, including its original line endings
and any final line terminator. It has no `H` framing or domain separator.

`artifactSha256` MUST be lowercase hexadecimal SHA-256 of the following canonical profile byte
sequence, also without `H` framing or a domain separator:

```text
UTF8("mode: " || go-mode || "\n") ||
  for each canonical-block in ascending raw-UTF-8-byte order:
    UTF8(canonical-block || "\n")
```

The profile is read as UTF-8 lines with LF separators removed and one trailing CR removed from each
line, including a final non-empty unterminated line. The first line MUST otherwise be exactly
`mode: <go-mode>`. Each later line is split on Unicode whitespace into exactly three fields and MUST
contain no remaining CR or tab. Its canonical block is exactly
`<profilePath>:<startLine>.<startColumn>,<endLine>.<endColumn> <statements> <count>`: `profilePath`
is preserved byte-for-byte from the first field; the six unsigned decimal values are parsed and
re-emitted without signs or leading zeroes; and the separators are the shown ASCII colon, periods,
comma, and single spaces. Duplicate canonical blocks MUST be rejected. Zero blocks produce only the
canonical mode line. `packages` MUST contain the unique mapped import-path strings for packages
having at least one accepted block, including a zero-count block, sorted by raw UTF-8 bytes. The
package array, coverage object, and all other metadata MUST NOT enter either digest preimage. This
clause records the pre-existing `internal/liveverify/gorunner/coverage.go` behavior and does not
change it.

- **GLTP-V0-037.** A unit-coverage receipt MUST use exactly `UNIT_COVERPROFILE_SET`,
`UNIT_COVERPROFILE_COUNT`, or `UNIT_COVERPROFILE_ATOMIC` for a validated profile whose Go mode is
respectively `set`, `count`, or `atomic`; a receipt carrying any other unit-coverage mode token is
invalid. `rawRootSha256` MUST be lowercase hexadecimal SHA-256 of the complete raw profile bytes
exactly as read. `artifactSha256` MUST be lowercase hexadecimal SHA-256 of the canonical profile
bytes produced by `parseCoverageProfile`: the exact UTF-8 mode line with one LF, followed by each
canonical block with one LF in ascending raw-UTF-8-byte order. The receipt's `packages` array MUST
contain the unique mapped import-path strings for accepted blocks in ascending raw-UTF-8-byte order
and MUST NOT enter either digest preimage. This clause freezes the pre-existing
`coverageObservationMode`, `coverageCapture.finish`, and `parseCoverageProfile` behavior in
`internal/liveverify/gorunner/coverage.go` and does not change it.

- **GLTP-V0-024.** `GOCOVERDIR` and `go tool covdata` are reserved for separately instrumented
integration binaries, not ordinary `go test` unit coverage. Their frozen observation contains
exactly:

```json
{"artifactSha256":"64-lowercase-hex|null","completeness":"COMPLETE|PARTIAL|REJECTED","mode":"INTEGRATION_COVDATA_SET|INTEGRATION_COVDATA_ATOMIC","packages":[],"profile":"go-live-coverage-observation/0","rawRootSha256":"64-lowercase-hex","runId":"go-live-attempt:sha256:64-lowercase-hex","toolchainId":"go-toolchain:sha256:64-lowercase-hex","wei":"workspace-execution:sha256:64-lowercase-hex"}
```

The producing plan must explicitly build an instrumented binary with the same pinned toolchain,
launch it with `GOCOVERDIR` set to a fresh run-owned directory, and bind its argv, binary digest,
environment, source, and process tree. No such producing plan is executable under this first slice;
the capability MUST NOT advertise an integration-covdata mode until a separate accepted execution
profile supplies it. Once supplied, Corvint accepts only bounded regular `covmeta.*` and
`covcounters.*` files from that directory and invokes the same pinned Go executable directly as
`go tool covdata merge`, `percent`, and `textfmt`. It rejects symlinks, external hard links, devices,
sockets, path escapes, pre-existing files, mixed build IDs/toolchains/modes, corruption, truncation,
or bounds. The canonical artifact is validated bounded `textfmt` output; raw-directory-set and
artifact digests are both retained in the observation.

- **GLTP-V0-025.** Either coverage form is one combined run/test-set observation. It does not establish per-test
coverage, an exclusion certificate, a test claim, branch correctness, or absence of an unobserved
edge. Crash, timeout, cancellation, nonzero exit, missing metadata/counters, mixed instrumentation,
coverage-tool failure, truncation, or source/toolchain drift makes it `PARTIAL|REJECTED`, never
`COMPLETE`. `ABSENT` and zero observed coverage are distinct.

### 9. Execution safety and bounds

- **GLTP-V0-026.** Every child uses direct structured argv, closed stdin, the verified cwd, minimal
bound environment, concurrently drained pipes, and an owned process tree. No shell, PATH relookup,
response file, Go toolchain download, repository text interpolation, or caller command string is
allowed. The complete dependency cache is verified read-only before launch and Go proxy/sumdb/VCS
access is disabled. P0 records network as `UNKNOWN`; therefore it is an experimental transcript and
cannot become parent-qualified or current policy evidence. It may run only after an explicit local
user action against repository-trusted code, remains off by default, and is not a sandbox or safe
service for untrusted tests. Output limits count combined raw stdout and stderr bytes before
decoding and never permit unbounded line/slice growth. The runner retains at most that bounded raw
prefix ephemerally for decoding; it exposes no arbitrary pre/post/stream callback whose failure to
return could defeat lifecycle bounds.

- **GLTP-V0-027.** The frozen plan limits in GLTP-V0-006 are fixed target ceilings, not values callers
may raise. Only the names in `limitsEnforced` are hard P0 enforcement claims; every omitted resource
limit is visibly unqualified. JSON depth is 16; any string is at most 1 MiB; argv has at most 4,096 elements and 4 MiB;
environment has at most 256 entries and 1 MiB; normalized output is digest-only and capped at 8
MiB raw per run. P0 enforces every `limitsEnforced` member and records CPU, memory, process, and
open-file target ceilings with observed peaks `null` where unavailable. These gaps block parent
qualification; later OS profiles must enforce and advertise them. A crossed enforced bound, or an
observed crossing of an unqualified resource target, terminates the owned
tree and yields `INCOMPLETE + UNKNOWN`; it is never silent truncation compatible with pass.

`PACKAGES` counts normalized discovery package objects, `TESTS` counts unique `go-test` identities,
and `EVENT_BYTES` counts canonical JSON body bytes of retained normalized facts before evidence
wrapping. The acquisition/reducer checks each increment before append. `RUN_TIME` ends at the plan
deadline, after which the POSIX runner allows one shared fixed two-second shutdown deadline.
`exec.Cmd.WaitDelay` bounds a child that survives cancellation; runner-owned OS pipes and drain
goroutines use the remainder of the same deadline, after which both read ends are closed. Expiry is
`EXECUTION_PIPE + INCOMPLETE`, never a successful drain; invocation return is bounded by the run
deadline plus two seconds and bounded post-identity/deletion work. After a normal child exit, an
inherited-pipe-only delay is independently capped at the same two seconds.

- **GLTP-V0-028.** Cancellation, timeout, SIGINT, SIGTERM, normal exit, decode failure, pipe failure,
provider crash recovery, upgrade, and uninstall must attempt descendant termination and remove
provider-owned temp/cache/current-result state. Qualification is per OS: Linux may use an
independently tested stronger primitive and Windows uses a kill-on-close Job Object. POSIX process
groups cannot contain escaped or daemonized descendants and therefore do not qualify a zero-
descendant or sandbox claim on their own. Unsupported containment is explicit and blocks promotion
for that OS. Group termination and normal-exit cleanup return an observable success/error; an
unexpected group-kill or fallback-kill error is `EXECUTION_CONTAINMENT`, sets process cleanup
incomplete, and prevents `PASSED|FAILED`. A process that creates a new session/group may escape P0;
the two-second pipe bound still prevents it from holding the invocation open, but P0 cannot claim to
terminate that daemon and remains unsuitable for untrusted or daemonizing tests.

- **GLTP-V0-029.** Network, CPU/memory/process/open-file enforcement, and strong containment remain
unqualified in executable P0 and are visible unknowns. The provider persists no
state: raw environment, runner output, credentials, payloads, caches, binaries, discovery bytes,
events, receipts, and coverage artifacts are caller-streamed or ephemeral and deleted before the
invocation returns. A terminal run contains `persistence:"NONE"` and
`ephemeralDeletion:"COMPLETE|INCOMPLETE"`; incomplete deletion makes execution `INCOMPLETE` and
blocks current evidence. Any future retention, expansion handle, or caller-owned storage is outside
the provider and requires an accepted retention/deletion profile.
GLTP-V0-044 separately governs the product-command interlock and future launch refusal for a new
containment profile.
Only provider-owned writable HOME/cache/temp state is covered by this deletion claim. The verified
read-only dependency materialization and source materialization are caller-owned inputs. A surviving
escaped descendant or timed-out deletion forces `ephemeralDeletion:"INCOMPLETE"`; P0 never reports
complete deletion while such use is observed.

### 10. Conformance and experimental implementation gate

- **GLTP-V0-030.** The dependency-free Go P0 implementation and one independent verifier must produce
byte-identical capability, plan, event, WEI, and terminal bytes for every accepted fixture and the
same stable rejection code for every rejected fixture. Fuzzing covers the canonical codec, runner
decoder, duration conversion, state machines, path mapper, and event/run verifier. Coverage
inventory/verifiers are a later gate and do not block the event-only P0 implementation.

- **GLTP-V0-031.** Hostile vectors include duplicate keys, invalid UTF-8, numeric overflow, deep/wide
JSON, newline amplification, giant lines, unknown Go fields/actions/`OutputType`, malformed elapsed
or time, ANSI/control output, hostile package/test names, identical tests in distinct packages,
parallel tests/subtests, naive slash-parent traps, cached results, no-tests packages, build/list
failure, panic, exit/event disagreement, missing/duplicate/conflicting terminals, sequence gaps,
post-terminal bytes, pipe-close races, and stale late events.

- **GLTP-V0-032.** Before a later coverage capability may be advertised, its vectors include empty, missing, partial, corrupt, truncated, oversized,
mixed-build/toolchain, forged-name, symlink, hard-link, device, socket, path-escape, cross-run residue,
and source-mutated covdata. Lifecycle vectors cover timeout, cancellation, SIGINT, SIGTERM, parent
crash, restart, client disconnect, descendant escape, cleanup failure, upgrade, downgrade, and
uninstall on every claimed OS.

- **GLTP-V0-033.** The event-only P0 may be implemented behind an experimental flag after the exact
Go 1.27 BuildEvent/TestEvent corpus, canonical golden corpus, malformed-input suite, timeout/
cancellation cleanup suite, nonmutation check, and independent verifier pass locally. It emits only
`EXPERIMENTAL_TRANSCRIPT + UNKNOWN` and has coverage `NONE`.

Runner-parity fixtures invoke the absolute pinned executable with the exact environment, source cwd,
and argv `<go> test -json -count=1 -vet=off <packages>`. Decoder-breadth fixtures may separately use
Go 1.27 flags such as `-artifacts`/`-outputdir`, but they are labelled decoder-only and cannot count
as runner parity. PATH lookup, inherited environment, reordered/extra flags, or default vet likewise
cannot satisfy the runner-parity gate.

Production-parent test fixtures select the installed Go 1.27.0 toolchain separately from the
compiler running the test harness. `CORVINT_GO_LIVE_TEST_GOROOT` is a test-only absolute root override;
without it the fixtures use the harness GOROOT. They resolve that root and require its executable's
actual `version` output to match the frozen version and host before constructing an authority
bundle or copying a drift-fixture executable. A missing or mismatched toolchain fails setup with
an actionable diagnostic; it is never skipped, downloaded, version-rewritten, or accepted by
weakening parent verification. The explicit override permits a newer harness compiler while all
production observations, toolchain hashes, and independent receipt checks use the pinned toolchain.

It remains off by default and cannot become parent-qualified or current evidence until all of the
following promotion evidence is recorded:

1. all requirement-to-test mappings and hostile vectors pass under Go 1.27.1;
2. independent canonical-byte verification is exact across 10,000 seeded runs;
3. Corvint self-dogfood and Beamfall each complete 200 explicit full-package shadow runs with zero
   stale attachment, event loss, false package-pass, repository mutation, or cleanup residue;
4. every observed mismatch with each repository's ordinary pinned Go command and canonical gate is
   retained, classified, repaired, and replayed; and
5. cold/warm latency, memory, output, cancellation, and 24-hour process/resource
   profiles are published without omitting failures.

This gate permits an explicit Go live-test observation, not automatic affected-test selection,
safe omission, repository-pass, merge authorization, cross-language support, or Wallaby parity.

- **GLTP-V0-034.** One false pass, stale attachment, cross-run coverage merge, command/policy
suppression, escaped owned descendant on a claimed platform, or conformance differential disables
the provider tuple immediately. Recovery deletes rebuildable state, restores the last qualified
binary/configuration atomically, and uses the repository's ordinary Go command. No repository file,
schema migration, database, or remote service is required for rollback.

- **GLTP-V0-035.** A P0 failure before child launch emits exactly one bounded error document and no run
receipt. After child launch, every handled decode, limit, cancellation, timeout, pipe, cleanup,
source, or toolchain failure closes with an `INCOMPLETE` run receipt under GLTP-V0-021; only an
unrecoverable coordinator crash may produce no terminal under GLTP-V0-022.

```json
{"code":"RUNNER_UNKNOWN_ACTION","detailSha256":"64-lowercase-hex","phase":"RUNNER","profile":"go-live-error/0","runId":"go-live-attempt:sha256:64-lowercase-hex|null"}
```

`phase` is exactly `CANONICAL|DISCOVERY|ENVIRONMENT|EXECUTION|IDENTITY|LIMIT|RUNNER|SOURCE|TOOLCHAIN`.
`code` is exactly one of `CANONICAL_DUPLICATE_KEY`, `CANONICAL_INVALID_UTF8`,
`CANONICAL_NONCANONICAL`, `CANONICAL_UNKNOWN_FIELD`, `DISCOVERY_DECODE`, `DISCOVERY_DRIFT`,
`DISCOVERY_EXIT`, `DISCOVERY_INPUT`, `ENVIRONMENT_DRIFT`, `EXECUTION_CANCELLED`,
`EXECUTION_CONTAINMENT`, `EXECUTION_EXIT`, `EXECUTION_PIPE`, `EXECUTION_TIMEOUT`,
`IDENTITY_MISMATCH`, `LIMIT_EXCEEDED`, `RUNNER_BUILD_STATE`, `RUNNER_FIELD_MATRIX`,
`RUNNER_MALFORMED`, `RUNNER_PACKAGE_STATE`, `RUNNER_TEST_STATE`, `RUNNER_UNKNOWN_ACTION`,
`SOURCE_STALE`, `SOURCE_UNKNOWN`, `TOOLCHAIN_STALE`, `TOOLCHAIN_UNKNOWN`, or
`UNSUPPORTED_PLATFORM`. `detailSha256` is the bare digest of
`H("go-live-error-detail","go-live-error-detail/0",{"code":<code>,"detail":<stable-token>,"phase":<phase>})`.
`detail` is a 1..128-byte uppercase ASCII token composed only of `A-Z`, `0-9`, `_`, `-`, `.`, and `/`;
it contains no host path, runner text, environment value, or secret. The typed diagnostic body is
local and never placed on the wire. The error document itself has no authority, pass, or
current-evidence meaning.

**Proposed pre-launch `detail` token table (AT-14, GLTP-V0-035).** `detail` above is an open lexical
field, not an enum, and this table closes nothing: it is proposed evidence enumerating the values
`prelaunchDiagnostic` can emit before child launch today
(`cmd/corvint-go-test-provider/main.go:208-220`). A later value is added to this table; the open field
above does not reject it.

| `detail` | `code` | `phase` | Emitted at |
|---|---|---|---|
| `UNSUPPORTED_PLATFORM` | `UNSUPPORTED_PLATFORM` | `EXECUTION` | `cmd/corvint-go-test-provider/main.go:210-211` |
| `DISCOVERY_OUTPUT_LIMIT` | `LIMIT_EXCEEDED` | `LIMIT` | `cmd/corvint-go-test-provider/main.go:212-213` |
| `PARENT_AUTHORITY_UNAVAILABLE` | `IDENTITY_MISMATCH` | `IDENTITY` | `cmd/corvint-go-test-provider/main.go:214-215` |
| `PARENT_AUTHORITY_DRIFT` | `IDENTITY_MISMATCH` | `IDENTITY` | `cmd/corvint-go-test-provider/main.go:216-217` |
| `PROVIDER_PRELAUNCH_REJECTED` | `IDENTITY_MISMATCH` | `IDENTITY` | `cmd/corvint-go-test-provider/main.go:218-219` |

### 11. AT-14 qualification requirements

This section, raised by roadmap ticket AT-14 ("Qualify current verification observations and
shadow exclusions"), is now fixtured. GLTP-V0-038..043 are experimental-slice qualification
fixtures for the provider surface already described above, delivered in
`internal/liveverify/provider/qualification_test.go`,
`internal/liveverify/parentverify/scope_qualification_test.go` (its
`composeScopeQualificationReceipt` helper is consumed by the composer-precedence cases in
`internal/liveverify/parentverify/verifier_test.go`, lines 123-145),
`cmd/corvint-go-test-provider/qualification_test.go`,
`cmd/corvint-go-test-provider/qualification_lifecycle_test.go`,
`conformance/go-live-test-v0/qualification_test.go`, and the STALE-precedence regression
`conformance/go-live-test-v0/event_test.go`'s `stale-cannot-be-cancelled-and-timed-out` case; none
changes the meaning of GLTP-V0-001..037. A conflict with any earlier clause in this document fails
closed in favor of the earlier clause. Amendment disclosure: the production changes this section
discloses for GLTP-V0-039 and GLTP-V0-040, neither confined to a single line, are delivered below.
GLTP-V0-039 clause (b) (a real 30-minute `Execute` timeout), broader Linux/external/full
qualification, and shadow/frontier/exclusion gates remain BLOCKED or NOT_RUN; see section 13. On
the REVALIDATE and DISCOVER path the parent's `errorCode` must survive as a typed sentinel across
seven edit sites: the parent transport-failure wrap at
`cmd/corvint-go-test-provider/main.go:425-426`, the parent-response unmarshal failure at
`cmd/corvint-go-test-provider/main.go:430`, the non-canonical-response return at
`cmd/corvint-go-test-provider/main.go:437`, the rejection wrap itself at
`cmd/corvint-go-test-provider/main.go:440`, the invalid-operation-shape return at
`cmd/corvint-go-test-provider/main.go:443`, the DISCOVER wrap at
`internal/liveverify/provider/provider.go:352`, and
`internal/liveverify/provider/provider.go:369-370`, which today discards the lease error entirely
and returns `fmt.Errorf("%w: pre-launch revalidation", ErrAuthorityDrift)`, and must instead
propagate that error with `%w`. The ACQUIRE rejection wrap at
`cmd/corvint-go-test-provider/main.go:287` is deliberately not an edit site: every `Acquire` failure
is re-wrapped as `ErrAuthorityUnavailable` at `internal/liveverify/provider/provider.go:315-316`,
so preserving a sentinel there would be a no-op. Because those seven sites discard the parent's
`errorCode` today, every `Revalidate` failure reaches the provider layer as `ErrAuthorityDrift` and
every `Discover` failure as `ErrAuthorityUnavailable`, and the `PARENT_AUTHORITY_DRIFT` token
discriminates nothing. No edit site may leave two named sentinels in one error chain. Every other
return on that requesting path — `cmd/corvint-go-test-provider/main.go:411`, `:417`, and `:420` —
attaches no named sentinel and is a stated default of `PROVIDER_PRELAUNCH_REJECTED`
(`cmd/corvint-go-test-provider/main.go:219`). GLTP-V0-039 below discloses the only other production
change this section requires, to the receipt composer's `failureClass` precedence.

- **GLTP-V0-038.** A recorded plan ceiling that is absent from the frozen `limitsEnforced` array of
GLTP-V0-006 is a target, not an enforcement claim, and does not by itself withhold `PASSED`; P0's
CPU, memory, open-file, and process-count ceilings are four such cases. This clause is scoped to
the state in which `limitsEnforced` is the fixed literal inside the plan map `BindCanonical` builds
at `internal/liveverify/parentverify/canonical.go:76`, with no runtime, profile, or fixture input
varying it; the package-level literal in that file is `unknownReasons`
(`internal/liveverify/parentverify/canonical.go:12-18`). Because `limitsEnforced` is not a
package-level symbol, its fixture MUST call `BindCanonical` and inspect the plan it returns.
While that literal stands, the wire representation of each of those four axes MUST be
the existing pair, unchanged: the axis absent from `limitsEnforced` and its `execution.resources.*`
peak `null` — the four nulls being the unconditional `execution.resources` object the receipt
composer writes at `internal/liveverify/provider/receipt.go:560`. This clause freezes that object's
four fields and nothing else at that site; the same line also emits `"signal":null`, which this
clause does not govern. The wire pair is therefore frozen only for the four axes that have an
`execution.resources` field. The plan's `coverageBytes` and `coverageFiles` ceilings
(`internal/liveverify/parentverify/canonical.go:71`) are likewise absent from `limitsEnforced`, and
`COVERAGE_BYTES` and `COVERAGE_FILES` are limit tokens
(`conformance/go-live-test-v0/event.go:571`), but they have no `execution.resources` peer to pair
with, so the target-not-claim rule above governs them alone and no wire pair is frozen for them.
That does not
narrow GLTP-V0-027. P0 observes no peak for any of the four, so
GLTP-V0-027's observed-crossing branch is unreachable for them in this state; that branch governs a
later profile which does observe those peaks, where they stop being `null`. This clause MUST add no
receipt key, no enum member, and no new token, leaving the closed `decimal|null` resource fields and
GLTP-V0-001's rejection of unknown keys and unknown enum values exactly as they stand. Ordinary P0
runs MUST keep the outcomes GLTP-V0-027 and GLTP-V0-021 already fix. P0 qualification therefore
asserts the current-profile invariants: the enforced set MUST be exactly `EVENT_BYTES`, `EVENTS`,
`LINE_BYTES`, `OUTPUT_BYTES`, `PACKAGES`, `RUN_TIME`, `TESTS` in the frozen GLTP-V0-006 order, with
the CPU, memory, open-file, and process-count axes absent from it and their four receipt resource
fields emitted as `null`. Two forward constraints hold once the enforced set stops being a literal.
A profile that advertises an axis its executing OS profile has no enforcement path for MUST NOT
report `PASSED`: that run closes `INCOMPLETE` with `failureClass:"UNKNOWN"`. That is an
independent obligation of this clause, not an extension of GLTP-V0-027's trigger, which stays
exactly as accepted — a crossed enforced bound or an observed crossing of an unqualified resource
target — and which an unmet enforcement claim with no crossing does not satisfy; GLTP-V0-021
supplies only `UNKNOWN`'s membership in the closed `failureClass` enum.
The mirror input is an axis the executing OS profile does enforce but which `limitsEnforced` omits:
that is under-advertisement, not an unmet claim, so it MUST NOT withhold `PASSED` on that ground
alone, and a crossing of it is an unqualified-target crossing, terminating the owned tree and
yielding `INCOMPLETE` with `failureClass:"UNKNOWN"` under GLTP-V0-027. Neither forward constraint is
exercisable while the literal stands: the current-profile invariants above are fixtured in P0, and it
is only these two forward constraints that lack a P0 antecedent.

- **GLTP-V0-039.** For an `Execute` call that launched a child, whose pre-run and post-run source and
toolchain observations are both current, which was not cancelled, in which no runner,
authority, or cleanup failure was raised, and in which the stream is incomplete, or a requested
package or test reported no terminal (`internal/liveverify/provider/receipt.go:773` requires
`allRequestedPassed` for `PASSED`, so such a run falls through to
`internal/liveverify/provider/receipt.go:789-790`), or the exit is
non-zero with no test terminal reporting a failure — these three disjuncts describe the reachable
production inputs once `TimedOut` forces `complete` false; they are not fixture discriminators,
since setting `Runner.TimedOut` alone already forces every one of them regardless of stream
completeness, terminal reporting, or exit code — termination on the elapsed run deadline MUST
(a) compose
a receipt with `failureClass:"TIMEOUT"` and `status:"INCOMPLETE"`, and MUST (b) leave zero surviving
descendants in the run's own process group once `Execute` returns — the scope cleanup can reach, a
descendant that has left the group being outside this obligation under GLTP-V0-028. Clause (b) is established by the observation protocol below, not
by the group-kill call sequence alone, and the two obligations carry separate evidence traced
separately in section 13. `PASSED` and `FAILED` are excluded unconditionally, not by the antecedent
above: `complete` at `internal/liveverify/provider/receipt.go:756` requires
`!input.Runner.TimedOut`, so a timed-out run can never reach the classifier's `PASSED`/`FAILED` arms
(`internal/liveverify/provider/receipt.go:773-780`) regardless of stream completeness or test
outcome. The fixture MUST assert this with an adversarial case: a synthetic input with `TimedOut`
true, a complete stream, exit zero, and every requested package/test `PASSED` still composes
`INCOMPLETE`/`TIMEOUT`, never `PASSED` or `FAILED`. Below those two, GLTP-V0-030's independent
verifier (`conformance/go-live-test-v0/event.go:631`, `:643`) requires `timedOut:true` to pair with
`failureClass:"TIMEOUT"` and `cancelled:false` unconditionally, so this clause discloses a second
production change, alongside GLTP-V0-040's: the classifier at
`internal/liveverify/provider/receipt.go:773-792` MUST rank STALE first, then TIMEOUT, then
INFRASTRUCTURE via the `ProviderFailure` input assembled at
`internal/liveverify/provider/provider.go:497`, then CANCELLATION, and MUST force the wire
`cancelled` field to `false` whenever `Runner.TimedOut` is true, because the verifier rejects any
receipt carrying both bits true regardless of `failureClass`. GLTP-V0-021 supplies only the closed
`failureClass` enum; this clause fixes the ranking and the bit-suppression the verifier requires,
and the composer and the independent verifier MUST be tested together on every coexistence case
below. Source or toolchain drift still keeps `INCOMPLETE + STALE`, unaffected by this reordering. A
pipe or drain failure and a cleanup failure raised alongside an otherwise clean timeout both set
the identical provider-failure input (`internal/liveverify/provider/provider.go:497`) — one
coexistence case, not two — and now classify `INCOMPLETE + TIMEOUT` with `limit:"RUN_TIME"`, not
`INFRASTRUCTURE`. Cancellation raised alongside an otherwise clean timeout likewise classifies
`INCOMPLETE + TIMEOUT` with the wire `cancelled` field forced `false`, not `CANCELLATION`. A failure
before child launch emits exactly one GLTP-V0-035 error document and no receipt at all. This is
independent of GLTP-V0-028's process-layer termination proof: it asserts the same typed-outcome,
zero-descendant guarantee at the `Execute`/receipt layer, not only at the process-group signal
layer that GLTP-V0-028 already covers.
For a current-currency run cancelled with a coexisting post-launch provider failure,
`ProviderFailure` still outranks `Runner.Cancelled`, but timeout-only bit suppression does not apply:
the composer MUST retain `cancelled:true` with `failureClass:"INFRASTRUCTURE"`, and the independent
verifier MUST accept exactly that non-`CANCELLATION` coexistence. It MUST continue to reject every
other current-currency `cancelled:true` receipt whose class is neither `CANCELLATION` nor
`INFRASTRUCTURE`. The producer/verifier round trip uses `validReceiptInput` with both inputs true;
a direct verifier discriminator uses `failureClass:"UNKNOWN"` and MUST reject it. No wire key or enum
member is added: the INFRASTRUCTURE failure class is the provider-failure record available to the
independent verifier.
When cancellation initiates termination and the bounded post-cancellation pipe drain subsequently
observes `OUTPUT_BYTES` exhaustion, cancellation remains the execution cause: the composer MUST
emit `cancelled:true`, `failureClass:"CANCELLATION"`, and `limit:null`. This is not GLTP-V0-027's
limit-initiated termination case; the decoder remains `TRUNCATED` and retained raw output remains
bounded. The composer and independent verifier MUST be tested together on this coexistence.
The elapsed run deadline has no fast fixture seam, not no host: the child deadline is
`plan.Timeout`, taken from `config.Timeout` (`internal/liveverify/provider/provider.go:435`) and
armed and handled at `internal/liveverify/gorunner/runner.go:186`, `:198-202`, so it does elapse and
terminate a real `Execute` call — just only after the frozen 30-minute
`gorunner.MaxRunTime` (`internal/liveverify/provider/provider.go:597-598`,
`internal/liveverify/gorunner/runner.go:34`), which `config.Timeout` MUST equal and which the
plan's own `runMilliseconds` freezes (`internal/liveverify/parentverify/canonical.go:74`) with no
runtime, profile, or fixture input varying it. Clause (a) is therefore qualified at the
receipt-composition layer over a synthetic runner result whose `TimedOut` the fixture sets
directly, never by letting a deadline elapse inside `Execute`; a real 30-minute `Execute` fixture is
not accepted here on time-budget grounds and this P0 slice records that block rather than running
one. Clause (b) inherits the same block: no P0 fixture lets `Execute` terminate on the deadline
within an acceptable run time, so its process-group evidence stays unevidenced until either a
disclosed fast timeout seam exists or a 30-minute fixture is accepted elsewhere.

- **GLTP-V0-040.** The identity boundary is qualified by three deterministic cases, one axis at a
time, and there is no binary-layer forgery seam to qualify. The `AuthorityBinding` the provider
binary hands to `Execute` is the parent's own ACQUIRE response
(`cmd/corvint-go-test-provider/main.go:286-289`); the attachment bundle carries no source, dependency,
toolchain, capability, or plan identity, which is what its own comment means by containing no
semantic identity (`cmd/corvint-go-test-provider/main.go:44-46`). It does carry two executable-integrity
digests, `verifierExecutableRawSha256` (`cmd/corvint-go-test-provider/main.go:50`) and
`gitExecutableRawSha256` (`cmd/corvint-go-test-provider/main.go:52`). The attachment exchange itself is
bound by challenge, parent capability digest, and response canonicality
(`cmd/corvint-go-test-provider/main.go:408-437`). "Execute against A while every commitment carries
B" is therefore unconstructible at that layer given the bundle-named parent verifier as trust root
(`cmd/corvint-go-test-provider/main.go:49`), which the binary validates only as a clean absolute path
(`cmd/corvint-go-test-provider/main.go:257`) whose file digest matches the bundle's own
`verifierExecutableRawSha256` (`cmd/corvint-go-test-provider/main.go:263`) — integrity against
post-bundle substitution of the named file, not authenticity of the name. Under that root the
dependency and toolchain axes have no forged input. The source axis is constructible one layer down. A library-layer `AuthorityLease` whose
`Binding()` returns B's `SourceIdentity` while its `ObserveReceiptIdentities` re-reads A is refused
by the pre-launch receipt observation, which compares `SourceIdentity` alone
(`internal/liveverify/provider/provider.go:385-387`, the comparison at line 386). That case MUST be
refused before launch, and its obligation splits across two layers, because the forged source
binding is constructible only at the library layer while the error document is written only by the
binary (`cmd/corvint-go-test-provider/main.go:203-204`, mapped by `prelaunchDiagnostic` at
`cmd/corvint-go-test-provider/main.go:208-220`). At the library layer the call MUST fail with typed
`ErrAuthorityUnavailable`, launch no child, and compose no receipt. At the binary layer a separate
unit case MUST assert `prelaunchDiagnostic`'s mapping directly — `ErrAuthorityUnavailable` to
`code:"IDENTITY_MISMATCH"`, `phase:"IDENTITY"`, and `detail:"PARENT_AUTHORITY_UNAVAILABLE"`
(`cmd/corvint-go-test-provider/main.go:211-215`) — so that deleting that mapping fails the fixture. For the dependency and
toolchain axes the only reachable variant is post-ACQUIRE mutation of a fixture-owned module cache
and a fixture-owned copy of the pinned toolchain executable, both created under `t.TempDir()` and
never the machine's own `go` binary or shared module cache, and that is drift, not forgery: both
digests feed the lease body
(`internal/liveverify/parentverify/snapshot.go:136-142`), so the next `Revalidate` recomputes a
different lease ID and returns `ErrDrift` (`internal/liveverify/parentverify/types.go:114-116`).
`AUTHORITY_DRIFT` is what the parent puts in the `errorCode` field of its attachment sub-protocol
response for that error (`cmd/corvint-go-test-provider/main.go:491`, via `productionErrorCode` at
`cmd/corvint-go-test-provider/main.go:617-628`); it is a field of that sub-protocol, explicitly not a
GLTP-V0-035 `code`, whose enum above is closed and admits no such member. That `errorCode` was
previously discarded: the requesting side wrapped the rejection into an untyped error with no `%w`,
and the provider layer wrapped every `Revalidate` failure as `ErrAuthorityDrift` regardless of
cause, so a parent transport failure (`cmd/corvint-go-test-provider/main.go:425-426`), a
non-canonical parent response (`cmd/corvint-go-test-provider/main.go:432-437`), and a genuine drift
rejection all arrived as drift and `detail:"PARENT_AUTHORITY_DRIFT"` discriminated nothing. The one
production change disclosed for this section, scoped to the REVALIDATE and DISCOVER path, is
delivered: the requesting side's rejection wrap now preserves the parent's `errorCode` as a typed
sentinel via `authoritySentinel` (`cmd/corvint-go-test-provider/main.go:610-616`, called with `%w` at
`:440`), and the provider layer's DISCOVER and REVALIDATE wraps propagate it through
`leaseSentinel` (`internal/liveverify/provider/provider.go:262-267`, called at `:352` and `:370`)
in place of the unconditional wrap, so the transport-failure and non-canonical-response paths
(neither `errors.Is`-tagged as drift) now fall through to `ErrAuthorityUnavailable` instead of
`ErrAuthorityDrift`. `TestDiscoveryAuthoritySentinelsRemainDistinct` and
`TestPrelaunchRevalidationErrorPreservesUnavailable`
(`internal/liveverify/provider/qualification_test.go`) qualify the library-layer distinction;
`TestProductionParentDriftAndTransportAreDistinct` and `TestUnavailableSourceDiagnosticMapping`
(`cmd/corvint-go-test-provider/qualification_test.go`) qualify it at the binary layer against the
real production parent.
ACQUIRE-time failures are outside this table and MUST keep arriving as `ErrAuthorityUnavailable`,
because `internal/liveverify/provider/provider.go:315-316` re-wraps every `Acquire` failure into
that sentinel whatever the binary returned; `cmd/corvint-go-test-provider/main.go:287` is therefore
not an edit site. The per-site obligation is exactly:

| Edit site | Obligation |
|---|---|
| `cmd/corvint-go-test-provider/main.go:425-426` (parent transport failure) | attach `ErrAuthorityUnavailable` |
| `cmd/corvint-go-test-provider/main.go:430` (parent response unmarshal failure) | attach `ErrAuthorityUnavailable` |
| `cmd/corvint-go-test-provider/main.go:437` (non-canonical or unattached response) | attach `ErrAuthorityUnavailable` |
| `cmd/corvint-go-test-provider/main.go:440` (parent rejection carrying `errorCode`) | map the parent's `errorCode` to a sentinel |
| `cmd/corvint-go-test-provider/main.go:443` (invalid operation shape) | attach `ErrAuthorityUnavailable` |
| `internal/liveverify/provider/provider.go:352` (DISCOVER wrap) | when `err != nil` wraps a recognised sentinel, propagate it with `%w` instead of the unconditional `ErrAuthorityUnavailable` wrap, so `ErrAuthorityDrift` survives; when `err != nil` wraps no recognised sentinel — including the untyped `errors.New` values `cmd/corvint-go-test-provider/main.go:297` and `:300-302` return for a transport failure or an invalid base64/oversized discovery payload — keep the `ErrAuthorityUnavailable` wrap so no naked error reaches the caller; when `err == nil` — the overflow-only branch at `internal/liveverify/provider/provider.go:351` — keep `ErrAuthorityUnavailable`, there being nothing to propagate |
| `internal/liveverify/provider/provider.go:369-370` (REVALIDATE wrap) | propagate with `%w`; map any unrecognised lease error to `ErrAuthorityUnavailable` |

That is seven disclosed edit sites, and no site may leave two named sentinels in one chain:
`prelaunchDiagnostic` tests `ErrAuthorityUnavailable` before `ErrAuthorityDrift`
(`cmd/corvint-go-test-provider/main.go:211-217`), so a chain carrying both would always report
`PARENT_AUTHORITY_UNAVAILABLE` and the drift token would stay unreachable. The `errorCode` mapping
the third of them applies is fixed here,
and `productionErrorCode` emits exactly four values: `AUTHORITY_DRIFT` MUST surface as
`ErrAuthorityDrift`; and `AUTHORITY_UNAVAILABLE`, `LIMIT_EXCEEDED`, and `INVALID_REQUEST` MUST all
surface as `ErrAuthorityUnavailable`.
Collapsing `LIMIT_EXCEEDED` and `INVALID_REQUEST` onto `ErrAuthorityUnavailable` is deliberate, not
a lost distinction: what this run reports is an identity-boundary refusal, and the parent's own
limit is not the child's `LIMIT` phase, so neither may be reported as a `LIMIT_EXCEEDED`/`LIMIT`
diagnostic for this run. `TestProductionParentDriftAndTransportAreDistinct`
(`cmd/corvint-go-test-provider/qualification_test.go`) directly faults only
`cmd/corvint-go-test-provider/main.go:425-426` (transport), `:440` (rejection wrap, the
`AUTHORITY_DRIFT` mapping), and `internal/liveverify/provider/provider.go:352` and `:369-370`;
`main.go:430` (unmarshal), `:437` (non-canonical response), and `:443` (invalid operation shape),
and the `AUTHORITY_UNAVAILABLE`/`LIMIT_EXCEEDED`/`INVALID_REQUEST` mappings onto
`ErrAuthorityUnavailable`, have no dedicated positive fault fixture and rely only on the
no-two-sentinels prose above; a per-site fault fixture with a positive and a negative `errors.Is`
assertion for each of those five sites and three mappings remains future evidence, not delivered by
this row. With that change in place each of those two cases MUST likewise be
refused before launch, with `code:"IDENTITY_MISMATCH"`, `phase:"IDENTITY"`, and
`detail:"PARENT_AUTHORITY_DRIFT"` (`cmd/corvint-go-test-provider/main.go:216-217`), and with no child
launched and no receipt composed. Because `parentverify.Discover` revalidates before it lists
(`internal/liveverify/parentverify/discovery.go:40`), a post-ACQUIRE mutation raises `ErrDrift`
inside the DISCOVER call, which `internal/liveverify/provider/provider.go:351-352` re-wraps as
`ErrAuthorityUnavailable` today. The mutation MUST be performed inside the ACQUIRE-side parent
process itself, by test-side code that runs after that process has answered ACQUIRE and before it
exits. That is the only constructible mechanism: the verifier executable is the test binary
(`cmd/corvint-go-test-provider/main_test.go:473`), re-exec'd once per attachment call by `TestMain`
(`cmd/corvint-go-test-provider/main_test.go:67-79`), so each exchange is a fresh process and `Execute`
offers no fixture seam between ACQUIRE and DISCOVER. That hook has no environment channel: the
re-exec'd parent is handed exactly one variable
(`cmd/corvint-go-test-provider/main.go:423`) and `runProductionAuthority` returns 2 unless its whole
environment is that single name (`cmd/corvint-go-test-provider/main.go:449`), so a test-only variable
is refused. `request.Operation` alone cannot gate the hook: `main_test.go:67-79` is the one shared
re-exec branch every attachment call carrying a `Bundle` takes, so keying on `Operation ==
"ACQUIRE"` would mutate the module cache and toolchain copy for every other test's
production-parent ACQUIRE in the package, not only the drift fixture's. The hook therefore keys on
a fixture-owned sentinel path segment the drift fixture alone writes into `request.Bundle` —
`moduleCacheDirectory` (`cmd/corvint-go-test-provider/main.go:58`) or `goExecutable`
(`cmd/corvint-go-test-provider/main.go:53`), attached to every request at
`cmd/corvint-go-test-provider/main.go:395-397` — mutating only when that segment is present, so an
unrelated test's ACQUIRE (with ordinary `t.TempDir()` paths carrying no such segment) never
matches.
`cmd/corvint-go-test-provider/main_test.go:78` calls `runQualificationAuthority`
(`cmd/corvint-go-test-provider/qualification_test.go:24`), the drift fixture's routing hook keyed on
`driftFixtureSegment` (`cmd/corvint-go-test-provider/qualification_test.go:19`); it is a test-only
file, so this is not a second production change. Both exchanges MUST route to the real
`runProductionAuthority` — which `runQualificationAuthority` calls directly at
`cmd/corvint-go-test-provider/qualification_test.go:27` and `:38` — and never to the stateless
`runAuthorityHelper` (`cmd/corvint-go-test-provider/main_test.go:544-600`), which always answers
`Status:"OK"` and sets no `ErrorCode`: a fixture-written `AUTHORITY_DRIFT` string would exercise
none of `internal/liveverify/parentverify/discovery.go:40`,
`internal/liveverify/parentverify/snapshot.go:136-142`, or
`internal/liveverify/parentverify/types.go:114-116`, leaving every production-parent citation in
this clause unexercised and the case a wire-mapping test.
`internal/liveverify/provider/provider.go:352` is a disclosed edit site that MUST propagate with
`%w` so `ErrAuthorityDrift` survives to `prelaunchDiagnostic` and the fixture can assert
`detail:"PARENT_AUTHORITY_DRIFT"`. The `detail` assertions are the binary-layer half only. At the
library layer the drift fixture MUST assert the error chain `Execute` returns — its second return
value, the error composed at `internal/liveverify/provider/provider.go:352` — directly:
`errors.Is(err, ErrAuthorityDrift)` true AND `errors.Is(err, ErrAuthorityUnavailable)` false, so an
implementation that keeps the unconditional `ErrAuthorityUnavailable` wrap and merely joins the
inner error fails rather than passing on the switch order at
`cmd/corvint-go-test-provider/main.go:211-215`. The drift token is falsifiable only alongside a
negative case, so the fixture MUST also assert that a parent transport failure on the same DISCOVER
exchange — pinned there, because at ACQUIRE
`internal/liveverify/provider/provider.go:315-316` re-wraps every failure regardless of all seven
edits and the control would pass with none of them applied — yields
`detail:"PARENT_AUTHORITY_UNAVAILABLE"` and not `detail:"PARENT_AUTHORITY_DRIFT"`, and mirrors the
chain assertion: `errors.Is(err, ErrAuthorityUnavailable)` true AND
`errors.Is(err, ErrAuthorityDrift)` false. Both chain assertions are taken under a clean-release
precondition. The deferred release at `internal/liveverify/provider/provider.go:319-327` runs on
every return once the lease is held and, on release failure, sets `ErrAuthorityDrift`
(`internal/liveverify/provider/provider.go:322`) and joins it into the returned error
(`internal/liveverify/provider/provider.go:324`), which would put `ErrAuthorityDrift` in the
negative control's own chain. Each fixture's parent therefore fails only the exchange under test —
the negative control's fails the DISCOVER transport alone and answers RELEASE `Status:"OK"` — and
each fixture MUST assert that release succeeded before it asserts the chain. That deferred join is
a disclosed exception to the no-two-sentinels rule above, which binds the seven edit sites and not
the release defer; no edit site introduces it. The dependency and toolchain drift cases MUST use a
fixture-owned temporary GOROOT, not a bare copied binary:
`internal/liveverify/provider/provider.go:579-580` requires `goExecutable` equal
`filepath.Join(goRoot, "bin", executableName(GOOS))`, so the fixture
lays out `<tmp>/bin/<name>` under `t.TempDir()` before ACQUIRE. Because
`cmd/corvint-go-test-provider/main.go:533-538` routes RELEASE through the same
`parentverify.Revalidate` call as REVALIDATE, a mutation left in place after the DISCOVER exchange
would make RELEASE re-observe the same drift and fail, breaking the clean-release precondition
above; the fixture MUST therefore restore the mutated module cache or toolchain executable to its
pre-mutation bytes immediately after the DISCOVER exchange completes and before the RELEASE
exchange is sent, and MUST assert a `RELEASE Status:"OK"` marker as evidence the restoration
succeeded. If that restoration cannot be made to land deterministically inside the re-exec'd
parent's single-process window, the mutation case is unconstructible in this harness and MUST be
labelled unevidenced and outside promotion rather than fixtured. All three cases therefore carry
the same GLTP-V0-035
`code` and
`phase` and are separated only by `detail`, which is an open lexical field rather than an enum; the
proposed pre-launch token table under GLTP-V0-035 lists every value `prelaunchDiagnostic` emits,
including the third live value `PROVIDER_PRELAUNCH_REJECTED`
(`cmd/corvint-go-test-provider/main.go:219`) that no case here claims. The parent's canonical binding does compare all three identities against a binding rebuilt
from a fresh observation (`internal/liveverify/parentverify/canonical.go:22-24`, the toolchain
comparison at line 24; line 25 is the return), reached per call at `cmd/corvint-go-test-provider/main.go:556`
(`cmd/corvint-go-test-provider/main.go:552` is the preceding `Revalidate`), but nothing can supply it a binding that differs from the
parent's own ACQUIRE response, so that comparison has no reachable forged input and is exercised
only by the drift cases. The discovery-commitment check is self-consistency only: it compares the
commitments against the same binding (`internal/liveverify/provider/provider.go:644`), so it
discriminates no axis. Every `detail` assertion in this clause is made against
`prelaunchDiagnostic`'s third return value (`cmd/corvint-go-test-provider/main.go:208-220`) or against
the recomputed `detailSha256`, never against a wire key: `writeDiagnostic` emits only `code`,
`detailSha256`, `phase`, `profile`, and `runId` (`cmd/corvint-go-test-provider/main.go:684-688`), and
GLTP-V0-035 keeps the typed diagnostic body local and off the wire. No run receipt is emitted for any of the
three refusals; the typed error document is the retained record, never a silent pass or an untyped
rejection.

- **GLTP-V0-041.** Every normal-completion path — an `Execute` call whose child exits without
cancellation, timeout, or provider failure — must leave zero surviving descendants in the run's own
process group, which is the scope cleanup can reach; a descendant that has put itself in another
process group is outside this obligation, because GLTP-V0-028 states that POSIX process groups
cannot contain escaped descendants and that P0 cannot claim to terminate them. The obligation is
independently verified by process-table inspection after `Execute` returns, not only
by the group-kill call sequence GLTP-V0-028 already asserts for the terminating paths. The runner
layer already carries a normal-exit descendant-liveness case
(`TestRunCleansInheritedDescendantAfterNormalExit`,
`internal/liveverify/gorunner/runner_posix_test.go:288`); the gap this clause closes is the
`Execute` integration above it, where run-root cleanup and lease release run alongside group
teardown.

**Observation protocol for GLTP-V0-039 and GLTP-V0-041.** No result or receipt exposes a PID or a
process-group ID, so a fixture obtains them by handshake: the launched helper writes the live
process-group ID and, for itself and every descendant it creates, that process's PID paired with
its kernel-reported start time, then blocks until the observer has read them, so every identifier
is in observer memory before any of those processes can exit. The fixture MUST start the
rendezvous read, and the unblock file write below, on a goroutine launched before the fixture calls
`Execute`: the helper writes and blocks inside the child that `Execute` itself blocks waiting on, so
a read or unblock attempted only after `Execute` returns would deadlock until the frozen 30-minute
timeout. That handshake has no run-time
channel: `Config` declares no environment field, the child's environment is the closed 21-name map
`createEnvironment` builds (`internal/liveverify/provider/provider.go:923-976`), which carries no
`PATH` and no fixture-supplied name, and every writable path the child does receive is under the
run root deleted at `internal/liveverify/provider/provider.go:474`. The fixture therefore generates
a temporary module under a fixture-owned root outside the observed repository — the fixture
supplies that root itself, so the source materialization the parent binds at ACQUIRE is unchanged and the
workspace stays clean — whose `go.mod` is exactly the module path plus `go 1.27.1` and carries no
`toolchain` directive, which would fight `GOTOOLCHAIN=local`. That root MUST be `git init`-ed with
the generated module committed: the parent observes the source over `git`
(`internal/liveverify/parentverify/snapshot.go:61`, which calls `observeRepository` at
`internal/liveverify/parentverify/repository.go:29-64`, the executable-digest check and the
`rev-parse --show-object-format HEAD HEAD^{tree}` identity) and the attachment bundle names a git
executable (`cmd/corvint-go-test-provider/main.go:51`), so ACQUIRE fails on a plain directory. Its
test source embeds the
observer-owned rendezvous directory path as a source literal; the helper writes the identities into
that directory and then blocks on a file the observer creates, on the pre-`Execute` goroutine above,
after reading them.
The in-scope population is the run's
own process group, so the helper MUST create at least one descendant that stays in that group: it
takes the plain `descendant` helper shape at
`internal/liveverify/gorunner/runner_posix_test.go:84-87`, which sets only `Env` before starting and
does NOT set `Setpgid`, so it inherits none of the run's pipes and remains a member of the group
cleanup signals. The `escaped-pipe` helper is not that
shape — it hands the descendant the run's stdout and stderr
(`internal/liveverify/gorunner/runner_posix_test.go:127-128`), which makes `Run` return
`ErrPipeWait` (`internal/liveverify/gorunner/runner_posix_test.go:323-325`), a provider failure that
GLTP-V0-041's antecedent excludes and that would put the fixture on a different path entirely. Every
descendant the helper creates MUST stay in the run's own process group, and the zero-survivor
assertion covers all of them; the helper creates no `Setpgid` descendant, so the run schedules no
escapee and GLTP-V0-029's surviving-escaped-descendant sentence is never engaged.
Probe sensitivity comes from outside the run instead. Before it calls `Execute`, the fixture MUST
launch one long-lived process of its own, outside the run entirely, and record its `(pid, start
time)` identity directly — it is the observer's own child, so no handshake is needed. That process
is never a descendant of the run and never a member of the run's process group, so it is outside
GLTP-V0-029 by construction; the fixture asserts `ephemeralDeletion:"COMPLETE"` on the strength of
the clean run-root removal alone (`internal/liveverify/provider/provider.go:474-476`). The fixture
MUST register the control's kill with `t.Cleanup`, not a plain return-path call, so it runs on
`t.Fatal`, a failed assertion, or an interrupting signal as well as normal return, and a dedicated
regression fixture MUST assert no leaked control process after a deliberately failed run. Its role
is to make the
probe decisive: a probe that cannot see a process known to be live at its recorded start-time
identity proves nothing about the in-group descendants, so the fixture MUST observe this
out-of-run control live at its recorded identity on the first probe and MUST fail if it does
not. The in-group descendants and that control MUST all be arranged to stay alive for at least 10
minutes, and that lifetime MUST exceed everything that can elapse before the last probe. Neither
run-length term is fixture-settable: `config.Timeout` MUST equal the frozen 30-minute
`gorunner.MaxRunTime` (`internal/liveverify/provider/provider.go:597-598`,
`internal/liveverify/gorunner/runner.go:34`) and the plan's `runMilliseconds` is the frozen literal
1800000 (`internal/liveverify/parentverify/canonical.go:74`), so the fixture caps neither and the
lifetime cannot be derived from them. What actually elapses is bounded by the helper itself, which
unblocks and exits as soon as the observer has read the identities, plus the shared two-second
shutdown deadline GLTP-V0-027 fixes (`internal/liveverify/gorunner/runner.go:195-213`), plus the
post-run work GLTP-V0-027 names as the fourth term of invocation return: post-run lease
revalidation, post-run identity observation, and lease release, each bounded by the 15-second
`postAuthorityTimeout` (`internal/liveverify/provider/provider.go:43`, applied at `:454`, `:458`,
and `:512`), the coverage merge and receipt composition, and the run-root removal
(`internal/liveverify/provider/provider.go:474-476`), which carries no timeout at all; then the
5-second probe window below. Because the run-root removal is unbounded, that budget is not a proof,
so the fixture MUST record the wall time from the `Execute` call to its return and MUST abort as
inconclusive, not fail, unless it stayed under 5 minutes: each of ACQUIRE and the post-run
observation walks the whole GOROOT and module cache into a fresh per-run build cache
(`internal/liveverify/parentverify/snapshot.go:65-76`), which a slow or cold host can legitimately
exceed with every descendant correctly cleaned up, mirroring the `/bin/ps`-failure rule below. That
wall assertion, not a cap the fixture has no seam to set, is what
keeps the 10-minute lifetime ahead of the last probe, so no recorded process can
disappear on its own before it is counted. The observer reaps nothing in the run's own process
group and claims no reaping ownership there, because it has none: the group leader is the provider's own
`exec.Cmd` child and is reaped by the provider's wait before `Execute` returns, and a descendant
that outlives the helper is reparented to `init`, which reaps it. Nothing therefore pins a PID or
the process-group ID after that wait, so liveness alone cannot separate survival from reuse. The
fixture MUST probe every recorded PID identity individually, counting a PID as a survivor only when
its kernel-reported start time equals the recorded one, and MUST do so whether or not
`kill(-pgid, 0)` returned `ESRCH`. The group probe is an additional check, never a substitute:
`ESRCH` proves only that no member of the run's process group survives, and a process outside that
group — the fixture's own out-of-run control, and equally any descendant that had left the group,
the `Setpgid` line at `internal/liveverify/gorunner/runner_posix_test.go:129` — is invisible to
it, as it is to cleanup, which blocks on group quiescence alone
(`internal/liveverify/gorunner/process_posix.go:56-62`). That same blindness is why the control is
placed outside the run rather than inside it. A
group probe that succeeds is likewise not by itself a failure; only the per-PID identity check over
the in-group identities decides, and the out-of-run control identity is scored separately as a
probe-sensitivity check. Platform limits: the existing build-tag split covers eight POSIX hosts
(`internal/liveverify/gorunner/process_posix.go:1`: `aix`, `darwin`, `dragonfly`, `freebsd`,
`linux`, `netbsd`, `openbsd`, `solaris`), but this fixture defines a start-time identity source for
only two of them and is scoped to Linux and Darwin; AIX, DragonFly, FreeBSD, NetBSD, OpenBSD, and
Solaris have no probe defined here and are unqualified for this identity until one is. The fixture
MUST record in its own evidence which identity source the platform used. Linux is the qualified
platform for this identity, `PROC_STAT_STARTTIME`: start time is field 22 of `/proc/<pid>/stat` in
`_SC_CLK_TCK` ticks since boot, and the fixture MUST parse it by taking the remainder of the line
after the LAST `)` — `comm` may itself contain spaces and parentheses — and counting
whitespace-separated fields from there, where field 22 of the whole line is the twentieth field of
that remainder. Darwin has no in-module path to `kinfo_proc.kp_proc.p_starttime`:
`syscall.Sysctl` is name-based and translates a name to a MIB itself (Go 1.27
`src/syscall/syscall_bsd.go:434-460`), its MIB-level `sysctl` wrapper is unexported
(`src/syscall/zsyscall_darwin_arm64.go:1816`), and the module declares no external packages (`go.mod`
carries only the module path and `go 1.27.1`), so `golang.org/x/sys/unix.SysctlKinfoProc` is
unavailable and hand-decoding `kinfo_proc` bytes is not implementable here. On Darwin the fixture
therefore uses the documented `ps -p <pid> -o lstart=` probe, identity source `PS_LSTART`, whose
resolution is one second and which is correspondingly weaker than the Linux identity; recording the
source is what keeps the two from being read as equivalent. Both the recording call at handshake
time and every probe call MUST invoke the absolute path `/bin/ps`, because the run environment
`createEnvironment` builds carries no `PATH`
(`internal/liveverify/provider/provider.go:944-966`), and MUST set `LC_ALL=C` and one fixed `TZ` on
that `ps` child's own environment — the helper sets them on the child it spawns, never through the
run environment, which the fixture cannot vary — so the two strings are
comparable byte for byte and neither locale nor a timezone change between the two can manufacture a
mismatch. A `ps` failure at either end — recording or probing — aborts the fixture as inconclusive;
it never resolves an identity as survived or as gone. A PID reused inside its identity's own
resolution window stays theoretically indistinguishable on both platforms, and the windows differ:
one `_SC_CLK_TCK` tick on Linux, and a full second on Darwin under `PS_LSTART`, which is materially
wider. Windows is out of scope. Probing opens no second survival
window for a member of the run's group, because that group's teardown completes inside `Execute`:
cleanup returns nil once the group has been observed quiescent for the quiescence window
(`internal/liveverify/gorunner/process_posix.go:61-62`), or `errProcessGroupNotQuiescent` at its own
deadline (`internal/liveverify/gorunner/process_posix.go:72-74`), an `EPERM` probe resetting that
window rather than concluding (`internal/liveverify/gorunner/process_posix.go:64-66`). There is a
third return: any other signal error is returned directly and unwrapped
(`internal/liveverify/gorunner/process_posix.go:53` for the initial `SIGKILL`, and
`internal/liveverify/gorunner/process_posix.go:68` inside the wait loop), and it is a cleanup
failure rather than a quiescence proof. All three returns land normally well before the shutdown
deadline GLTP-V0-027 fixes. Only the quiescent return
carries that meaning: `errProcessGroupNotQuiescent` is returned AT the cleanup deadline
(`internal/liveverify/gorunner/process_posix.go:72-74`) and is consistent with group members still
live. The fixture MUST therefore assert a clean cleanup outcome — the cleanup return is nil and the
run did not close with `EXECUTION_CONTAINMENT` — as a precondition of the probe. Under that
precondition the first probe of each recorded identity after `Execute` returns is decisive. An
in-group descendant observed live at that first probe fails
the fixture. Re-probing is permitted only to absorb observation latency on an identity that was
live at the first probe but whose start time could not be read; it is bounded to 250 ms per such
identity and can never turn an unresolved identity into a pass, because a later disappearance is
consistent with a survivor that has since exited and so proves nothing about the first probe. The
whole probe window — the first probe of every recorded identity plus every re-probe — is bounded to
5 seconds in total, which is the probe term of the lifetime arithmetic above. The recorded
population is exactly three processes — the helper itself and its one in-group child, which are
the two in-group identities, plus the fixture's own out-of-run control, scored only as the
probe-sensitivity check; the short-lived `/bin/ps` children the helper spawns to record start times
are not recorded and are excluded from the probe set. An unresolved identity
stays unresolved, and the fixture fails on any in-group identity that resolves to the recorded start
time and on any in-group identity still unresolved when either bound expires.

- **GLTP-V0-042.** GLTP-V0-012 keeps `excluded` unconditionally `[]` in V0 and this clause does not
change that. Its one MUST is forward-only: no GLTP run schedules a non-empty `excluded` set, and a
run whose scope carried one MUST be refused before launch with `code:"IDENTITY_MISMATCH"`,
`phase:"IDENTITY"`, and `detail:"PROVIDER_PRELAUNCH_REJECTED"`
(`cmd/corvint-go-test-provider/main.go:219`, the default arm of `prelaunchDiagnostic` and the third
live token the GLTP-V0-035 table above lists), launching no child and composing no receipt. That
antecedent is unconstructible in P0, so this clause carries no P0 fixture of its own, exactly as
GLTP-V0-043 carries none: `excluded` is the fixed `[]any{}` literal inside the plan map
`BindCanonical` builds (`internal/liveverify/parentverify/canonical.go:80`), `provider.Config`
exposes no exclusion field for a fixture to set, and `detail` is produced only by
`prelaunchDiagnostic` in package `main`, which a `provider` package test cannot observe. Its P0
evidence is the frozen-literal change detector on `excluded` alone. Beyond
that this clause adds nothing GLTP-V0-012 and LPCV-V0-020 do not already carry. It restates the
parent transition in full, as the precondition any future
caller-side exclusion would first have to satisfy. Under LPCV-V0-020, one confirmed
release-blocking failure missed by an automatic exclusion must disable autonomous narrow selection
for the affected provider/profile until the cause is fixed, the shadow corpus is replayed, and the
promotion gate passes again. "Missed by an automatic
exclusion" is the parent's wording and is not narrowed here
to failures inside the excluded packages, and the parent states no enabled-by-default posture.
LPCV-V0-020 states no recording obligation, and requiring the disabling and each of the three
re-enable conditions to be recorded as an explicit gate outcome rather than a silent continuation
is a real gap — but it governs the LPCV policy layer above the provider, so it belongs in
`live-proof-carrying-verification-v0.md` as a future LPCV requirement id, not in this document and
not as a GLTP obligation. The GLTP-side boundary this clause carries is the one GLTP-V0-012 already
fixes: `excluded` is unconditionally `[]`.

- **GLTP-V0-043.** A shadow (non-blocking) full-verification run whose scope reaches an explicitly
unsupported frontier already enumerated in `unknownReasons` (GLTP-V0-012) must retain that reason in
its composed receipt exactly as GLTP-V0-012 requires for any other run; being non-blocking is never
grounds to omit, collapse, or upgrade an unsupported-frontier reason into a passing conclusion. The
antecedent is unreachable in P0: no shadow or non-blocking run mode exists anywhere in
`internal/liveverify`, and `unknownReasons` is an unconditional package-level literal copied into
every plan (`internal/liveverify/parentverify/canonical.go:12-18`), so no run can reach a frontier
this clause would let it drop. This clause is therefore forward-only, and its P0 fixture is a
change-detector on that literal rather than a falsifier, exactly as GLTP-V0-038's is.

- **GLTP-V0-044.** Until an accepted containment profile exists, no command in `cmd/corvint` may
invoke `internal/liveverify/provider` or `internal/liveverify/gorunner`, and the product command's
transitive dependency closure must exclude both packages. While `go-live-run/0` is frozen, the
existing nested `execution.containment` emission is the only containment statement; this clause
adds no terminal member and changes no wire byte. Once a containment profile is accepted under a
new profile version, `Execute` MUST refuse to launch with `EXECUTION_CONTAINMENT` whenever the host
sandbox is unavailable.

**Non-goals for this section.** General untrusted external-execution containment is AT-10's
(`tools/compat-trial`) scope, not this slice; additional-language qualification and the LPCV P1-P4
promotion gates (`live-proof-carrying-verification-v0.md`) also remain out of scope here.

GLTP-V0-039 clause (b), GLTP-V0-042's pre-launch refusal MUST, and GLTP-V0-043 have no P0 fixture
and no P0 antecedent; they are unevidenced and outside any promotion claim under
`docs/SPEC-DRIVEN-DEVELOPMENT.md:52` until a P0 host or an accepted `EXTERNAL_DEPENDENT` label
exists for each.

### 12. Foreground live-test session (experimental, preview-only)

This section covers `internal/liveverify/session` and the `corvint-go-test-provider session
--foreground` subcommand: a bounded-file poll/debounce loop giving interactive edit feedback. It
carries no qualification authority, is not a daemon (it exits when its foreground process ends via
SIGINT/SIGTERM/stdin close), and never replaces a repository gate; every requirement in this section
is preview-tier under the same distinction GLTP-V0-033 draws for the event-only P0.

- **GLTP-V0-045.** The session polls a bounded, explicitly enumerated file set (`Config.Files`) at a
fixed interval for mtime/size change, debounces detected changes for a fixed window, then computes a
current-input identity: a SHA-256 digest over the sorted `(path, content-digest)` pairs of every
watched file plus the pinned toolchain version string. This identity is an internal
debounce/staleness key, distinct from any canonical GLTP wire identity, and is never placed on the
wire as one. The change baseline (the mtime/size snapshot) is sampled before the first identity is
computed, so an edit landing between the two is detected as a change rather than absorbed into a
baseline that no longer matches the identity labelling the result. With no `-watch` flag the `session` subcommand enumerates every `.go`, `go.mod`, and
`go.sum` file under the repository root, skipping dot-named directories below the root but never the
root itself, whatever its base name. Evidence: `TestResolveWatchedFilesWalksDotNamedRoot`
(`cmd/corvint-go-test-provider/session_test.go`), `TestEditBeforeBaselineSnapshotIsDetected`
(`internal/liveverify/session/session_test.go`).

- **GLTP-V0-046.** Exactly one run is active at a time. A newer settled identity detected while a run
is in flight does not cancel that run; it is queued and started only after the in-flight run
completes. Only explicit session shutdown (caller context cancellation) cancels an in-flight run,
reported as state `cancelled`. When an in-flight run completes and its identity no longer equals the
session's current identity (a newer edit settled during the run), its outcome is reported as state
`stale` regardless of pass/fail/infrastructure classification — satisfying "a second edit makes an
older result stale even if it finishes later" without discarding the run's own resource cost.
A completing run whose identity still equals the current identity re-digests the watched files, and
an edit the poll has not yet seen or the debounce has not yet settled (a different on-disk identity,
or a digest that cannot be computed) also reports `stale`; that run then clears the current
identity, so the next settle starts a run even when it restores the identity this run carried.
An edit reverted to identical bytes before the run completes is not detected. Evidence:
`TestStaleOnLateArrival`, `TestUnsettledEditAtCompletionIsStale`
(`internal/liveverify/session/session_test.go`).

- **GLTP-V0-047.** The batch is every change settled since the last started run: a batch queued
behind an in-flight run absorbs each later settle, and a settle whose identity cannot be computed
keeps its changes for the next one, so a later test-only settle never erases an earlier non-test
change. A settled batch of changed files may narrow the run's package scope away from the
configured full scope (`Config.Scope`) only when every changed file in the batch is a known test file
(`Config.TestFiles`); editing a test file cannot change another package's runtime behavior. Any other
changed file — a non-test `.go` file, `go.mod`, `go.sum`, or a path outside the watched/module tree —
forces fallback to the full configured `Config.Scope`, as does a changed test file in a nested
module (a `go.mod` in its directory or an ancestor below `Config.WorkingDirectory`), whose package
no root-module import path names. Narrowed patterns are expressed as plain
import paths (`Config.ModulePath`, optionally joined with the changed file's relative directory); an
empty `Config.ModulePath` forces every batch to fall back to `Config.Scope`, since gorunner's package
validator admits a dot-relative pattern only in the exact literal form `./...`. The `session`
subcommand reads `Config.ModulePath` from the repository root's `go.mod` `module` directive as
`go mod edit -json` reports it — an interpreted-string (quoted) path is unquoted and a trailing `//`
comment is ignored — and leaves it empty for any form it cannot read. Evidence:
`TestAffectedScopeFallsBackWhenSelectionCannotBeJustified`, `TestAffectedScopeFallsBackForNestedModuleTest`, `TestReadModulePathMatchesGoModEdit`,
`TestQueuedNarrowSettleKeepsEarlierFallback`, `TestFailedDigestSettleKeepsFallback`.

- **GLTP-V0-048.** Each run is executed via `gorunner.Run` — the same contained runner
`provider.Execute` uses internally for process-group spawn, output-byte capping, and
cancellation/timeout handling — under the session's configured timeout and output-limit bytes,
clamped to `gorunner.MaxRunTime`/`gorunner.MaxOutputBytes`. The session does not invoke
`provider.Execute`'s authority-gated canonical receipt path (`ExecutionAuthority`/`ExecutionLease`)
per debounce cycle: that protocol is a single-run, heavyweight, IPC-attested pipeline unsuited to a
tight interactive loop, and its production authority implementation is out of scope for this
preview tier. This is a stated design boundary, not an oversight. The `session` subcommand's run
environment is exactly `GOCACHE` and `GOPATH` under the bundle's temporary parent, `GOENV=off`,
`GOTOOLCHAIN=local`, and the inherited `HOME`; `GOENV=off` keeps a `go env -w` file under that HOME
(a `GOFLAGS` `-run` filter, for example) from changing which tests run. Cancellation, timeout, output-cap,
and shutdown leave no descendant process: shutdown cancels the active run's context and blocks until
`gorunner.Run`'s bounded process-group cleanup completes before the session's `Run` call returns.
A run whose `go` command started but was terminated by a signal instead of exiting publishes
`infrastructure` with detail `process terminated by signal`, never `failed`: with no exit status the
run states no test result. A run that exits 0 while its retained `go test -json` stream holds a
`fail` action (a `TestMain` that calls `os.Exit(0)` after a failed test) publishes `infrastructure`
with detail `exit status 0 disagrees with a fail event`, never `passed`. A caller cancellation that kills the process is a cancellation even when
`command.Wait` returns before `gorunner.Run` observes the context's `Done` channel, so it never
publishes as a signal termination. Evidence: `TestCancellationLeavesNoDescendants` (adapts the `ownedFixture`/`assertChildGone` pattern
from `internal/console/lifecycle_test.go`); `TestRunReportsCancelThatWinsTheKillButLosesTheSelect`
(a context whose `AfterFunc` fires while its `Done` channel stays open); `TestSessionIgnoresTheUserGoEnvFile`
(`cmd/corvint-go-test-provider/session_test.go`); `TestExitZeroWithFailedTestIsInfrastructure`
(`internal/liveverify/session/session_test.go`).

- **GLTP-V0-049.** The session publishes one JSON line per state transition
(`corvint-go-live-session-event/0`: `state`, `identity`, `sequence`, `scope`, `detail`, the
shared `projection` GLTP-V0-050 adds, and the `testProjections`/`testProjectionsOmitted` pair
GLTP-V0-051 adds) to stdout, over
states `idle`, `running`, `passed`, `failed`, `stale`, `infrastructure`, `cancelled`. The supported
platform is exactly darwin/arm64 (the same POSIX process-group containment as the rest of this spec;
`supportedProviderHost()` gates `UNSUPPORTED_PLATFORM` identically to the `run` subcommand). No
session state is a qualification result, a repository-gate substitute, or current evidence; required
repository gates (`go test ./...`, `go vet ./...`, and the accepted release checklist) still run
independently and are never replaced by a foreground session result. `detail` stays a plain string
with no per-test data structure (per-test data lives only in GLTP-V0-051's `testProjections`): a `failed` event's `detail` is a bounded, deterministic summary
(`failureDetail`, `internal/liveverify/session/session.go`) derived from the retained `go test -json`
stdout the run's `gorunner.Plan.RetainOutput:true` already captures — the package `FAIL` line(s) and
up to 5 distinct failing test names, in first-seen order, joined and capped at 512 bytes with a
truncation marker, cut at a UTF-8 rune boundary; stdout lines of any length are read, so one
oversized build-output event never hides a later failure; a `failed` event whose run retained no stdout reports `output-not-retained`
instead of an empty string. Every other terminal state's `detail` is unchanged. Evidence:
`TestRunningFailedPassed` (edit-driven `running`→`failed`→`passed` transitions), `TestFailureDetail`
(fixture stdout with two failing tests and a package `FAIL` line, the no-retained-output case, and a
multi-byte truncation), `TestOutputParsersReadPastAnOversizedLine`,
and a manual transcript recorded on 2026-09-12 (an internal evidence record, not part of the
public tree).

- **GLTP-V0-050.** Every published event also carries `projection`: one
`internal/testvalidity.Projection` (`internal/testvalidity/projection.go`), the same five-axis
shape `corvint-js-test-provider` already emits for its run and per-test projections, so a consumer
reads Go and JavaScript results through one representation (roadmap IPR-06, `LPCV-V0-047`). It is
session-level, not per-test (GLTP-V0-051 adds the per-test projections), and states no fact
`detail` does not already carry: `projectionFor`
(`internal/liveverify/session/session.go`) derives it from the published state alone, reuses
`internal/testvalidity`'s own types and cause vocabulary rather than defining a parallel shape, and
calls nothing on `provider.Execute`'s authority-gated canonical receipt path — GLTP-V0-048's design
boundary is unchanged, and no wire shape `corvint-js-test-provider` already produces changes. An axis
with no session evidence behind it stays explicitly unstated rather than defaulting to a pass: a
session run carries no TCQ claim, so `association` and `hygiene` are `UNSUPPORTED`; no mutation run
is ever attempted here, so `strength` stays `NOT_MEASURED` however `execution` reads
(`LPCV-V0-048`); `idle` and `running` have no execution report, so their `execution` is
`UNSUPPORTED` and `freshness` `UNKNOWN`. `passed` and `failed` project `PASSED`/`FAILED` with
`CURRENT` currency, since GLTP-V0-046 already resolved that the completing run's identity still
matched the current identity and the watched files on disk; `stale` projects `INCOMPLETE` cause `STALE` with `STALE` currency, matching the frozen
`stale-execution` vector in `conformance/test-validity-v0/vectors.json`; `infrastructure` and
`cancelled` produced no test result at all, so they project `INCOMPLETE` with cause
`INFRASTRUCTURE`/`CANCELLATION` and leave `freshness` `UNKNOWN` rather than claiming currency. The
free-form failure summary stays on `detail`, keeping `ExecutionFacts.Cause` inside its frozen
vocabulary. This projection is no more a qualification result or a repository-gate substitute than
any other session state. Evidence: `TestProjectionForState` pins the whole state-to-projection
mapping including the two axes no session evidence can support, and `TestRunningFailedPassed` and
`TestStaleOnLateArrival` assert the real passing, failing, and identity-superseded runs publish it
(`internal/liveverify/session/session_test.go`).

- **GLTP-V0-051.** A terminal event whose run completed — classified `passed` or `failed` before any
GLTP-V0-046 stale override, so also a `stale` event superseding such a run — also carries
`testProjections`: one entry `{package, name, action, projection}` per `(Package, Test)` pair the
run's retained `go test -json` stdout reported, where `action` is that test's terminal action
(`pass`, `fail`, `skip`), or `none` when it never reached one or the stream reports two different
terminal actions for it. A test that prints `go test` framing lines can make test2json report a pass
for a test that already failed, and one name in both the internal and external test package shares
one identity, so neither terminal is published. `projection` is one
`internal/testvalidity.Projection` built by `testvalidity.Project` from execution facts alone
(`testProjections`, `internal/liveverify/session/session.go`). Every other event carries `null`,
since an `infrastructure` or `cancelled` run produced no complete test result. Package-level events
and undecodable lines add no entry; no line-length bound ends parsing early. `pass`/`fail` project `PASSED`/`FAILED` (cause
`ASSERTION_OR_TEST`); a `skip` projects `SKIPPED` (reason `test-skipped`, no cause) under
GLTP-V0-052 and `LPCV-V0-052`, never a passing state; a `none` has no outcome, so its execution
axis abstains as `UNSUPPORTED` (`no-execution-input`) rather than inventing one. Currency is `CURRENT`, or `STALE` on a `stale` event, so a superseded run's per-test results
keep their own execution state beside a stale freshness axis. Association and hygiene stay
`UNSUPPORTED` and strength `NOT_MEASURED`, for the same reasons GLTP-V0-050 gives. Entries list
failing tests first, then the rest, each in first-seen order, capped at `MaxTestProjections` (128)
so one event line stays well inside the editor's per-record byte bound; `testProjectionsOmitted`
counts the entries dropped by the cap (0 otherwise), so a consumer can show them as unknown instead
of silently losing them. This derives from output `gorunner.Plan.RetainOutput:true` already
captures and crosses no part of GLTP-V0-048's boundary. Per-test projections are no more a
qualification result than any other session state. Evidence: `TestTestProjections` pins the
pass/fail/skip/none mapping, stale currency, package-level exclusion, and failures-first order;
`TestTestProjectionsCap` pins the cap and omitted count; `TestOutputParsersReadPastAnOversizedLine`
pins that a test after an oversized line is still projected; `TestRunningFailedPassed` asserts a real
failing run publishes the failing test's `FAILED` per-test projection;
`TestFramedPassCannotMaskFailedTest` asserts a real run whose test prints a framed pass for a failed
test projects that test as `none`
(`internal/liveverify/session/session_test.go`).

- **GLTP-V0-052.** A `skip` action MUST project the per-test execution axis `SKIPPED` with reason
`test-skipped` (`LPCV-V0-052`), and MUST NOT read as `PASSED`. A per-test entry MAY also carry a
source anchor `<repository-relative path>:<line>` as the first element of its execution and
freshness anchors, ahead of `input-identity:<identity>`. `go test -json` records no declaration
location, and its `Output` text is written by the test itself (a test can print any `file:line`),
so the stream alone cannot establish an anchor honestly. A `(Package, Test)` pair with no observed
`run` action MUST carry action `none` and an `UNSUPPORTED` execution axis with reason
`no-execution-input`, regardless of any other named event; in particular, a terminal `pass` without
that start evidence MUST NOT project `PASSED`. The anchor is instead taken from the
source whose content the run's identity already binds. The session keeps the per-file content
digests behind the current identity. At event time `sourceLocator`
(`internal/liveverify/session/session.go`) maps the package to a directory through the module path
(a package outside the module has no directory), reads every `_test.go` file in that directory,
and indexes top-level functions by name. It MUST attach an anchor only when every one of those
files is a watched file whose bytes still hash to the retained digest, every file parses, and
exactly one top-level function has the test's name; the line is that declaration's `func` keyword.
Otherwise the entry stays unanchored. That covers a subtest (a name containing `/`), a duplicate
declaration (for example under build tags), an unwatched, unreadable, changed, or unparseable test
file, and every `stale` event, whose source no longer matches the run. An anchor states only where
the test is declared. It adds no fact to the execution axis. Evidence: `TestTestProjections` pins
skip to `SKIPPED`; `TestSourceLocator` pins the resolved anchors, the ambiguity, subtest,
out-of-module, changed-file, and unwatched-file refusals, and the anchor order; and
`TestRunningFailedPassed` asserts a real failing run anchors the failing test at its declaration
(`internal/liveverify/session/session_test.go`). `TestPassWithoutRunAbstains` drives the session's
real `Run` entry point with an exit-zero malformed stream whose test appears only as `pass`, and
asserts action `none` with `UNSUPPORTED/no-execution-input` rather than a ghost pass.

## 13. Non-goals

- Affected-test selection, test exclusion, learned omission, dependency-graph authority, or claiming
  that only the tests a change needs were run.
- Wallaby parity; time-travel debugging; runtime-value capture; breakpoint, heap, profiler, race,
  fuzz, benchmark, or deterministic-replay replacement.
- Replacing Go's runner, coverage semantics, build cache, module resolver, repository gate, CI,
  review, merge policy, or release policy.
- JavaScript, TypeScript, Python, non-Go tests, nested-module orchestration, cross-platform
  equivalence, remote execution, a daemon/watch service, editor decorations, or an MCP API.
- Treating coverage as per-test evidence, behavior proof, an absence certificate, or a safe-removal
  signal.
- A security sandbox, default network-denial claim, arbitrary untrusted-code service, telemetry,
  cloud storage, database, graph database, vector database, or embeddings.
- Persisting raw test output or environment values, mutating source, generating tests, fixing code,
  weakening assertions, merging, deploying, publishing, licensing, or entitlement enforcement.

## 14. Traceability

| Requirement group | Delivery | First evidence |
|---|---:|---|
| GLTP-V0-001..004 codec and identities | not-started | independent canonical corpus |
| GLTP-V0-005..009 capability, plan, source, toolchain | not-started | exact Go 1.27 tuple fixtures |
| GLTP-V0-010..013 package and scope truth | not-started | Corvint/Beamfall explicit-package corpus |
| GLTP-V0-014..017 runner normalization | not-started | hostile test2json differential corpus |
| GLTP-V0-018..022 stream and terminal closure | not-started | reordered/stale/crash adversarial schedules |
| GLTP-V0-023..025, 036..037 coverage | not-started | covdata corruption and completeness corpus |
| GLTP-V0-026..029 execution safety | not-started | per-OS lifecycle/containment matrix |
| GLTP-V0-030..035 conformance, rollback, errors | not-started | sealed shadow reports and kill-switch drill |
| GLTP-V0-038 unenforced-target invariants | delivered 2026-09-04 | `internal/liveverify/parentverify/verifier_test.go`'s `TestCanonicalBindingIsDeterministicAndAuthorityBound` (lines 123-145) calls `BindCanonical` and asserts the returned plan's `limitsEnforced` literal is exactly the seven frozen members in GLTP-V0-006 order and omits the `CPU`, `MEMORY`, `OPEN_FILES`, and `PROCESSES` tokens (the GLTP-V0-018 limit vocabulary, `conformance/go-live-test-v0/event.go:571`; `CPU_TIME` and `MEMORY_BYTES` are not tokens in this format); `internal/liveverify/provider/qualification_test.go`'s `TestFrozenReceiptResourceAndScopeClaims` composes a clean run and asserts `status:"PASSED"` alongside that receipt's `execution.resources` object carrying `null` for all four, so the clause's does-not-withhold-`PASSED` half is exercised rather than assumed. Both are freeze/change-detector fixtures on the literal, not falsifiers: the enforced set has no injection seam, so neither forward constraint — the advertised-but-unenforceable axis nor its enforced-but-unadvertised mirror — has a reachable discriminator until a profile makes the set variable. |
| GLTP-V0-039 Execute-level timeout receipt proof | clause (a) delivered 2026-09-04; clause (b) BLOCKED | Clause (a) typed `TIMEOUT`/`INCOMPLETE` receipt is delivered: `internal/liveverify/provider/qualification_test.go`'s `TestDeadlineReceiptComposition` runs eight synthetic-runner cases at the receipt-composition layer, each over a runner result whose `Runner.TimedOut` the fixture sets directly, including the adversarial complete-stream/exit-zero/all-`PASSED` case proving `complete` at `internal/liveverify/provider/receipt.go:756` is forced false unconditionally by `TimedOut`, and the STALE/TIMEOUT/INFRASTRUCTURE/CANCELLATION coexistence cases run against both the composer and GLTP-V0-030's independent verifier. `conformance/go-live-test-v0/qualification_test.go`'s `TestProducerDeadlineCompositionRoundtrip` re-runs that producer in a separate process (never importing it or its bytes directly) and independently re-verifies each emitted transcript with `VerifyTranscript`; `TestProducerCancellationProviderFailureRoundtrip` separately composes `validReceiptInput` with cancellation and provider failure true, preserves `cancelled:true` plus `failureClass:"INFRASTRUCTURE"`, and verifies it independently. `conformance/go-live-test-v0/event_test.go`'s `cancelled-unknown-still-rejected` case pins the narrow exception, while `stale-cannot-be-cancelled-and-timed-out` pins the `cancelled`+`timedOut` mutual-exclusion rule. The composer precedence change this clause discloses is delivered at `internal/liveverify/provider/receipt.go:773-792` (ranks STALE, then TIMEOUT, then INFRASTRUCTURE, then CANCELLATION, and forces the wire `cancelled` field `false` whenever `TimedOut` is true). Clause (b) — zero surviving descendants in the run's own process group under the real elapsed run deadline — remains BLOCKED: the child deadline is `config.Timeout` (`internal/liveverify/provider/provider.go:435`), armed and handled at `internal/liveverify/gorunner/runner.go:186`, `:198-202`, after the frozen 30-minute `gorunner.MaxRunTime` (`internal/liveverify/provider/provider.go:597-598`, `internal/liveverify/gorunner/runner.go:34`), and the plan's `runMilliseconds` is a frozen literal (`internal/liveverify/parentverify/canonical.go:74`); no fast fixture seam exists and a real 30-minute fixture is not accepted here on time-budget grounds — `internal/liveverify/provider/qualification_test.go` records this in a comment: "GLTP-V0-039(b) remains BLOCKED by the frozen 30 minutes." |
| GLTP-V0-040 identity refusal: forged source, dependency/toolchain drift | delivered 2026-09-04 | Three deterministic cases plus the disclosed production change, all delivered. Source axis, library layer: `internal/liveverify/provider/qualification_test.go`'s `TestForgedSourceBindingRefusesBeforeLaunch` asserts typed `ErrAuthorityUnavailable` from the pre-launch receipt observation, no child launched, no receipt composed. Source axis (binary-layer mapping) and the dependency/toolchain axes, against the real production parent: `cmd/corvint-go-test-provider/qualification_test.go`'s `TestProductionParentDriftAndTransportAreDistinct` and `TestUnavailableSourceDiagnosticMapping`, which mutate a fixture-owned module cache and toolchain copy under `t.TempDir()` after ACQUIRE, keyed on a fixture-owned bundle-path sentinel segment (`driftFixtureSegment`, `"at14-production-drift"`) so only that fixture's bundle selects fault injection while every semantic response still originates from `runProductionAuthority`. The disclosed production change — preserving the parent's `errorCode` as a typed sentinel across the seven edit sites (`cmd/corvint-go-test-provider/main.go:425-426`, `:430`, `:437`, the rejection wrap at `:440` via `authoritySentinel` at `:610-616`, and `:443`, plus `internal/liveverify/provider/provider.go:352` and `:369-370` via `leaseSentinel` at `:262-267`, `cmd/corvint-go-test-provider/main.go:287` deliberately excluded) — is delivered and exercised at the library layer by `internal/liveverify/provider/qualification_test.go`'s `TestDiscoveryAuthoritySentinelsRemainDistinct` and `TestPrelaunchRevalidationErrorPreservesUnavailable`. There is no binary-layer forged-binding case, because the binding is the parent's own ACQUIRE response and no forged input can reach it. |
| GLTP-V0-041 zero-descendant proof | delivered 2026-09-04 (Darwin host); real-deadline termination NOT_RUN | `cmd/corvint-go-test-provider/qualification_lifecycle_test.go`'s `TestExecuteNormalCompletionHasNoRecordedSurvivors` and `TestExecuteInterruptedHasNoRecordedSurvivors` (both via `verifyExecuteRecordedCleanup`) probe the run's own process group for a normal-exit and a cancelled `Execute` respectively, each with a fixture-owned long-lived control process launched outside the run and observed live at its recorded `(pid, start time)` as the probe-sensitivity control, asserted alongside the `ephemeralDeletion:"COMPLETE"` the composer emits. Both cases passed on the unsandboxed Darwin host; per the AT-14 handoff, a sandboxed `ps`/PS_LSTART denial makes the probe `t.Skipf("INCONCLUSIVE: ...")`, never a silent pass. Termination on the real elapsed 30-minute run deadline is still not exercised — both delivered cases terminate by normal exit or context cancellation, not by `gorunner.MaxRunTime` elapsing — so that antecedent remains NOT_RUN, consistent with GLTP-V0-039(b). |
| GLTP-V0-042 no non-empty `excluded` scheduled | P0 fixture delivered 2026-09-04; pre-launch refusal half BLOCKED (no host) | The pre-launch refusal half still has no P0 fixture and no P0 antecedent, exactly as GLTP-V0-043 has none: `excluded` is the fixed `[]any{}` literal at `internal/liveverify/parentverify/canonical.go:80`, `provider.Config` exposes no exclusion field for a fixture to set, and `detail` is produced only by `prelaunchDiagnostic` in package `main` (`cmd/corvint-go-test-provider/main.go:208-220`), which a `provider` package test cannot observe; that gate-outcome-record obligation belongs to the LPCV policy layer and is left for a future requirement id in `live-proof-carrying-verification-v0.md`. The delivered P0 fixture is `internal/liveverify/parentverify/verifier_test.go`'s `TestCanonicalBindingIsDeterministicAndAuthorityBound` (lines 123-145), which calls the exported `provider.ComposeReceipt` — via the `composeScopeQualificationReceipt` helper in `internal/liveverify/parentverify/scope_qualification_test.go` — for the receipt-side `"excluded":[]` literal (already fixed by GLTP-V0-012 at `internal/liveverify/provider/receipt.go:704`) and `BindCanonical` for the plan-side literal at `internal/liveverify/parentverify/canonical.go:80`, and asserts the two agree, avoiding the `provider`/`parentverify` import cycle a same-package comparison would require. |
| GLTP-V0-043 unsupported observations retained | P0 fixture delivered 2026-09-04; shadow-mode antecedent BLOCKED (no host) | No shadow or non-blocking run mode exists in `internal/liveverify`, so the clause's antecedent has no host and this row stays blocked on one. The delivered P0 fixture is a change-detector, not a falsifier: `internal/liveverify/provider/qualification_test.go`'s `TestFrozenReceiptResourceAndScopeClaims` asserts the composed receipt's `scope.unknownReasons` is exactly the fourteen-member `allUnknownReasons` literal at `internal/liveverify/provider/receipt.go:675-680` in that order; `internal/liveverify/parentverify/verifier_test.go`'s `TestCanonicalBindingIsDeterministicAndAuthorityBound` (lines 123-145) cross-checks the plan side against the receipt side via `composeScopeQualificationReceipt` and the plan-side `unknownReasons` literal (`internal/liveverify/parentverify/canonical.go:12-18`), so the two cannot drift apart unnoticed. |
| GLTP-V0-044 product-command containment interlock | no-caller gate delivered 2026-09-05; new-profile refusal BLOCKED (no accepted containment profile) | `internal/liveverify/provider/interlock_test.go`'s `TestCorvintGoDependencyClosureExcludesUncontainedLiveTestExecution` runs `GOTOOLCHAIN=local go list -deps -f {{.ImportPath}} ./cmd/corvint` from the module root and rejects exact dependency edges to `internal/liveverify/provider` and `internal/liveverify/gorunner`; decision 0062 preserves the existing nested `execution.containment` statement and every frozen `go-live-run/0` byte until a new profile is accepted. |
| GLTP-V0-045 bounded polling, debounce, and current-input identity | delivered 2026-09-12 (preview tier) | `TestRunningFailedPassed` and `TestDebounceDuringActiveRunRace` exercise bounded polling and settled edit batches; `TestResolveWatchedFilesWalksDotNamedRoot` pins the default watched set, including a repository root whose base name starts with a dot; `TestEditBeforeBaselineSnapshotIsDetected` pins that an edit before the change baseline is sampled is still detected. The exact sorted path/content/toolchain identity preimage has no direct discriminator and remains `NOT_TESTED`. |
| GLTP-V0-046 one active run and stale late arrival | delivered 2026-09-12 (preview tier) | `TestStaleOnLateArrival`, `TestDebounceDuringActiveRunRace`, `TestUnsettledEditAtCompletionIsStale` (an unsettled edit at completion is stale, and reverting it reruns), and `TestCancellationLeavesNoDescendants`. |
| GLTP-V0-047 conservative affected scope | delivered 2026-09-12 (preview tier) | `TestAffectedScopeFallsBackWhenSelectionCannotBeJustified` exercises test-only narrowing and the non-test, mixed, empty, and absent-module-path fallbacks; `TestAffectedScopeFallsBackForNestedModuleTest` pins the nested-module fallback; `TestQueuedNarrowSettleKeepsEarlierFallback` and `TestFailedDigestSettleKeepsFallback` pin that a queued or failed-identity settle keeps an earlier non-test change in the batch. |
| GLTP-V0-048 contained shared runner boundary | delivered 2026-09-12 (preview tier) | `TestCancellationLeavesNoDescendants` exercises shutdown through `gorunner.Run` and descendant cleanup; `TestClassify` exercises timeout/output-limit and signal-termination classification; `TestRunReportsCancelThatWinsTheKillButLosesTheSelect` pins that a cancellation whose kill beats the context select is reported `Cancelled`. A real elapsed-timeout and output-cap session run are `NOT_TESTED`; the authority-gated canonical receipt path remains excluded by design. |
| GLTP-V0-049 session wire and darwin/arm64 host gate | delivered 2026-09-12 (preview tier); qualification `NOT_RUN` | `TestRunningFailedPassed`, `TestFailureDetail`, `TestOutputParsersReadPastAnOversizedLine`, and `TestSupportedProviderPlatformRefusesLinuxAMD64`; the Darwin transcript is retained in `docs/evidence/go-live-test-session-2026-09-12.md`. No qualified session matrix has run. |
| GLTP-V0-050 shared session projection | delivered 2026-09-12 (preview tier) | `TestProjectionForState`, `TestRunningFailedPassed`, and `TestStaleOnLateArrival`. |
| GLTP-V0-051 per-test projections on the session wire | delivered 2026-09-12 (preview tier) | `internal/liveverify/session/session_test.go`'s `TestTestProjections`, `TestTestProjectionsCap`, `TestOutputParsersReadPastAnOversizedLine`, and `TestRunningFailedPassed`; consumed by the editor per `vscode-extension-v0.md` VSC-V0-067. |
| GLTP-V0-052 skip outcome, run evidence, and per-test source anchor | delivered 2026-09-12 and amended 2026-09-13 (preview tier, decisions 0149 and 0283) | `internal/liveverify/session/session_test.go`'s `TestTestProjections`, `TestPassWithoutRunAbstains`, `TestSourceLocator`, and `TestRunningFailedPassed`; consumed by the editor per `vscode-extension-v0.md` VSC-V0-069. |
| GLTP-V0-021 transcript and receipt agree on `PASSED` | requested-package pass clause delivered 2026-09-13; remaining GLTP-V0-021 clauses under the 018..022 row | `internal/liveverify/provider/provider_test.go`'s `TestRequestedPackageWithoutTestsIsNotPassed` runs real Go 1.27 through `Execute` on a requested package with no test files (runner terminal `skip`, exit zero) and asserts both the transcript execution and the receipt status are `INCOMPLETE`; before the fix the transcript reported `PASSED` while the receipt said `INCOMPLETE`. |
| GLTP-V0-007 parent plan binding refuses an environment outside the profile | delivered 2026-09-13 | `internal/liveverify/parentverify/verifier_test.go`'s `TestCanonicalBindingRefusesEnvironmentOutsideProfile` calls `BindCanonical` with the closed profile environment (bound), then a two-name partial array, a `GOFLAGS` digest other than `-mod=readonly`, and an added `USER` name, each self-consistent with its `environmentSha256`, and asserts all three return `ErrInvalid`; before the fix all three were bound into a plan. |
| GLTP-V0-004 native path digest agrees across the parent boundary | delivered 2026-09-13 | `cmd/corvint-go-test-provider/main_test.go`'s `TestProductionParentBindsNonASCIIRepositoryPath` runs `executeProvider` against the production parent on a repository path containing U+00A0 and asserts the canonical plan binds; before the fix the provider hashed the path body with Go `%q` quoting, which escapes U+00A0, so the parent's `cwdPathSha256` differed and `BindCanonical` refused. |
| GLTP-V0-009 parent lease binds the module-cache and `go.work` paths | delivered 2026-09-13 | `internal/liveverify/parentverify/verifier_test.go`'s `TestLeaseRevalidationRefusesAnotherSelectedPath` acquires a workspace-mode lease, then calls `Revalidate` with the same lease ID under a content-identical module-cache copy and under a second `go.work` in the repository, and asserts `ErrDrift` for both; before the fix both revalidated and returned a binding naming the other path. |
