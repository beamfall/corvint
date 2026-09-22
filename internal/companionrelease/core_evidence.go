package companionrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/docmaintain"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

func decodeCoreClosed(raw []byte, target any) error {
	if len(raw) == 0 || len(raw) > installedTotalOutputLimit || !utf8.Valid(raw) {
		return fmt.Errorf("core JSON exceeds UTF-8/size bounds")
	}
	if _, e := wire.Parse(raw); e != nil {
		return e
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(target); e != nil {
		return e
	}
	if e := requireJSONEOF(decoder); e != nil {
		return e
	}
	return requiredCoreFields(raw, reflect.TypeOf(target).Elem())
}
func requiredCoreFields(raw []byte, t reflect.Type) error {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("required core %s cannot be null", t)
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(raw, &fields); e != nil {
			return e
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			name, options, _ := strings.Cut(tag, ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			value, ok := fields[name]
			if !ok {
				if strings.Contains(options, "omitempty") {
					continue
				}
				return fmt.Errorf("required core field %s missing", name)
			}
			if f.Tag.Get("coreNullable") == "true" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				continue
			}
			if e := requiredCoreFields(value, f.Type); e != nil {
				return e
			}
		}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return nil
		}
		var rows []json.RawMessage
		if e := json.Unmarshal(raw, &rows); e != nil {
			return e
		}
		for _, r := range rows {
			if e := requiredCoreFields(r, t.Elem()); e != nil {
				return e
			}
		}
	case reflect.Map:
		var rows map[string]json.RawMessage
		if e := json.Unmarshal(raw, &rows); e != nil {
			return e
		}
		for _, r := range rows {
			if e := requiredCoreFields(r, t.Elem()); e != nil {
				return e
			}
		}
	}
	return nil
}
func coreObject(raw []byte, keys string) (map[string]json.RawMessage, error) {
	if _, e := wire.Parse(raw); e != nil {
		return nil, e
	}
	var value map[string]json.RawMessage
	if e := json.Unmarshal(raw, &value); e != nil {
		return nil, e
	}
	names := strings.Fields(keys)
	if len(value) != len(names) {
		return nil, fmt.Errorf("closed native object field count differs")
	}
	for _, name := range names {
		if _, ok := value[name]; !ok {
			return nil, fmt.Errorf("native field %s missing", name)
		}
	}
	return value, nil
}
func rawString(raw json.RawMessage) string { var s string; _ = json.Unmarshal(raw, &s); return s }
func rawFalse(raw json.RawMessage) bool    { return bytes.Equal(bytes.TrimSpace(raw), []byte("false")) }
func rawBool(raw json.RawMessage) bool     { var b bool; _ = json.Unmarshal(raw, &b); return b }
func evidenceMap(rows []InstalledEvidence, names []string) (map[string]InstalledEvidence, error) {
	if len(rows) != len(names) {
		return nil, fmt.Errorf("core witness count differs")
	}
	allowed := map[string]bool{}
	for _, n := range names {
		allowed[n] = true
	}
	out := map[string]InstalledEvidence{}
	for _, r := range rows {
		if !allowed[r.Name] || out[r.Name].Name != "" || !coreDigest.MatchString(r.SHA256) || len(r.Raw) > installedStageOutputLimit || sha256Hex([]byte(r.Raw)) != r.SHA256 {
			return nil, fmt.Errorf("invalid, duplicate or unknown core evidence %s", r.Name)
		}
		if e := validateCoreNativeJSON([]byte(r.Raw)); e != nil {
			return nil, e
		}
		out[r.Name] = r
	}
	return out, nil
}

// ValidateCoreInstalledReport is the closed core reader. It never accepts a
// historical editor receipt or substitutes an unmeasured axis for execution.
func ValidateCoreInstalledReport(body []byte) error {
	var report CoreInstalledReport
	if e := decodeCoreClosed(body, &report); e != nil {
		return e
	}
	if report.Profile != coreInstalledProfile || report.Status != "PASS" || !coreGitID.MatchString(report.SourceCommit) || !coreGitID.MatchString(report.SourceTree) {
		return fmt.Errorf("invalid core profile/status/source")
	}
	for _, s := range []string{report.BundleSHA256, report.NodeSHA256, report.NPMSHA256, report.PythonSHA256} {
		if !coreDigest.MatchString(s) {
			return fmt.Errorf("invalid core digest")
		}
	}
	identityRows, e := evidenceMap([]InstalledEvidence{report.Identity}, []string{"identity"})
	if e != nil {
		return e
	}
	var identity coreIdentity
	if e = decodeCoreClosed([]byte(identityRows["identity"].Raw), &identity); e != nil {
		return e
	}
	if e = validateCoreIdentity(identity, report); e != nil {
		return e
	}
	providers, e := evidenceMap(report.Providers, coreProviderNames())
	if e != nil {
		return e
	}
	if e = validateCoreProviders(providers); e != nil {
		return e
	}
	docs, e := evidenceMap(report.Docs, coreDocsNames)
	if e != nil {
		return e
	}
	if e = validateCoreDocs(docs); e != nil {
		return e
	}
	roadmap, e := evidenceMap(report.Roadmap, coreRoadmapNames)
	if e != nil {
		return e
	}
	if e = validateCoreRoadmap(roadmap); e != nil {
		return e
	}
	console, e := evidenceMap(report.Console, []string{"browser"})
	if e != nil {
		return e
	}
	var browser coreConsole
	if e = decodeCoreClosed([]byte(console["browser"].Raw), &browser); e != nil {
		return e
	}
	if e = browser.validate(); e != nil {
		return e
	}
	browserBound := false
	for _, r := range identity.Runtimes {
		if r.Name == "browser" && r.SHA256 == browser.BrowserExecutableSHA256 && strings.Contains(r.Version, browser.BrowserVersion) {
			browserBound = true
		}
	}
	if !browserBound {
		return fmt.Errorf("browser witness differs from frozen runtime")
	}
	if len(report.Stages) != len(coreStageNames) {
		return fmt.Errorf("core stage count differs")
	}
	for i, s := range report.Stages {
		if s.Name != coreStageNames[i] || len(s.Stdout)+len(s.Stderr) > installedStageOutputLimit || !utf8.ValidString(s.Stdout) || !utf8.ValidString(s.Stderr) || s.StdoutSHA256 != sha256Hex([]byte(s.Stdout)) || s.StderrSHA256 != sha256Hex([]byte(s.Stderr)) {
			return fmt.Errorf("core stage %s bytes differ", s.Name)
		}
	}
	return nil
}
func validateCoreIdentity(i coreIdentity, r CoreInstalledReport) error {
	if i.Profile != "corvint-core-installed-identity/0" || !i.UnchangedAfter || i.ArchiveSHA256 != r.BundleSHA256 || i.SourceCommit != r.SourceCommit || i.SourceTree != r.SourceTree || !coreGitID.MatchString(i.TasksCommit) || i.Platform.OS == "" || i.Platform.Arch == "" {
		return fmt.Errorf("core identity binding differs")
	}
	for _, s := range []string{i.ChecksumsSHA256, i.SmokeSHA256, i.NPMCacheSHA256, i.BrowserCacheSHA256, i.GoAuthoritySHA256} {
		if !coreDigest.MatchString(s) {
			return fmt.Errorf("core identity digest missing")
		}
	}
	want := []string{"bin/corvint", "bin/corvint-console", "bin/corvint-dashboard-snapshot", "bin/corvint-docs-mcp", "bin/corvint-go-test-provider", "bin/corvint-js-test-provider", "bin/corvint-mcp", "bin/corvint-tasks", "bin/corvint-test-validity-mcp"}
	var got []string
	for _, b := range i.Binaries {
		if !coreDigest.MatchString(b.SHA256) {
			return fmt.Errorf("binary identity missing")
		}
		got = append(got, b.Name)
	}
	if !sameCoreNames(got, want) {
		return fmt.Errorf("core binary inventory differs")
	}
	got = nil
	for _, v := range i.Runtimes {
		if !coreDigest.MatchString(v.SHA256) || v.Version == "" {
			return fmt.Errorf("runtime identity missing")
		}
		got = append(got, v.Name)
		switch v.Name {
		case "node":
			if v.SHA256 != r.NodeSHA256 {
				return fmt.Errorf("Node identity differs")
			}
		case "npm":
			if v.SHA256 != r.NPMSHA256 {
				return fmt.Errorf("npm identity differs")
			}
		case "python":
			if v.SHA256 != r.PythonSHA256 {
				return fmt.Errorf("Python identity differs")
			}
		}
	}
	if !sameCoreNames(got, []string{"node", "npm", "python", "browser", "git", "go"}) {
		return fmt.Errorf("runtime inventory differs")
	}
	got = nil
	for _, f := range i.Fixtures {
		got = append(got, f.Name)
		if !coreDigest.MatchString(f.TreeSHA256) {
			return fmt.Errorf("fixture identity missing")
		}
		for _, s := range []string{f.PackageSHA256, f.LockSHA256, f.ConfigSHA256, f.TestSHA256} {
			if s != "" && !coreDigest.MatchString(s) {
				return fmt.Errorf("fixture binding invalid")
			}
		}
		if strings.HasPrefix(f.Name, "js-") && (f.PackageSHA256 == "" || f.LockSHA256 == "" || f.ConfigSHA256 == "" || f.TestSHA256 == "") {
			return fmt.Errorf("JS fixture binding missing")
		}
	}
	if !sameCoreNames(got, []string{"js-unit", "js-e2e", "go", "docs", "planning"}) {
		return fmt.Errorf("fixture inventory differs")
	}
	return nil
}
func sameCoreNames(got, want []string) bool {
	a := append([]string(nil), got...)
	b := append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	return reflect.DeepEqual(a, b)
}

type coreBirth struct {
	PID         int    `json:"pid"`
	StartTime   string `json:"startTime"`
	Observation string `json:"observation"`
}

func validCoreBirths(observed, remaining []coreBirth) bool {
	if len(observed) == 0 || len(remaining) != 0 {
		return false
	}
	seen := map[string]bool{}
	for _, b := range observed {
		key := fmt.Sprint(b.PID, ":", b.StartTime)
		if b.PID <= 0 || b.StartTime == "" || b.Observation != "ps-lstart-seconds" || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

type coreTransition struct {
	Kind                    string `json:"kind"`
	CommandSHA256           string `json:"commandSha256"`
	FixtureBeforeSHA256     string `json:"fixtureBeforeSha256"`
	FixtureFailedSHA256     string `json:"fixtureFailedSha256"`
	FixtureFixedSHA256      string `json:"fixtureFixedSha256"`
	InitialSHA256           string `json:"initialSha256"`
	FailedSHA256            string `json:"failedSha256"`
	FixedSHA256             string `json:"fixedSha256"`
	PendingFailSHA256       string `json:"pendingFailSha256"`
	PendingFixSHA256        string `json:"pendingFixSha256"`
	RetainedBytesMatch      bool   `json:"retainedBytesMatch"`
	SupersededSuccessAbsent bool   `json:"supersededSuccessAbsent"`
	SupersededProviderRaw   string `json:"supersededProviderRaw"`
	SupersededMcpRaw        string `json:"supersededMcpRaw"`
}
type coreMCPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Meta map[string]struct {
			Description string `json:"description"`
			Name        string `json:"name"`
			Version     string `json:"version"`
		} `json:"_meta"`
		Content []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"content"`
		IsError    bool   `json:"isError"`
		ResultType string `json:"resultType"`
		Structured struct {
			Document json.RawMessage `json:"document"`
			Mutates  bool            `json:"mutates"`
			Receipt  *string         `json:"receipt" coreNullable:"true"`
			Schema   string          `json:"schema"`
			Tool     string          `json:"tool"`
		} `json:"structuredContent"`
	} `json:"result"`
}

func corePair(provider, mcp InstalledEvidence, expected string) (testvaliditydoc.Document, error) {
	input, e := testvaliditydoc.Decode([]byte(provider.Raw))
	if e != nil {
		return testvaliditydoc.Document{}, e
	}
	projected := testvaliditydoc.Project(input)
	if len(projected.Tests) == 0 || projected.TestsOmitted != 0 {
		return projected, fmt.Errorf("provider omitted actual tests")
	}
	for _, t := range projected.Tests {
		if string(t.Projection.Execution.State) != expected || string(t.Projection.Strength.State) != "NOT_MEASURED" {
			return projected, fmt.Errorf("provider execution or strength differs")
		}
	}
	var response coreMCPResponse
	if e = decodeCoreClosed([]byte(mcp.Raw), &response); e != nil {
		return projected, e
	}
	result := response.Result
	info, ok := result.Meta["io.modelcontextprotocol/serverInfo"]
	if response.JSONRPC != "2.0" || response.ID != 1 || !ok || len(result.Meta) != 1 || info.Name != "corvint-test-validity-mcp" || result.IsError || result.ResultType != "complete" || result.Structured.Mutates || result.Structured.Receipt != nil || result.Structured.Schema != "corvint-mcp-test-validity-result/0" || result.Structured.Tool != "corvint.test_validity" {
		return projected, fmt.Errorf("MCP envelope differs")
	}
	var doc testvaliditydoc.Document
	native := json.NewDecoder(bytes.NewReader(result.Structured.Document))
	native.DisallowUnknownFields()
	if e = native.Decode(&doc); e != nil {
		return projected, e
	}
	wantFreshness := "UNKNOWN"
	if projected.Run.Freshness.State == "STALE" {
		wantFreshness = "STALE"
	}
	if doc.Discovery == nil || doc.Discovery.Freshness == nil || !strings.HasPrefix(doc.Discovery.Evidence, ".corvint/test-evidence/") || string(doc.Discovery.Freshness.State) != wantFreshness {
		return doc, fmt.Errorf("MCP lost explicit unverified freshness")
	}
	if doc.Kind == "go-session" && (doc.Tier != "preview" || doc.Promotable == nil || *doc.Promotable) {
		return doc, fmt.Errorf("Go preview evidence promoted")
	}
	projected.Discovery = doc.Discovery
	if projected.Run.Freshness.State != "STALE" {
		projected.Run.Freshness = *doc.Discovery.Freshness
	}
	for j := range projected.Tests {
		if projected.Tests[j].Projection.Freshness.State != "STALE" {
			projected.Tests[j].Projection.Freshness = *doc.Discovery.Freshness
		}
	}
	if !reflect.DeepEqual(projected, doc) {
		return doc, fmt.Errorf("MCP projection differs from original provider")
	}
	return doc, nil
}
func validateCoreProviders(rows map[string]InstalledEvidence) error {
	var transitions struct {
		Profile  string           `json:"profile"`
		Status   string           `json:"status"`
		Sessions []coreTransition `json:"sessions"`
	}
	if e := decodeCoreClosed([]byte(rows["provider-transitions"].Raw), &transitions); e != nil {
		return e
	}
	if transitions.Profile != "corvint-core-provider-transitions/0" || transitions.Status != "PASS" || len(transitions.Sessions) != 3 {
		return fmt.Errorf("provider transition inventory differs")
	}
	kinds := []string{}
	for _, s := range transitions.Sessions {
		kinds = append(kinds, s.Kind)
		p := "js-" + s.Kind
		if s.Kind == "go" {
			p = "go"
		}
		for _, digest := range []string{s.CommandSHA256, s.FixtureBeforeSHA256, s.FixtureFailedSHA256, s.FixtureFixedSHA256} {
			if !coreDigest.MatchString(digest) {
				return fmt.Errorf("transition input digest missing")
			}
		}
		if s.FixtureBeforeSHA256 != s.FixtureFixedSHA256 || s.FixtureFailedSHA256 == s.FixtureFixedSHA256 || !s.RetainedBytesMatch || !s.SupersededSuccessAbsent {
			return fmt.Errorf("automatic transition not observed")
		}
		for _, binding := range []struct{ name, want string }{{"initial-provider", s.InitialSHA256}, {"fail-provider", s.FailedSHA256}, {"fix-provider", s.FixedSHA256}, {"pending-fail-mcp", s.PendingFailSHA256}, {"pending-fix-mcp", s.PendingFixSHA256}} {
			if rows[p+"-"+binding.name].SHA256 != binding.want {
				return fmt.Errorf("transition original bytes differ")
			}
		}
		var previousSequence uint64
		for _, stage := range []struct{ suffix, state string }{{"initial", "PASSED"}, {"fail", "FAILED"}, {"fix", "PASSED"}} {
			provider := rows[p+"-"+stage.suffix+"-provider"]
			if _, e := corePair(provider, rows[p+"-"+stage.suffix+"-mcp"], stage.state); e != nil {
				return e
			}
			if s.Kind == "go" {
				var event struct {
					Sequence uint64 `json:"sequence"`
					Identity string `json:"identity"`
				}
				if e := json.Unmarshal([]byte(provider.Raw), &event); e != nil || event.Sequence <= previousSequence || !coreDigest.MatchString(event.Identity) {
					return fmt.Errorf("Go sequence/identity did not advance")
				}
				previousSequence = event.Sequence
			}
		}
		for _, pending := range []struct{ prior, name, state string }{{"initial", "pending-fail", "PASSED"}, {"fail", "pending-fix", "FAILED"}} {
			doc, e := corePair(rows[p+"-"+pending.prior+"-provider"], rows[p+"-"+pending.name+"-mcp"], pending.state)
			if e != nil {
				return e
			}
			var prior coreMCPResponse
			_ = json.Unmarshal([]byte(rows[p+"-"+pending.prior+"-mcp"].Raw), &prior)
			var priorDoc testvaliditydoc.Document
			if e := json.Unmarshal(prior.Result.Structured.Document, &priorDoc); e != nil || priorDoc.Discovery == nil {
				return fmt.Errorf("prior MCP document missing")
			}
			if doc.Discovery.Evidence != priorDoc.Discovery.Evidence {
				return fmt.Errorf("pending MCP selected another document")
			}
		}
		if s.Kind == "go" {
			var event struct {
				State    string `json:"state"`
				Sequence uint64 `json:"sequence"`
			}
			_ = json.Unmarshal([]byte(s.SupersededProviderRaw), &event)
			if event.State != "stale" || event.Sequence <= previousSequence {
				return fmt.Errorf("Go stale negative missing")
			}
			doc, e := corePair(InstalledEvidence{Raw: s.SupersededProviderRaw}, InstalledEvidence{Raw: s.SupersededMcpRaw}, "FAILED")
			if e != nil {
				return e
			}
			if doc.Run.Freshness.State != "STALE" || doc.Run.Execution.State == "PASSED" {
				return fmt.Errorf("Go stale negative promoted")
			}
		} else if s.SupersededProviderRaw != "" || s.SupersededMcpRaw != "" {
			return fmt.Errorf("JS superseded completion was published")
		}
	}
	if !sameCoreNames(kinds, []string{"unit", "e2e", "go"}) {
		return fmt.Errorf("provider kinds differ")
	}
	var interruption struct {
		Profile string `json:"profile"`
		Status  string `json:"status"`
		Runs    []struct {
			Kind      string      `json:"kind"`
			Signal    string      `json:"signal"`
			ExitCode  *int        `json:"exitCode"`
			Observed  []coreBirth `json:"observedIdentities"`
			Remaining []coreBirth `json:"remainingIdentities"`
			Published bool        `json:"cancelledReceiptPublished"`
		} `json:"runs"`
	}
	if e := decodeCoreClosed([]byte(rows["provider-interruption"].Raw), &interruption); e != nil {
		return e
	}
	var names []string
	if interruption.Profile != "corvint-core-provider-interruption/0" || interruption.Status != "PASS" {
		return fmt.Errorf("interruption profile differs")
	}
	for _, r := range interruption.Runs {
		names = append(names, r.Kind+"-"+r.Signal)
		if r.ExitCode == nil || (*r.ExitCode != 0 && *r.ExitCode != 1) || r.Published || !validCoreBirths(r.Observed, r.Remaining) {
			return fmt.Errorf("interruption observation incomplete")
		}
	}
	if !sameCoreNames(names, []string{"unit-TERM", "unit-INT", "e2e-TERM", "e2e-INT", "go-TERM", "go-INT"}) {
		return fmt.Errorf("interruption matrix incomplete")
	}
	if e := validateCoreCatalogue(rows["test-validity-discover"].Raw, rows["test-validity-list"].Raw, "corvint-test-validity-mcp", []string{"corvint.test_validity"}); e != nil {
		return e
	}
	return nil
}

type coreConsole struct {
	Profile                 string      `json:"profile"`
	Status                  string      `json:"status"`
	BrowserName             string      `json:"browserName"`
	BrowserVersion          string      `json:"browserVersion"`
	BrowserExecutableSHA256 string      `json:"browserExecutableSha256"`
	Tickets                 int         `json:"tickets"`
	Forms                   int         `json:"forms"`
	BodySHA256              string      `json:"bodySha256"`
	SourceDigest            string      `json:"sourceDigest"`
	OriginRefusal           bool        `json:"originRefusal"`
	HostRefusal             bool        `json:"hostRefusal"`
	SessionRefusal          bool        `json:"sessionRefusal"`
	StoreBeforeSHA256       string      `json:"storeBeforeSha256"`
	StoreAfterSHA256        string      `json:"storeAfterSha256"`
	Observed                []coreBirth `json:"observedIdentities"`
	Remaining               []coreBirth `json:"remainingIdentities"`
}

func (c coreConsole) validate() error {
	if c.Profile != "corvint-core-console-proof/0" || c.Status != "PASS" || c.BrowserName != "Chromium" || c.BrowserVersion == "" || c.Tickets != 11 || c.Forms != 0 || !c.OriginRefusal || !c.HostRefusal || !c.SessionRefusal || c.StoreBeforeSHA256 != c.StoreAfterSHA256 || !validCoreBirths(c.Observed, c.Remaining) {
		return fmt.Errorf("real console proof incomplete")
	}
	for _, s := range []string{c.BrowserExecutableSHA256, c.BodySHA256, c.SourceDigest, c.StoreBeforeSHA256} {
		if !coreDigest.MatchString(s) {
			return fmt.Errorf("console identity missing")
		}
	}
	return nil
}

func nativeMCP(raw string) (map[string]json.RawMessage, error) {
	outer, e := coreObject([]byte(raw), "jsonrpc id result")
	if e != nil {
		return nil, e
	}
	if rawString(outer["jsonrpc"]) != "2.0" {
		return nil, fmt.Errorf("native MCP version differs")
	}
	var id int
	if e = json.Unmarshal(outer["id"], &id); e != nil || id <= 0 {
		return nil, fmt.Errorf("native MCP id invalid")
	}
	var result map[string]json.RawMessage
	if e = json.Unmarshal(outer["result"], &result); e != nil {
		return nil, e
	}
	return result, nil
}
func validateCoreCatalogue(discover, list, server string, tools []string) error {
	d, e := nativeMCP(discover)
	if e != nil {
		return e
	}
	var meta map[string]struct {
		Name string `json:"name"`
	}
	if e = json.Unmarshal(d["_meta"], &meta); e != nil || meta["io.modelcontextprotocol/serverInfo"].Name != server {
		return fmt.Errorf("MCP discovery server differs")
	}
	var versions []string
	_ = json.Unmarshal(d["supportedVersions"], &versions)
	if !reflect.DeepEqual(versions, []string{"2026-07-28"}) {
		return fmt.Errorf("MCP protocol differs")
	}
	l, e := nativeMCP(list)
	if e != nil {
		return e
	}
	var rows []struct {
		Name string `json:"name"`
	}
	if e = json.Unmarshal(l["tools"], &rows); e != nil {
		return e
	}
	names := []string{}
	for _, r := range rows {
		names = append(names, r.Name)
	}
	if !sameCoreNames(names, tools) {
		return fmt.Errorf("MCP tool inventory differs")
	}
	return nil
}
func validateCoreDocs(rows map[string]InstalledEvidence) error {
	var watch docmaintain.WatchReceipt
	if e := decodeCoreClosed([]byte(rows["watch-receipt.json"].Raw), &watch); e != nil {
		return e
	}
	if watch.Profile != "corvint-docmaintain-watch/0" || watch.Writes != 2 || watch.StoppedReason != "max-writes" || watch.OmittedSummaries != 0 {
		return fmt.Errorf("automatic docs watch not observed")
	}
	if e := validateCoreCatalogue(rows["mcp-discover.json"].Raw, rows["mcp-list.json"].Raw, "corvint-docs-mcp", []string{"corvint.docs_consume", "corvint.docs_draft"}); e != nil {
		return e
	}
	draft, e := nativeMCP(rows["mcp-draft.json"].Raw)
	if e != nil {
		return e
	}
	consume, e := nativeMCP(rows["mcp-consume.json"].Raw)
	if e != nil {
		return e
	}
	if !rawFalse(draft["isError"]) || !rawFalse(consume["isError"]) {
		return fmt.Errorf("docs MCP failed")
	}
	d, e := coreObject(draft["structuredContent"], "behavior commit derivation limitations markdown markdown_sha256 mutates package schema source state tool tree")
	if e != nil {
		return e
	}
	c, e := coreObject(consume["structuredContent"], "behavior commit derivation draft_sha256 limitations profile results state task tree validation")
	if e != nil {
		return e
	}
	markdown := rawString(d["markdown"])
	if markdown == "" || sha256Hex([]byte(markdown)) != rawString(d["markdown_sha256"]) || rawString(c["draft_sha256"]) != rawString(d["markdown_sha256"]) || rawString(c["validation"]) != "SOURCE_REDERIVED" || !rawFalse(d["mutates"]) {
		return fmt.Errorf("docs original draft/consume binding differs")
	}
	var conflict struct {
		Profile        string `json:"profile"`
		Status         string `json:"status"`
		HumanBefore    string `json:"humanBeforeSha256"`
		HumanAfter     string `json:"humanAfterSha256"`
		EligibleCommit string `json:"eligibleCommit"`
		Exit           int    `json:"refusalExitCode"`
		Receipt        string `json:"refusalReceipt"`
	}
	if e = decodeCoreClosed([]byte(rows["conflict-receipt.json"].Raw), &conflict); e != nil {
		return e
	}
	if conflict.Profile != "corvint-core-docs-conflict/0" || conflict.Status != "PASS" || conflict.Exit != 2 || !coreDigest.MatchString(conflict.HumanBefore) || conflict.HumanBefore != conflict.HumanAfter || !coreGitID.MatchString(conflict.EligibleCommit) {
		return fmt.Errorf("human docs conflict not preserved")
	}
	var refused docmaintain.WatchReceipt
	if e = decodeCoreClosed([]byte(conflict.Receipt), &refused); e != nil {
		return e
	}
	if refused.Profile != watch.Profile || refused.StoppedReason != "maintenance-conflict" || refused.Writes != 1 {
		return fmt.Errorf("native docs refusal differs")
	}
	return nil
}

func taskEnvelope(raw []byte, command []string, outcome string) (map[string]json.RawMessage, error) {
	v, e := coreObject(raw, "profile command outcome codes snapshot mutation items page untrusted warnings")
	if e != nil {
		return nil, e
	}
	var argv []string
	_ = json.Unmarshal(v["command"], &argv)
	if rawString(v["profile"]) != "taskman-command-result/0" || !reflect.DeepEqual(argv, command) || rawString(v["outcome"]) != outcome || string(v["mutation"]) != "null" {
		return nil, fmt.Errorf("native Tasks result differs")
	}
	return v, nil
}
func validateCoreRoadmap(rows map[string]InstalledEvidence) error {
	type seed struct {
		Profile      string `json:"profile"`
		Phase        string `json:"phase"`
		SourceSHA256 string `json:"sourceSha256"`
		Before       string `json:"storeBeforeSha256"`
		After        string `json:"storeAfterSha256"`
		Stdout       string `json:"stdout"`
	}
	var first, replay seed
	if e := decodeCoreClosed([]byte(rows["tasks-seed-first"].Raw), &first); e != nil {
		return e
	}
	if e := decodeCoreClosed([]byte(rows["tasks-seed-replay"].Raw), &replay); e != nil {
		return e
	}
	if first.Profile != "corvint-core-tasks-seed/0" || replay.Profile != first.Profile || first.Phase != "first" || replay.Phase != "replay" || replay.Before != replay.After || first.SourceSHA256 != replay.SourceSHA256 || !coreDigest.MatchString(first.SourceSHA256) || !coreDigest.MatchString(first.Before) || !coreDigest.MatchString(first.After) || !coreDigest.MatchString(replay.Before) || first.After != replay.Before || first.Stdout == "" || replay.Stdout == "" {
		return fmt.Errorf("real seed replay not idempotent")
	}
	roadmap, e := taskEnvelope([]byte(rows["tasks-roadmap"].Raw), []string{"roadmap"}, "OK")
	if e != nil {
		return e
	}
	var tickets []json.RawMessage
	if e = json.Unmarshal(roadmap["items"], &tickets); e != nil || len(tickets) != 11 {
		return fmt.Errorf("roadmap count differs")
	}
	var ids []string
	for _, raw := range tickets {
		v, e := coreObject(raw, "milestone ticketId title status kind priority order owner requiredGates gateResults eligibility nextAction")
		if e != nil {
			return e
		}
		ids = append(ids, rawString(v["ticketId"]))
	}
	want := []string{}
	for n := 1; n <= 11; n++ {
		want = append(want, fmt.Sprintf("ticket:corvint:planning:IPR-%02d", n))
	}
	if !sameCoreNames(ids, want) {
		return fmt.Errorf("roadmap IDs differ")
	}
	var details struct {
		Profile   string   `json:"profile"`
		Responses []string `json:"responses"`
	}
	if e = decodeCoreClosed([]byte(rows["tasks-ticket-details"].Raw), &details); e != nil {
		return e
	}
	if details.Profile != "corvint-core-tasks-details/0" || len(details.Responses) != 11 {
		return fmt.Errorf("ticket details omitted")
	}
	ids = nil
	for _, raw := range details.Responses {
		v, e := taskEnvelope([]byte(raw), []string{"ticket", "show"}, "OK")
		if e != nil {
			return e
		}
		var items []json.RawMessage
		_ = json.Unmarshal(v["items"], &items)
		if len(items) != 1 {
			return fmt.Errorf("detail item count differs")
		}
		item, e := coreObject(items[0], "acceptanceRevision archivedFrom blockers completion currentAttempt eligibility executionClass gateResults holds intentChecks kind milestone nextAction order owner priority publication record requiredGates revision status ticketId title tracked unknowns")
		if e != nil {
			return e
		}
		record, e := coreObject(item["record"], "acceptanceCriteria acceptanceRevision approvals archivedFrom body capabilities completion createdAt dependencies dueDate effects estimateMinutes executionClass holds kind labels milestone order owner previousRecordSha256 priority profile requiredGates requirementRefs revision shadowOverlay source status supersededBy supersedes ticketId title updatedAt updatedBy")
		if e != nil {
			return e
		}
		if rawString(record["executionClass"]) != "MANUAL" {
			return fmt.Errorf("planning ticket is executable")
		}
		ids = append(ids, rawString(record["ticketId"]))
	}
	if !sameCoreNames(ids, want) {
		return fmt.Errorf("detail IDs differ")
	}
	var noDispatch struct {
		Profile string `json:"profile"`
		Queue   string `json:"queueStatusRaw"`
		Attempt string `json:"attemptNotRunRaw"`
		Before  string `json:"stateBeforeSha256"`
		After   string `json:"stateAfterSha256"`
		Absent  bool   `json:"laneArtifactsAbsent"`
	}
	if e = decodeCoreClosed([]byte(rows["tasks-no-dispatch"].Raw), &noDispatch); e != nil {
		return e
	}
	if noDispatch.Profile != "corvint-core-tasks-no-dispatch/0" || !noDispatch.Absent || !coreDigest.MatchString(noDispatch.Before) || noDispatch.Before != noDispatch.After {
		return fmt.Errorf("no-dispatch observation differs")
	}
	q, e := taskEnvelope([]byte(noDispatch.Queue), []string{"queue", "status"}, "OK")
	if e != nil {
		return e
	}
	var qrows []json.RawMessage
	_ = json.Unmarshal(q["items"], &qrows)
	if len(qrows) != 1 {
		return fmt.Errorf("queue status missing")
	}
	status, e := coreObject(qrows[0], "attempts barrier blocked byStatus canonicalWriter executionCutover fixture generation headSeq intentBranch intentChecksPassed policySha256 publication queueId tickets writeBarrier")
	if e != nil {
		return e
	}
	if !rawBool(status["fixture"]) || !rawFalse(status["executionCutover"]) || rawString(status["attempts"]) != "NOT_OBSERVED" || rawString(status["publication"]) != "NOT_OBSERVED" || rawString(status["tickets"]) != "11" {
		return fmt.Errorf("native queue uncertainty promoted")
	}
	_, e = taskEnvelope([]byte(noDispatch.Attempt), []string{"attempt"}, "NOT_RUN")
	return e
}
