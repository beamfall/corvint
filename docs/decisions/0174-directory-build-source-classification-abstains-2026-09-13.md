# Decision 0174 — `dotnet.directory-build` mints no `dotnet.source` fact

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

The 2026-09-12 bug-hunt hypotheses entry in `docs/agent-memory/ideas.md` flagged
`internal/analyzerdotnet/project.go:52@bfe48561` (`baseDirectory` applied to a Directory.Build file's own
path): `readItems` resolves a `Compile Include` against the directory of the input it is parsing,
so a `Directory.Build.props` under `src/` resolves `Compile Include="Shared.cs"` to `src/Shared.cs`
and `emitSourceClassification` (`project.go:364`) minted `dotnet.source` for that path, pinned by
`TestDirectoryBuildWitnessesOnlyClassification` (`analyzer_test.go:538`).

Governing contract. `docs/specs/analyzer-candidate-profiles.md`'s fact matrix (`dotnet.source` row)
listed `dotnet.project` or `dotnet.directory-build` as the admissible family. MSBuild imports a
Directory.Build.props/.targets into the *importing* project's evaluation: a relative `Include` on an
item declared there resolves against `$(MSBuildProjectDirectory)` -- the importing project's own
directory -- not the props file's folder, and a project may sit at any depth below it. `project.go`'s
own comment already states this candidate never walks an import chain, so it has no way to learn
which project (if any) imports a given Directory.Build file.

The call: a Directory.Build file's `Compile`/`None`/`ProjectReference` items are validated (a
malformed one still rejects the whole input, per AGENTS.md invariant 1's schema pinning) but never
promoted into a fact. Resolving `Compile Include` against the props file's own directory would be an
invented certainty about a path this candidate cannot place — the honest output is to withhold it
(AGENTS.md invariant 2), the same principle `emitSourceClassification`'s existing `defaultItems`
guard already applies to glob-determined compile sets.

Consequences:
- `internal/analyzerdotnet/project.go`'s `emitProject` now returns before calling
  `emitSourceClassification` for any family other than `dotnet.project`, so `dotnet.directory-build`
  witnesses nothing.
- `docs/specs/analyzer-candidate-profiles.md`'s `dotnet.source` matrix row drops
  `dotnet.directory-build`; the `dotnet.directory-build` family row states the abstention and cites
  this decision.
- `TestDirectoryBuildWitnessesOnlyClassification` is renamed `TestDirectoryBuildWitnessesNothing` and
  now asserts an empty fact set for a Directory.Build file that declares a framework, a project
  reference, and a `Compile Include`.

Rollback: revert the commit. That restores `emitSourceClassification`'s call for every family,
`dotnet.directory-build` as an admissible `dotnet.source` family in the matrix, and the prior test
name and assertion.
