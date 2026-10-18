# V1-1697: STA-V0 conformance job, version pins and OIF-V0 acceptance

Ticket V1-1697, lane 4 of [spec-toolchain-adapters-v0](../specs/spec-toolchain-adapters-v0.md):
`STA-V0-001` and `STA-V0-011` to `STA-V0-013`. The owner accepted the `OIF-V0` intent in chat on
2026-10-18, recorded as decision 0541. Neither spec's delivery is promoted: `STA-V0` waits for a
passing `spec-toolchain-conformance` CI run, as the owner asked.

## Decisions

- **Pins come from the registries only.** OpenSpec `@fission-ai/openspec` 1.14.3: npm tarball
  SHA-256 `f7a4eaf9199036713edf331f1a781abebc4ed9bd3fbf5a8aa549f482749e14c2` and the registry's
  sha512 integrity, which matched a recomputation over the downloaded tarball. Spec Kit v1.2.1: the
  annotated tag `0d1be13e` peels to commit `9dca9bf42a031765ee5e5f2371137f2f26967e11`; the GitHub
  archive of that commit hashes to `c416d6ab6af39aa03af35d0704ed620b43d8927852a06fc3dd87e61e396ff07e`
  on two downloads and through both archive URLs, and `git get-tar-commit-id` names the commit.
- **Install verification.** OpenSpec installs by `npm ci --ignore-scripts` from a committed
  lockfile in which all 58 entries carry sha512 integrity and a registry URL; the script also
  re-hashes the OpenSpec tarball against `versions.json`. Spec Kit installs with
  `pip --require-hashes --only-binary=:all:` from `uv pip compile --generate-hashes` output, with
  only `specify-cli` built from the hashed commit archive, and the `hatchling` backend preinstalled
  from its own hashed file because hash mode forbids unhashed build isolation.
- **Upstream's view.** OpenSpec obligations are compared with `openspec show --json`: requirement
  names for main specs, and for deltas the `deltas[]` operations and renames, both for obligations
  and `excludedRequirements`. `show` reads only active changes, so the archived change is copied
  into a scratch project as an active change. Spec Kit has no FR/SC parser, so its view is the
  bundled `spec-template.md` line grammar; Corvint's form is also run over upstream's own template
  (11 IDs, 2 clarifications). Validators: `openspec validate --all --strict --json` and the bundled
  `check-prerequisites.sh --json --require-tasks --include-tasks`.
- **Spec Kit project setup.** The job copies the bundled `core_pack` scripts into the fixture's
  `.specify/` instead of running `specify init`, which needs an agent choice and network. The
  extension is installed with `specify extension add --dev`; the hooks in `.specify/extensions.yml`
  must be `before_implement` → `speckit.corvint.context` and `after_implement` →
  `speckit.corvint.review`, both optional. No agent integration is initialized, so the extension's
  registered command list is empty, and the job does not assert on it.
- **CI shape.** One path-filtered job on the forms, fixtures, integrations, pins, scripts and the
  workflow, plus `workflow_dispatch`; never on every PR (the `Makefile` is left out so unrelated
  target edits do not pay for the network job) and not in `make gate`.
  It uses the runner's preinstalled node, npm, python3 and jq (no extra actions to pin), and the
  script refuses node below 20.19 (OpenSpec's engine) and Python below 3.11. Telemetry is off and
  `HOME`, caches and `TMPDIR` live in a throwaway directory.
- **Latest check.** `script/spec-toolchain-latest.sh` reads the npm `latest` dist-tag and the
  GitHub latest release; any difference from the pin is `BEHIND`, a draft, prerelease or malformed
  answer is `UNAVAILABLE`, and it never edits the manifest.

## Evidence

- `TestSTAV0001_VersionsManifest` passes; with one hex digit of the archive SHA-256 changed it
  fails on the `requirements.txt` cross-check.
- `make spec-toolchain-conformance` passed locally on 2026-10-18 (macOS, node from Homebrew,
  Python 3.12) in about 17 s:

  ```
  ok   openspec-spec openspec/specs/auth/spec.md: 4 item(s)
  ok   openspec-spec openspec/specs/billing/invoices/spec.md: 2 item(s)
  ok   openspec-delta .../add-session-timeout/specs/auth/spec.md obligations: 2 item(s)
  ok   openspec-delta ... excludedRequirements: 2 item(s)
  ok   openspec-delta .../archive/2026-10-01-add-invoice-export/... obligations: 1 item(s)
  ok   ... excludedRequirements: 0 item(s)
  ok   speckit-spec specs/000-upstream-template/spec.md FR/SC IDs: 11 item(s)
  ok   ... needs-clarification: 2 item(s)
  ok   speckit-spec specs/001-session-timeout/spec.md FR/SC IDs: 6 item(s)
  ok   ... needs-clarification: 1 item(s)
  specify extension add --dev: installed corvint with hooks {"before_implement":["speckit.corvint.context"],"after_implement":["speckit.corvint.review"]}
  openspec instructions: config snippet reaches apply context, apply guidance and tasks rules
  spec-toolchain-conformance: PASS (openspec 1.14.3, spec-kit 1.2.1, 6 intent forms checked)
  ```

  A wrapper Corvint that drops the first worklist item made the same run exit 1 with six `FAIL`
  lines.
- `script/spec-toolchain-latest.sh` at 2026-10-18T11:14:38Z:

  ```
  openspec 1.14.3 CURRENT
  spec-kit 1.2.1 CURRENT
  ```

  `make spec-toolchain-latest-test` passes six offline cases.
- `NOT_RUN`: the first `spec-toolchain-conformance` CI run. The ubuntu-24.04 runner's node version
  and `python3 -m venv` are unobserved; the script fails closed on either.

## Route dispositions

`corvint affected --base` used before the focused tests; `dogfood` keyed workflow used for the
change binding. `prove`, mutation and evaluation routes not applicable: the change adds a CI job,
pins and prose, and no ranking or evidence code.
