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
JS/TS named functions/arrows, literal route/Angular registrations; React JSX conditional candidates;
and literal Angular HTML binding attributes. Dynamic dispatch and unsupported forms remain unknown.
Restricted findings, arbitrary source bodies and source literals are never copied into prose.

Check compares each original claim/paragraph's span hash using its logical identity. Unchanged spans
remain fresh after line insertion. Changed spans become stale; missing/ambiguous identities are
unresolved and retired, without guessing renames. It also lists added identities. Optional output
checking compares the original generation's bytes, including unexpected files. Neither check nor
finalize writes files. Check exits 0 without drift, 1 for drift and 2 for invalid input. `--previous`
on generate validates the supplied predecessor; use check to obtain the explicit transition delta.

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
