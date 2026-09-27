package cli

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/importer"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importCommand runs `corvint-tasks import` (CTS-V0-003): it writes a foreign
// export's new and changed items as shadow IMPORT records, never eligible
// until a separate cutover.
func importCommand(env Env, args []string) *wire.Result {
	cmd := []string{"import"}
	role, file := "OWNER", ""
	values := map[string]*string{"--role": &role, "--file": &file}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "import requires --file PATH and optional --role OWNER|OPERATOR; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if !seen["--file"] {
		return usage(cmd, "import requires --file PATH")
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(env.Cwd, file)
	}
	raw, err := intent.ReadFile(file, importer.MaxExportBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, err := snapshot.Probe(repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.Import(context.Background(), repo, actor, observed.Head.QueueID.Raw, raw, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	receipts := []string{}
	for i, batch := range report.Batches {
		if batch.Receipt != "" {
			receipts = append(receipts, batch.Receipt)
			continue
		}
		res := mutateResult(cmd, batch)
		res.Warnings = append(res.Warnings, prose("batch "+string(wire.CountOf(int64(i+1)))+" refused; earlier batches stay committed and a rerun of the same export skips them"))
		return res
	}
	o := wire.NewObject()
	o.Set("items", wire.String(string(wire.CountOf(int64(report.Items)))))
	o.Set("written", wire.String(string(wire.CountOf(int64(report.Planned)))))
	o.Set("unchanged", wire.String(string(wire.CountOf(int64(report.Items-report.Planned)))))
	o.Set("receipts", wire.Strings(receipts))
	res := &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	if len(report.Batches) > 0 && report.Batches[0].Redone {
		res.Warnings = append(res.Warnings,
			"a receipt left pending by an interrupted run was completed before this import (§5.2 redo)")
	}
	res.Warnings = append(res.Warnings,
		"imported records are shadow IMPORT records: each reads CUTOVER_MISSING and is never eligible until a separate cutover")
	return res
}
