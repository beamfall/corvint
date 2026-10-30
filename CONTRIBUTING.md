# Contributing to Corvint

Contributions are welcome where they fit the product invariants in
[AGENTS.md](AGENTS.md) and the development contract in
[docs/SPEC-DRIVEN-DEVELOPMENT.md](docs/SPEC-DRIVEN-DEVELOPMENT.md).

## Before you start

- Read the [README](README.md) for what the product does and does not claim.
- Read [AGENTS.md](AGENTS.md); the eight product invariants are not negotiable in a change.
- Check [docs/specs/README.md](docs/specs/README.md) for the spec that governs the area you want
  to change. Substantive behaviour is spec-driven: a change that alters behaviour or a wire contract
  updates its spec in the same change.

## Building and testing

```sh
GOTOOLCHAIN=local go build ./...
corvint affected --base FULL_BASE_SHA   # the tests your change reaches
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./PACKAGE/...
GOTOOLCHAIN=local go vet ./PACKAGE/...
```

Run the packages `affected` selects, keep any unknowns it reports visible, and widen to
`go test ./...` when it cannot narrow the set.

`make gate` runs the full release gate set. Individual gates are listed in the `Makefile`; the ones
most changes need are `make line-citations-check`, `make decision-numbers-check`, and
`make requirement-definitions-check`.

## Change hygiene

- One concern per change. Keep requirement IDs stable and trace implemented requirements to tests
  or measured evidence.
- Documentation line citations (`path:N-M@sha`) are checked; run `make line-citations-check` after
  editing any cited document.
- Design decisions, findings and evaluation results go in one new file per entry under
  `docs/build-log/` (decision 0423); entries from before 1.0 are in the
  [`beamfall/corvint-history`](https://github.com/beamfall/corvint-history) archive (decision 0587).
- Material design decisions go in `docs/decisions/` using the next free number and the existing
  template; `make decision-numbers-check` enforces uniqueness.
- Do not add network dependencies, hosted services, embeddings, or a UI to the default local
  product (invariant 7).

## Licensing

Corvint is licensed under Apache-2.0 across the whole repository (decision 0586); see
[LICENSING.md](LICENSING.md). Contributions to the public repository are licensed under Apache-2.0,
as section 5 of that license provides, with no further agreement required. Issue reports and
discussion are welcome.

All contributions must disclose third-party material and preserve its applicable terms and notices.
The owner records the author or rights-holding organization of each outside contribution as
[PROVENANCE.md](PROVENANCE.md) describes. The [contributor agreement](CONTRIBUTOR-AGREEMENT.md)
remains available as an optional additional grant; contributions previously merged under it keep
its recorded scope. Do not place sensitive identity or employment documents in Git or public
comments; the agreement describes a private alternative.
