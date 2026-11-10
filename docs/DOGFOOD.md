# Corvint dogfood contract (retired)

Retired 2026-11-09 by decision 0594, the test-selection refocus. The dogfood workflow (the
`dogfood`, `cem`, `ocm`, `frontier`, `local` and `prove` command families, the `dogfood-*` make
targets and scripts, and the CEM bind, check and seal loop) was removed from this repository.
Corvint development no longer binds changes to a CEM or seals them under `.corvint/changes/`.

For current contributor guidance, read [AGENTS.md](../AGENTS.md) (spec-driven development and
verification) and [SELF-DEVELOPMENT.md](SELF-DEVELOPMENT.md) (which Corvint features to use while
developing Corvint). The last full text of this contract is `docs/DOGFOOD.md` at commit
`4663b4e2b`; earlier history lives in the `beamfall/corvint-history` archive (decision 0587).
