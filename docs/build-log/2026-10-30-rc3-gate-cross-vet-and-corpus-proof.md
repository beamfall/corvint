# rc.3 full-gate repairs: cross-vet FIFO constraint and corpus proof status (V1-3196, V1-3195)

Release-gates run 40988175187 at public root `9477995d` failed `full-gate` on ubuntu-24.04 and
macos-15 at two steps: `cross-vet` and `go-test`.

## V1-3196: cross-vet

`internal/doccompiler/draft_fifo_unix_test.go` (SDD-V0-019) called `syscall.Mkfifo` under a
constraint that included `aix` and `solaris`, whose `syscall` packages have no `Mkfifo`. Decision:
narrow the constraint to `darwin || dragonfly || freebsd || linux || netbsd || openbsd`. We did not
switch to `golang.org/x/sys/unix`, because that would add a dependency for one test. The FIFO
refusal is still exercised on the qualified hosts, and the other platforms keep compiling the
product code.

Evidence (Go 1.27.3): `go vet ./internal/doccompiler/` passes for solaris/amd64, aix/ppc64,
illumos/amd64, freebsd/amd64, linux/amd64 and darwin/arm64. The whole `make cross-vet` target passes
(rc 0), and so does `make windows-vet`. `TestSDDV0019ReadDryRunOwnerRefusesFIFO` passes rather than
skipping on WSL2 linux/amd64 and on darwin/arm64.

## V1-3195: corpus binding

This lane changes nothing for V1-3195. The failure at `9477995d` was already repaired on
`origin/main` by `88136389e` ("restore strict public-clone corpus proof"), which is recorded in
`2026-10-30-security-ci-repair.md` and in the CEM-V1-201 text of `docs/specs/cem-stable-v1.md`. In
that repair, the eleven maps retained by the root are verified byte-for-byte against the root tree
and a pinned sparse archive fixture. The four post-root maps bind to reachable commits, and
`TestCEMV1201MissingCurrentBindingFailsCI` first checks the positive corpus, then injects an
unbound map and observes that `CI=true` fails at lookup.

Evidence at lane base `b469ab71`: with `CI=true`, the whole `./internal/cem/corpusbinding` package
passes on WSL2 (historical 1+10, current 0+4). It also passes in a `--no-local` clone that holds
only the 22 public commits. Both release-gates and the CI `go-product-shard` check out with
`fetch-depth: 0`, so the public root is always present. A shallow clone that lacks the root still
fails under `CI=true` instead of skipping, by design.

## Remaining release-gates failure (out of scope)

`core-n1-replay-v0.8.1` and `core-n1-replay-v1.0.0-rc.1` fail with `fatal: not a valid object
name`, because those tags exist only in the archived history. This is the same pre-root class, but it
is in `.github/workflows/release-gates.yml` and the N-1 baseline policy, and it is already tracked by
V1-3177.
