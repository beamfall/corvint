## 2026-10-05 V1-0545: owner accepts PCH-V0 and declines an audit command and a build label

Human-owned intent: the owner decided on 2026-10-05, recorded on native ticket V1-0545 (rev 17,
receipt 2745 for decisions 1 and 2; receipt 2746 for decision 3):

1. Accept PCH-V0 (`docs/specs/postmerge-ci-host-v0.md`). Intent status moves from proposed to
   accepted in the spec header, `docs/specs/README.md` and `docs/specs/INDEX.json`.
2. Do not publish an audit command. It stays a PCH-V0 non-goal, now worded as declined by the
   owner rather than deferred. `internal/postmergehost.Audit` stays an internal package exercised
   by tests.
3. `install-pinned.sh` does not stamp a build label. It keeps building without link-time flags,
   so the delta record's `build` field stays the binary's default `"0"`. A consumer must not read
   that field as provenance; the pin binds only through the operator's digest file. This is added
   as a PCH-V0 non-goal and stated in the delta failure modes.

Delivery status stays experimental. Acceptance of the intent does not qualify physical isolation,
a hosted dry-run of the #395 replay set, a hosted run of the delta job, delta consumption by later
steps, or native completion; those promotion conditions are unchanged.

Rollback: revert this entry's spec, README and INDEX edits; the spec returns to proposed, the audit
command is listed as a plain non-goal, and the build-label non-goal is removed.
