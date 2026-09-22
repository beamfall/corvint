# Decision 0310 — External evidence repositories are identified by a declared root commit

Date: 2026-09-18. Status: accepted (delegated call on the owner's own feature request
Beamfall/corvint#3; owner review of the intent is welcome and changes only the digest line).

## Decision

Cross-repository external evidence is a new record schema, `external-evidence-provider/1`,
specified in `docs/specs/external-evidence-provider-v1.md` (`EEP-V1`). V0 records keep their decoder
and their exact section bytes.

- **Identity.** A declared repository is identified by its record-local `id` and its `origin`, a
  parentless commit id. A root commit is content-addressed, already present in every clone, and
  independent of path, branch, remote, and hosting service. `remote` and `role` are advisory: they
  are validated and echoed, and never bind. A remote must arrive normalized, because deciding that
  two hosting URLs are equal is policy that Core does not own.
- **Binding.** The invocation checkout binds a repository whose origin is one of its root commits.
  Any other repository binds only through an explicit `--repository ID=DIR` whose `HEAD` has that
  root. Core never clones, fetches, or searches for a checkout.
- **Fail closed.** A missing origin, a shared origin, two declared roots of one `HEAD`, a checkout
  of another history, an unreadable checkout, and an undeclared repository each produce a named
  state. Freshness and verification are evaluated per repository, and a relation is `fresh` only
  when every side is.
- **Scope.** Path-to-path relations are listed as `unsupported`, and relations are never inferred.

## Alternatives weighed

- *Remote URL as identity*: mirrors, renames, and local-only repositories break it, and it
  invites credentials into records. Kept as an advisory field only.
- *Declared id alone*: unverifiable. Any directory could be passed as any id.
- *First commit of the current branch*: not a single value when histories were merged. The chosen
  rule accepts any root and refuses when more than one declared repository qualifies.

The known cost is that a fork and its upstream share a root commit and cannot be declared together.
The spec's kill criterion covers it.

## Rollback

Remove `record1.go`, `repository.go`, the `--repository` option and its help text, the V1
fixtures, the spec, its index rows, and this record, and restore `Section`'s V0 signature. V0 wire
bytes are unaffected in both directions.
