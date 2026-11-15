# sql-native ratchet go1.27.4 pin row (V1-5411)

The V1-5408 pin move made `go1.27.4` the only version the Makefile `go-version` target admits, but
`script/check-sql-native-ratchets.sh` still held one darwin/arm64 Homebrew `go1.27.3` row. On
BASE `1a02c5f39` `make sql-native-ratchets-test` exits 2 with `FAIL: pin go1.27.3 is not admitted
by the Makefile go-version target`, and the gate itself refuses every go1.27.4 host with
`TOOLCHAIN_MISMATCH`.

## Measurement (2026-11-15, owner's WSL2 x86_64 host, Linux 6.18)

GOROOT `golang.org/toolchain@v0.0.1-go1.27.4.linux-amd64` from the module cache:

- go binary `shasum -a 256`: `de0ae3e082ce7599eae4fb69fa2ef1e43b8f31ffd3fce62cdf81930aa9005050`
- receipt tool, built with the gate's `run_go` environment and
  `build -trimpath -buildvcs=false -ldflags='-s -w -buildid='`:
  `ce485de2ed246b946e202dd9fa20dbdb639cd915fbfe0c9166235d687f8d11aa`
- GOROOT receipt: `{"Entries":13475,"SHA256":"ea503b0c42bbd04df6b0efeb8944cfea6d50f9420c53a50eafb5233be2ce05b2"}`

The `golang.org/dl` install at `~/sdk/go1.27.4` (the go.dev tarball) has the same go binary digest
and the same files but writable modes, so its receipt is
`187ed541c7e20a743313e4b34abd4f0d49a24d12aaf8b6e87c820200b4cf0d47`. `SNR-V0-031` forbids a second row
with the same version and go binary digest, so that install refuses with `goroot-content`, and
`script/check-sql-native-ratchets_test.sh` case 9 fails on it (its rule: a host toolchain that
matches a row by version and go binary digest must match the other two). Admitting it needs an
owner decision on `SNR-V0-031`. The removed `go1.27.3` row is not re-admitted; darwin/arm64
`go1.27.4` is `NOT_OBSERVED`.

## Gate runs (core_base `1a02c5f39`)

1. `TMPDIR=<private dir> PATH=<module>/bin:$PATH timeout 590 make sql-native-ratchets`: rc 124
   after 592 s. Under `set -eu` it had passed the source ceiling (`source_bytes=64763`), every
   `check_toolchain`, the candidate build and binary ceiling, byte-identical Core binaries and
   dependency lists, the five cross-compiles and the `SNR-V0-010` complexity test. The lane's
   foreground bound then stopped it during the benchmark samples.
2. The same script with the `SNR-V0-006`–`009` build block cut, same environment: rc 0 after
   496 s, printing `go_version=go1.27.4 goroot_entries=13475 candidate_max_b_op=93462
   candidate_max_allocs_op=393 causal_candidate_max_b_op=93462 causal_candidate_max_allocs_op=393
   causal=NOT_RUN-causal-parent-unavailable latency=NOT_RUN`.

One uninterrupted run is `NOT_RUN`: it needs about 18 minutes at `GOMAXPROCS=1`, beyond the
10-minute foreground bound. An earlier run without `TMPDIR` failed after 326 s with `error obtaining
VCS status: exit status 128`: the host has an empty `/tmp/.git` directory, and `go list -deps
./cmd/corvint` in the `git archive` export under `/tmp` found it while walking up for VCS stamping.

`make sql-native-ratchets-test`: rc 0 with the module GOROOT (case 9 matched the row in all three
digests); rc 2 with the `~/sdk` GOROOT, as described above.
