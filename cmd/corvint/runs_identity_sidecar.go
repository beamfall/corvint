package main

import (
	"errors"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/playwrightruns"
)

// runsIdentityOpts are the identity sidecar options of the runs reads (PWR-V0-065): explicit
// --identity REPORT=FILE mappings by resolved report path, and --identity-sidecars, which reads
// REPORT.identity.json beside every other report.
type runsIdentityOpts struct {
	explicit map[string]string
	sidecars bool
}

// runsIdentityFlags defines --identity and --identity-sidecars on f; relative paths resolve
// against root as the report options resolve them.
func runsIdentityFlags(f *flag.FlagSet, root string) *runsIdentityOpts {
	o := &runsIdentityOpts{explicit: map[string]string{}}
	f.Func("identity", "REPORT=FILE identity sidecar for one report, repeatable (PWR-V0-065)", func(v string) error {
		report, sidecar, ok := strings.Cut(v, "=")
		if !ok || report == "" || sidecar == "" || len(o.explicit) >= playwrightruns.MaxRuns {
			return errRunsArgument
		}
		report, sidecar = runsUnder(root, []string{report})[0], runsUnder(root, []string{sidecar})[0]
		key := filepath.Clean(report)
		if _, dup := o.explicit[key]; dup {
			return errRunsArgument
		}
		o.explicit[key] = sidecar
		return nil
	})
	f.BoolVar(&o.sidecars, "identity-sidecars", false, "read REPORT.identity.json beside each report (PWR-V0-065)")
	return o
}

func (o *runsIdentityOpts) active() bool { return o != nil && (o.sidecars || len(o.explicit) > 0) }

// need is the requirement that every --identity REPORT names one of the command's reports,
// given as resolved paths.
func (o *runsIdentityOpts) need(reports ...[]string) runsNeed {
	if o == nil {
		return runsNeed{}
	}
	known := map[string]bool{}
	for _, names := range reports {
		for _, name := range names {
			known[filepath.Clean(name)] = true
		}
	}
	for report := range o.explicit {
		if !known[report] {
			return runsNeed{"identity", runsArgValueInvalid}
		}
	}
	return runsNeed{}
}

// apply binds the report's sidecar, if any, to run and states its identity source (PWR-V0-064..067).
// It does nothing when no sidecar option was given. On a refusal it writes runs-report-unavailable
// naming the report option, its 1-based position and the reason, and returns false.
func (o *runsIdentityOpts) apply(option string, position int, name string, run *playwrightruns.Run, diagnostic io.Writer) bool {
	if !o.active() {
		return true
	}
	sidecar, explicit := o.explicit[filepath.Clean(name)]
	if !explicit && o.sidecars {
		sidecar = name + ".identity.json"
		// Under --identity-sidecars an absent sidecar leaves the report as it is; anything present
		// must bind or refuse.
		if _, err := os.Lstat(sidecar); errors.Is(err, fs.ErrNotExist) {
			sidecar = ""
		}
	}
	if sidecar != "" {
		raw, err := playwrightruns.ReadSidecarFile(sidecar)
		if err == nil {
			err = playwrightruns.ApplySidecar(run, raw)
		}
		if err != nil {
			fields := map[string]any{"reason": err.Error(), "run": position}
			if option != "" {
				fields["option"] = option
			}
			runsRefuse(diagnostic, "runs-report-unavailable", fields)
			return false
		}
	}
	playwrightruns.MarkIdentitySource(run)
	return true
}
