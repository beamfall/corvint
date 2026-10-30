package jstestprovider

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testvalidity"
)

// PWP-V0-039: every row of the shared table is an exact release, and
// receipts and freshness both admit from it, so a row change moves both.
func TestPWPV0039ReceiptsAndFreshnessAdmitFromOneTable(t *testing.T) {
	exact := regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	for _, row := range admittedRuntimeTuples {
		if !exact.MatchString(row.runner) || !strings.HasPrefix(row.node, "v") || !exact.MatchString(row.node[1:]) || (row.source != BrowserSourceBundled && row.source != BrowserSourceSystem) {
			t.Fatalf("row is not an exact tuple: %+v", row)
		}
		if isExternalProfile(row.profile) && ReceiptRuntimeTuple(Receipt{Profile: row.profile, Identity: Identity{RunnerVersion: row.runner, NodeVersion: row.node}}) != RuntimeTupleCandidate {
			t.Fatalf("receipt classification disagrees with row %+v", row)
		}
	}
	if runtimeTupleAdmitted(FreshnessProfile, "1.63.0", qualifiedNodeVersion, BrowserSourceBundled) || runtimeTupleAdmitted(FreshnessProfile, "1.64.0", admittedBundledNodeVersion, BrowserSourceBundled) || runtimeTupleAdmitted(FreshnessProfile, "1.64.0", qualifiedNodeVersion, BrowserSourceSystem) {
		t.Fatal("freshness admitted a tuple without a live freshness run")
	}
	current := func(r Receipt) string { return ReceiptTestProjection(r, r.Tests[0]).Freshness.State }
	if got := current(freshFixture(t)); got != testvalidity.FreshnessCurrent {
		t.Fatalf("admitted freshness row read %s", got)
	}
	onNode := func(node string) Receipt {
		r := freshFixture(t)
		r.Identity.NodeVersion = node
		r.Freshness.IdentityAfter = &r.Identity
		r.Tests[0].Project.Use = []byte(strings.Replace(string(r.Tests[0].Project.Use), `"nodeVersion":"v22.23.3"`, `"nodeVersion":"`+node+`"`, 1))
		r.Tests[0].ID = qualifiedTestID(r.Identity, r.Tests[0])
		return r
	}
	if got := current(onNode(qualifiedNodeVersion)); got != testvalidity.FreshnessUnknown {
		t.Fatalf("freshness on an unadmitted Node read %s", got)
	}
	saved := admittedRuntimeTuples
	t.Cleanup(func() { admittedRuntimeTuples = saved })
	admittedRuntimeTuples = append(slices.Clone(saved), admittedRuntimeTuple{FreshnessProfile, "1.63.0", qualifiedNodeVersion, BrowserSourceBundled})
	if got := current(onNode(qualifiedNodeVersion)); got != testvalidity.FreshnessCurrent {
		t.Fatalf("freshness did not admit from the shared table: %s", got)
	}
	admittedRuntimeTuples = slices.DeleteFunc(slices.Clone(saved), func(row admittedRuntimeTuple) bool {
		return row.profile == ExternalProfile && row.runner == "1.64.0" && row.node == admittedBundledNodeVersion
	})
	if got := ReceiptRuntimeTuple(Receipt{Profile: ExternalProfile, Identity: Identity{RunnerVersion: "1.64.0", NodeVersion: admittedBundledNodeVersion}}); got != RuntimeTupleUnqualified {
		t.Fatalf("receipt did not admit from the shared table: %s", got)
	}
}

// PWP-V0-040: a tuple refusal names the failing field and the exact values
// the shared table admits for it.
func TestPWPV0040TupleRefusalNamesFieldAndAdmittedValues(t *testing.T) {
	for _, tc := range []struct {
		profile, runner, node, source string
		field                         string
		admitted                      []string
	}{
		{ExternalProfile, "1.65.0", "", "", TupleFieldRunner, []string{"1.63.0", "1.64.0"}},
		{FreshnessProfile, "1.60.0", "", "", TupleFieldRunner, []string{"1.63.0", "1.64.0"}},
		{FreshnessProfile, "1.64.0", "v22.23.3", "", TupleFieldNode, []string{"v22.23.2"}},
		{FreshnessProfile, "1.64.0", "v24.11.1", "", TupleFieldNode, []string{"v22.23.2"}},
		{FreshnessProfile, "1.64.0", "v22.23.2", BrowserSourceSystem, TupleFieldBrowserSource, []string{BrowserSourceBundled}},
		{FreshnessProfile, "1.63.0", "v22.23.2", "", TupleFieldNode, []string{"v22.23.3"}},
		{ExternalProfile, "1.64.0", "v22.23.3", "", TupleFieldNode, []string{"v22.23.2", "v24.11.1"}},
		{SensitiveExternalProfile, "1.64.0", "v24.11.1", "", TupleFieldNode, []string{"v22.23.2"}},
		{AttemptExternalProfile, "1.64.0", "v22.23.2", BrowserSourceSystem, TupleFieldBrowserSource, []string{BrowserSourceBundled}},
		{ExternalProfile, "1.64.0", "v24.11.1", BrowserSourceSystem, TupleFieldBrowserSource, []string{BrowserSourceBundled}},
		{ExternalProfile, "1.64.0", "v22.23.2", BrowserSourceSystem, "", nil},
		{FreshnessProfile, "1.63.0", "v22.23.3", BrowserSourceBundled, "", nil},
		{FreshnessProfile, "1.64.0", "v22.23.2", BrowserSourceBundled, "", nil},
	} {
		m := AdmittedRuntimeTupleMismatch(tc.profile, tc.runner, tc.node, tc.source)
		if tc.field == "" {
			if m != nil {
				t.Fatalf("%+v refused %+v", tc, m)
			}
			continue
		}
		if m == nil || m.Field != tc.field || m.Profile != tc.profile || !slices.Equal(m.Admitted, tc.admitted) {
			t.Fatalf("%+v: got %+v", tc, m)
		}
	}

	_, err := RunE2E(context.Background(), E2EConfig{Config: Config{RunnerVersion: "1.60.0"}, Freshness: &FreshnessConfig{}})
	var refusal *E2ERefusal
	if !errors.As(err, &refusal) || refusal.Code != "freshness-runtime-tuple-unqualified" {
		t.Fatalf("freshness runner refusal untyped: %v", err)
	}
	// Direct freshness callers get the same typed refusal (PWP-V0-040).
	var direct *E2ERefusal
	if _, err := RunFreshE2E(context.Background(), E2EConfig{Config: Config{RunnerVersion: "1.60.0"}, Freshness: &FreshnessConfig{}}); !errors.As(err, &direct) || direct.Code != refusal.Code || direct.Tuple == nil || direct.Tuple.Field != "runner" {
		t.Fatalf("direct freshness refusal untyped: %v", err)
	}
	want := `{"schema":"corvint-playwright-e2e-refusal/0","code":"freshness-runtime-tuple-unqualified","message":"freshness-runtime-tuple-unqualified: runner not admitted for corvint-playwright-freshness/0; admitted runner values: 1.63.0, 1.64.0","qualifiedRunnerVersions":["1.60.0","1.63.0","1.64.0"],"runtimeTuple":{"profile":"corvint-playwright-freshness/0","field":"runner","admitted":["1.63.0","1.64.0"]}}` + "\n"
	if got := string(EncodeE2ERefusal(refusal)); got != want {
		t.Fatalf("document\n got %s\nwant %s", got, want)
	}
	if got := negateTupleDetail(ExternalProfile, "1.64.0", "v22.23.3", RuntimeTupleUnqualified); got != "runner or Node version is not a qualified tuple (unqualified): node not admitted for corvint-playwright-external/0; admitted node values: v22.23.2, v24.11.1" {
		t.Fatalf("negate detail %q", got)
	}
}
