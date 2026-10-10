package ticket

import (
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-206 / V1-1052: the wire decoder's closed facet key sets are the
// ticket enums the summary counts, and its next-action set is every value a
// view can carry.
func TestCALV0206_WireFacetKeysMatchTicketEnums(t *testing.T) {
	for name, pair := range map[string][2][]string{
		"status":         {Statuses, wire.FacetStatuses},
		"priority":       {Priorities, wire.FacetPriorities},
		"kind":           {Kinds, wire.FacetKinds},
		"executionClass": {ExecutionClasses, wire.FacetExecutionClasses},
		"eligibility":    {{EligibilityBlocked, EligibilityUnknown}, wire.FacetEligibilities},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s: ticket %v, wire %v", name, pair[0], pair[1])
		}
	}
	src, err := os.ReadFile("view.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	start := strings.Index(body, "func nextAction(")
	if start < 0 {
		t.Fatal("nextAction not found")
	}
	body = body[start:]
	if end := strings.Index(body, "\n}\n"); end > 0 {
		body = body[:end]
	}
	set := map[string]bool{NextActionCompleteManual: true}
	for _, m := range regexp.MustCompile(`return "([a-z-]+)"`).FindAllStringSubmatch(body, -1) {
		set[m[1]] = true
	}
	var got []string
	for k := range set {
		got = append(got, k)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, wire.FacetNextActions) {
		t.Fatalf("next actions: view %v, wire %v", got, wire.FacetNextActions)
	}
}
