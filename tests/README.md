# Native regression ownership

Decision 0088 retires the Python implementation and its Python test drivers. The previous
files remain available at Git revision `9ca27f9a62a2add263ff559fd711feea5cdfd93d`.
No test in the current source gate starts the retired implementation or requires its interpreter.

| Retired driver family | Current checks |
| --- | --- |
| CLI, context, cache, claims, learning | `cmd/corvint`, `internal/contextindex`, frozen `conformance/cli-parity-v0` receipt/state replay |
| Trace records, secrets and migration | `internal/trace`, `internal/secretscreen`, `cmd/corvint/migrate_traces_test.go`; migration inputs and expected bytes are independently authored |
| CEM/OCM, workflows and canonical binding | `internal/cem`, `internal/ocm`, repository verification packages, `cmd/corvint`, and independent `interop/cem01-go` |
| CEM/LRF/TCQ conformance | Original frozen corpus documents, native package suites, `internal/frontierrepo/tcq_conformance_test.go`; historical failures remain failures |
| Genesis inventory | `internal/genesis`, native init/adopt CLI cases and frozen CLI corpus |
| Codex/Claude lifecycle and handoff | `cmd/corvint` host-adapter, local-completion, native-hook and source-handoff tests; `internal/gokernel` closed harness protocol tests |
| Gemini/OpenCode host boundary | `integrations/host-adapters.test.mjs`, run by `TestHostAdapterJavaScriptHosts`; uses those hosts' Node runtime, never Python |
| Legacy Python wrapper process/forged-child output | Wrapper retired; Go adapters call the linked kernel directly. Strict host input and kernel response tests replace the removed IPC boundary; `internal/procgroup` retains native interruption/ownership regressions |
| Self-use batch and source views | `benchmarks/selfuse-batch`, real native batch/fallback parity and signal cleanup, `cmd/corvint/source_handoff_test.go` |
| Benchmark contract/learned arm | `benchmarks/runner` independent case accounting, source proof, frozen provenance and threshold tests |
| External CEM interop | `tools/cem-interop-runner`; original frozen packet bytes and matrix remain unchanged |
| CEM 30×30 experiment | `experiments/cem-30x30`; frozen synthetic gate controls, exact rational arithmetic, bound human task registry and blinded ingestion |
| First-use wheel trial | Retired with wheel distribution. Historical results stay historical; the native archive gate does not claim equivalent human first-use evidence |

The source gate is `make gate`. The explicit external project-integration profile may run
user-project tools such as pytest; that is an external toolchain qualification, not a Corvint
runtime dependency. Frozen Python project inputs and historical evidence bytes remain data.
