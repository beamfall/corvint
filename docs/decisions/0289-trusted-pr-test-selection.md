# 0289: Trusted PR test execution and frozen shadow qualification

Date: 2026-09-13
Status: accepted, owner release request; narrowed CI promotion pending

The owner requested Corvint to select and run relevant PR tests. AFP-V0-013/014 admit a
separate trusted native driver reusing `gate-affected-select`, with typed argv and exact
merge-event identities. Main, release, security, static, vet, formatting, build and nested
interop gates stay full. No JavaScript/E2E executable selection is admitted.

An empty or mismatched qualification pin executes the full root Go suite. A separately
reviewed artifact commit holds the qualification JSON; a workflow pin supplies its immutable
commit and SHA256 independently of the tested source. The qualification pins the earlier
reviewed tool-source commit and actual planner, selector, driver, Go, OS/architecture and C
compiler identities. Candidate tools are built with `-trimpath -buildvcs=false` outside the PR
checkout. Artifact and workflow pin updates occur after the tool-candidate shadow run, before
the final release/CEM/OCM/canonical gate freeze; no self-referential source hash is claimed.

Qualification requires exactly 200 frozen distinct contiguous first-parent pairs, complete
root package-universe outcomes, retained raw JSON/stderr and per-row digests. Each row captures
selection before one full race invocation. A selected-set miss, invalid row, build failure,
timeout or incomplete outcome fails promotion; no replacement rows or automatic UNKNOWN
waivers. A wholly green corpus has limited counterfactual evidence. The same fixed race argv,
closed Go environment and exact platform/compiler tuple must match execution. A macOS run
cannot qualify Linux CI. Measure one row before authorizing the remaining bounded run.

Initial delivery is shadow/full, with empty trust pins. Local fixtures establish execution and
cleanup behavior only. Hosted checks are currently billing/spend-limit blocked; that state is
not changed by this decision, and local PASS is not hosted PASS. Rollback clears the workflow
pins or removes the driver step, restoring full Go tests without affecting any other gate.

Protected workflow/ruleset status: **NOT_VERIFIED**. The repository workflow and literal pins
alone do not protect their own control plane. Before enabling any trust pin, the owner must
configure and review the applicable GitHub required-workflow/ruleset policy so a PR cannot
replace the trusted workflow or its pins. Keep pins empty until that admission is established.
The runtime environment is an allowlist with exact recorded bytes, a fixed absolute Go PATH,
`/usr/bin/cc`, `GOENV=off`, `LANG=C`, `LC_ALL=C`, `TZ=UTC`, and exclusively owned HOME/TMP/cache
under `/tmp/corvint-pr-tests-runtime`. An existing runtime path is refused; owned runtime state is
removed on return; cleanup failure is reported with a nonzero exit. Output/runtime paths are
resolved through their nearest existing ancestors before creation and rechecked as real external
directories, so a symlinked parent cannot create inside the tested tree. Historical rows use that same fixed layout and reset HOME/TMP/GOCACHE immediately before each test invocation.
Stdout is capped at 128 MiB and stderr at 8 MiB; overflow immediately cancels the process group
and invalidates execution. Hosted log preview is capped at 1 MiB/256 KiB. Planner/selector drift
at launch falls back to full; Go/compiler/environment drift fails safely. Freeze ignores grafts;
row reuse binds the execution source/environment and raw plan/audit hashes as well as outcomes.

AFP-V0-015 adds the owner-approved shared Linux amd64 container preparation path. The existing
official Go 1.27.0 full image supplies Debian userland, Go, Git and C compiler; no custom image
or image-publication step is needed. A fresh container handles one unchanged indexed corpus
pair, and exports bounded evidence before cleanup. The 14 GiB tmpfs shares an 8 GiB memory
ceiling, with 2 CPUs/GOMAXPROCS=2 on both Windows Docker Desktop and hosted Linux. Actual
inspected image/mount/resource identity and read-only trusted binary hashes govern matching.
A cold cache immediately before execution removes historical package-enumeration warming.
Source preparation and local fake-Docker tests do not claim actual container, 200-row or
hosted qualification. The first real row is a viability gate; no orchestration service is added.

The Windows worker's 2026-09-13 preflight observed Docker containerd image ID equal to the
pinned manifest digest and `no-new-privileges:true`. Admission accepts only the verified
manifest/config ID pair alongside the exact digest-qualified requested reference, and exactly
one bare/true security option. Existing explicit tmpfs exec/resources remain unchanged.
Worker readiness and its standard-library race probe do not qualify the Corvint runner or corpus.

Actual Windows Git 2.47.3 cloning under UID65532 rejected a read-only uid0 source even with
both exact command-scope safe.directory entries: upload-pack discards that command config.
The private protected-scope file probe succeeded with unchanged source HEAD/tree and container
cleanup. Cloning alone receives the exact-path file through GIT_CONFIG_GLOBAL; normal reads
and test environments retain /dev/null. Files are exclusive, quoted, removed before tests,
and never change the user's global configuration. The local child-transport regression checks
this propagation gap; neither it nor the Git-only Windows probe qualifies Corvint execution.

The actual Windows Corvint fixture exposed Docker cp's tmpfs limitation: the inner test exited1,
but all evidence copies failed. A bounded actual-image probe exported the same19-byte tmpfs
file through GNU tar1.35 in4.254s; its10240-byte archive includes8192 trailing zero bytes after
tar EOF. The launcher therefore uses typed exec-tar, bounded zero-padding drain and retained
capped stderr. Run-mode exit1 requires coherent terminal evidence; export/control/cleanup
failures remain infrastructure errors. This transport proof does not qualify Corvint or200rows.
