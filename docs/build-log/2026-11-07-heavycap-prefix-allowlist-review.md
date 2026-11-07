# Heavycap independent review: closed environment-prefix admission

Date: 2026-11-07. Repair starts at `82720e20a935d7a94512e63af209b62177dc3ec7`.
Affected and CEM base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: supplied independent review P1, V1-5007 / ASD-V0-2478.

## Finding, class and decision

The reported path-qualified env receiver omission was already repaired in commit
`ebba1d5bb`: the receiver recognizes both literal env and paths ending in /env.
Its existing absolute-env-explicit-empty regression executes the real offload path
with receiver-only GOENV exclusions and requires the broken existing test to fail.
Retain that repair and its assertions; no additional production change is needed.

The soundness class is unmodeled command-prefix syntax changing the child's Go
defaults after admission. The existing admission and command_goflags scanners use
a closed prefix allowlist: NAME=VALUE with shell identifier names, env by basename
(including absolute paths), and go by basename as the terminal executable. All
other prefix words return unproven; automatic whole-package admission refuses them.
No env options are certified, and no list of known dangerous options grants authority.
The receiver's prefix scan is only a rewrite of already admitted command words;
it does not establish admission authority.

Add unknown-env-form-refused alongside the existing real offload cases. An invented
env option must return the existing heavy.sh refusal with exit 2 before either
remote execution or local fallback. This distinguishes fail-closed admission from
an env executable rejecting the option after admission. The reported supported form
and the unknown-form default now share the receiver-environment fixture.

No diagnostic, exit contract, wire shape, golden or existing assertion changes.
The spec's evidence table records this new witness without changing requirements.
Rollback: remove the new regression and evidence-table phrase together; keep the
previous path-qualified env repair. Other charter tickets receive no implementation
changes. V1-5007's previously evidenced missing external pre-commit integration
remains PARTIAL; this local finding is independently testable through heavy.sh.

## Verification scope

Corvint affected selects tools/test-commands-check as DIRECT_TEST_CHANGE. Its
69 unknowns and self-binary-stale warning remain retained. The whole touched package
runs through script/heavy.sh without a -run filter and without exported commit
identity. Original pre-change capture is reused. Raw logs stay outside the repository
under /home/russell/lanes/heavycap-evidence; scratch is the private lane TMPDIR.
Independent review and broad gates remain orchestrator-owned. Final results and
NOT_PRODUCED reasons are retained in the handback, with no publication or seal.

The initial package run failed only the newly added assertion's incorrect marker
spelling (46.734 s): heavy.sh already returned exit 2 and its frozen
`REFUSED whole-reason-missing (exit 2)` marker. The new assertion now checks that
exact marker. No prior assertion or production output was changed. The final whole
package passes; the external handback retains its timing and the checked target.
Spec and citation checks pass (internal/specindex 0.982 s, internal/lrfrepo 0.070 s).
Repin found no moved citations; pinned Go is go1.27.3. Configured heavy slots were
used unchanged; the runner reported low /tmp space but no lock refusal.

External evidence SHA-256:

- prefix-affected.json: `c2ef941d87ec7162c2b72d60453be0982596a34569392915c364df627c60b166`
- prefix-affected.stderr: `0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e`
- prefix-package-initial.log: `eda27b8b8899c00718921e136a68efad7b1e363029ede0f5641dd44552007c6b`
- prefix-package.log: `d47a237cb9a2657cc871214a1eec68a31d1eb2bbc7684cf17145ceed6d1d6690`
- prefix-repin.log: `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133`
- prefix-spec-lines.log: `57fc7c07b86df6ee2f91c6f7da8f4d49a048f12ef89920ee1664adbe3c032dc7`
