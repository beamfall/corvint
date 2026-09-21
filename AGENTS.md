# Corvint agent contract

Corvint is a local-first, proof-carrying context compiler for software agents.

## Product invariants

1. Evidence is pinned to immutable Git content and always explains its inclusion.
2. Missing evidence produces uncertainty or abstention, never invented certainty.
3. Project-owned authority outranks syntax, history, and learned task traces.
4. Read commands do not mutate repository or trace state. The one exception is the bounded local
   self-observation ledger `.corvint/self-observations.jsonl`, written by `harness event` on the
   three `SOL-V0-001` events, by the commands `SOL-V0-007` enumerates on an `unsupported-*`
   failure, and by the host `adapter` on a `SOL-V0-010` adapter degradation; it is private derived state and never an input to ranking, learning, evidence,
   or authority. A second private ledger `.corvint/unplanned-reads.jsonl` is written only while the
   operator-created marker `.corvint/unplanned-reads.enabled` exists; like the self-observation ledger
   it is bounded, local, derived state and never an input to ranking, learning, evidence, or
   authority.
5. Learning is explicit, local, bounded, secret-screened, and evaluation-gated.
6. Corvint integrates with existing specs and agents; it does not require a new spec language.
7. The default local product remains one native-Go binary with no account, network dependency,
   mutable/external database service, hosted service, embeddings, permanent daemon, UI, or broad
   language rewrite. A selected immutable embedded index encoding is derived state, not a database
   service; it still requires the accepted benchmark/format gate. A
   capability outside that local boundary requires its own accepted profile, must reuse the same
   transport-neutral immutable index contract, and cannot silently broaden the local path. The
   deployment-neutral index direction is governed by
   `docs/specs/deployment-neutral-index-platform-v0.md`; a measured production hot path may move
   languages only under an accepted exact-parity migration contract with staged rollback, and
   `docs/specs/go-production-kernel-migration-v0.md` governs the current Go migration. An optional,
   separately-built, loopback-only, read-through local console may be started explicitly by the
   operator (decision 0081, `docs/specs/local-admin-console-v0.md`). It is not part of the default
   local product, installs no service, holds no database, opens no outbound connection, and carries
   no authority; the default install remains one native-Go binary with no UI.
8. Substantive capabilities are spec-driven. Record the human-owned intent, numbered requirements,
   non-goals, failure modes, acceptance evidence, and rollback before calling implementation done.
   Generated or inferred documentation can propose intent but cannot accept or govern it.

## Spec-driven development

Follow `docs/SPEC-DRIVEN-DEVELOPMENT.md`. Every active build slice has one entry in
`docs/specs/README.md` and one executable spec or explicitly cited product contract. Keep requirement
IDs stable, trace implemented requirements to tests or measured evidence, and update the spec in the
same change when behavior or a wire contract changes. A prototype may precede an accepted technical
contract only while it is labelled experimental and cannot be promoted or advertised as delivered.

Record material design decisions, independent findings, failed evaluations, and promotion evidence
in `docs/BUILD-LOG.md`. Search headings or IDs and read only matching entries, never the whole
log. Document contracts and decisions,
not a narration of individual code lines.

## Dogfood Corvint

For Corvint development, select applicable existing feature routes from `docs/SELF-DEVELOPMENT.md`;
retain actual use evidence and explicit exclusions without weakening the existing gates.

Substantive Corvint work must follow `docs/DOGFOOD.md`: use Corvint to collect pre-change context, bind
the delivered diff to a CEM, inspect the reviewer report, run the relevant frozen evaluations, and
record the explicit local outcome. Corvint self-use is product evidence and friction discovery, never
a substitute for independent interoperability or external outcome validation.

At change start run `make dogfood-change BASE=<sha>`. Final binding and checking are post-commit:
follow `docs/DOGFOOD.md` §4, commit the CEM, then rerun `dogfood-change` and `dogfood-check` from
a clean worktree against the same base, and finish with `make dogfood-seal BASE=<sha>`, which moves
the checked CEM out of the shared tracked path. Keep every `NOT_PRODUCED` reason visible.

## Verify

Use `corvint affected --base FULL_SHA` before running tests for a change. During implementation and
repair, run the selected units plus any checks needed to resolve retained unknowns; do not rerun the
exhaustive gate after each repair. Run the exhaustive commands below only once at the terminal
boundary when repository or release policy requires them. If the owner explicitly waives that gate,
retain it as `NOT_RUN` with the affected-plan unknowns instead of implying equivalent coverage.

```sh
test "$(GOTOOLCHAIN=local go env GOVERSION)" = "go1.27.1"
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...
GOTOOLCHAIN=local go vet ./...
(cd interop/cem01-go && GOTOOLCHAIN=local go test -count=1 -timeout 30m ./... && GOTOOLCHAIN=local go vet ./...)
```

`-timeout 30m` matches `GO_TEST_TIMEOUT` in the Makefile: it is a per-package hang detector, not
a performance budget (decision 0082). Go's 600s default is too tight to use here, because
`cmd/corvint` alone takes roughly 591s on an otherwise quiet host and panics under any load.

## Agent routing

Use the matching row in `docs/AGENT-ROUTES.md` for a bounded route from question to original evidence.
Spec questions: read `docs/specs/INDEX.json` or the target spec's Agent digest first. Look up
requirement IDs in `docs/specs/REQUIREMENTS.tsv`; open a full spec body only after the digest points
to a section. Backlog questions: `rg -n '^### ' docs/agent-memory/*.md` first.

Corvint is licensed under the GNU Affero General Public License v3.0 or later. `LICENSE` carries the
AGPL text and `PROVENANCE.md` records the copyright basis. `LICENSING.md` is the authoritative path
boundary: the enumerated protocol/interoperability paths are plain Apache-2.0 (`LICENSE-APACHE-2.0`),
and every other path is AGPL-3.0. A new Apache-2.0 file belongs in `protocol/**`; widening that
boundary requires amending `docs/decisions/0002-future-publication-transition.md`. Preserve each
path's applicable terms, third-party notices, and provenance when publishing or extracting source.
