# V1-1890, V1-1891: `flows appmap actions` and `flows appmap branches`

Owner input: GitHub #1174 (V1-1890, ROUTE-ACTION-MISSING source check) and #1175 (V1-1891, per
cited branch: tests by boundary plus the setup that can produce its precondition). Requirements
AMAP-V0-100..107 and AMAP-V0-110..116 in `docs/specs/application-map-v0.md` are accepted under
the owner's standing auto-accept of lane requirements (2026-10-11). Delivery is experimental.

Spec home: both checks read the application map's `rails-routes/0` routers and test files and
refuse with the AMAP error codes, so they belong in the application-map spec rather than the
test-source or review-packet specs; no TSA ID is used and no CAL-V0 ID is added.

Decisions:

- `actions` compiles every supported Rails route to a controller and action, then reads the
  controller class, its superclass chain and included concerns from Git objects at the map
  revision. A route reports `ROUTE-ACTION-MISSING` only when the whole chain was read and none
  of it can define the action at run time; any doubt (unresolved ancestor or module, dynamic
  definition, unsupported syntax, depth) abstains with a named reason. A non-public method with
  no template is reported missing with basis `non-public-method`. Exit 1 on a finding makes the
  verb usable as a check.
- `branches` reads branches from cited lines only, joins tests by boundary through static
  naming (constant and method, routed path or helper, visited screen, controller callbacks) and
  labels every listed test with its basis. A guard input that does not resolve to a model
  attribute keeps the branch `unresolved-guard`, so the report never says "no setup" for a
  guard it could not read.
- V1-1900 reuse point: the model-attribute writer index `newSetupWriterIndex(app, routes)` and
  `writersOf(model, attr)` in `internal/appmap/setup_writers.go` (setup endpoints, FactoryBot
  factories and traits, YAML fixtures). It is the only setup join built here; the general
  data-setup model stays with V1-1900.
- The lane's Ruby structure reader (`internal/appmap/ruby_structure.go`) treats a keyword after
  a masked string literal as a modifier: `contextindex.FlowCodeLines` masks a literal, quotes
  included, to spaces, so the raw line decides (`codeBefore`). Without it
  `flash[:x] = "y" if cond` read as an open block and the file abstained. Rails options written
  `key: :sym` (`on: :collection`, `only: :create`, `module: :api`) are options, not constants.

Evidence: `go test` through the heavy-test semaphore, one package per command:
`./internal/appmap` (whole package, and `-run 'TestAMAPV01[01]'`) and `./cmd/corvint -run
'TestAMAPV0106ActionsCLI|TestAMAPV0116BranchesCLI|TestAMAPV0098RefsCLI'` pass. One early run
called `go test` without the semaphore; later runs used it.

NOT_PRODUCED: adopter-scale qualification on a real Rails application; the checks run over the
inline fixtures only.

Rollback: remove the files and verbs named in the spec's "Route action and branch query slices"
section and its requirements. No stored state or map schema changes.

## Review round (PR 1190, Codex findings 1-14)

Each finding was fixed with a regression test that fails on the reviewed head `89f20f49f`, or
turned into an abstention where a definite answer would risk a false `ROUTE-ACTION-MISSING`.

| # | Finding | Disposition | Test |
|---|---|---|---|
| 1 | `to: "/health#show"` kept the enclosing module | Fixed: a leading `/` ignores the module (`controllerPath`) | `TestAMAPV0101AbsoluteControllersAndUpdateVerbs` |
| 2 | `::X` superclass or include resolved lexically | Fixed: `::X` resolves at top level only | `TestAMAPV0103LookupOrderCases/absolute-*` |
| 3 | First found `def` stopped the walk, even when private | Fixed: the walk continues; a non-public action with `action_missing` abstains `dynamic-definition` | `.../private-with-action-missing` |
| 4 | `def` inside `if`/block at body level read as defined | Abstained: `dynamic-definition` with path and line | `.../conditional-definition` |
| 5 | `include A, B` order reversed | Fixed: parts appended in Ruby order | `.../include-argument-order` |
| 6 | `private :x`/`public :x` on an inherited method ignored | Fixed: visibility tracked along the chain | `.../inherited-made-private`, `.../inherited-made-public` |
| 7 | Reopened class bodies in one file lost all but one | Fixed: bodies merged in file order | `.../reopened-class` |
| 8 | Implicit template looked only under the controller path | Fixed: ancestor view prefixes (`application` included) | `.../ancestor-view-prefix` |
| 9 | Callbacks ignored `only:`/`except:`/`skip_*` and conditions | Fixed: replayed in file order; `if:`/`unless:`/unread options give `possible` rows | `TestAMAPV0112CallbackRestrictions` |
| 10 | Calls in test comments and strings counted | Fixed: test text masked before matching | `TestAMAPV0112CommentedCallsIgnored` |
| 11 | `update` routed as `PATCH` only | Fixed: verb `PATCH\|PUT` | `TestAMAPV0101AbsoluteControllersAndUpdateVerbs` |
| 12 | Nested factory inheritance depended on map order | Fixed: parents resolved in ascending child order | `TestAMAPV0114FactoryInheritanceAndClass` |
| 13 | `class: "X"` / `class: :x` unread, factory bound by name | Fixed: quoted and symbol classes read; an unread class binds no model | `TestAMAPV0114FactoryInheritanceAndClass` |
| 14 | Read budget per root, and branches loaded every root | Fixed: one budget per command shared with map tests; branches loads only cited roots | `TestAMAPV0116SharedReadBudget` |

Old-code proof: the new tests were run against `89f20f49f` in a scratch worktree; every
finding-specific case failed (22 FAIL lines). To compile `TestAMAPV0116SharedReadBudget` there,
the `newAppReadBudget` seam was added to the old `loadRailsApp` as a fresh budget per call, which
is the old per-root behavior. Two control subtests (`except: :preview`, `only: %i[show]`) pass on
both heads, as intended.

Spec: AMAP-V0-101..105, 107, 110, 112 and 114 are amended to the fixed behavior and the
traceability rows cite the new tests. These amend requirements already accepted on 2026-10-11;
the standing auto-accept does not cover amendments, so they await owner acceptance.

Evidence: `script/heavy.sh env GOTOOLCHAIN=local go test -race -count=1 ./internal/appmap` and
`go test -race -count=1 -run 'TestAMAPV0106ActionsCLI|TestAMAPV0116BranchesCLI|TestAMAPV0098RefsCLI'
./cmd/corvint` pass. Host load average reached 47.8 during the race run (peer lanes).

Residuals: callbacks inherited from `ApplicationController` or concerns are not mapped to subclass
routes; `prepend_view_path`, `_prefixes` overrides, engine view paths and `abstract!` are not read;
visibility order within one scope (`private :x` before a re-definition) is approximate;
unit-boundary matching still reads raw test text; multi-line and hash-rocket callback statements
give only `possible`; an unread factory class drops the model silently rather than emitting an
uncertainty row. NOT_PRODUCED: adopter-scale qualification.
