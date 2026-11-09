# Application map adoption

This guide covers the two steps that give an application repository an experimental
application map (`docs/specs/application-map-v0.md`, AMAP-V0): a hand-written manifest, and
`corvint flows appmap build` to check it. The ordered chain, with each step's flags, inputs and
commits, is the `workflows` member of `corvint help --json`, and
`corvint flows appmap build --help` prints the build step's own detail.

```text
manifest -> corvint flows appmap build
```

The change-compiled E2E chain that once continued from here left with the E2E proof lab
(decision 0593). To choose which existing tests a change must rerun, use `corvint affected
--base REV` and `corvint flows appmap impact`.

## 1. The minimum manifest

The manifest is hand-written: no command generates it. Write `appmap.json` at the repository
root. Its schema is `application-map-manifest/0` (AMAP-V0-001).

```json
{
  "schema": "application-map-manifest/0",
  "app": "web",
  "flows": "flows",
  "routers": [{"path": "src/routes.tsx", "dialect": "react-router/0"}],
  "tests": {"root": "e2e", "specs": ["e2e/specs"], "page_objects": ["e2e/pages"]}
}
```

- **Required members:** `schema`; `app`, matching `^[a-z0-9][a-z0-9._-]{0,63}$`; 1 to 64
  `routers`, each with a dialect (`ui-router-states/0`, `rails-routes/0` or `react-router/0`);
  and `tests`, with `root`, `specs` and `page_objects`, every directory under `tests.root`. A flat
  layout lists the same directory in both: there, `*.spec.ts` and `*.test.ts` files are specs and
  every other file is a page object (AMAP-V0-780). `web_root` (optional) names the dev-server
  root (AMAP-V0-781, AMAP-V0-782).
- **Optional `flows`:** a directory of `application-flow-intent/1` files.
- **Other optional members:** the application-map spec lists them, for example `workflows`,
  `scenarios`, `page_object_screens`, `hash_prefix` and `base_path`.

## 2. Check it: `corvint flows appmap build`

```sh
git add appmap.json && git commit -m "Add application map manifest"
corvint flows appmap build --manifest appmap.json --revision HEAD > /tmp/appmap-map.json
```

The build reads the manifest, routers, flows and tests from Git objects at `--revision` (default
`HEAD`), never from the worktree, so commit them first. A local commit is enough. Read the
`application-map/0` output, check its routes, screens, flows and page objects, and fix the
manifest until no unknown is unexpected. Do not commit this output.

`--from MAP` carries a previous map forward when no input changed since its revision
(AMAP-V0-211), and `--verify` also builds in full and refuses unless the two agree.

## Two repositories: application and tests apart

The application can stay in `--root` while its Playwright suite lives in a second repository
(AMAP-V0-017, AMAP-V0-200):

- `--repo ALIAS=ABSOLUTE_ROOT` (repeatable) binds an alias to the test checkout. The root must
  be absolute, the top of a Git worktree with a `HEAD` commit, and not the `--root` repository.
- `--manifest-repo ALIAS` reads the manifest from that aliased root.
- `--repo-revision ALIAS=REV` (repeatable) pins that root to commit REV instead of its `HEAD`.
- `--workspace FILE` reads the three root flags from a `corvint-workspace/0` file (WSD-V0), and
  `--no-workspace` stops the discovery of `ROOT/.corvint/workspace.json` (WSD-V0-003).

The manifest names the test repository's alias in `tests.repo`. Routers and flows always come from
`--root`.
