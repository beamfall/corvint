package benchmarks_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"testing"
)

// The frozen LSP qualification record is the contract downstream LSP slices build on
// (docs/specs/lsp-quality-platform-v0.md, LQP-V0-019..021). Its schema is closed so a
// field such as held-out gold cannot be added silently.
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
	PublicBaselines    []json.RawMessage     `json:"publicBaselines"`
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

var lspSHA256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)
var lspGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)

func lspFreezeRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source root unavailable")
	}
	return filepath.Dir(filepath.Dir(source))
}

func lspStrictDecode(t *testing.T, data []byte, into any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		t.Fatalf("decode: %v", err)
	}
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

func TestLSPQualificationFreezeRecord(t *testing.T) {
	root := lspFreezeRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "benchmarks", "lsp-quality", "qualification-freeze-v0.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record lspFreezeRecord
	lspStrictDecode(t, data, &record)
	if record.Profile != "corvint-lsp-qualification-freeze/0" || record.Version != "0" {
		t.Fatalf("profile/version = %q/%q", record.Profile, record.Version)
	}

	t.Run("LQP-V0-019 tuples cite declared identities and none is qualified", func(t *testing.T) {
		platforms := map[string]bool{record.Platform.ID: true}
		if !lspSHA256Hex.MatchString(record.Platform.GoSDKCopyManifestSHA256) || record.Platform.ReleaseAuthenticity != "NOT_OBSERVED" {
			t.Fatalf("platform identity must carry a digest and unobserved authenticity: %+v", record.Platform)
		}
		identities := map[string]map[string]bool{"provider": {}, "client": {}}
		for kind, list := range map[string][]lspIdentity{"provider": record.Providers, "client": record.Clients} {
			for _, identity := range list {
				if identity.ID == "" || identity.Version == "" || len(identity.Digests) == 0 {
					t.Fatalf("%s identity incomplete: %+v", kind, identity)
				}
				for name, value := range identity.Digests {
					if !lspSHA256Hex.MatchString(value) && !regexp.MustCompile(`^(sha512-[A-Za-z0-9+/=]+|[0-9a-f]{40})$`).MatchString(value) {
						t.Fatalf("%s %s digest %s malformed: %q", kind, identity.ID, name, value)
					}
				}
				if identity.ReleaseAuthenticity != "NOT_OBSERVED" && identity.ReleaseAuthenticity != "OBSERVED" {
					t.Fatalf("%s %s authenticity %q", kind, identity.ID, identity.ReleaseAuthenticity)
				}
				identities[kind][identity.ID] = true
			}
		}
		consumers := map[string]bool{"agent-cli": true, "agent-mcp": true, "editor-definition": true, "editor-context": true}
		seen := map[string]bool{}
		for _, tuple := range record.Tuples {
			if seen[tuple.ID] {
				t.Fatalf("duplicate tuple %s", tuple.ID)
			}
			seen[tuple.ID] = true
			if !consumers[tuple.Consumer] || !platforms[tuple.Platform] || !identities["provider"][tuple.Provider] {
				t.Fatalf("tuple %s cites undeclared consumer/platform/provider", tuple.ID)
			}
			editor := tuple.Consumer == "editor-definition" || tuple.Consumer == "editor-context"
			if editor != (tuple.Client != nil) || (tuple.Client != nil && !identities["client"][*tuple.Client]) {
				t.Fatalf("tuple %s client binding inconsistent with consumer", tuple.ID)
			}
			if tuple.Status != "UNQUALIFIED" && tuple.Status != "FALLBACK" && tuple.Status != "UNSUPPORTED" {
				t.Fatalf("tuple %s status %q: a frozen record cannot qualify a tuple", tuple.ID, tuple.Status)
			}
			if tuple.Entry == "" || tuple.PositionEncoding == "" || len(tuple.Layouts) == 0 || len(tuple.Methods) == 0 ||
				len(tuple.Unsupported) == 0 || len(tuple.Budgets) == 0 || len(tuple.OwningTickets) == 0 {
				t.Fatalf("tuple %s incomplete", tuple.ID)
			}
		}
		if len(record.Tuples) == 0 || len(record.Unsupported) == 0 {
			t.Fatal("tuples and unsupported cases are required")
		}
		if record.CorvintBuild.State != "PER_CANDIDATE" {
			t.Fatalf("Corvint build must be bound per candidate, got %q", record.CorvintBuild.State)
		}
	})

	t.Run("LQP-V0-020 floors mirror the accepted policy and every baseline is explicit", func(t *testing.T) {
		if record.Policy.Path != "docs/specs/lsp-qualification-evaluation-policy-v0.json" ||
			lspFileSHA256(t, root, record.Policy.Path) != record.Policy.SHA256 {
			t.Fatalf("policy binding drifted: %+v", record.Policy)
		}
		if lspFileSHA256(t, root, record.PublicCorpus.Path) != record.PublicCorpus.SHA256 ||
			!lspGitObject.MatchString(record.PublicCorpus.SourceBase) || record.PublicCorpus.Protected {
			t.Fatalf("public corpus binding drifted: %+v", record.PublicCorpus)
		}
		policyBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(record.Policy.Path)))
		if err != nil {
			t.Fatal(err)
		}
		var policy struct {
			Protocol map[string]json.RawMessage `json:"protocol"`
		}
		if err := json.Unmarshal(policyBytes, &policy); err != nil {
			t.Fatal(err)
		}
		want := map[string]any{}
		for _, group := range []string{"hardFloors", "qualityFloors", "resourceFloors"} {
			var floors map[string]any
			if err := json.Unmarshal(policy.Protocol[group], &floors); err != nil || len(floors) == 0 {
				t.Fatalf("policy %s unreadable: %v", group, err)
			}
			for key, value := range floors {
				want[group+"."+key] = value
			}
		}
		got := map[string]bool{}
		for _, metric := range record.Metrics {
			floor, ok := want[metric.PolicyKey]
			if !ok || got[metric.PolicyKey] {
				t.Fatalf("metric %q is unknown to the policy or duplicated", metric.PolicyKey)
			}
			got[metric.PolicyKey] = true
			var mirrored any
			if err := json.Unmarshal(metric.Floor, &mirrored); err != nil || !reflect.DeepEqual(mirrored, floor) {
				t.Fatalf("metric %s floor %s does not mirror policy %v", metric.PolicyKey, metric.Floor, floor)
			}
			measure := metric.Heldout
			switch measure.State {
			case "MEASURED":
				if len(measure.Value) == 0 || string(measure.Value) == "null" || measure.EvidenceSHA256 == nil || !lspSHA256Hex.MatchString(*measure.EvidenceSHA256) {
					t.Fatalf("metric %s measured without value and evidence digest", metric.PolicyKey)
				}
			case "NOT_RUN", "NOT_OBSERVED":
				if (len(measure.Value) != 0 && string(measure.Value) != "null") || measure.EvidenceSHA256 != nil || measure.Reason == "" {
					t.Fatalf("metric %s %s must carry no value and a reason", metric.PolicyKey, measure.State)
				}
			default:
				t.Fatalf("metric %s held-out state %q", metric.PolicyKey, measure.State)
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
			t.Fatalf("policy floors without a baseline state: %v", missing)
		}
		if len(record.PublicBaselines) == 0 {
			t.Fatal("public calibration baseline evidence is required")
		}
	})

	t.Run("LQP-V0-021 protected held-out stays custodian-owned and blocks promotion", func(t *testing.T) {
		heldout := record.ProtectedHeldout
		if heldout.RequiredTasks != 20 || heldout.RequiredRepos != 3 || heldout.Custodian == "" || heldout.Reason == "" {
			t.Fatalf("held-out custody incomplete: %+v", heldout)
		}
		switch heldout.State {
		case "NOT_PRODUCED":
			if heldout.ManifestSHA256 != nil {
				t.Fatal("NOT_PRODUCED held-out cannot carry a manifest digest")
			}
		case "FROZEN":
			if heldout.ManifestSHA256 == nil || !lspSHA256Hex.MatchString(*heldout.ManifestSHA256) {
				t.Fatal("FROZEN held-out requires a manifest digest")
			}
		default:
			t.Fatalf("held-out state %q", heldout.State)
		}
		if len(record.RealRepositories) < 3 {
			t.Fatalf("need at least three real repository pins, got %d", len(record.RealRepositories))
		}
		for _, repository := range record.RealRepositories {
			if !lspGitObject.MatchString(repository.Revision) || repository.State == "" || len(repository.Layouts) == 0 {
				t.Fatalf("repository pin incomplete: %+v", repository)
			}
			if repository.State != "ADMITTED" && len(repository.Blockers) == 0 {
				t.Fatalf("unadmitted repository %s must name its blockers", repository.ID)
			}
		}
		if len(record.AcceptanceCriteria) != 4 {
			t.Fatalf("expected four V1-0478 acceptance criteria, got %d", len(record.AcceptanceCriteria))
		}
		for index, criterion := range record.AcceptanceCriteria {
			met := criterion.State == "MET"
			if criterion.Criterion != index+1 || (!met && criterion.State != "PARTIAL" && criterion.State != "NOT_MET") || criterion.Evidence == "" {
				t.Fatalf("acceptance criterion %+v", criterion)
			}
			if met && criterion.Criterion == 2 && heldout.State != "FROZEN" {
				t.Fatal("criterion 2 cannot be MET before the protected held-out is frozen")
			}
		}
	})
}
