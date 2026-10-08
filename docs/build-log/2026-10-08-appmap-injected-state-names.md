# Application map: state names read through AngularJS injected constants (V1-1027)

Date: 2026-10-08. Ticket V1-1027; GitHub [#685](https://github.com/beamfall/corvint/issues/685).
Requirements: `AMAP-V0-021`..`AMAP-V0-023` (proposed) in `docs/specs/application-map-v0.md`.

## Problem

After AMAP-V0-016, an AngularJS + UI-Router adopter still read most `.state(...)` calls as
`non-literal-name`. Their routers do not import the name table. They receive it as an injected
parameter, `($stateProvider, Names) => $stateProvider.state(Names.SCREEN, {...})`, and the table
is registered elsewhere as `.constant('Names', T)`. The adopter registers the same constant name
in two separate apps in one repository, so a repository-wide search for the registration would
always be ambiguous.

## Decision

- Opt-in, project-owned scope: the manifest member `di_constants` (1..16 repository-relative
  paths) names the sources whose `.constant(...)` registrations belong to the app (AMAP-V0-021).
  Without the member, nothing resolves through injection and existing manifests compile to the
  same bytes. A malformed scope, or a path with no source, refuses with `appmap-invalid-manifest`.
  Scanning more than 20000 sources or 128 MiB of text refuses with `appmap-bound-exceeded`. The
  scan is lazy: it runs only when a router reads a name through injection.
- A name resolves only when the static reader proves the binding (AMAP-V0-022):
  - every binding of `X` in the router file is a plain parameter of an injectable function: a
    top-level arrow or function, or a `.config(...)` argument, either direct or the last element
    of an inline array annotation;
  - any annotation, inline or `F.$inject`, matches the parameter's position and the parameter
    count;
  - `F` is used only in its declaration, its annotation, `.config(F)` or an export;
  - exactly one literal-named `.constant('X', T)` exists in scope;
  - `T` is an object literal, or an identifier that the AMAP-V0-016 rules read whole in the
    registering file. The registration argument is the one use of `T` exempted from the
    "only member reads" rule.
- Evidence: the anchors are the table's declaring line, the import in the registering file, the
  `.constant(...)` call, and the router function's parameter list, in that order. An inline
  object literal on the call's own line is carried once. The anchors join lineage freshness, so
  re-pointing a registration reads the screen's lineage `STALE`.
- Fail closed (AMAP-V0-023): a source in scope that is excluded, unreadable or over 4 MiB, or
  that has a `.constant(...)` call whose first argument is not a literal, makes every injected
  name in that build unknown. Lodash `_.constant(x)` is exempt. Unknown reasons stay
  `non-literal-name` / `non-literal-value`. Owner question 16 asks whether that poisoned case
  should get its own reason.

## Alternatives rejected

- Inferring the scope from the router's directory, or from an AngularJS module graph: either one
  guesses which app a registration belongs to. A module graph would also need evaluation, which
  is outside the token-reader contract.
- Taking the first, or the nearest, of several registrations: this is a guess, and with the
  adopter's two apps it would be wrong half the time.
- Accepting an unannotated function in a file that has `$inject` annotations: the function's
  annotation could live elsewhere, so its parameter order is unproven.

## Evidence

The tests are in `internal/appmap/stateinject_test.go`:

- `TestAMAPV0021DIConstantsManifest`: 7 refusals plus an unchanged literal screen.
- `TestAMAPV0022InjectedStateNames`: an imported table, a declared table, an inline table and a
  parent, with the anchor order checked and lineage going `STALE` after a re-point. A second
  app's registration outside the scope is ignored.
- `TestAMAPV0022InjectionAnnotations`: 6 function and annotation shapes.
- `TestAMAPV0023UnprovableInjectionStaysUnknown`: 28 fail-closed cases plus a control.
- `TestAMAPV0023ScopePoisoned`.

The full `internal/appmap` package passes. No adopter-scale re-measurement was run, because the
reporter's app is not available here.
