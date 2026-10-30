# Go live-test V0 independent verifier

This Apache-2.0 conformance program independently checks the discovery,
canonical event, WEI, and terminal-run wire and identity rules in
`go-live-test-provider-v0.md`. It uses only the Go standard library and imports
no Corvint implementation package.

It verifies exact canonical JSON plus LF, the closed `go-live-discovery/0` and
`go-package/0` shapes, classification, ordering, requested-runner commitment,
domain-separated identities, bounds, and source-free output. Independent
functions and vectors also verify normalized `go-module/0`, `go-input-set/0`,
`go-discovery-inputs/0`, `go-live-event/0`, `go-live-wei/0`, and
`go-live-run/0` preimages, sequence/event-root closure, test/package state, and
terminal scope inventories. A production-binary black-box vector independently
compares ordinary pinned-Go acquisition and child output with the provider's
package terminal and byte digest. Coverage production and LPCV qualification
remain `NOT_RUN`.

Run the suite with Go 1.27:

```sh
GOTOOLCHAIN=local go test -count=1 ./conformance/go-live-test-v0
GOTOOLCHAIN=local go test -race -count=1 ./conformance/go-live-test-v0
GOTOOLCHAIN=local go vet ./conformance/go-live-test-v0
```

Structurally verify one canonical discovery document without echoing repository
content:

```sh
GOTOOLCHAIN=local go run ./conformance/go-live-test-v0 DISCOVERY.json
```

Verify a production discovery/transcript pair plus its privacy-safe test-owned
binding context:

```sh
GOTOOLCHAIN=local go run ./conformance/go-live-test-v0 DISCOVERY.json TRANSCRIPT.jsonl CONTEXT.json
```

This mode additionally recomputes the closed capability, provider-build, plan,
environment, and toolchain identities and their source/CWD and discovery
relations before accepting the transcript.

Success means every published package, module, input-set, aggregate-input, and
discovery identity was independently recomputed. Output marks external
environment and raw acquisition preimages `NOT_RUN`; the document contains only
their digests, so this command cannot prove producer equivalence for them.
Failure emits one stable rejection code. The event/WEI/run verifier is exercised
by the fixed JSONL corpus and hostile vectors in the Go suite.
`testdata/valid-transcript.jsonl` is literal output from the independent
`provider.ComposeReceipt` producer, paired with its literal
`testdata/producer-context.json` and canonical
`testdata/receipt-producer-discovery.json`; their exact SHA-256 values are pinned
and their context/discovery relation is checked before the dependency-free
verifier recomputes every event, WEI, event-root, and run identity. The
conformance package does not import the producer.
`testdata/producer-discovery.json` is likewise literal output from
`godiscovery.Decode` followed by `godiscovery.Compose`; the independent
verifier pins its exact bytes and recomputes its complete published commitment
graph without importing the producer.
