package doccorpus

import (
	jsonstd "encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

const behaviorEventsLive = "testdata/behavior-events/live-1.61.1/report.json"

// DCP-V1-081: the producer's Node unit suite, including the byte-for-byte reproduction of the
// committed report fixture's annotations from synthetic pages and locators.
func TestDCPV1081ProducerNodeUnit(t *testing.T) {
	t.Parallel()
	out, err := exec.Command("node", "--test", "behavior-events_test.cjs").CombinedOutput()
	if err != nil {
		t.Fatalf("behavior-events producer: %v\n%s", err, out)
	}
}

// reportAnnotations returns the first attempt's annotations of one spec title and project.
func reportAnnotations(t *testing.T, raw []byte, title, project string) []any {
	t.Helper()
	var found []any
	editPlaywrightReportTest(t, raw, title, project, func(_, test map[string]any) {
		found = asSlice(asSlice(test["results"])[0].(map[string]any)["annotations"])
	})
	if found == nil {
		t.Fatalf("report has no %q in %s", title, project)
	}
	return found
}

// DCP-V1-081 DCP-V1-082: the live producer run publishes exactly the annotations of the committed
// report fixture, the importer witnesses them, and the other live attempts stay well-shaped.
func TestDCPV1081LiveProducerIsWitnessed(t *testing.T) {
	t.Parallel()
	live, err := os.ReadFile(filepath.FromSlash(behaviorEventsLive))
	if err != nil {
		t.Fatal(err)
	}
	repo := newPlaywrightBehaviorRepo(t)
	produced := reportAnnotations(t, live, "lists items", "alpha")
	if committed := reportAnnotations(t, repo.report, "lists items", "alpha"); !reflect.DeepEqual(produced, committed) {
		t.Fatalf("live producer annotations differ from the committed fixture:\nlive %v\nfixture %v", produced, committed)
	}
	report := editPlaywrightReportTest(t, repo.report, "lists items", "alpha", func(_, test map[string]any) {
		asSlice(test["results"])[0].(map[string]any)["annotations"] = produced
	})
	bundle := importPlaywright(t, repo.request(t, repo.discover(t), nil, nil, nil), "receipt", report)
	if len(bundle.Witnesses) != 1 || bundle.Witnesses[0].TestID != repo.receiptIDs["lists items"] {
		t.Fatalf("live annotations not witnessed: %+v", bundle)
	}

	decoded := func(title string) ([]playwrightReportAnnotation, []BehaviorEvent, string, []string) {
		raw, err := jsonstd.Marshal(reportAnnotations(t, live, title, "alpha"))
		if err != nil {
			t.Fatal(err)
		}
		var annotations []playwrightReportAnnotation
		if err := jsonstd.Unmarshal(raw, &annotations); err != nil {
			t.Fatal(err)
		}
		var events []BehaviorEvent
		for _, annotation := range annotations {
			var event BehaviorEvent
			if annotation.Type == PlaywrightEventAnnotation && decode([]byte(annotation.Description), &event) == nil && behaviorEventShapeOK(event, len(events)+1) {
				events = append(events, event)
			}
		}
		_, _, _, reason, _ := playwrightAnnotations(annotations)
		return annotations, events, reason, playwrightMissingAnnotations(annotations, BehaviorTest{})
	}
	annotations, events, reason, missing := decoded("walks a redirect and a popup")
	navigations := []string{}
	for _, event := range events {
		navigations = append(navigations, event.Kind+":"+event.Navigation+":"+event.ParentPage)
	}
	if len(events) != len(annotations) || !slices.Equal(navigations, []string{"page:redirect:", "page:main-frame:", "page:popup:page-1", "assertion::"}) || reason != "cleanup-unobserved" || !slices.Equal(missing, []string{"cleanup"}) {
		t.Fatalf("redirect and popup attempt: events=%+v reason=%q missing=%v", events, reason, missing)
	}
	annotations, events, _, _ = decoded("records no event for inexact or frame assertions")
	if len(annotations) != 1 || len(events) != 1 || events[0].Kind != "page" {
		t.Fatalf("inexact assertions recorded events: %+v", annotations)
	}
}
