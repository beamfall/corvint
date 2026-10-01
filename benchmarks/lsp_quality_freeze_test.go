package benchmarks_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The frozen LSP qualification record is the contract downstream LSP slices build on
// (docs/specs/lsp-quality-platform-v0.md, LQP-V0-019..021). Its schema is closed so a
// field such as held-out gold cannot be added silently: every nested object, including each
// public baseline and its results, is a closed struct decoded with DisallowUnknownFields.
type lspFreezeRecord struct {
	Profile            string                `json:"profile"`
	Version            string                `json:"version"`
	Ticket             string                `json:"ticket"`
	FrozenAt           string                `json:"frozenAt"`
	Status             string                `json:"status"`
	Policy             lspFileBinding        `json:"policy"`
	PublicCorpus       lspCorpusBinding      `json:"publicCorpus"`
	Platform           lspPlatform           `json:"platform"`
	Providers          []lspIdentity         `json:"providers"`
	Clients            []lspIdentity         `json:"clients"`
	CorvintBuild       lspRule               `json:"corvintBuild"`
	Tuples             []lspTuple            `json:"tuples"`
	Unsupported        []string              `json:"unsupported"`
	RealRepositories   []lspRepository       `json:"realRepositories"`
	ProtectedHeldout   lspHeldout            `json:"protectedHeldout"`
	PublicBaselines    []lspPublicBaseline   `json:"publicBaselines"`
	Metrics            []lspMetric           `json:"metrics"`
	AcceptanceCriteria []lspAcceptanceStatus `json:"acceptanceCriteria"`
	Limits             []string              `json:"limits"`
}

type lspFileBinding struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type lspCorpusBinding struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	SourceBase string `json:"sourceBase"`
	Cases      int    `json:"cases"`
	Protected  bool   `json:"protected"`
	Gold       string `json:"gold"`
}

type lspPlatform struct {
	ID                      string `json:"id"`
	OS                      string `json:"os"`
	Arch                    string `json:"arch"`
	Go                      string `json:"go"`
	GoSDKCopyManifestSHA256 string `json:"goSdkCopyManifestSha256"`
	ReleaseAuthenticity     string `json:"releaseAuthenticity"`
}

type lspIdentity struct {
	ID                  string            `json:"id"`
	Version             string            `json:"version"`
	Digests             map[string]string `json:"digests"`
	PositionEncoding    string            `json:"positionEncoding,omitempty"`
	ReleaseAuthenticity string            `json:"releaseAuthenticity"`
	Notes               string            `json:"notes,omitempty"`
}

type lspRule struct {
	State string `json:"state"`
	Rule  string `json:"rule"`
}

type lspTuple struct {
	ID               string   `json:"id"`
	Consumer         string   `json:"consumer"`
	Platform         string   `json:"platform"`
	Provider         string   `json:"provider"`
	Client           *string  `json:"client"`
	Entry            string   `json:"entry"`
	PositionEncoding string   `json:"positionEncoding"`
	Layouts          []string `json:"layouts"`
	Methods          []string `json:"methods"`
	Unsupported      []string `json:"unsupported"`
	Budgets          []string `json:"budgets"`
	Status           string   `json:"status"`
	OwningTickets    []string `json:"owningTickets"`
}

type lspRepository struct {
	ID         string   `json:"id"`
	Repository string   `json:"repository"`
	Ref        string   `json:"ref"`
	Revision   string   `json:"revision"`
	Layouts    []string `json:"layouts"`
	State      string   `json:"state"`
	Blockers   []string `json:"blockers"`
}

type lspHeldout struct {
	State          string  `json:"state"`
	RequiredTasks  int     `json:"requiredTasks"`
	RequiredRepos  int     `json:"requiredRepositories"`
	Custodian      string  `json:"custodian"`
	ManifestSHA256 *string `json:"manifestSha256"`
	Reason         string  `json:"reason"`
}

// A public baseline is descriptive calibration only (LQP-V0-020). Its closed shape has no place
// for tasks, gold or held-out results (LQP-V0-021).
type lspPublicBaseline struct {
	ID              string             `json:"id"`
	Profile         string             `json:"profile"`
	Command         string             `json:"command"`
	CorvintCommit   string             `json:"corvintCommit"`
	CorvintSHA256   string             `json:"corvintSha256"`
	Revision        string             `json:"revision"`
	CorpusSHA256    string             `json:"corpusSha256"`
	ReportSHA256    string             `json:"reportSha256"`
	ReportRetention string             `json:"reportRetention"`
	Repeats         int                `json:"repeats"`
	HostLoadAverage string             `json:"hostLoadAverage"`
	Results         lspBaselineResults `json:"results"`
	Interpretation  string             `json:"interpretation"`
}

type lspBaselineResults struct {
	UpstreamDefinitionExactSpan string `json:"upstreamDefinitionExactSpan,omitempty"`
	UpstreamDefinitionPath      string `json:"upstreamDefinitionPath,omitempty"`
	CoreExpectedPath            string `json:"coreExpectedPath,omitempty"`
	CombinedExpectedRelation    string `json:"combinedExpectedRelation,omitempty"`
	CombinedProviderStates      string `json:"combinedProviderStates,omitempty"`
	CorePacketParity            string `json:"corePacketParity,omitempty"`
	SampleExitCodes             string `json:"sampleExitCodes,omitempty"`
	ExitCode                    *int   `json:"exitCode,omitempty"`
}

type lspMetric struct {
	PolicyKey         string          `json:"policyKey"`
	Floor             json.RawMessage `json:"floor"`
	Heldout           lspMeasure      `json:"heldout"`
	PublicCalibration *string         `json:"publicCalibration"`
}

type lspMeasure struct {
	State          string          `json:"state"`
	Value          json.RawMessage `json:"value"`
	EvidenceSHA256 *string         `json:"evidenceSha256"`
	Reason         string          `json:"reason"`
}

type lspAcceptanceStatus struct {
	Criterion int    `json:"criterion"`
	State     string `json:"state"`
	Evidence  string `json:"evidence"`
}

// lspFreezeRecordSHA256 pins qualification-freeze-v0.json, the digest downstream claims cite
// (LQP-V0-019). Version 0 is frozen: any in-place edit fails here, and a change to a floor, tuple,
// corpus, custody rule or baseline needs owner acceptance and a new record version (LQP-V0-020).
const lspFreezeRecordSHA256 = "3c6b8990c4be8fce06882139392ae54d9e8eaf5cf5a5d80aa3646acd94328663"

var lspSHA256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
var lspGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Provider and client digests name external binaries, archives and packages outside the
// repository. The test checks their format only; it never re-hashes those artifacts.
var lspIdentityDigest = regexp.MustCompile(`^([0-9a-f]{64}|sha512-[A-Za-z0-9+/=]+|[0-9a-f]{40})$`)

func lspFreezeRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source root unavailable")
	}
	return filepath.Dir(filepath.Dir(source))
}

func lspFileSHA256(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// lspDecodeFreeze decodes the closed schema and requires every top-level field to be present and
// non-empty, so deleting or blanking status, ticket, limits or any other field is refused.
func lspDecodeFreeze(data []byte) (lspFreezeRecord, error) {
	var record lspFreezeRecord
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, fmt.Errorf("decode: %w", err)
	}
	var present map[string]json.RawMessage
	if err := json.Unmarshal(data, &present); err != nil {
		return record, fmt.Errorf("decode: %w", err)
	}
	fields := reflect.TypeOf(record)
	for index := 0; index < fields.NumField(); index++ {
		name := strings.Split(fields.Field(index).Tag.Get("json"), ",")[0]
		raw, ok := present[name]
		value := strings.Join(strings.Fields(string(raw)), "")
		if !ok || value == "null" || value == `""` || value == "[]" || value == "{}" {
			return record, fmt.Errorf("required top-level field %q missing or empty", name)
		}
	}
	return record, nil
}

func lspCheckTuples(record lspFreezeRecord) error {
	platforms := map[string]bool{record.Platform.ID: true}
	if !lspSHA256Hex.MatchString(record.Platform.GoSDKCopyManifestSHA256) || record.Platform.ReleaseAuthenticity != "NOT_OBSERVED" {
		return fmt.Errorf("platform identity must carry a digest and unobserved authenticity: %+v", record.Platform)
	}
	identities := map[string]map[string]bool{"provider": {}, "client": {}}
	for kind, list := range map[string][]lspIdentity{"provider": record.Providers, "client": record.Clients} {
		for _, identity := range list {
			if identity.ID == "" || identity.Version == "" || len(identity.Digests) == 0 {
				return fmt.Errorf("%s identity incomplete: %+v", kind, identity)
			}
			for name, value := range identity.Digests {
				if !lspIdentityDigest.MatchString(value) {
					return fmt.Errorf("%s %s digest %s malformed: %q", kind, identity.ID, name, value)
				}
			}
			if identity.ReleaseAuthenticity != "NOT_OBSERVED" && identity.ReleaseAuthenticity != "OBSERVED" {
				return fmt.Errorf("%s %s authenticity %q", kind, identity.ID, identity.ReleaseAuthenticity)
			}
			identities[kind][identity.ID] = true
		}
	}
	consumers := map[string]bool{"agent-cli": true, "agent-mcp": true, "editor-definition": true, "editor-context": true}
	seen := map[string]bool{}
	for _, tuple := range record.Tuples {
		if seen[tuple.ID] {
			return fmt.Errorf("duplicate tuple %s", tuple.ID)
		}
		seen[tuple.ID] = true
		if !consumers[tuple.Consumer] || !platforms[tuple.Platform] || !identities["provider"][tuple.Provider] {
			return fmt.Errorf("tuple %s cites undeclared consumer/platform/provider", tuple.ID)
		}
		editor := tuple.Consumer == "editor-definition" || tuple.Consumer == "editor-context"
		if editor != (tuple.Client != nil) || (tuple.Client != nil && !identities["client"][*tuple.Client]) {
			return fmt.Errorf("tuple %s client binding inconsistent with consumer", tuple.ID)
		}
		if tuple.Status != "UNQUALIFIED" && tuple.Status != "FALLBACK" && tuple.Status != "UNSUPPORTED" {
			return fmt.Errorf("tuple %s status %q: a frozen record cannot qualify a tuple", tuple.ID, tuple.Status)
		}
		if tuple.Entry == "" || tuple.PositionEncoding == "" || len(tuple.Layouts) == 0 || len(tuple.Methods) == 0 ||
			len(tuple.Unsupported) == 0 || len(tuple.Budgets) == 0 || len(tuple.OwningTickets) == 0 {
			return fmt.Errorf("tuple %s incomplete", tuple.ID)
		}
	}
	if len(record.Tuples) == 0 || len(record.Unsupported) == 0 {
		return errors.New("tuples and unsupported cases are required")
	}
	if record.CorvintBuild.State != "PER_CANDIDATE" {
		return fmt.Errorf("Corvint build must be bound per candidate, got %q", record.CorvintBuild.State)
	}
	return nil
}

func lspCheckFloors(root string, record lspFreezeRecord) error {
	fileSHA256 := func(path string) (string, error) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:]), nil
	}
	if record.Policy.Path != "docs/specs/lsp-qualification-evaluation-policy-v0.json" {
		return fmt.Errorf("policy binding drifted: %+v", record.Policy)
	}
	if sum, err := fileSHA256(record.Policy.Path); err != nil || sum != record.Policy.SHA256 {
		return fmt.Errorf("policy binding drifted: %+v (%v)", record.Policy, err)
	}
	if sum, err := fileSHA256(record.PublicCorpus.Path); err != nil || sum != record.PublicCorpus.SHA256 ||
		!lspGitObject.MatchString(record.PublicCorpus.SourceBase) || record.PublicCorpus.Protected {
		return fmt.Errorf("public corpus binding drifted: %+v (%v)", record.PublicCorpus, err)
	}
	policyBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(record.Policy.Path)))
	if err != nil {
		return err
	}
	var policy struct {
		Protocol map[string]json.RawMessage `json:"protocol"`
	}
	if err := json.Unmarshal(policyBytes, &policy); err != nil {
		return err
	}
	want := map[string]any{}
	for _, group := range []string{"hardFloors", "qualityFloors", "resourceFloors"} {
		var floors map[string]any
		if err := json.Unmarshal(policy.Protocol[group], &floors); err != nil || len(floors) == 0 {
			return fmt.Errorf("policy %s unreadable: %v", group, err)
		}
		for key, value := range floors {
			want[group+"."+key] = value
		}
	}
	got := map[string]bool{}
	for _, metric := range record.Metrics {
		floor, ok := want[metric.PolicyKey]
		if !ok || got[metric.PolicyKey] {
			return fmt.Errorf("metric %q is unknown to the policy or duplicated", metric.PolicyKey)
		}
		got[metric.PolicyKey] = true
		var mirrored any
		if err := json.Unmarshal(metric.Floor, &mirrored); err != nil || !reflect.DeepEqual(mirrored, floor) {
			return fmt.Errorf("metric %s floor %s does not mirror policy %v", metric.PolicyKey, metric.Floor, floor)
		}
		measure := metric.Heldout
		switch measure.State {
		case "MEASURED":
			if len(measure.Value) == 0 || string(measure.Value) == "null" || measure.EvidenceSHA256 == nil || !lspSHA256Hex.MatchString(*measure.EvidenceSHA256) {
				return fmt.Errorf("metric %s measured without value and evidence digest", metric.PolicyKey)
			}
		case "NOT_RUN", "NOT_OBSERVED":
			if (len(measure.Value) != 0 && string(measure.Value) != "null") || measure.EvidenceSHA256 != nil || measure.Reason == "" {
				return fmt.Errorf("metric %s %s must carry no value and a reason", metric.PolicyKey, measure.State)
			}
		default:
			return fmt.Errorf("metric %s held-out state %q", metric.PolicyKey, measure.State)
		}
	}
	var missing []string
	for key := range want {
		if !got[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("policy floors without a baseline state: %v", missing)
	}
	if len(record.PublicBaselines) == 0 {
		return errors.New("public calibration baseline evidence is required")
	}
	baselines := map[string]bool{}
	for _, baseline := range record.PublicBaselines {
		if baseline.ID == "" || baselines[baseline.ID] || baseline.Profile != "corvint-lsp-public-baseline/0" ||
			baseline.Command == "" || (baseline.CorvintCommit != "NOT_OBSERVED" && !lspGitObject.MatchString(baseline.CorvintCommit)) ||
			!lspSHA256Hex.MatchString(baseline.CorvintSHA256) || !lspGitObject.MatchString(baseline.Revision) ||
			!lspSHA256Hex.MatchString(baseline.CorpusSHA256) || !lspSHA256Hex.MatchString(baseline.ReportSHA256) ||
			baseline.ReportRetention == "" || baseline.Repeats <= 0 || baseline.HostLoadAverage == "" ||
			baseline.Results == (lspBaselineResults{}) || baseline.Interpretation == "" {
			return fmt.Errorf("public baseline %q incomplete, duplicated or malformed", baseline.ID)
		}
		baselines[baseline.ID] = true
	}
	return nil
}

func lspCheckHeldout(record lspFreezeRecord) error {
	heldout := record.ProtectedHeldout
	if heldout.RequiredTasks != 20 || heldout.RequiredRepos != 3 || heldout.Custodian == "" || heldout.Reason == "" {
		return fmt.Errorf("held-out custody incomplete: %+v", heldout)
	}
	switch heldout.State {
	case "NOT_PRODUCED":
		if heldout.ManifestSHA256 != nil {
			return errors.New("NOT_PRODUCED held-out cannot carry a manifest digest")
		}
	case "FROZEN":
		if heldout.ManifestSHA256 == nil || !lspSHA256Hex.MatchString(*heldout.ManifestSHA256) {
			return errors.New("FROZEN held-out requires a manifest digest")
		}
	default:
		return fmt.Errorf("held-out state %q", heldout.State)
	}
	if len(record.RealRepositories) < 3 {
		return fmt.Errorf("need at least three real repository pins, got %d", len(record.RealRepositories))
	}
	for _, repository := range record.RealRepositories {
		if !lspGitObject.MatchString(repository.Revision) || repository.State == "" || len(repository.Layouts) == 0 {
			return fmt.Errorf("repository pin incomplete: %+v", repository)
		}
		if repository.State != "ADMITTED" && len(repository.Blockers) == 0 {
			return fmt.Errorf("unadmitted repository %s must name its blockers", repository.ID)
		}
		if repository.State == "ADMITTED" && len(repository.Blockers) != 0 {
			return fmt.Errorf("admitted repository %s still lists blockers %v", repository.ID, repository.Blockers)
		}
	}
	baselineNotRun := false
	for _, metric := range record.Metrics {
		baselineNotRun = baselineNotRun || metric.Heldout.State == "NOT_RUN"
	}
	if len(record.AcceptanceCriteria) != 4 {
		return fmt.Errorf("expected four V1-0478 acceptance criteria, got %d", len(record.AcceptanceCriteria))
	}
	for index, criterion := range record.AcceptanceCriteria {
		met := criterion.State == "MET"
		if criterion.Criterion != index+1 || (!met && criterion.State != "PARTIAL" && criterion.State != "NOT_MET") || criterion.Evidence == "" {
			return fmt.Errorf("acceptance criterion %+v", criterion)
		}
		if met && criterion.Criterion == 2 && heldout.State != "FROZEN" {
			return errors.New("criterion 2 cannot be MET before the protected held-out is frozen")
		}
		if met && (criterion.Criterion == 1 || criterion.Criterion == 3) && (heldout.State == "NOT_PRODUCED" || baselineNotRun) {
			return fmt.Errorf("criterion %d cannot be MET while the protected held-out is %s or a held-out baseline is NOT_RUN", criterion.Criterion, heldout.State)
		}
	}
	return nil
}

func lspValidateFreeze(root string, data []byte) error {
	record, err := lspDecodeFreeze(data)
	if err != nil {
		return err
	}
	for _, check := range []func() error{
		func() error { return lspCheckTuples(record) },
		func() error { return lspCheckFloors(root, record) },
		func() error { return lspCheckHeldout(record) },
	} {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func TestLSPQualificationFreezeRecord(t *testing.T) {
	root := lspFreezeRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "benchmarks", "lsp-quality", "qualification-freeze-v0.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := lspDecodeFreeze(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.Profile != "corvint-lsp-qualification-freeze/0" || record.Version != "0" {
		t.Fatalf("profile/version = %q/%q", record.Profile, record.Version)
	}

	t.Run("LQP-V0-019 record digest is pinned against in-place edits", func(t *testing.T) {
		if got := lspFileSHA256(t, root, "benchmarks/lsp-quality/qualification-freeze-v0.json"); got != lspFreezeRecordSHA256 {
			t.Fatalf("qualification-freeze-v0.json sha256 %s, pinned %s: version 0 is frozen; publish a change as a new record version", got, lspFreezeRecordSHA256)
		}
	})
	t.Run("LQP-V0-019 tuples cite declared identities and none is qualified", func(t *testing.T) {
		if err := lspCheckTuples(record); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("LQP-V0-020 floors mirror the accepted policy and every baseline is explicit", func(t *testing.T) {
		if err := lspCheckFloors(root, record); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("LQP-V0-021 protected held-out stays custodian-owned and blocks promotion", func(t *testing.T) {
		if err := lspCheckHeldout(record); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("LQP-V0-019..021 mutated records are rejected", func(t *testing.T) {
		at := func(value any, path ...any) any {
			for _, step := range path {
				switch key := step.(type) {
				case string:
					value = value.(map[string]any)[key]
				case int:
					value = value.([]any)[key]
				}
			}
			return value
		}
		object := func(doc map[string]any, path ...any) map[string]any { return at(doc, path...).(map[string]any) }
		cases := []struct {
			name   string
			mutate func(doc map[string]any)
			want   string
		}{
			{"unchanged record", func(map[string]any) {}, ""},
			{"unknown top-level field", func(doc map[string]any) { doc["heldoutGold"] = "x" }, `unknown field "heldoutGold"`},
			{"gold in a public baseline", func(doc map[string]any) { object(doc, "publicBaselines", 0)["heldoutGold"] = []any{"x"} }, `unknown field "heldoutGold"`},
			{"tasks in public baseline results", func(doc map[string]any) { object(doc, "publicBaselines", 0, "results")["heldoutTasks"] = 20 }, `unknown field "heldoutTasks"`},
			{"malformed public baseline digest", func(doc map[string]any) { object(doc, "publicBaselines", 1)["reportSha256"] = "abc" }, "public baseline"},
			{"public baseline without interpretation", func(doc map[string]any) { delete(object(doc, "publicBaselines", 0), "interpretation") }, "public baseline"},
			{"deleted status", func(doc map[string]any) { delete(doc, "status") }, `required top-level field "status"`},
			{"deleted ticket", func(doc map[string]any) { delete(doc, "ticket") }, `required top-level field "ticket"`},
			{"deleted limits", func(doc map[string]any) { delete(doc, "limits") }, `required top-level field "limits"`},
			{"deleted frozenAt", func(doc map[string]any) { delete(doc, "frozenAt") }, `required top-level field "frozenAt"`},
			{"blank status", func(doc map[string]any) { doc["status"] = "" }, `required top-level field "status"`},
			{"empty limits", func(doc map[string]any) { doc["limits"] = []any{} }, `required top-level field "limits"`},
			{"null public baselines", func(doc map[string]any) { doc["publicBaselines"] = nil }, `required top-level field "publicBaselines"`},
			{"qualified tuple", func(doc map[string]any) { object(doc, "tuples", 0)["status"] = "QUALIFIED" }, "cannot qualify a tuple"},
			{"drifted floor", func(doc map[string]any) { object(doc, "metrics", 0)["floor"] = 1 }, "does not mirror policy"},
			{"missing floor metric", func(doc map[string]any) {
				metrics := doc["metrics"].([]any)
				doc["metrics"] = metrics[:len(metrics)-1]
			}, "policy floors without a baseline state"},
			{"AC1 MET while held-out NOT_PRODUCED", func(doc map[string]any) { object(doc, "acceptanceCriteria", 0)["state"] = "MET" }, "criterion 1 cannot be MET"},
			{"AC2 MET while held-out NOT_PRODUCED", func(doc map[string]any) { object(doc, "acceptanceCriteria", 1)["state"] = "MET" }, "criterion 2 cannot be MET"},
			{"AC3 MET while held-out NOT_PRODUCED", func(doc map[string]any) { object(doc, "acceptanceCriteria", 2)["state"] = "MET" }, "criterion 3 cannot be MET"},
			{"AC3 MET on a frozen held-out with baselines NOT_RUN", func(doc map[string]any) {
				heldout := object(doc, "protectedHeldout")
				heldout["state"] = "FROZEN"
				heldout["manifestSha256"] = strings.Repeat("a", 64)
				object(doc, "acceptanceCriteria", 2)["state"] = "MET"
			}, "criterion 3 cannot be MET"},
			{"admitted repository with blockers", func(doc map[string]any) { object(doc, "realRepositories", 0)["state"] = "ADMITTED" }, "admitted repository"},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				var doc map[string]any
				if err := json.Unmarshal(data, &doc); err != nil {
					t.Fatal(err)
				}
				testCase.mutate(doc)
				mutated, err := json.Marshal(doc)
				if err != nil {
					t.Fatal(err)
				}
				err = lspValidateFreeze(root, mutated)
				switch {
				case testCase.want == "" && err != nil:
					t.Fatalf("unmutated record rejected: %v", err)
				case testCase.want != "" && (err == nil || !strings.Contains(err.Error(), testCase.want)):
					t.Fatalf("mutation accepted or rejected for the wrong reason: got %v, want %q", err, testCase.want)
				}
			})
		}
	})
}
