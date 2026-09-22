# Decision 0021 — Nested modules without `go.work` are admitted only through the importing module's own `replace`

Date: 2026-09-01. Status: accepted, implementation deferred. Authority: repository owner, verbatim
instruction "answer the questions.md yourself using experts" (2026-09-01), delegating the
`docs/agent-memory/questions.md` entry "affected: should nested `go.mod` modules be indexed when
there is no `go.work`?". Personas applied inline: an ad-hoc Go toolchain / module-resolution
engineer (no catalog role owns Go module semantics), `test-conformance-architect`, and
`product-strategy-discovery-lead`.

## Question

AFP-V0-008 observes the root module or the `go.work` use set; a nested `go.mod` no workspace lists
is the `go:nested-module-frontier`. Pre-workspace etcd snapshots (twelve nested modules, no
`go.work`) therefore abstain on any edit inside a nested module. May the plan admit nested modules
without a toolchain answer on which build compiles them from this checkout?

## Toolchain facts (go.dev/ref/mod, read 2026-09-01)

- "`replace` directives only apply in the main module's `go.mod` file and are ignored in other
  modules." Replacement is not transitive.
- A path on the right of the arrow "beginning with `./` or `../`" is "the local file path to the
  replacement module root directory, which must contain a `go.mod` file."
- "If the `go` version in `go.mod` is `1.14` or higher and a `vendor` directory is present, the
  `go` command acts as if `-mod=vendor` were used": a vendored module builds from `vendor/`, not
  from the replaced directory, until `go mod vendor` runs again.
- In workspace mode, `replace` directives in `go.work` override those in workspace modules; that
  path is AFP-V0-008 already.

## Ruling

1. Without `go.work`, every nested `go.mod` is observed as its own unit universe under its own
   module path (intra-module edges resolve exactly as the root module's do today). This alone
   moves a dirty nested source from `UNINDEXED_SOURCE_PATH` to a selection of its own module's
   tests, with a `DIRECT_SOURCE_CHANGE` witness.
2. A cross-module edge from importing module M to imported module N is admitted only when M's own
   `go.mod` holds `replace N => PATH` with PATH relative (`./` or `../`), resolving inside the
   repository to a directory whose `go.mod` `module` line is exactly N. A version-qualified
   `replace N vX => PATH` counts only when M's `require` names N at vX. Nothing is inherited: N's
   replaces are not consulted when M is the main module.
3. An importer without such a replace resolves N to a released version, so its tests cannot
   compile the dirty checkout; it is excluded with the existing reason and a detail naming the
   resolution (`requires N at vX, no replace to this checkout`). That exclusion is a toolchain
   fact, not a claim.
4. A module holding `vendor/modules.txt` keeps its cross-module edges at a frontier
   (`go:vendored-module-frontier`): its build reads `vendor/`, and whether that copy matches the
   checkout is not a text question. Its intra-module edges are unaffected.
5. A replace whose PATH leaves the repository, names no `go.mod`, or names a `go.mod` with another
   module line is `go:module-path-unresolved` for that edge, never a silent drop.
6. Environment overrides (`GOFLAGS=-mod=mod`, `GOWORK`) are outside the text; the plan already
   states the toolchain is never executed and its answer is for default flags.

## Falsification the implementation must carry

One etcd-shaped fixture with: M replacing N (selected on dirty N, witness M's replace line); M2
requiring N without a replace (excluded, detail names the version); the chain M replaces N, N
replaces P, dirty P (M's tests excluded: non-transitivity); a replace to a directory whose module
line differs (`go:module-path-unresolved`); a version-qualified replace M does not require (not
applied); a module with `vendor/modules.txt` (frontier). Each is one focused test, sized like
`workspace_test.go`.

## Sequencing

The gain is development-only: the fifty held-out trial tasks hold 24 Go tasks, all on
single-module snapshots (scan of the 128 Go snapshots on disk, 2026-09-01: 98 single-module, 14
`go.work`, 15 multi-module without `go.work`, every one of them etcd). The pre-workspace etcd
snapshots sit in `v2_code2test` (10), `v2_comment2context` (4), and `v2_edit2ripple` (2). The rule
is accepted as written above so the implementation is mechanical (about a day: replace scan per
observed module, AFP-V0-008 amendment, the fixture), and it is scheduled after the
confidently-wrong trial's first run, which it cannot improve and must not delay. The work item
lives in `docs/agent-memory/ideas.md` until then.
