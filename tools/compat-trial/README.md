# Experimental compatibility replay runner

Implements CRR-V0-001..007 using `internal/procgroup`. This is owned synthetic
fixture replay, not external execution, candidate scoring, or host qualification.
Every case keeps `accepted_by: NOT_PRODUCED`.

The descriptor JSON has exactly the CTR-V0-001 keys. Concrete experimental
encodings are: environment as an array of `NAME=VALUE` strings, build flags as a
string array, CGO_ENABLED as `"0"` or `"1"`, timeouts as `"10s"` and `"120s"`,
gold provenance as a string, and build_bounds as an object. Reference paths are
relative to the manifest directory. `argv` excludes argv[0] and must begin with
`--owned-fixture` and one of the closed operations in `fixture.go`. Missing,
unknown, duplicate and null fields are rejected. `source` is nonempty prose.
The only implemented host_profile is `owned-synthetic-process-group`.

Snapshots are canonical sorted USTAR archives of regular files/directories, with
zero timestamps/ownership metadata and preserved permission modes. Symlink archive
entries and nonregular reference files are refused. Each repetition gets a fresh
scratch tree and a private executable copy of the verified bytes, preserving its
execute permission. Reference hashes and native Go build identity are checked
before any replay launch; later edits to source artifact paths cannot replace the
captured bytes.

`go build -trimpath -o /tmp/compat-trial ./tools/compat-trial` builds a runner whose
registry permits only its own native fixture entrypoint and four fixed stdin
fixtures. `/tmp/compat-trial -fixture-dir /tmp/new-owned-fixture` creates an example
bundle; `-manifest /tmp/new-owned-fixture/manifest.json` replays it. The fixture
directory must not already exist.

For owned versioned fixtures, run the repository's fixed build helper from the
repository root:

```
GOTOOLCHAIN=local go run ./tools/compat-trial/build-fixtures -out /tmp/new-owned-build
```

The helper compiles the two repository-owned `fixtures/old` and `fixtures/new`
packages under Go 1.27.0, CGO disabled, `-trimpath`, no build VCS data, and disabled
module-network access. It embeds their artifact SHA256s into the resulting runner.
`fixture-manifest.json` records the source and artifact hashes and build identity.
Use those `old` and `new` artifact paths/hashes in a synthetic descriptor to observe
`different`. The registry is never loaded from a descriptor, its adjacent files,
or an arbitrary executable-registration option. The generated registry files are
receipts, not authorization inputs. Tests build the same fixed owned variants and
exercise validation and actual old/new replay.

Output records describe observed exits only. Case launch counts remain separate
from record counts. Overflow records preserve bounded captured prefixes, name the
stream, and withhold comparison; their digests are not complete-output claims.
An overflow aborts only the rest of its case. Each task entry's 120-second budget actively
cancels its current repetition; every repetition has a 10-second ceiling and a
one-second shutdown bound. SIGINT/SIGTERM enter that same cancellation path.

Containment covers the owned process group. Session escape, forced termination of
the supervisor itself, external execution and an E:99 sandbox remain unqualified.
The filesystem observer sees only net scratch-tree transitions: transient changes
and outward effects are unobservable. Effect observations list declared and undeclared observed
effects and declared-but-absent effects; declared effects do not fail a case. The caller
must not read PASS as a broader sandbox or behavioral-correctness claim.

Focused validation:

```
GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go test -race -count=1 ./internal/procgroup ./tools/compat-trial/...
GOCACHE=/tmp/corvint-go-build-cache GOTOOLCHAIN=local go vet ./internal/procgroup ./tools/compat-trial/...
```

The coordinator owns the full frozen parity replay. An earlier unfiltered package
run exceeded its 10-minute test-wide timeout; filtered parity checks passing does
not clear that result. No external trial, label acceptance, cost score or release
qualification is produced by these tests.
