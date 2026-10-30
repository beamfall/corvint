# Documentation corpus estate inventory V1

Owner: Russell Lewis
Date: 2026-10-29
Intent status: proposed
Delivery status: experimental

Authoritative inputs: native tickets V1-3101, V1-3102 and V1-3103 and GitHub issues
[#1998](https://github.com/beamfall/corvint/issues/1998),
[#1999](https://github.com/beamfall/corvint/issues/1999) and
[#2000](https://github.com/beamfall/corvint/issues/2000), with V1-3112 to V1-3115 and GitHub #2006 to #2009
for `DCI-V1-020..026`, and V1-3162 and GitHub #2021 for `DCI-V1-027..030`. This companion extends the repository
inventory of the [documentation corpus contract](documentation-corpus-v1.md) (`DCP-V1-700..706`),
whose Requirements section is at the 256-obligation OCM limit (`maxObligations`). The parent's
input and authority boundary, failure codes and acceptance gates govern these requirements.

## Agent digest
- Claim: An estate receipt joins corpora by exact name, and inventory rules derive packages and deploys only from evidence, such as a publish step tied to its manifest.
- Status: proposed; experimental; implemented with focused tests.
- Exists: `docs corpus estate`, the estate receipt, the named shallow-checkout refusal, the kept empty inventory declaration and the derivation-11 inventory rules (publish ties, name-derived environments, CircleCI jobs, over-bound inputs as gaps, deploy-word matching, declared environment words and deploy jobs), the IDF-mass weak search signal and the kept checkout refusal reason, each with focused tests.
- Blocked on: owner acceptance and independent external utility qualification, as for the parent contract.
- Read next: Requirements; Failure modes; Traceability.

## User and current state

The repository inventory (`DCP-V1-700..706`) lists and reconciles repositories inside one corpus,
and one corpus pins at most eight revisions: the primary and seven additional repositories
(`DCP-V1-175`). That bound keeps one build's checkouts, identity checks and Git sessions bounded,
and it stays. An estate of dozens of repositories is therefore split over several corpora, and a
library in one corpus consumed by a service in another stayed an `outside-corpus-reference` gap in
the service's corpus, so the result depended on how the repositories were grouped. A shallow
further checkout refused the whole manifest without naming the checkout.

## Requirements

### Estate receipt across corpora (GitHub #1998)

Status for `DCI-V1-001..005`: proposed, experimental; GitHub #1998.

- `DCI-V1-001`: `docs corpus estate --input ESTATE.json` reads one closed
  `corvint-corpus-estate/1` object, `{"schema", "corpora": [{"root", "artifact"}]}`, with 1 to 64
  corpora, decoded as one whole document: trailing data after it and a member repeated at any
  level are refused, like an unknown member. `root` is the primary checkout the corpus was built from, a clean relative slash path
  from `--root` that may climb with `../` (`.` for `--root` itself); `artifact` is a canonical
  relative path inside that checkout. The corpora are sorted by `root/artifact`, and a repeated
  corpus, an absolute or unclean root, an unknown member or more than 64 corpora is refused
  (`corpus-refused`, or `corpus-capacity-exceeded` for the count). The per-corpus bound of
  `DCP-V1-175` is unchanged: the estate opens one verified artifact at a time and keeps only its
  inventory records.
- `DCI-V1-002`: Each corpus is admitted by the verified read (`DCR-V1-410`) at its own root, so
  its additional repositories resolve as in any read. A refused corpus refuses the estate with its
  own code and the message prefixed `estate corpus ROOT/ARTIFACT: `. The receipt
  `corvint-corpus-estate-receipt/1` lists `corpora` (label, `manifest_sha256`, primary repository
  and revision, and whether it declares `repository_inventory`), `repositories` (each covered
  repository once by root-commit identity, with the sorted union of its names, workspace and
  declared aliases, pinned revisions and covering corpora), `references`, `unknown` and
  `limitations`. A corpus without `repository_inventory` is listed with an `unknown` entry naming
  it, its packages and references NOT_OBSERVED. The receipt is query-time output, never an input
  to any corpus, and is the same for the same artifacts in any order.
- `DCI-V1-003`: Every reference a corpus leaves as an `outside-corpus-reference` gap is one
  receipt reference, with its corpus, claim, owner, ecosystem and name recovered from the
  reference claim (`DCP-V1-704`, `DCP-V1-706`); one whose claim is not in that form is an
  `unknown` entry instead, never dropped. A gap is matched to its claim by fields, its subject
  being the claim's owner and its reason starting with the claim's ID, `: `, its text and `; `,
  never by splitting the reason at a separator an admitted path may contain; a gap that matches
  no reference claim is an `unknown` entry naming its corpus, kind and subject. It is joined by exact name, under the normalization of
  `DCP-V1-704` (an image by its repository and an action by its alias), to the declared packages
  and charts (`package-json/*`, `go-mod/*`, `helm-chart/*` records) and declared aliases of every
  inventory corpus of the estate. With one covered repository among the targets it is `resolved`
  and lists the targets; with several it is `ambiguous` and lists them without choosing; with none
  it is `outside-estate` with a NOT_OBSERVED reason. The targets of all references together are
  bounded at 16384, counted before any reference is expanded; a larger total refuses the estate
  (`corpus-capacity-exceeded`) with the message prefixed `estate corpus ROOT/ARTIFACT reference
  CLAIM: `, naming the reference that crossed the bound. A join is an exact-name fact about declared
  inputs at each corpus's pinned revisions; version compatibility and run-time use stay unknown,
  and the receipt's limitations say so.
- `DCI-V1-004`: A further repository's checkout that cannot establish its corpus identity, such
  as a shallow or grafted clone, is refused at `docs corpus manifest` with the native code and the
  message prefixed `repository ALIAS: `, and at build with `additional repository ID: `, so the
  operator can tell which checkout to repair. The primary's refusal is unchanged. A refusal, not a
  per-repository gap, keeps `DCP-V1-003` and the root-commit identity of `DCP-V1-175` intact: a
  manifest never pins a repository whose identity it could not verify.
- `DCI-V1-005`: An empty `repository_inventory: {}` declaration is kept in the encoded manifest.
  It was dropped by the encoder while the build still derived its inventory records, so the
  artifact's embedded manifest did not declare what it held and the estate could not tell an
  inventory corpus from one without inventory. Manifests whose declaration is absent or non-empty
  keep their bytes.

### Inventory rules that need evidence (GitHub #1999)

Status for `DCI-V1-006..010`: proposed, experimental; GitHub #1999. These change the records an
inventory build derives, so they are derivation `9` (`DCX-V1-471`); a manifest without
`repository_inventory` derives the same records as before.

- `DCI-V1-006`: `package-json/2` replaces `package-json/1`. A string `name` without
  `private: true` yields a `package` only with publish evidence: a `publishConfig` object member,
  or a release or publish job (`DCI-V1-009`) in the same repository. Its derivation claim names
  that evidence and its line. Without it the record is a `package_manifest`, a new closed subject
  kind, with the same ID and anchor, whose claim states that publish evidence is NOT_OBSERVED. A
  manifest still joins references by exact name (`DCP-V1-704`), and its file still counts as
  package evidence (`DCP-V1-705`). `go.mod` is unchanged. `DCI-V1-021` narrows the job evidence
  to the manifest a job's publish step publishes.
- `DCI-V1-007`: `gemspec/1` applies to `*.gemspec`. Its first single-line `NAME.name =`, when
  its whole value is one literal string with no interpolation (an optional trailing comment
  aside), yields a gem `package`, `NATIVE:package:[ALIAS:]PATH`, because a gemspec exists to build a gem,
  and each `add_dependency` or `add_runtime_dependency` (not `add_development_dependency`) is a
  `gem` reference when its first argument is such a literal. A name, dependency or Gemfile gem
  whose argument is an expression or interpolated string (`"a" + "b"`, `"x#{v}"`, `"a".freeze`)
  derives nothing, not even its literal prefix: its lines are named in one
  `inventory-evidence-unreadable` gap for the file, and a gemspec without a literal name yields no
  gem. `gemfile/1` applies to `Gemfile`: each `gem "NAME"` line is a `gem` reference on the
  repository. A `gem` reference joins a gem package by exact name. A gemspec counts as package
  evidence; a Gemfile does not.
- `DCI-V1-008`: A declared native input that is another CI system's configuration
  (`.gitlab-ci.yml`, `.circleci/config.yml`, `Jenkinsfile`, `azure-pipelines.yml`, `.travis.yml`,
  `bitbucket-pipelines.yml`, `.buildkite/pipeline.yml`, `.drone.yml`, `.woodpecker.yml`,
  `buildspec.yml`, `cloudbuild.yaml`) is an `inventory-ci-unsupported` gap on its file subject,
  naming the format, with its jobs, deployments and environments NOT_OBSERVED. The repository's
  `deployable-evidence-unknown` reason then says so. No record is derived from it.
  `DCI-V1-023` reads `.circleci/config.yml` instead.
- `DCI-V1-009`: `github-workflow-job/2` replaces `github-workflow-job/1`. Only a job key
  containing `deploy` yields a `deployable_unit`. When the job's block names a literal
  `environment: NAME` or `environment:` with `name: NAME` (`[A-Za-z0-9][A-Za-z0-9._/-]{0,63}`),
  the unit is named `JOB (NAME)` and carries a second anchor at the name line; otherwise it is
  named `JOB` and its claim states that the environment is NOT_OBSERVED, as for an expression.
  A job containing `release` or `publish` but not `deploy` is no deployable unit: it is a claim
  `NATIVE:repository:ROOT_ID:publish-job:[ALIAS:]PATH:JOB` on the repository, anchored at its key,
  and the publish evidence of `DCI-V1-006`. `DCI-V1-021` and `DCI-V1-022` amend the publish
  evidence and the environment of a job without an environment key.
- `DCI-V1-010`: `repository_inventory` accepts `first_party`, 1 to 64 sorted, distinct name
  prefixes matching `[A-Za-z0-9@][A-Za-z0-9._/@:-]{0,127}` (an invalid list is refused with
  `invalid repository inventory declaration: first_party`). An unjoined reference whose
  normalized name (`DCP-V1-704`) starts with a declared prefix is a
  `first-party-reference-outside-corpus` gap naming the longest such prefix, instead of an
  `outside-corpus-reference`. Another unjoined reference stays `outside-corpus-reference`:
  without a declared prefix, ownership is NOT_OBSERVED, so it is ungraded, never asserted
  third-party. The estate receipt reads both gap kinds and gives each reference `grade`
  `first-party` or `ungraded`, and treats gem packages and package manifests as declared names.

### Input and evidence bounds (GitHub #2000)

Status for `DCI-V1-011..012`: proposed, experimental; GitHub #2000; V1-3103. They amend parent
requirements and live here because the parent's Requirements section is at the 256-obligation
OCM limit.

- `DCI-V1-011`: Amending `DCP-V1-156` and `DCP-V1-157`; its refusal is superseded by `DCI-V1-024`,
  which pins over-bound inputs as gaps, and its header-size reporting stays. `docs corpus manifest`, single-root or workspace, refuses
  a scope holding any input over the 4 MiB per-input bound with `corpus-refused` and `input bound
  exceeded (N inputs; narrow the scope to exclude them): repository=<label> path=<path>
  revision=<rev> bytes=<size> bound=4194304; ...`, naming every such input in manifest order
  before any body is read. A blob over the 64 MiB Git read bound is one of them: the header batch
  now reports its size instead of refusing the whole batch as an unnamed `internal-error`, and
  the body batch still refuses to read it. As with generated output (`DCP-V1-608`), dropping the
  input would leave it undeclared in its scope (`DCP-V1-003`), so the remedy is a narrower scope,
  not a per-path exclusion. Non-goals: an exclude list per scope, and raising either bound.
  Failure modes: a malformed or negative header still refuses the batch as unavailable; build of
  a hand-written manifest naming an over-bound input keeps the unnamed per-input refusal.
  Rollback: restore the header refusal; nothing is stored. Acceptance:
  `TestDCIV1024OverBoundInputsAreDeclaredGaps` (`internal/doccorpus`), which replaces
  `TestDCIV1011ManifestNamesEveryOverBoundInput`; that test failed before the change with
  `internal-error`.
- `DCI-V1-012`: Wire contract, specifying the existing bound. Every evidence object in a provider record or
  adoption record holds at most 64 anchors and 64 limitations, and more refuse the build with
  `corpus-refused` and `evidence bound exceeded`. A record that cites more files declares one
  relation per further file instead, for example one `implements` relation from each module
  subject to the flow it serves, each with its own anchors; module ownership
  (`DCP-V1-720..724`) counts a module joined by such a relation as owned. The bound keeps every
  normalized record within the 1 MiB bounded-reader limit. Non-goals: raising the bound or a
  per-kind bound. Rollback: none; the text documents existing behavior. Acceptance:
  `TestDCIV1012EvidenceAnchorBound` (`internal/doccorpus`) and the guide section in
  `docs/DOCUMENTATION-CORPUS.md`.

### Search signal, CI inventory, input bounds and checkout refusals (GitHub #2006 to #2009)

Status for `DCI-V1-020..026`: proposed, experimental; GitHub #2006, #2007, #2008 and #2009;
V1-3112 to V1-3115. `DCI-V1-021..023` change the records an inventory build derives, so with
`DCI-V1-024` they are derivation `10` (`DCI-V1-026`). `DCI-V1-020` and `DCI-V1-025` change query
output and a refusal message only.

- `DCI-V1-020`: Amending `DCP-V1-761`. A search whose query has an absent word is still marked
  `search_quality` `weak` when a rare word (IDF above `ln 2`) matched, if the absent words carry
  more of the IDF mass of the query's distinct words than the matched words do. Each distinct word
  counts once, whatever its spelling, and an absent word has `df=0`. Such a signal has reason
  `unmatched-terms-carry-most-idf-mass` and `unmatched_idf_share`, the absent words' share of the
  mass rounded to 4 places. A query with no matched rare word keeps reason
  `all-rare-query-terms-unmatched` and no share. The schema stays
  `corvint-corpus-search-quality/1`: the reason value and the optional member are additive, and
  results are never dropped or reordered. Rejected alternative: weak whenever the highest-IDF word
  is absent, which one rare stray word decides. Failure mode: a few generic words, rare only
  because the corpus is large, no longer hide that the query's specific words are absent. A
  query whose matched words carry the mass stays unmarked. Rollback: restore the
  any-rare-match rule. Acceptance: `TestDCIV1020WeakSignalAtRealisticSize`.
- `DCI-V1-021`: Amending `DCI-V1-006` and `DCI-V1-009`. `package-json/3` and
  `github-workflow-job/3` replace their `/2` rules. A release or publish job is publish evidence
  only for the manifest directory one of its executed npm, pnpm or yarn publish commands
  publishes. Only a `run` script command counts: a step `name`, an `env` value, a comment, an
  `echo` or `printf` argument and a `--dry-run` publish nothing. The workflow file is read as a
  block YAML tree, so key order changes no result. The directory is the step's
  `working-directory`, else the job's `defaults.run.working-directory`, else the workflow's, else
  the repository root; each replaces the next and is relative to the root, never joined. A plain
  `cd DIR` or `pushd DIR` before the command (at a line start, after `;` or after `&&`) moves it,
  and a quoted or bare `.`, `./DIR` or `../DIR` folder argument resolves from it. The manifest's
  derivation claim names the job, its key line and the publish line. A command is opaque and ties
  nothing, not even the working directory's manifest, when: its folder is absolute, `~`, an
  expression or variable, a bare word, a tarball, or leaves the repository; a workspace selector
  or an option before the subcommand chooses its package (`yarn workspace`, `--workspaces`,
  `-w`, `--filter`, `-r`, `--prefix`); an unknown option precedes a folder; it has several
  folders; its tool or subcommand is expanded or another command wraps it (`npm run publish`,
  `env npm publish`); a `cd` is conditional, structured, piped or expanded; the script uses a
  heredoc or cannot be tokenized; the step's shell or the job's runner is not POSIX (`pwsh`,
  `windows-*`); no `actions/checkout` step precedes it, or that checkout sets `path` or
  `repository`; or the YAML uses aliases, merge keys, tags, flow collections, tabs in
  indentation, duplicate keys, several documents or a folded script naming publish. A manifest
  no job ties stays a `package_manifest`; its claim says no release or publish job ties its
  directory, names every untied or opaque release or publish job of its repository as a candidate
  with publish evidence NOT_OBSERVED, and names each observed job tied to other directories. The
  job's own claim names the directories it publishes, or says it is a candidate only. Failure
  mode: a nightly release job that publishes nothing no longer makes every non-private manifest a
  package. Rollback: revert to the `/2` rules and derivation `9`. Acceptance:
  `TestDCIV1021PublishJobIsTiedToTheManifestItPublishes`, `TestDCIV1021OnlyExecutedPublishCommandsTie`,
  `TestDCIV1021OpaqueFolderArgumentsTieNothing`, `TestDCIV1021StepWorkingDirectoryReplacesTheJobDefault`,
  `TestDCIV1021KeyOrderChangesNoTie` and `TestDCIV1021AdversarialPublishSteps`.
- `DCI-V1-022`: Amending `DCI-V1-009`. A deploy job with no `environment:` key takes an
  environment from its job key, else its workflow's top-level `name:`, else its file name, when
  that text names exactly one word of a closed vocabulary: `production`, `prod`,
  `preproduction`, `preprod`, `preprd`, `staging`, `stg`, `uat`, `qa`, `sandbox`, `development`
  or `dev`, with `pre-prod` read as one word. The word is kept as written. The unit is named
  `JOB (WORD)`. Its claim says the environment is inferred from that source, labelled
  name-derived and weaker evidence, and the subject carries the limitation that no environment
  key declared it. A workflow name source adds an anchor at its line. An environment key, even an
  expression, infers nothing. Text naming no word or several words leaves the environment
  NOT_OBSERVED and says so, and so does an unreadable top-level `name:`. Ambiguity at any level
  stops the inference: a later level is not read. Failure mode: a workflow named for two
  environments infers neither, even from its file name. Rollback: as `DCI-V1-021`. Acceptance:
  `TestDCIV1022DeployEnvironmentFromItsName` and `TestDCIV1022AmbiguityStopsAndTheWordIsKept`.
- `DCI-V1-023`: Amending `DCI-V1-008`. `.circleci/config.yml` is read by `circleci-job/1`
  instead of being an `inventory-ci-unsupported` gap. Its `jobs:` keys follow the deploy and
  publish rules of `DCI-V1-009`, `DCI-V1-021` and `DCI-V1-022`, with `working_directory` as the
  directory key. A CircleCI `environment:` holds variables, so it never names an environment, and
  a `~/...` or absolute step `working_directory` is opaque. The job's `working_directory` is
  where `checkout` places the repository, so it is the root step directories resolve from. A
  publish ties a manifest only after a `checkout` step that sets no `path`, and only when the job's
  `shell` is POSIX or, unset, its `executor` and `machine` name no Windows image. The repository's `deployable-evidence-unknown` reason
  names GitHub workflow or CircleCI deploy jobs. The other CI files of `DCI-V1-008` stay
  unsupported gaps. A manifest-declared deploy mapping was a non-goal here; `DCI-V1-029` now
  proposes it as its own requirement. Rollback: as `DCI-V1-021`. Acceptance:
  `TestDCIV1023CircleCIJobs` and `TestDCIV1023CircleCIDirectoriesAndCheckout`.
- `DCI-V1-024`: Amending `DCI-V1-011`. `docs corpus manifest`, single-root or workspace, no
  longer refuses a scope holding inputs over the 4 MiB per-input bound. Each such tracked file is
  pinned in `over_bound_inputs` (`path`, `revision`, `blob`, `bytes` and `provider`) from its Git
  header only, never read. A blob over the 64 MiB Git read bound is included. The build reports
  each one as an `input-over-bound` gap on its file subject, naming its size, the bound and its
  blob, and the rest of the scope builds. The member is omitted when empty, so other manifests
  keep their bytes. At build, an entry must be in a declared native scope, unique, not also an
  input, still the pinned blob and size, and still over the bound. A shrunk, changed, missing,
  forged or out-of-scope entry refuses the build, and so does an entry for a file within the
  bound. A scope whose every file is over the bound inventories with no read input and builds
  with only its gaps. A freshness read notes an over-bound input
  whose blob changed or is absent. Failure modes: more entries than the record limit refuse with
  `corpus-capacity-exceeded`. A malformed header still refuses the batch as unavailable.
  Rollback: restore the `DCI-V1-011` refusal. A manifest with `over_bound_inputs` is then refused
  as an unknown member and must be inventoried again. Acceptance:
  `TestDCIV1024OverBoundInputsAreDeclaredGaps`, which did not compile before the change, and
  `TestDCIV1024OnlyOverBoundScopeBuildsWithGaps`.
- `DCI-V1-025`: Amending `DCI-V1-004`. A further checkout that `gitauth` refuses keeps that
  refusal's code and message after the operator prefix. For example, a `git clone --shared`
  checkout reads `additional repository ID: unsupported-object-alternates: ...` with the repack
  or re-clone fix, where it previously read only `checkout unavailable`, at manifest and at
  build. The outer code stays `corpus-input-unavailable`. Rollback: drop the kept reason.
  Acceptance: `TestDCIV1025CheckoutRefusalKeepsGitauthReason`.
- `DCI-V1-026`: Derivation `10` (`DCX-V1-471`) names `DCI-V1-021..024`: an artifact of
  derivation 9 or earlier is refused by the verified read until rebuilt. `docs/RELEASE-NOTES.md`
  lists it. Acceptance: `TestDCXV1471DerivationVersionGuard` and
  `TestDCXV1564ReleaseNotesListEveryDerivation`.

### Deploy words and declared environments (GitHub #2021)

Status for `DCI-V1-027..030`: proposed, experimental; GitHub #2021; V1-3162. `DCI-V1-027..029`
change the records an inventory build derives, so they are derivation `11` (`DCI-V1-030`).

- `DCI-V1-027`: Amending `DCI-V1-009` and `DCI-V1-022`. `prd`, `sbx` and `eph` join the closed
  environment vocabulary, kept as written like every other word, so `deploy-prd` is
  `deploy-prd (prd)`. A GitHub workflow or CircleCI job is a deploy job only when one of its key's
  words is `deploy` in any case, the words split at `-`, `_` and a lower-to-upper case change
  (`deploy`, `deploy-prd`, `deploy_sbx`, `prd-deploy`, `build-and-deploy-prod`, `deployProd`),
  replacing the substring match. A `deploy` word whose preceding words end with `can` or `can i`
  names a can-i-deploy contract check, which deploys nothing, so `pact_can_deploy`,
  `pact_can_deploy_sbx` and `can-i-deploy` are not deployable units; nor are `deployment`,
  `pact_record_deployment_staging` or `redeploy`, which hold no `deploy` word. A job that is no
  longer a deploy job is read by the release and publish rule when its key names one. Rejected
  alternative: reading the job's steps for a deploy command, which needs a closed list of deploy
  tools the inventory does not have. Failure mode: a deploy job named only inside another word,
  such as `redeploy`, is no longer a deployable unit; declare it with `DCI-V1-029`. Rollback: as
  `DCI-V1-030`. Acceptance: `TestDCIV1027DeployWordsAndShortEnvironments`.
- `DCI-V1-028`: Amending `DCI-V1-022`. `repository_inventory.environment_words`, when present,
  is a sorted, distinct, non-empty list of at most 64 words, each 1 to 32 lower-case letters or
  digits; anything else refuses the manifest. Each word is read like a built-in vocabulary word,
  with the same ambiguity rule, and the environment it names stays name-derived, weaker evidence
  labelled as inferred. No word is mapped to another, so a declared word cannot rename `prd` to
  `production`; `DCI-V1-029` declares a job's environment instead. Absent, the vocabulary is the
  built-in one and the manifest keeps its bytes. Rollback: as `DCI-V1-030`; a manifest holding the
  member is then refused as an unknown member. Acceptance: `TestDCIV1028DeclaredEnvironmentWords`.
- `DCI-V1-029`: Amending `DCI-V1-009`, `DCI-V1-022` and `DCI-V1-023`.
  `repository_inventory.deploy_jobs`, when present, is a non-empty list of at most 256
  `{repository, path, job, environment}` declarations, sorted by repository, path and job as
  written and distinct after alias resolution. `repository` is a declared root-commit identity or
  workspace alias, `path` a `.github/workflows/*.yml`/`.yaml` file or `.circleci/config.yml`, and
  `job` and `environment` 1 to 128 letters, digits, `-` or `_`; anything else refuses the
  manifest. The project owns this declaration, so it outranks the job's name and environment key
  (product invariant 3): the named job is a deployable unit whatever its name, named
  `JOB (ENVIRONMENT)`, and its claim says the environment is declared by
  `repository_inventory deploy_jobs`. When the job's own GitHub environment key names another
  environment, or no literal one, the claim names that too, so the conflict stays visible. The
  job's anchor is still its key line; the declaration grants no deployment, runtime or hosting
  state. A declaration whose job no read workflow or CircleCI input of that repository holds
  derives nothing and is a `deploy-declaration-unmatched` gap on the repository subject.
  Non-goal: reading CircleCI workflow branch filters as environment evidence. Rollback: as
  `DCI-V1-028`. Acceptance: `TestDCIV1029DeclaredDeployJob`.
- `DCI-V1-030`: Derivation `11` (`DCX-V1-471`) names `DCI-V1-027..029`: an artifact of
  derivation 10 or earlier is refused by the verified read until rebuilt. `docs/RELEASE-NOTES.md`
  lists it. Acceptance: `TestDCXV1471DerivationVersionGuard` and
  `TestDCXV1564ReleaseNotesListEveryDerivation`.

## Non-goals

- Raising the eight-revision bound of `DCP-V1-175`, or a higher bound for inventory-only
  repositories: an inventory-only repository is still a pinned revision with its own identity,
  index and Git session, and the estate receipt reconciles any number of corpora without one.
- Writing estate joins back into any corpus, or treating an estate join as evidence of deployment.
- A per-repository `inventory-evidence-unknown` for an unverifiable checkout; see `DCI-V1-004`.
- Reading CI systems other than GitHub workflows and CircleCI (`DCI-V1-023`), CircleCI branch
  filters as environment evidence (`DCI-V1-029`), registry lookups, and publish evidence for Go modules, which
  are published by their repository's tags.
- Following shell scripts, Make targets or package scripts that a CI step runs to find a publish
  command (`DCI-V1-021`).

## Failure modes

- A corpus refused by the verified read refuses the estate, naming the corpus (`DCI-V1-002`).
- Two repositories declaring one name never produce a chosen join (`DCI-V1-003`).
- An estate document with trailing data or a repeated member is refused, never read
  last-value-wins (`DCI-V1-001`).
- Repeated names cannot grow the receipt multiplicatively: the target budget refuses by name
  before any expansion (`DCI-V1-003`).
- An admitted path containing `: consumes ` cannot hide a reference; an unmatched gap is an
  `unknown` entry (`DCI-V1-003`).
- A Ruby expression never yields its literal prefix as a name (`DCI-V1-007`).
- A corpus built from another revision than its checkout's is refused by the verified read, as for
  any read.
- A release or publish job made every non-private `package.json` of its repository a `package`,
  including one it does not publish. This was fixed by `DCI-V1-021`: only a literal publish step
  ties a manifest. A publish hidden behind a script stays a NOT_OBSERVED candidate.
- A deploy job's environment inferred from its name is weaker evidence than an environment key.
  It is labelled as inferred, never presented as declared (`DCI-V1-022`).
- A job key holding `deploy` inside another word, such as `pact_can_deploy` or
  `record_deployment`, made a deployable unit. This was fixed by `DCI-V1-027`. A deploy job its
  name does not reveal is declared, never guessed (`DCI-V1-029`).
- A declared deploy job no read input holds stays a `deploy-declaration-unmatched` gap, never a
  silent unit (`DCI-V1-029`).
- An over-bound input is pinned without its content: what it documents is NOT_OBSERVED, and
  its gap says so (`DCI-V1-024`).
- A derivation-9 or earlier inventory artifact is refused by the verified read until rebuilt
  (`DCI-V1-026`).

## Acceptance evidence

`TestDCIV1001DCIV1002DCIV1003DCIV1005EstateReconcilesAcrossCorpora`,
`TestDCIV1001DocsCorpusEstateCommand`, `TestDCIV1001EstateTargetBudgetAndStrictFile`,
`TestDCIV1003EstateGapsAreMatchedNotSplit`, `TestDCIV1007RubyExpressionsAreNoNames` and `TestDCIV1004ShallowRepositoryRefusalNamesIt`, with the
flag-matrix and help tests for the new operation;
`TestDCIV1006ToDCIV1010InventoryRulesNeedEvidence`, `TestDCIV1024OverBoundInputsAreDeclaredGaps`
(replacing `TestDCIV1011ManifestNamesEveryOverBoundInput`), `TestDCIV1012EvidenceAnchorBound`,
`TestDCIV1020WeakSignalAtRealisticSize`, `TestDCIV1021PublishJobIsTiedToTheManifestItPublishes`,
`TestDCIV1022DeployEnvironmentFromItsName`, `TestDCIV1023CircleCIJobs`,
`TestDCIV1025CheckoutRefusalKeepsGitauthReason`, `TestDCIV1027DeployWordsAndShortEnvironments`,
`TestDCIV1028DeclaredEnvironmentWords`, `TestDCIV1029DeclaredDeployJob`,
`TestDCXV1471DerivationVersionGuard` and `TestDCXV1564ReleaseNotesListEveryDerivation`. Each of `TestDCIV1020` to `TestDCIV1025` failed, or
did not compile, at base `0a04f081337c`. Each of `TestDCIV1027` to `TestDCIV1029` fails, or does not compile, at base
`b7a631909d4c`.

## Rollback

Revert the implementing change. The estate operation and receipt are additive; the empty
declaration fix returns to dropping `{}`. Reverting `DCI-V1-006..010` returns derivation `8`,
whose artifacts must then be rebuilt again. Reverting `DCI-V1-021..024` returns derivation `9`.
Manifests holding `over_bound_inputs` must then be inventoried again. Reverting `DCI-V1-027..030`
returns derivation `10`; manifests holding `environment_words` or `deploy_jobs` must then drop
them. `DCI-V1-020` and `DCI-V1-025` revert with no stored state.

## Traceability

| Requirements | Implementation boundary | Required evidence |
| --- | --- | --- |
| DCI-V1-001..003 | `internal/doccorpus/estate.go`, `cmd/corvint/docs_corpus.go`, `docs_corpus_help.go`, `docs.go` | `TestDCIV1001DCIV1002DCIV1003DCIV1005EstateReconcilesAcrossCorpora`, `TestDCIV1001EstateTargetBudgetAndStrictFile`, `TestDCIV1003EstateGapsAreMatchedNotSplit`, `TestDCIV1001DocsCorpusEstateCommand`, `TestDocsCorpusOperationFlagMatrix`, `TestDocsCorpusOperationHelpPrintsFlags` |
| DCI-V1-004 | `internal/doccorpus/compile.go`, `estate.go` | `TestDCIV1004ShallowRepositoryRefusalNamesIt` |
| DCI-V1-005 | `internal/doccorpus/types.go` | `TestDCIV1001DCIV1002DCIV1003DCIV1005EstateReconcilesAcrossCorpora` |
| DCI-V1-006..010 | `internal/doccorpus/repository_inventory.go`, `estate.go`, `providers.go`, `encoding.go` | `TestDCIV1006ToDCIV1010InventoryRulesNeedEvidence`, `TestDCIV1007RubyExpressionsAreNoNames`, `TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation`, `TestDCXV1471DerivationVersionGuard` |
| DCI-V1-011..012 | `BlobHeaders`, `BlobBatch` in `internal/cem/gitauth/pathset.go`; `oversize` in `internal/doccorpus/batch_input.go`; `inventoryScopes` in `compile.go`; `checkEvidence` in `providers.go`; `docs/DOCUMENTATION-CORPUS.md` | `TestDCIV1024OverBoundInputsAreDeclaredGaps`, `TestDCIV1012EvidenceAnchorBound` |
| DCI-V1-020 | `weakSearchQuality` in `internal/doccorpus/search_rank.go`; `SearchQuality` in `types.go` | `TestDCIV1020WeakSignalAtRealisticSize` |
| DCI-V1-021..023 | `internal/doccorpus/inventory_ci.go`, `repository_inventory.go` | `TestDCIV1021PublishJobIsTiedToTheManifestItPublishes`, `TestDCIV1021OnlyExecutedPublishCommandsTie`, `TestDCIV1021OpaqueFolderArgumentsTieNothing`, `TestDCIV1021StepWorkingDirectoryReplacesTheJobDefault`, `TestDCIV1021KeyOrderChangesNoTie`, `TestDCIV1021AdversarialPublishSteps`, `TestDCIV1022DeployEnvironmentFromItsName`, `TestDCIV1022AmbiguityStopsAndTheWordIsKept`, `TestDCIV1023CircleCIJobs`, `TestDCIV1023CircleCIDirectoriesAndCheckout`, `TestDCPV1703DCPV1704DCPV1705DeployablesPackagesAndReconciliation` |
| DCI-V1-024 | `OverBoundInputs` in `internal/doccorpus/types.go`; `compile.go`, `batch_input.go`, `query.go`, `workspace_inventory.go` | `TestDCIV1024OverBoundInputsAreDeclaredGaps`, `TestDCIV1024OnlyOverBoundScopeBuildsWithGaps` |
| DCI-V1-025 | `checkoutUnavailable` in `internal/doccorpus/compile.go` | `TestDCIV1025CheckoutRefusalKeepsGitauthReason` |
| DCI-V1-026 | `DerivationVersion` in `internal/doccorpus/encoding.go`; `docs/RELEASE-NOTES.md` | `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation` |
| DCI-V1-027 | `deployJobName`, `environmentWords` in `internal/doccorpus/inventory_ci.go`; `workflow` in `repository_inventory.go` | `TestDCIV1027DeployWordsAndShortEnvironments` |
| DCI-V1-028..029 | `RepositoryInventory`, `DeployJobDeclaration`, `validateRepositoryInventory`, `deployJob` in `internal/doccorpus/repository_inventory.go`; `namedEnvironment` in `inventory_ci.go`; `docs/DOCUMENTATION-CORPUS.md` | `TestDCIV1028DeclaredEnvironmentWords`, `TestDCIV1029DeclaredDeployJob` |
| DCI-V1-030 | `DerivationVersion` in `internal/doccorpus/encoding.go`; `docs/RELEASE-NOTES.md` | `TestDCXV1471DerivationVersionGuard`, `TestDCXV1564ReleaseNotesListEveryDerivation` |
