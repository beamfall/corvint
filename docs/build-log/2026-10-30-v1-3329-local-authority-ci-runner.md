# CI runs the nested tools/local-authority module — V1-3329

No CI job ran the tests of `tools/local-authority`, a nested module (`replace` of the root module,
plus external `github.com/tetratelabs/wazero` and `golang.org/x/sys`). Decision (lane, within the
ticket; CIN-V0-008): a `go-static` step runs `go test -count=1 -timeout 30m ./...` and `go vet
./...` in the module on every run that starts `go-static`. That covers every change a plan can
carry to the module, its own files included, without a new planner unit; selecting the module
precisely belongs to the affected planner (V1-0872). The step sits before the interop step, so
AFP-V0-380's single run script after the interop cut is unchanged.

`go-static` runs with `GOPROXY=off`, and the module's external dependencies are not vendored. The
step fetches them once with `GOFLAGS=-mod=readonly go mod download` against `proxy.golang.org`
and `sum.golang.org`, which fails on a hash missing from or differing with `go.sum`; test and vet
then run offline under the job's `GOPROXY=off`. An unreachable proxy fails `go-static` loudly.

Local evidence (darwin, go1.27.3): module tests PASS in 11.2 s; a CI simulation with an empty
`GOMODCACHE` ran the verified download (rc 0), then offline `go test` PASS (10.8 s) and `go vet`
clean; `GOOS=linux go vet` clean. `TestCINV0008LocalAuthorityModuleTestsRunInGoStatic` and
`TestAFPV0380InteropTestsRunOnceInGoStatic` PASS (`tools/ci-reuse-plan`, whole package PASS).
The first CI run ID is recorded after the push.
