# GitHub #805: docs corpus help topics and the unknown-topic list

## Finding

`corvint help docs corpus` was refused as `unknown help topic`, and `corvint docs corpus OP --help`
printed the generic `docs` orientation for every operation. The help scan mapped every `docs`
invocation to the `docs` topic unless the subcommand was `flows` or `maintain`. The corpus parser
kept its admitted and required options in local maps, so no help could be derived from them. An
unknown help topic named no valid topic.

## Change

- One operation table (`cmd/corvint/docs_corpus_help.go`) now drives both `parseCorpusInvocation`
  and the corpus help topics. Each entry holds the operation's purpose and its required and optional
  options. The admitted and required checks, refusal messages and their order are unchanged.
  `corpusInputPathError` iterates the shared `corpusLocalInputFlags` list.
- `help docs corpus` and `docs corpus --help` list every operation with its purpose.
  `docs corpus OP --help` prints the usage, the options and a `Relative to --root:` line for the
  `DCP-V1-054` inputs that operation admits. The Playwright operations also print the shared
  `DCP-V1-053` receipt guidance, now the `playwrightReceiptHelp` constant, which `docs --help` still
  includes. An unknown operation keeps the parser refusal.
- `help TOPIC` with an unknown topic keeps `invalid-arguments` and exit 2. Its message now follows
  the argparse invalid-choice convention already used by `invalidChoice`. It lists every
  `topLevelCommands` verb, then the nested topics `harness event`, `docs corpus`, `docs flows` and
  `docs maintain`. `help docs flows` and `help docs maintain` now resolve as well. The message text
  changed, but the envelope and its members did not. The pinned
  `TestUnknownHelpTopicIsConventionalError` was updated. No `conformance/cli-parity-v0` fixture pins
  the message.

Spec: `DCP-V1-056` and `DCP-V1-057` are proposed, and `DCP-V1-053` is amended to name the per-operation topics.

## Evidence

Before the change, `TestDocsCorpusHelpListsOperations`, `TestDocsCorpusOperationHelpPrintsFlags` and
`TestUnknownHelpTopicListsValidTopics` failed against a0e56f3c1. After it, they pass with the
existing help, corpus and Playwright CLI tests.
