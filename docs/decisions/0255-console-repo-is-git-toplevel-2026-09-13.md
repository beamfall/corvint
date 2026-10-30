# Decision 0255 — the console's `--repo` must be the Git toplevel

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`corvint-console --repo DIR` ran its Git reads with `DIR` as the working directory. Git discovers the
enclosing repository from a subdirectory, so `--repo internal/console` rendered the whole Corvint
repository while the operator believed a narrower root was being served. The same held for
`--specs`, and a directory inside no repository failed only on the first page load.

Call: the console refuses to start unless `--repo`, and `--specs` when given, each resolves
(symlinks followed) to the path `git rev-parse --show-toplevel` reports for it. A subdirectory or a
non-repository directory is refused before the listener binds, naming the actual toplevel. This is
the refusal-over-reinterpretation rule ESV-V0-001 already applies to its repository root, and it is
recorded as `LAC-V0-031` in `docs/specs/local-admin-console-v0.md`.

Rejected: serving the discovered toplevel silently (the operator's argument would be reinterpreted),
and scoping reads to the subdirectory (the console's views are repository-wide by contract).

Rollback: drop the `RequireToplevel` check in `cmd/corvint-console/main.go` and remove `LAC-V0-031`.
