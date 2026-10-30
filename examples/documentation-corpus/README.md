# Independent flow provider example

`flow-provider.py` is an example adopter adapter, outside Corvint Core. It maps explicitly authored
screens/routes/ordered flows to generic `ui_surface` subjects, claims and journey records. It imports
only Python's standard library. It never discovers routes, infers click paths, executes tests or
marks a journey verified. Missing steps remain `missing_journey`.

Input is one JSON object with `provider`, `repository: {id, revision}`, `screens` and `flows`.
Each screen has `id`, `name`, `route`, and already collected `anchors`. Each flow has `id`, `screen`,
`anchors`, `preconditions`, `cleanup`, and explicitly supplied `steps`; a step has `id`, `action`,
`operation`, `expected`, `anchors` and optional `observation`. Anchors use the corpus contract and
retain `external-provider` authority. No credentials or network configuration are needed.

The source commit comes first. Run the adapter with authored JSON on stdin; commit its output as a
provider artifact afterward. Declare that later commit, the artifact blob/digest, and the earlier
source inputs in the corpus manifest. The compiler refuses undeclared/missing inputs or mismatched
versions. This separate-revision sequence avoids embedding a commit's own ID inside its contents.
`TestCorpusIndependentFlowAdapter` executes this independent adapter, compiles its result and queries
the resulting journey; `TestCorpusIndependentProviderAndNativeObservation` covers separately pinned
retained run artifacts. Synthetic fixtures are explicitly labelled and establish no external utility.

The governing proposed contract is `docs/specs/documentation-corpus-v1.md`. The native compiler,
renderer and MCP reader share one artifact; an adopter does not need another database or MCP server.
