package appflows

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func extractorImportFixture(t *testing.T, schema string) []byte {
	t.Helper()
	subject := func(kind, name, path string, lines ...int) doccorpus.Subject {
		s := doccorpus.Subject{ID: "app:" + kind + ":" + name, Kind: kind, Name: name, Provider: "app"}
		for _, line := range lines {
			s.Evidence.Anchors = append(s.Evidence.Anchors, doccorpus.Anchor{Path: path, Start: line, End: line, Kind: "source"})
		}
		return s
	}
	inventory := doccorpus.ExtractorInventory{Schema: schema, Status: "generated", Provider: doccorpus.ProviderRecord{Subjects: []doccorpus.Subject{
		subject("endpoint", "GET /orders", "app/config/routes.rb", 2),
		subject("endpoint", "POST /orders", "app/config/routes.rb", 3, 9),
		subject("ui_surface", "/orders", "app/web/router.ts", 2),
		subject("job", "OrderMailerJob", "app/jobs/order_mailer_job.rb", 1),
		subject("data_entity", "orders", "app/db/schema.rb", 1),
	}}}
	data, err := json.Marshal(inventory)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// AFU-V1-122 AFU-V1-123 AFU-V1-124: an extractor inventory becomes proposed api and ui flows with
// one inferred source link per exact subject anchor; other subject kinds are not flows.
func TestAFUV1122ImportExtractorInventory(t *testing.T) {
	root := intentRepo(t)
	written, err := Import(root, "flows", extractorImportFixture(t, doccorpus.ExtractorInventorySchema), FormatExtractor, "inventory.json")
	if err != nil || !reflect.DeepEqual(written, []string{"flows/get-orders.json", "flows/post-orders.json", "flows/ui-orders.json"}) {
		t.Fatalf("extractor import %v %v", written, err)
	}
	set, err := LoadIntents(root, "flows")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]FlowIntent{}
	for _, f := range set.Flows {
		byID[f.FlowID] = f
	}
	imported := []FlowIntent{byID["get-orders"], byID["post-orders"], byID["ui-orders"]}
	assertProposed(t, imported)
	post := byID["post-orders"]
	if post.Kind != "api" || post.Steps[0].Action != "POST /orders" || post.Variations[0].VariationID != "post-orders.call" || len(post.Links) != 2 ||
		post.Links[1].Target != (LinkTarget{Type: "source", Path: "app/config/routes.rb", StartLine: 9, EndLine: 9}) {
		t.Fatalf("endpoint flow %+v", post)
	}
	ui := byID["ui-orders"]
	if ui.Kind != "ui" || ui.Steps[0].Action != "visit /orders" || ui.Links[0].Target != (LinkTarget{Type: "source", Path: "app/web/router.ts", StartLine: 2, EndLine: 2}) {
		t.Fatalf("ui flow %+v", ui)
	}
	if len(set.Flows) != 4 {
		t.Fatalf("job or data_entity subject imported as a flow: %d flows", len(set.Flows))
	}
}

// AFU-V1-122: only the extractor inventory schema is admitted, and a refusal writes nothing.
func TestAFUV1122ImportExtractorInventoryRefusals(t *testing.T) {
	root := intentRepo(t)
	if _, err := Import(root, "flows", extractorImportFixture(t, "corvint-extractor-inventory/0"), FormatExtractor, ""); err == nil || !strings.Contains(err.Error(), doccorpus.ExtractorInventorySchema) {
		t.Fatalf("foreign schema accepted: %v", err)
	}
	unanchored, _ := json.Marshal(doccorpus.ExtractorInventory{Schema: doccorpus.ExtractorInventorySchema, Provider: doccorpus.ProviderRecord{Subjects: []doccorpus.Subject{{ID: "app:endpoint:GET /", Kind: "endpoint", Name: "GET /"}}}})
	if _, err := Import(root, "flows", unanchored, FormatExtractor, ""); err == nil || !strings.Contains(err.Error(), "no flow identity or anchor") {
		t.Fatalf("unanchored subject accepted: %v", err)
	}
	if set, _ := LoadIntents(root, "flows"); len(set.Flows) != 1 {
		t.Fatalf("refused import wrote %d flows", len(set.Flows))
	}
	if _, err := Import(root, "flows", []byte("{}"), "yaml", ""); err == nil || !strings.Contains(err.Error(), "extractor-inventory") {
		t.Fatalf("format refusal omits extractor-inventory: %v", err)
	}
}
