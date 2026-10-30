# Accepting and falsifying tests against an external authenticated app

Status: experimental. Contracts: NEA-V0-012..017 in
[new-e2e-test-acceptance-v0](specs/new-e2e-test-acceptance-v0.md) and BBF-V0-217..219 in
[browser-behavior-falsification-v0](specs/browser-behavior-falsification-v0.md).

Use this profile when your app is already running on loopback, you started it yourself, and its
Playwright tests log in through a storage-state file for a disposable account. `corvint-test
accept` (NEA) and `corvint-test falsify` (BBF) then run against that app. Neither command starts,
signals or stops it.

## What you provide

1. A disposable app that answers on an HTTP loopback IP URL, for example
   `http://127.0.0.1:4173/`. Do not use a hostname, credentials in the URL, or a remote host.
2. A disposable account and its Playwright storage-state file (`cookies` and `origins` only). The
   file must lie outside both repositories, be at most 1 MiB and have no symlinked path component.
   Point your Playwright config at it through a declared environment value, for example
   `E2E_STORAGE_STATE=/abs/path/state.json`.
3. A health hook: one executable file with no arguments, pinned by SHA-256. It reads one JSON
   document on stdin, `{"schema":"corvint-new-e2e-external-health/0","phase":"reset"|"health","kind":...,"ordinal":N}`.
   Use `reset` to restore the app's disposable data and `health` to confirm it still answers. A
   pass is exit 0 within 10 seconds. Its output is discarded unread, so do not use it for evidence.

## `corvint-test accept` request (/0 only)

Leave `server` empty and add:

```json
"external_app": {
  "profile": "authenticated-disposable",
  "health": {"argv": ["/abs/path/health-hook"], "executable_sha256": "<sha256 hex>"},
  "environment": [{"key": "E2E_STORAGE_STATE", "value": "/abs/path/state.json"}],
  "auth_files": [{"path": "/abs/path/state.json", "sha256": "<sha256 hex>"}]
}
```

- Environment keys are uppercase and non-secret, and their values are pinned in the approval
  digest. Keys with a credential segment such as `TOKEN`, `PASSWORD`, `SESSION` or `KEY` are
  refused, and so are reserved keys and prefixes such as `PATH`, `HOME`, `BASH_ENV`,
  `HTTPS_PROXY`, `NODE_`, `PLAYWRIGHT_` and `CORVINT_` (the spec has the full list). Values that look like secrets are refused too. Pass secrets only through the
  storage-state file.
- Tracked inputs and argv are secret-screened under the profile. A test file containing an
  assignment such as `password=...` is refused as `input-secret-shaped`. Log in through storage
  state, not hard-coded credentials.
- The reset hook runs before every repeat and probe run, and the health hook runs after it. Each
  run executes a private copy of the pinned hook bytes, and the storage-state pins are rechecked
  after the reset. The ready URL is polled before the runner starts and again after it exits;
  redirects are not followed.
- The report carries `external` (profile, `server_ownership: caller`, the URL and digests only) and
  a body row `Authentication | authenticated-disposable`. Storage-state contents never appear.
  If any cookie or storage value would appear, nothing is published (`auth-material-in-report`).

## `corvint-test falsify` request

The request must declare the BBF-V0-019 `shared-server` isolation (your reset hook owns the
server). Add:

```json
"authentication": {
  "profile": "authenticated-disposable",
  "storage_state": [{"path": "/abs/path/state.json", "sha256": "<sha256 hex>"}]
}
```

A credential-named or secret-shaped `declared_env_keys` entry is refused. The digest is checked
when the plan is built, before execution and after the last control.

## Blocked verdicts

A /0 `blocked` report now names the reason in its `blocked` field:

- `infrastructure`: a run, cleanup, control or test row was incomplete or failed, which includes a
  failed hook or an app that stopped answering (`server-unavailable-at-publish`). Fix the
  environment and rerun.
- `freshness-unknown`: everything ran clean and nothing was rejected. /0 cannot observe
  freshness, so acceptance still needs a /1 freshness run. The profile is not available under /1.

## Threat model and limits

The companion can only check what is pinned. It cannot tell whether the account is truly
disposable, whether the server state is fresh, or whether the storage state still logs in, so the
report keeps these as unknowns. Treat storage-state files as credentials: keep them out of
repositories, rotate them with the disposable account, and rebuild and re-approve the plan after
any change, since a changed file fails with `auth-file-drift` or `authentication-input-drift`.
Failure codes are listed in each spec's code table.
