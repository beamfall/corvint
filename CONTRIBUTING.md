# Contributing to Corvint

Corvint is an alpha. Contributions are welcome where they fit the product invariants in
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
GOTOOLCHAIN=local go test -count=1 -timeout 30m ./...
GOTOOLCHAIN=local go vet ./...
```

`make gate` runs the full release gate set. Individual gates are listed in the `Makefile`; the ones
most changes need are `make line-citations-check`, `make decision-numbers-check`, and
`make requirement-definitions-check`.

## Change hygiene

- One concern per change. Keep requirement IDs stable and trace implemented requirements to tests
  or measured evidence.
- Documentation line citations (`path:N-M@sha`) are checked; run `make line-citations-check` after
  editing any cited document.
- Material design decisions go in `docs/decisions/` using the next free number and the existing
  template; `make decision-numbers-check` enforces uniqueness.
- Do not add network dependencies, hosted services, embeddings, or a UI to the default local
  product (invariant 7).

## Licensing

Corvint is AGPL-3.0-or-later with an Apache-2.0 interoperability layer; the path boundary is
recorded in [LICENSING.md](LICENSING.md). Contributions to the public repository remain licensed
under the terms that apply to their destination paths.

The owner preserves the option of separately licensing the Corvint implementation commercially.
Outside contributions to AGPL product paths require an explicit acceptance of the
[contributor agreement](CONTRIBUTOR-AGREEMENT.md), or a separately recorded sufficient rights grant,
before merge. Contributors retain ownership. This additional permission includes proprietary
commercial sublicensing; submission under AGPL or a DCO signoff alone is insufficient.

Use the authenticated pull-request acceptance statement in the agreement, identify its immutable
text revision and covered contribution commits, and obtain acknowledgment from the owner or an
expressly authorized maintainer. Each rights holder must grant the necessary permissions; include
the represented organization and authority where an employer or client owns the work. Public
acceptance is manual and requires no paid signing service or outside legal review.

Maintainers retain the agreement text, accepted contribution bytes, authenticated acceptance and
acknowledgment, and any authority evidence in private rights records before merging. Later commits
need coverage. For squashed or rewritten commits, record the accepted-to-merged mapping, verify
preserved content, and resolve rights to any additional or conflict-resolution material. Missing
or uncertain grants keep the affected contribution on hold. Do not place sensitive identity or
employment documents in Git or public comments; the agreement describes a private alternative.

Apache-2.0 contributions continue under Apache-2.0 without this additional agreement. Mixed-path
contributions follow the product-path rule for their AGPL portion. Issue reports and discussion
are welcome without agreement acceptance. All contributions must disclose third-party material
and preserve its applicable terms and notices. Previously merged work is not retroactively covered;
obtain any additional permission needed before including it under alternative commercial terms.
