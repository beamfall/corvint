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
