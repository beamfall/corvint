# appmap qualification report (V1-1138, GitHub #780)

`flows appmap stats --map FILE [--budget N | --full] [--min-resolved PCT]` reads only the map and
reports screens resolved/total, top-5 unknown reasons with up to 3 example anchors, test-join and
page-object binding counts, edges by basis and the selector strength histogram
(`application-map-stats/0`, AMAP-V0-060..065, proposed, pending owner acceptance).

- Counts reconcile with the map: unknown reasons are grouped from `unknowns`, which already holds
  one entry per unresolved screen, so screens are not double counted.
- `--min-resolved` writes the report, then exits 1 (a threshold miss, not a refusal) naming the
  dominant reasons; range and flag errors exit 2. `flows.go` maps the new `gateFailure` to exit 1.
- Not run: adopter-scale qualification; no claim of equivalence with a hand-built map.
