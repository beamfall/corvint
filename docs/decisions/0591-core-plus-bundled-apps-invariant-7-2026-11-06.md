# Decision 0591: Corvint ships as a core plus bundled apps; invariant 7 is amended

- Date: 2026-11-06
- Status: accepted (owner, 2026-11-06, "Accept as drafted": invariant text, default install of
  bundled apps, shared release version)
- Authority:
  - Owner chat, 2026-11-06:
    - reopened D6 (Tasks becomes its own module and app) and D7 (the app split is a goal);
    - set four split criteria: easy for an LLM to work in, isolated tests, plugins where they make
      sense, fast and light on memory.
  - `docs/specs/core-bundled-apps-v0.md` (`CMS-V0-001..011`).
- Amends:
  - `AGENTS.md` invariant 7;
  - `docs/PRODUCT.md`;
  - `README.md`;
  - `docs/specs/public-release-v0.md`;
  - `docs/specs/observed-ui-evidence-v0.md`;
  - `docs/specs/local-admin-console-v0.md`;
  - `docs/specs/language-plugins-v0.md`;
  - `docs/specs/go-production-kernel-migration-v0.md`.

  Decision 0540 stays as history.
- Keeps:
  - decision 0081 (the console stays optional and never enters the default manifest);
  - `deployment-neutral-index-platform-v0`;
  - the parity and rollback rule of `go-production-kernel-migration-v0`;
  - invariant 4.

## Context

Invariant 7 said the default product is one native-Go binary. In practice:

- `cmd/` already holds 54 entry points and 10 installed executables.
- `corvint` links about 444 packages (about 812k production lines), including all of docs, flows
  and Tasks.
- `cmd/corvint` imports `doccorpus` 34 times, `appflows` 9 times and `taskman` 3 times.

The prohibitions that matter (no account, network dependency, database service, hosted service,
embeddings, daemon or UI) do not depend on there being exactly one executable. "One binary" stood in
for "nothing joins the default product without a decision". This decision keeps every prohibition
and states that rule directly.

**Terminology.** This decision says *bundled app*, not "companion". "Companion" keeps its existing
meaning in PUB-V0 and CCF-V1: optional, separately distributed and unfrozen.

## Decision

1. Invariant 7 now reads: "The default local product is one release of native-Go executables: the
   `corvint` core plus the bundled apps and plugins its release manifest names".
   - Adding an executable to the manifest needs its own accepted spec and decision.
   - Bundled apps reach the core and each other only through published, versioned envelopes or
     `corvint-plugin/1`.
   - Bundled apps write no core evidence, index or trace state, and hold no authority the core lacks.
   - The core's frozen command surface works with no bundled app installed and never forwards to one.
   - The console is never in the manifest.
2. Docs, flows and Tasks install by default as bundled apps. The release ships one shared version
   and one checksummed manifest, so no app is versioned separately.
3. The split is judged by the four owner criteria. An extraction that improves none of them is not
   done (`CMS-V0-008..011`).
4. Plugins are separate executables (never `.so`), used only where many interchangeable
   implementations exist. Each run starts one plugin process that serves many requests; there is no
   per-file process and no daemon. A plugin boundary that costs more than 5% of `index` or
   `affected` wall time stays linked behind its registry.
5. Order of work:
   1. baseline measurements;
   2. kernel moves (V1-4807, V1-4808, V1-4809), then V1-4810;
   3. per unit, first cut the import edges into envelope calls, then extract the module, in this
      order: Tasks, docs, flows, adapters, developer tooling;
   4. the plugin contract (V1-4819) and conformance kit (V1-4817) proceed alongside.

## Rollback

- Each extraction is one PR; reverting it re-links the code into `corvint`. `CMS-V0-004` keeps
  CLI names and wire formats, so no wire change is needed.
- Reverting the invariant text is a new decision superseding this one.
