# Source-derived application flow documentation

`docs flows` generates source observations with `generated` trust. It does not execute an application
or accept intent. Existing accepted-outcome AFU documentation keeps its separate PROVEN/UNPROVEN rules.

```sh
corvint --root /path/to/repo docs flows generate \
  --revision FULL_SOURCE_SHA --scope app --output-dir /existing/parent/fresh-output
corvint --root /path/to/repo docs flows check \
  --revision FULL_LATER_SHA --previous /existing/parent/fresh-output/generation.json \
  --output-dir /existing/parent/fresh-output
```

Use an absolute output path under existing parents without symlinks. The output directory must not
exist. Each flow gets README, entry-points, flow-diagram, functional-overview, technical-deep-dive,
entities, exceptions and related-flows Markdown files. `provider.json` contains typed corpus records;
`generation.json` pins the source, renderer version, logical identities, anchors and output digests.
The default bounded profile allows 256 flows and 2,048 Markdown pages. Rendered pages plus provider
are bounded to 64 MiB. Existing corpus input retains its 128 MiB bound; the generation manifest may
reach 256 MiB including that corpus and generated metadata. Total output is bounded to 320 MiB.
Narrow scope above those limits.

The construct matrix includes Ruby methods, jobs, literal Rails routes and class validation/hooks;
JS/TS named functions/arrows (`.js`, `.mjs`, `.cjs`, `.jsx`, `.ts`, `.tsx`), literal route/Angular registrations; React JSX conditional candidates;
and literal Angular HTML binding attributes. Dynamic dispatch and unsupported forms remain unknown.
Restricted findings, arbitrary source bodies and source literals are never copied into prose.

Check compares each original claim/paragraph's span hash using its logical identity. Unchanged spans
remain fresh after line insertion. A flow also binds its static call inventory, so adding, removing
or changing a call in its body is stale even when the declaration line is unchanged. A manifest
from generator version 1 or 2 (before the V1-0461 fixes) is refused as `flowdocs-generator-stale`; regenerate it. Changed spans become stale; missing/ambiguous identities are
unresolved and retired, without guessing renames. It also lists added identities. Optional output
checking compares the original generation's bytes, including unexpected files. Neither check nor
finalize writes files. Check exits 0 without drift, 1 for drift and 2 for invalid input. `--previous`
on generate validates the supplied predecessor; use check to obtain the explicit transition delta.

## Use your own flow inventory

By default every supported declaration in scope becomes its own flow. A project that keeps a
reviewed flow list can pass it with `--flows FILE` instead:

```json
{"schema": "corvint-flow-inventory/1", "flows": [
  {"id": "checkout", "title": "Checkout",
   "members": [{"path": "app/controllers/checkout_controller.rb"},
               {"path": "app/services/charge.rb", "symbol": "Charge#call"}],
   "entry_points": [{"path": "app/controllers/checkout_controller.rb"}]}]}
```

Each declared flow becomes one flow, titled as declared, that aggregates its members' calls, claims
and corpus context. A member without `symbol` takes every declaration in its path; `symbol` is the
declaration name generate reports (for example `Charge#call`). Entry points must be members.
Members that do not resolve at the revision are listed with a reason in `generation.json`
(`inventory.flows[].unresolved`) and in the flow README; a declared flow with no resolved member is
reported but gets no pages. In-scope declarations that no declared flow claims are listed in
`inventory.unassigned` and are not turned into flows. Members must lie inside `--scope`. The
inventory is kept in `generation.json`, so `check` rederives the same identities and reports each
flow, paragraph and claim fresh, stale or unresolved; a removed member's claims are unresolved and
retired. Malformed, duplicate or overlapping inventories are refused. Contract: `AFU-V1-101..106`.

## Admit the provider to the corpus

Generate at immutable source commit A. Explicitly retain `provider.json` in the repository and commit
it at B under the project's normal approval policy. Do not rewrite its source anchors to B.

```sh
corvint --root /path/to/repo docs flows finalize \
  --previous /existing/parent/fresh-output/generation.json \
  --provider-revision FULL_PROVIDER_SHA --provider-path evidence/flowdocs-provider.json \
  > /existing/parent/corpus-manifest.json
corvint --root /path/to/repo docs corpus build --manifest /existing/parent/corpus-manifest.json
```

Finalization checks the provider's exact committed bytes and compiles the returned manifest before
returning it. The source stays at A; provider input and its scope use B. Generated paragraph IDs and
claim kinds use the existing corpus adoption contract. No source or provider commit is automatic. Opaque gitlink path/commit identities remain explicit
unknown module records with nested contents outside coverage. Finalize scopes each regular
superproject source file separately so a gitlink is never submitted as a missing blob. The generic
`docs corpus manifest` scope rules remain unchanged; use flow finalization for this profile.
Gitlink identity changes appear separately in `opaque_changes` and make check fail closed.

## Original journeys, tests and accepted variations

Supply `--corpus /path/to/validated-corpus.json` to import original evidence context. The artifact must
pass `doccorpus.Open` against this repository and have the same source repository/revision as generation.
Supported shape: the documented repository at `--revision` is the corpus's primary repository. It may
declare further `additional_repositories`, whose roots resolve against `--root`. A corpus whose primary
is another repository is refused up front as `flowdocs-corpus-repository-mismatch`, which names both
repositories. This includes a multi-repository corpus that declares the documented repository only as
an additional repository. Build the corpus with the documented repository as its primary.
The artifact retains its original manifest, records and anchors in the generation manifest. Finalize
preserves that manifest and adds missing source inputs and the new provider. A corpus already owning
a `flowdocs` provider must be regenerated from the caller's original input manifest first.

For an AFU/Playwright join, first ingest the actual original provider/retained runtime receipt through
its existing corpus route. An adoption subject with a source anchor overlapping a generated flow's
source span can locate original `Journey.Subject` records and `asserts`/`navigates` relations. Keep
`TestLinkDetails.Role`, `JoinConfidence`, target test identity/file, ordered step evidence and coverage
`Denominator`/`Numerator`/`Rule` unchanged. The generator displays those original records and explicitly
labels source-overlap locating as semantically unresolved. A runtime result alone cannot supply a
missing subject/test/variation join. Existing `/3` native Playwright receipt ingestion and validation
still govern actual observed evidence; never construct an observation from this generator's source
claims. Unjoined flows retain unknown runtime coverage and an unknown accepted variation denominator.

Only corpus-admitted security severity and total summaries are rendered. Restricted detail inputs,
paths and bodies remain in the separate local-only input family. Historical ticket context and intent
comparisons retain their original evidence meaning; no generated text becomes accepted intent.
