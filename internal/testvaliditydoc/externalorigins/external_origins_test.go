package externalorigins

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var updateExternalOriginsGolden = flag.Bool("update-external-origins-golden", false, "rewrite testdata/external-origins/report.golden.json")

func externalOriginsFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "external-origins", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func externalOriginsPolicy(t *testing.T) ExternalOriginsPolicy {
	t.Helper()
	policy, err := DecodeExternalOriginsPolicy(externalOriginsFixture(t, "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func externalOriginsRun(t *testing.T, name string) NetworkEvidence {
	t.Helper()
	run, err := DecodeNetworkEvidence(externalOriginsFixture(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return run
}

func buildExternalOrigins(t *testing.T, policy ExternalOriginsPolicy, runs ...NetworkEvidence) ExternalOriginsReport {
	t.Helper()
	report, err := BuildExternalOriginsReport(policy, runs)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func findExternalOrigin(report ExternalOriginsReport, origin string) *ExternalOrigin {
	for i := range report.Origins {
		if report.Origins[i].Origin == origin {
			return &report.Origins[i]
		}
	}
	return nil
}

// PWP-V0-022: the network-evidence document is closed and bounded; a request URL reduces to its
// canonical origin and a query-free locator, and data:, blob: and about: URLs have no origin.
func TestPWPV0022NetworkEvidenceIsClosedAndOriginsAreCanonical(t *testing.T) {
	for raw, want := range map[string][2]string{
		"https://CDN.Example.test:443/npm/x@1/+esm?token=secret#frag": {"https://cdn.example.test", "https://cdn.example.test/npm/x@1/+esm"},
		"http://127.0.0.1:3002":                   {"http://127.0.0.1:3002", "http://127.0.0.1:3002/"},
		"wss://socket.example.test:8443/live?x=1": {"wss://socket.example.test:8443", "wss://socket.example.test:8443/live"},
		"http://[::1]:80/api":                     {"http://[::1]", "http://[::1]/api"},
		"http://[::1]:9000/api":                   {"http://[::1]:9000", "http://[::1]:9000/api"},
	} {
		origin, locator, ok, err := RequestOrigin(raw)
		if err != nil || !ok || origin != want[0] || locator != want[1] {
			t.Errorf("RequestOrigin(%q) = %q %q %v %v, want %q %q", raw, origin, locator, ok, err, want[0], want[1])
		}
	}
	for _, raw := range []string{"data:image/png;base64,AA==", "blob:http://127.0.0.1:3002/1", "about:blank"} {
		if _, _, ok, err := RequestOrigin(raw); ok || err != nil {
			t.Errorf("RequestOrigin(%q) has a network origin or refused: %v %v", raw, ok, err)
		}
	}
	for _, raw := range []string{"ftp://files.example.test/x", "https://user:pass@cdn.example.test/x", "/relative", "chrome-extension://abc/x"} {
		if _, _, _, err := RequestOrigin(raw); err == nil {
			t.Errorf("RequestOrigin(%q) accepted", raw)
		}
	}

	valid := string(externalOriginsFixture(t, "run-pass.json"))
	for name, mutate := range map[string]func(string) string{
		"unknown member": func(s string) string { return strings.Replace(s, `"run": "ci-1"`, `"run": "ci-1", "extra": 1`, 1) },
		"trailing data":  func(s string) string { return s + "{}" },
		"wrong profile": func(s string) string {
			return strings.Replace(s, "corvint-network-evidence/0", "corvint-network-evidence/1", 1)
		},
		"short revision": func(s string) string {
			return strings.Replace(s, "0123456789abcdef0123456789abcdef01234567", "0123456", 1)
		},
		"open resource type": func(s string) string {
			return strings.Replace(s, `"resourceType": "module"`, `"resourceType": "wasm"`, 1)
		},
		"awaited on a script": func(s string) string {
			return strings.Replace(s, `"resourceType": "script"`, `"resourceType": "script", "awaited": false`, 1)
		},
		"open outcome":        func(s string) string { return strings.Replace(s, `"outcome": "passed"`, `"outcome": "flaky"`, 1) },
		"status out of range": func(s string) string { return strings.Replace(s, `"status": 204`, `"status": 704`, 1) },
		"userinfo url": func(s string) string {
			return strings.Replace(s, "https://cdn.jsdelivr.example/", "https://u:p@cdn.jsdelivr.example/", 1)
		},
		"repeated test": func(s string) string {
			return strings.Replace(s, `"title": "catalog > filters items"`, `"title": "catalog > renders the micro-frontend"`, 1)
		},
	} {
		_, err := DecodeNetworkEvidence([]byte(mutate(valid)))
		if err == nil || !strings.HasPrefix(err.Error(), NetworkEvidenceInvalid+": ") {
			t.Errorf("%s: err = %v, want %s", name, err, NetworkEvidenceInvalid)
		}
	}
	if _, err := DecodeNetworkEvidence(bytes.Repeat([]byte(" "), MaxNetworkEvidenceBytes+1)); err == nil {
		t.Error("an oversized document was accepted")
	}
}

// PWP-V0-023: script, module, stylesheet, document, websocket and other are blocking; fetch and xhr
// are blocking unless stated not awaited; beacon, image, font and media are incidental.
func TestPWPV0023BlockingVersusIncidental(t *testing.T) {
	no, yes := false, true
	for _, tc := range []struct {
		request NetworkRequest
		want    string
	}{
		{NetworkRequest{ResourceType: "module"}, RequestBlocking},
		{NetworkRequest{ResourceType: "script"}, RequestBlocking},
		{NetworkRequest{ResourceType: "stylesheet"}, RequestBlocking},
		{NetworkRequest{ResourceType: "document"}, RequestBlocking},
		{NetworkRequest{ResourceType: "websocket"}, RequestBlocking},
		{NetworkRequest{ResourceType: "other"}, RequestBlocking},
		{NetworkRequest{ResourceType: "fetch"}, RequestBlocking},
		{NetworkRequest{ResourceType: "xhr", Awaited: &yes}, RequestBlocking},
		{NetworkRequest{ResourceType: "fetch", Awaited: &no}, RequestIncidental},
		{NetworkRequest{ResourceType: "beacon"}, RequestIncidental},
		{NetworkRequest{ResourceType: "image"}, RequestIncidental},
		{NetworkRequest{ResourceType: "font"}, RequestIncidental},
		{NetworkRequest{ResourceType: "media"}, RequestIncidental},
	} {
		if got := RequestDependence(tc.request); got != tc.want {
			t.Errorf("%s awaited=%v: %s, want %s", tc.request.ResourceType, tc.request.Awaited, got, tc.want)
		}
	}
}

// PWP-V0-023: a run whose only external traffic is analytics beacons lists the origin as
// incidental, raises no finding and is CLEAN.
func TestPWPV0023BeaconOnlyTrafficIsIncidental(t *testing.T) {
	run := externalOriginsRun(t, "run-pass.json")
	run.Tests = run.Tests[:1]
	report := buildExternalOrigins(t, externalOriginsPolicy(t), run)
	if report.Verdict != ExternalOriginsClean || len(report.Findings) != 0 {
		t.Fatalf("verdict %s findings %+v", report.Verdict, report.Findings)
	}
	beacon := findExternalOrigin(report, "https://analytics.example.test")
	if beacon == nil || beacon.Class != OriginUndeclared || beacon.Blocking != 0 || beacon.Incidental != 1 || !reflect.DeepEqual(beacon.ResourceTypes, []string{"beacon"}) {
		t.Fatalf("beacon origin = %+v", beacon)
	}
}

// PWP-V0-024: a module fetched from an undeclared CDN yields one external-origin-dependence finding
// naming the origin, the resource type and the first affected test in run order; declared
// environment origins (the application and its mock) are never listed.
func TestPWPV0024CDNModuleDependenceFinding(t *testing.T) {
	report := buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"))
	if report.Verdict != ExternalOriginsDependent {
		t.Fatalf("verdict = %s", report.Verdict)
	}
	want := []ExternalOriginFinding{{
		Code: ExternalOriginDependence, Origin: "https://cdn.jsdelivr.example", ResourceType: "module", FirstRun: "ci-1",
		FirstTest: ExternalTestRef{File: "tests/catalog.spec.ts", Title: "catalog > renders the micro-frontend"}, Tests: 2,
	}}
	if !reflect.DeepEqual(report.Findings, want) {
		t.Fatalf("findings = %+v\nwant %+v", report.Findings, want)
	}
	var listed []string
	for _, origin := range report.Origins {
		listed = append(listed, origin.Origin+" "+origin.Class)
	}
	wantListed := []string{"https://analytics.example.test undeclared", "https://assets.example-cdn.test allowed", "https://cdn.jsdelivr.example undeclared"}
	if !reflect.DeepEqual(listed, wantListed) {
		t.Fatalf("origins = %v, want %v", listed, wantListed)
	}
}

// PWP-V0-025: an allow-listed deterministic origin is listed with its class and reason and raises
// no finding; allow-listing the CDN too makes the run CLEAN. A malformed policy is refused.
func TestPWPV0025AllowListedOriginRaisesNoFinding(t *testing.T) {
	policy := externalOriginsPolicy(t)
	report := buildExternalOrigins(t, policy, externalOriginsRun(t, "run-pass.json"))
	allowed := findExternalOrigin(report, "https://assets.example-cdn.test")
	if allowed == nil || allowed.Class != OriginAllowed || allowed.Reason != "pinned content-hashed assets" || allowed.Blocking != 1 {
		t.Fatalf("allowed origin = %+v", allowed)
	}
	for _, finding := range report.Findings {
		if finding.Origin == allowed.Origin {
			t.Fatalf("allowed origin raised %+v", finding)
		}
	}
	policy.Deterministic = append(policy.Deterministic, AllowedOrigin{Origin: "https://cdn.jsdelivr.example", Reason: "pinned catalog-mfe@1 build"})
	if report := buildExternalOrigins(t, policy, externalOriginsRun(t, "run-pass.json")); report.Verdict != ExternalOriginsClean || len(report.Findings) != 0 {
		t.Fatalf("allow-listed CDN: verdict %s findings %+v", report.Verdict, report.Findings)
	}

	valid := string(externalOriginsFixture(t, "policy.json"))
	for name, mutated := range map[string]string{
		"path in origin":        strings.Replace(valid, `"http://127.0.0.1:4010"`, `"http://127.0.0.1:4010/api"`, 1),
		"upper-case origin":     strings.Replace(valid, `"https://assets.example-cdn.test"`, `"https://Assets.example-cdn.test"`, 1),
		"default port":          strings.Replace(valid, `"https://assets.example-cdn.test"`, `"https://assets.example-cdn.test:443"`, 1),
		"declared twice":        strings.Replace(valid, `"http://127.0.0.1:4010"`, `"http://127.0.0.1:3002"`, 1),
		"empty reason":          strings.Replace(valid, `"pinned content-hashed assets"`, `""`, 1),
		"missing environment":   strings.Replace(valid, `"environment": ["http://127.0.0.1:3002", "http://127.0.0.1:4010"],`, ``, 1),
		"unknown member":        strings.Replace(valid, `"deterministic"`, `"mocks": [], "deterministic"`, 1),
		"environment and allow": strings.Replace(valid, `"https://assets.example-cdn.test"`, `"http://127.0.0.1:3002"`, 1),
	} {
		if _, err := DecodeExternalOriginsPolicy([]byte(mutated)); err == nil || !strings.HasPrefix(err.Error(), ExternalOriginsPolicyInvalid+": ") {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

// PWP-V0-026: when one test passes in one run and fails in another at the same revision, the
// correlation names each external request that differed: the module that made no follow-up
// requests and was slower, and the beacon missing from the failed run. Unchanged requests and
// environment requests are not named.
func TestPWPV0026PassFailCorrelationNamesDifferingRequests(t *testing.T) {
	report := buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"), externalOriginsRun(t, "run-fail.json"))
	if len(report.Correlations) != 1 {
		t.Fatalf("correlations = %+v", report.Correlations)
	}
	correlation := report.Correlations[0]
	if correlation.State != CorrelationDiffers || correlation.PassedRun != "ci-1" || correlation.FailedRun != "ci-2" ||
		correlation.Test.Title != "catalog > renders the micro-frontend" || correlation.Revision != "0123456789abcdef0123456789abcdef01234567" {
		t.Fatalf("correlation = %+v", correlation)
	}
	want := []ExternalRequestDifference{
		{
			Origin: "https://analytics.example.test", Locator: "https://analytics.example.test/collect", ResourceType: "beacon",
			Kinds: []string{DifferenceMissingFailed}, Passed: &RequestObservation{Status: 204, DurationMs: 25},
		},
		{
			Origin: "https://cdn.jsdelivr.example", Locator: "https://cdn.jsdelivr.example/npm/catalog-mfe@1/+esm", ResourceType: "module",
			Kinds:  []string{DifferenceFollowUps, DifferenceSlowerInFailed},
			Passed: &RequestObservation{Status: 200, DurationMs: 300, FollowUps: 4}, Failed: &RequestObservation{Status: 200, DurationMs: 15300},
		},
	}
	if !reflect.DeepEqual(correlation.Differences, want) {
		got, _ := json.Marshal(correlation.Differences)
		t.Fatalf("differences = %s", got)
	}

	// A status split alone is named, and a split with identical external requests says so.
	fail := externalOriginsRun(t, "run-fail.json")
	fail.Tests[1].Requests[1] = NetworkRequest{URL: "https://cdn.jsdelivr.example/npm/catalog-mfe@1/+esm", ResourceType: "module", Status: 503, DurationMs: 300, FollowUps: 4}
	fail.Tests[1].Requests = append(fail.Tests[1].Requests, NetworkRequest{URL: "https://analytics.example.test/collect?sid=other", ResourceType: "beacon", Status: 204, DurationMs: 25})
	report = buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"), fail)
	if got := report.Correlations[0].Differences; len(got) != 1 || !reflect.DeepEqual(got[0].Kinds, []string{DifferenceStatus}) {
		t.Fatalf("status split differences = %+v", got)
	}
	fail.Tests[1].Requests[1].Status = 200
	report = buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"), fail)
	if got := report.Correlations[0]; got.State != CorrelationNoDifference || len(got.Differences) != 0 {
		t.Fatalf("identical external requests = %+v", got)
	}

	// Different revisions are never paired.
	fail.Revision = strings.Repeat("f", 40)
	if report := buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"), fail); len(report.Correlations) != 0 {
		t.Fatalf("cross-revision correlation = %+v", report.Correlations)
	}
}

// PWP-V0-027: a test without network evidence is listed as unknown, and a report with no finding
// and any unknown test (or no test) is UNKNOWN, never CLEAN; a correlation missing either side's
// evidence is UNKNOWN.
func TestPWPV0027MissingNetworkEvidenceIsUnknown(t *testing.T) {
	policy := externalOriginsPolicy(t)
	absent := externalOriginsRun(t, "run-absent.json")
	report := buildExternalOrigins(t, policy, absent)
	if report.Verdict != ExternalOriginsUnknown || len(report.Unknown) != 2 || report.Unknown[0].Reason != NetworkEvidenceAbsent ||
		report.Runs[0].Network != NetworkNotProduced || report.Runs[0].TestsWithoutEvidence != 2 {
		t.Fatalf("absent evidence report = %+v", report)
	}

	pass := externalOriginsRun(t, "run-pass.json")
	pass.Tests = pass.Tests[:1] // beacon-only, otherwise CLEAN
	report = buildExternalOrigins(t, policy, pass, absent)
	if report.Verdict != ExternalOriginsUnknown {
		t.Fatalf("clean run plus absent run = %s", report.Verdict)
	}

	empty := absent
	empty.Tests = []NetworkTest{}
	if report := buildExternalOrigins(t, policy, empty); report.Verdict != ExternalOriginsUnknown {
		t.Fatalf("empty run = %s", report.Verdict)
	}

	report = buildExternalOrigins(t, policy, externalOriginsRun(t, "run-pass.json"), absent)
	if report.Verdict != ExternalOriginsDependent || len(report.Correlations) != 1 ||
		report.Correlations[0].State != CorrelationUnknown || report.Correlations[0].Reason != NetworkEvidenceAbsent {
		t.Fatalf("finding plus absent evidence = %s %+v", report.Verdict, report.Correlations)
	}

	if _, err := BuildExternalOriginsReport(policy, nil); err == nil {
		t.Fatal("no run was accepted")
	}
	if _, err := BuildExternalOriginsReport(policy, []NetworkEvidence{absent, absent}); err == nil {
		t.Fatal("a repeated run label was accepted")
	}
}

// PWP-V0-027: the report output is the closed corvint-external-origins/0 document; it lists every
// external origin with its class and counts, and is byte-stable for the same inputs.
func TestPWPV0027ReportOutputListsExternalOrigins(t *testing.T) {
	report := buildExternalOrigins(t, externalOriginsPolicy(t), externalOriginsRun(t, "run-pass.json"), externalOriginsRun(t, "run-fail.json"), externalOriginsRun(t, "run-absent.json"))
	got, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	golden := filepath.Join("testdata", "external-origins", "report.golden.json")
	if *updateExternalOriginsGolden {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if want := externalOriginsFixture(t, "report.golden.json"); !bytes.Equal(got, want) {
		t.Fatalf("report differs from %s:\n%s", golden, got)
	}
	var decoded ExternalOriginsReport
	if err := decodeClosed(got, &decoded); err != nil || !reflect.DeepEqual(decoded, report) {
		t.Fatalf("report does not round-trip closed: %v", err)
	}
	if report.Profile != ExternalOriginsReportProfile || len(report.Origins) != 3 || len(report.Runs) != 3 {
		t.Fatalf("report = %+v", report)
	}
}

// PWP-V0-027: a run that read no test stays NOT_PRODUCED and keeps the verdict UNKNOWN even when
// another run recorded complete evidence; uncertainty is per run, never aggregated away.
func TestPWPV0027EmptyRunStaysUnknownBesideARecordedRun(t *testing.T) {
	const revision = "0123456789abcdef0123456789abcdef01234567"
	recorded := NetworkEvidence{Profile: NetworkEvidenceProfile, Run: "a", Revision: revision,
		Tests: []NetworkTest{{File: "tests/home.spec.ts", Title: "home", Outcome: "passed", Requests: []NetworkRequest{}}}}
	empty := NetworkEvidence{Profile: NetworkEvidenceProfile, Run: "b", Revision: revision, Tests: []NetworkTest{}}
	for _, runs := range [][]NetworkEvidence{{recorded, empty}, {empty, recorded}} {
		report := buildExternalOrigins(t, externalOriginsPolicy(t), runs...)
		if report.Verdict != ExternalOriginsUnknown {
			t.Errorf("verdict = %s, want %s", report.Verdict, ExternalOriginsUnknown)
		}
		for _, run := range report.Runs {
			want := NetworkRecorded
			if run.Run == "b" {
				want = NetworkNotProduced
			}
			if run.Network != want {
				t.Errorf("run %s network = %s, want %s", run.Run, run.Network, want)
			}
		}
	}
	if report := buildExternalOrigins(t, externalOriginsPolicy(t), recorded); report.Verdict != ExternalOriginsClean {
		t.Errorf("recorded run alone verdict = %s, want %s", report.Verdict, ExternalOriginsClean)
	}
}

// PWP-V0-022: status, durationMs and followUps are required and non-null; an omitted or null
// observation refuses network-evidence-invalid instead of decoding as 0.
func TestPWPV0022RequestObservationsAreRequired(t *testing.T) {
	const prefix = `{"profile":"corvint-network-evidence/0","run":"r","revision":"0123456789abcdef0123456789abcdef01234567","tests":[{"file":"f","title":"t","outcome":"passed","requests":[`
	const suffix = `]}]}`
	complete := `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"durationMs":5,"followUps":0}`
	if _, err := DecodeNetworkEvidence([]byte(prefix + complete + suffix)); err != nil {
		t.Fatalf("complete request refused: %v", err)
	}
	for name, request := range map[string]string{
		"status omitted":     `{"url":"https://cdn.example.test/m.js","resourceType":"module","durationMs":5,"followUps":0}`,
		"status null":        `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":null,"durationMs":5,"followUps":0}`,
		"durationMs omitted": `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"followUps":0}`,
		"durationMs null":    `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"durationMs":null,"followUps":0}`,
		"followUps omitted":  `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"durationMs":5}`,
		"followUps null":     `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"durationMs":5,"followUps":null}`,
		"request null":       `null`,
		"unknown member":     `{"url":"https://cdn.example.test/m.js","resourceType":"module","status":200,"durationMs":5,"followUps":0,"extra":1}`,
	} {
		_, err := DecodeNetworkEvidence([]byte(prefix + request + suffix))
		if err == nil || !strings.HasPrefix(err.Error(), NetworkEvidenceInvalid+": ") {
			t.Errorf("%s: err = %v, want %s", name, err, NetworkEvidenceInvalid)
		}
	}
}

// PWP-V0-022, PWP-V0-025: numeric ports and IP hosts are canonicalized before matching, so a
// zero-padded default port or an expanded IPv6 address matches the policy origin, and the policy
// refuses those noncanonical spellings.
func TestPWPV0025PortAndIPSpellingsMatchCanonicalOrigins(t *testing.T) {
	for raw, want := range map[string]string{
		"https://assets.example-cdn.test:0443/app.js":     "https://assets.example-cdn.test",
		"https://assets.example-cdn.test:08443/app.js":    "https://assets.example-cdn.test:8443",
		"http://[0:0:0:0:0:0:0:1]:080/api":                "http://[::1]",
		"http://[2001:DB8:0:0:0:0:0:1]:3002/":             "http://[2001:db8::1]:3002",
		"http://127.0.0.1:03002/":                         "http://127.0.0.1:3002",
		"wss://[0000:0000:0000:0000:0000:0000:0000:0001]": "wss://[::1]",
	} {
		if origin, _, ok, err := RequestOrigin(raw); err != nil || !ok || origin != want {
			t.Errorf("RequestOrigin(%q) = %q %v %v, want %q", raw, origin, ok, err, want)
		}
	}
	if _, _, _, err := RequestOrigin("https://assets.example-cdn.test:70000/"); err == nil {
		t.Error("port above 65535 accepted")
	}

	policy := externalOriginsPolicy(t)
	const revision = "0123456789abcdef0123456789abcdef01234567"
	run := NetworkEvidence{Profile: NetworkEvidenceProfile, Run: "r", Revision: revision, Tests: []NetworkTest{{
		File: "tests/catalog.spec.ts", Title: "catalog", Outcome: "passed", Requests: []NetworkRequest{
			{URL: "https://assets.example-cdn.test:0443/app.js", ResourceType: "script", Status: 200, DurationMs: 3},
			{URL: "http://127.0.0.1:03002/", ResourceType: "document", Status: 200, DurationMs: 3},
		}}}}
	report := buildExternalOrigins(t, policy, run)
	if report.Verdict != ExternalOriginsClean || len(report.Findings) != 0 || len(report.Origins) != 1 ||
		report.Origins[0].Origin != "https://assets.example-cdn.test" || report.Origins[0].Class != OriginAllowed {
		t.Fatalf("report = %+v", report)
	}

	for _, origin := range []string{"https://assets.example-cdn.test:0443", "https://assets.example-cdn.test:443", "http://[0:0:0:0:0:0:0:1]", "HTTPS://assets.example-cdn.test"} {
		document := `{"profile":"corvint-external-origins-policy/0","environment":[],"deterministic":[{"origin":"` + origin + `","reason":"pinned"}]}`
		if _, err := DecodeExternalOriginsPolicy([]byte(document)); err == nil || !strings.HasPrefix(err.Error(), ExternalOriginsPolicyInvalid+": ") {
			t.Errorf("policy origin %q: err = %v, want %s", origin, err, ExternalOriginsPolicyInvalid)
		}
	}
}
