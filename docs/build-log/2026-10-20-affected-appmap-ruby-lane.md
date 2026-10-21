# Affected, appmap and Ruby lane: decisions and selection attribution

Date: 2026-10-20. Base: `7d78a56b0de4f443df260228ef8dddc85aec7a84` (origin/main).
Branch: `claude/batch-affected-appmap-ruby`. Tickets: V1-1816, V1-1850, V1-2058, V1-1712, V1-0975,
V1-1777, V1-1832 and V1-1828 delivered; V1-0999 and V1-1878 attributed here.

## Design decisions

- V1-1712 (`AMAP-V0-190`): a JSX `<Routes>` tree is read only when no enclosing bracket level is
  gated by a conditional, loop, logical operator or a non-JSX call. The reader fails toward the
  existing unknowns: an expression child, such as `{cond && <Route/>}`, still records
  `non-literal-react-children` at its own line, and the file-wide `dynamic-route-context` is
  recorded unless every `<Routes>` tree was placed statically.
- V1-1832 (`RUBY-AFFECTED-V0-043`): the lexical-nesting scan never narrows by guesswork. A file
  whose block keywords cannot be balanced, or that holds `=begin` or `__END__`, falls back to the
  previous whole-file namespace set. A heredoc passed to an evaluating call, or one with no
  terminator, stays unmasked. Before this change, an apostrophe inside heredoc text opened a quote
  that masked the rest of the file and dropped real edges. The heredoc mask removes that
  under-selection as well as the extra edges.
- V1-1828 (`AFP-V0-066` amended): the NGI-V0-052 gem patterns and their 256 KiB bound are mirrored
  in `cmd/corvint` rather than shared, because `internal/contextindex` belongs to another lane.
  Extracting one helper for both surfaces is follow-up work.
- V1-2058: a single JS/TS test-file predicate now admits `__tests__` segments. Before, two
  predicates disagreed.
- V1-1777: the actor-authorization analysis indexes factory default roles in one pass. Measured
  with `BenchmarkActorAuthorizationLargeSuite` (200 fixtures, 2,000 filler files, `-benchtime 3x`)
  at 126.6 ms/op before and 13.0 ms/op after. An oracle test keeps the old per-call scan.

## V1-0999: attribution, not narrowed

A scratch change to `internal/appmap/verify.go` selected 66 units with scope `UNKNOWN`:

| Witness | Units |
| --- | ---: |
| `BINARY_EXEC` | 25 |
| `UNBOUNDED_READER` | 24 |
| `DEPENDENCY_PATH` | 9 |
| `PATH_LITERAL_READER` | 4 |
| `DECLARED_READ_SCOPE` | 3 |
| `DIRECT` | 1 |

The unbounded readers are justified by their text:

- test literals that reach `..` or `../../..`;
- `--show-toplevel` root location;
- `os.Getwd` in `internal/tasks/cli` and `internal/testrunner/sql`;
- nine `shell:runs-toolchain` scripts.

Narrowing them needs declared read scopes (AFP-V0-023). Those scopes are Landlock-enforced only
in CI, so they cannot be verified on this host, and two of the readers sit in paths other lanes
own. The ticket stays open.

## V1-1878: attribution

Reproduced in a detached worktree at `8a21fb19c` with `--base 6ceee260b`. The plan selected 148
units with scope `UNKNOWN`.

107 units come from the change's Go sources:

| Witness | Units |
| --- | ---: |
| `DEPENDENCY_PATH` | 59 |
| `BINARY_EXEC` | 24 |
| `PATH_LITERAL_READER` | 16 |
| `DECLARED_READ_SCOPE` | 3 |
| `DIRECT_SOURCE_CHANGE` | 5 |

Every `BINARY_EXEC` unit runs the built `cmd/corvint`, because `cmd/corvint/test_validity.go`
changed.

41 units come from docs and data paths:

| Witness | Units |
| --- | ---: |
| `PATH_LITERAL_READER` | 36 |
| `UNBOUNDED_READER` | 4 |
| `DECLARED_READ_SCOPE` | 1 |

By dirty path:

| Dirty path | Units |
| --- | ---: |
| the CEM under `.corvint/changes/` | 17 |
| `docs/specs/INDEX.json` | 9 |
| the build-log entry | 6 |
| `docs/specs/README.md` | 5 |
| `docs/specs/REQUIREMENTS.tsv` | 4 |

Each of those is a literal reader of that file, for example `internal/specindex` and the
line-citation and requirement-definition check scripts.

The `LANGUAGE_FRONTIER` unknowns are repository-wide frontiers of fixture languages, such as
cfamily, dotnet and swift. They are not caused by this diff.

Conclusion: the breadth is mostly genuine. A change to `cmd/corvint` reaches every user of the
built binary. The one class worth narrowing is the CEM path: a committed `.corvint/changes/*.cem.json`
reaches 17 readers. That needs a readers decision for the dogfood artifacts, which is outside this
lane. The ticket stays open with this attribution.
