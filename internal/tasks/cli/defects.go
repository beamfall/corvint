package cli

import (
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/defects"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// absentFlag marks a defect flag the caller did not give.
const absentFlag = "\x00"

// defectsCommand runs `defects add|update|list` (TDR-V0-010). add and update
// append one line to the register under its lock; list reads it without the
// lock and writes nothing. None of them touches the Tasks journal.
func defectsCommand(env Env, args []string) *wire.Result {
	if len(args) == 0 {
		return usage([]string{"defects"}, "expected add, update or list")
	}
	cmd := []string{"defects", args[0]}
	switch args[0] {
	case "add", "update":
	case "list":
		return defectsList(env, cmd, args[1:])
	default:
		return usage([]string{"defects"}, "expected add, update or list")
	}
	rest := args[1:]
	id := ""
	if args[0] == "update" {
		if len(rest) == 0 || strings.HasPrefix(rest[0], "--") {
			return usage(cmd, "the first argument is the defect id")
		}
		id, rest = rest[0], rest[1:]
	}
	vals := map[string]*string{}
	single := map[string]*string{}
	for _, name := range []string{"--title", "--location", "--evidence", "--severity", "--status", "--category", "--duplicate-of"} {
		v := absentFlag
		vals[name], single[name] = &v, &v
	}
	if res := pairFlags(cmd, rest, single, nil, nil); res != nil {
		return res
	}
	get := func(name string) *string {
		if *vals[name] == absentFlag {
			return nil
		}
		return vals[name]
	}
	f := defects.Fields{Title: get("--title"), Location: get("--location"), Evidence: get("--evidence"), Severity: get("--severity"),
		Status: get("--status"), Category: get("--category"), DuplicateOf: get("--duplicate-of")}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	// The store read may retry its callback when Tasks state moves, so it
	// only resolves the register; the one append runs after it returns
	// (TDR-V0-010).
	var path, prefix string
	var private bool
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		path, private = defects.Location(rc.repo, rc.store.Policy)
		prefix = rc.store.Policy.DefectPrefix()
		return nil
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	var d *defects.Defect
	if id == "" {
		d, err = defects.Add(path, private, prefix, f, now)
	} else {
		d, err = defects.Update(path, private, id, f, now)
	}
	if err != nil {
		return failure(cmd, rc, err)
	}
	res := success(cmd, rc)
	res.Items = []wire.Value{d.Value()}
	res.Untrusted = true
	return res
}

func defectsList(env Env, cmd, args []string) *wire.Result {
	status, category := "", ""
	if res := pairFlags(cmd, args, map[string]*string{"--status": &status, "--category": &category}, nil, nil); res != nil {
		return res
	}
	var reg *defects.Register
	rc, err := withInventoryStore(env, func(rc *readCtx) error {
		path, private := defects.Location(rc.repo, rc.store.Policy)
		r, rerr := defects.Read(path, private)
		reg = r
		return rerr
	})
	if err != nil {
		return failure(cmd, rc, err)
	}
	items := []wire.Value{}
	for _, d := range reg.Defects() {
		if (status == "" || d.Status == status) && (category == "" || d.Category == category) {
			items = append(items, d.Value())
		}
	}
	res := success(cmd, rc)
	res.Items = items
	res.Untrusted = true
	if !reg.Exists {
		res.Warnings = append(res.Warnings, "the defect register does not exist yet; defects add creates it")
	}
	return res
}
