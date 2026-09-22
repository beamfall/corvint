package adapters

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

func TestRegistryIsClosedSortedAndDefensivelyCopied(t *testing.T) {
	first := Registry()
	if len(first) != 11 {
		t.Fatalf("registry rows = %d, want 11", len(first))
	}
	ids := make([]string, len(first))
	for index, descriptor := range first {
		ids[index] = string(descriptor.AdapterID)
		if descriptor.AdapterID == "" || descriptor.SourceKind == "" || descriptor.VerifierID == "" {
			t.Fatalf("incomplete registry row: %+v", descriptor)
		}
	}
	want := append([]string(nil), ids...)
	sort.Strings(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("registry is not sorted: %v", ids)
	}
	first[0].AcceptedProfiles = append(first[0].AcceptedProfiles, "hostile")
	first[0].IssueCodes = append(first[0].IssueCodes, "hostile")
	first[0].VerifierID = "hostile"
	second := Registry()
	if second[0].VerifierID == "hostile" || contains(second[0].AcceptedProfiles, "hostile") || contains(second[0].IssueCodes, "hostile") {
		t.Fatal("caller mutated compiled registry")
	}
}

func TestRegistrationsProduceFrozenRegistryHash(t *testing.T) {
	_, encoded, err := model.Compile(model.Input{
		GeneratedAt: "2026-08-23T12:00:00.000000000Z",
		Observation: model.ObservationInput{
			ClockSource: model.ClockCaller, Start: "2026-08-23T12:00:00.000000000Z",
			End: "2026-08-23T12:00:00.000000000Z", ScanState: model.ScanComplete,
		},
		Repository: model.Repository{WorktreeState: model.WorktreeUnknown},
		Registry:   Registrations(),
	})
	if err != nil {
		t.Fatalf("compile registry: %v", err)
	}
	if !strings.Contains(string(encoded), `"adapterRegistrySha256":"`+model.ExpectedAdapterRegistrySHA256+`"`) {
		t.Fatalf("snapshot omitted frozen registry hash: %s", encoded)
	}
}

func TestUndeliveredAdaptersAreExplicitlyNotObserved(t *testing.T) {
	for _, adapterID := range []AdapterID{
		AdapterCEMOCMBundle, AdapterQueryEnvelope, AdapterImpactEnvelope,
		AdapterHeadSpecIndex, AdapterBeamfallShadow, AdapterPulseDogfood,
		AdapterHarnessUsage, AdapterGoLiveUsage, AdapterFrontierUsage,
	} {
		state, ok := Unavailable(adapterID)
		if !ok {
			t.Fatalf("missing unavailable state for %s", adapterID)
		}
		if state.EpistemicClass != "NOT_OBSERVED" || state.Completeness != "UNKNOWN" || state.AuthorityClass != "NONE" {
			t.Fatalf("dishonest unavailable state for %s: %+v", adapterID, state)
		}
	}
	state, _ := Unavailable(AdapterCEMOCMBundle)
	if state.DeliveryStage != DeliveryNotStarted || state.Validity != "UNSUPPORTED" {
		t.Fatalf("CEM/OCM base state = %+v", state)
	}
	descriptor, _ := Lookup(AdapterCEMOCMBundle)
	if descriptor.DefaultLocation != nil {
		t.Fatal("CEM/OCM adapter must not discover a current-path bundle")
	}
	if _, ok := Unavailable(AdapterLocalTrace); ok {
		t.Fatal("implemented trace adapter reported unavailable base state")
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
