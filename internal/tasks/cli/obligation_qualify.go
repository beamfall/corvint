package cli

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/obligation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// obligationsQualifyVersion runs `ticket obligations qualify-version`
// (TOL-V0-060..062). --print-fixture prints the embedded fixture and its
// expected classes and writes nothing. --from-playwright-report checks a
// json-reporter document of that fixture and, only when every fixture id has
// its expected class, records the version with its evidence in the
// store-local allow-list. It writes no journal record and no ticket. The
// allow-list read-modify-write runs under the store mutation lock, so
// concurrent runs each keep their version (V1-1745).
func obligationsQualifyVersion(env Env, cmd []string, args []string) *wire.Result {
	var report string
	printFixture := false
	if res := pairFlags(cmd, args, map[string]*string{"--from-playwright-report": &report}, nil, map[string]*bool{"--print-fixture": &printFixture}); res != nil {
		return res
	}
	if printFixture == (report != "") {
		return usage(cmd, "exactly one of --print-fixture and --from-playwright-report FILE is required")
	}
	if printFixture {
		credited, failed, defects := obligation.FixtureExpected()
		o := wire.NewObject().Set("fixtureName", wire.String(obligation.FixtureName)).
			Set("fixtureSha256", wire.String(string(obligation.FixtureSha256()))).
			Set("credited", wire.Strings(credited)).Set("failed", wire.Strings(failed)).
			Set("defectConfirmed", wire.Strings(defects)).
			Set("fixture", wire.String(string(obligation.Fixture())))
		return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
	}
	rep, err := obligation.ReadQualificationReport(report)
	if err != nil {
		return errorResult(cmd, err)
	}
	q, err := obligation.Qualify(rep)
	if err != nil {
		return errorResult(cmd, err)
	}
	q.QualifiedAt = nowStamp()
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	lock, err := authority.AcquireLock(context.Background(), repo, authority.LockOptions{})
	if err != nil {
		return errorResult(cmd, err)
	}
	defer lock.Close()
	p := obligation.AllowListPath(repo.CommonDir)
	a, err := obligation.ReadAllowList(p)
	if err != nil {
		return errorResult(cmd, err)
	}
	_, replaced := a.Entries[q.Version]
	if err := a.Put(*q); err != nil {
		return errorResult(cmd, err)
	}
	if err := obligation.WriteAllowList(p, a); err != nil {
		return errorResult(cmd, wire.Annotatef(err, obligation.AllowListFile, "cannot write the qualification allow-list: %v", err))
	}
	o := wire.NewObject().Set("version", wire.String(q.Version)).Set("replaced", wire.Bool(replaced)).
		Set("reportSha256", wire.String(string(q.ReportSha256))).Set("fixtureSha256", wire.String(string(q.FixtureSha256))).
		Set("credited", wire.Strings(q.Credited)).Set("failed", wire.Strings(q.Failed)).
		Set("defectConfirmed", wire.Strings(q.DefectConfirmed)).Set("qualifiedAt", wire.String(string(q.QualifiedAt))).
		Set("allowList", wire.String(p))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}
