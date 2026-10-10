package doccorpus

import (
	"bytes"
	jsonstd "encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func behaviorAdapterEditDocument(t *testing.T, request *BehaviorAdapterRequest, inputID string, edit func(map[string]any)) {
	t.Helper()
	input := behaviorAdapterRequestInput(request, inputID)
	var document map[string]any
	if err := jsonstd.Unmarshal([]byte(input.Document), &document); err != nil {
		t.Fatal(err)
	}
	edit(document)
	input.Document = string(behaviorAdapterRaw(t, document))
	input.Anchor.SHA256 = Digest([]byte(input.Document))
	input.Anchor.SpanSHA256 = input.Anchor.SHA256
}

func behaviorAdapterCheckStates(report BehaviorAdapterCheck) map[string]BehaviorAdapterCheckStage {
	states := map[string]BehaviorAdapterCheckStage{}
	for _, stage := range report.Stages {
		states[stage.Stage] = stage
	}
	return states
}

// TestBehaviorAdapterCheckParity proves DCP-V1-044: a check is accepted exactly
// when Build succeeds, its first refusal is Build's error, and the report is
// deterministic.
func TestBehaviorAdapterCheckParity(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	other := strings.Repeat("e", 40)
	previousResult := buildBehaviorAdapter(t, fixture.request, nil)
	previousRaw, err := Encode(previousResult)
	if err != nil {
		t.Fatal(err)
	}
	staleResult := previousResult
	staleResult.Fallback = "none"
	staleRaw, err := Encode(staleResult)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		previous []byte
		raw      []byte
		edit     func(*testing.T, *BehaviorAdapterRequest)
	}{
		{name: "accepted"},
		{name: "accepted with previous", previous: previousRaw},
		{name: "request decode", raw: []byte(`{"schema":`)},
		{name: "previous decode", previous: []byte(`[]`)},
		{name: "previous lineage", previous: staleRaw},
		{name: "identity", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Schema = "unknown" }},
		{name: "revisions", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.DocumentationRevision = "bad" }},
		{name: "mapping bound", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Mappings = r.Mappings[:3] }},
		{name: "input anchor", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterRequestInput(r, "flows").Anchor.Repository = other
		}},
		{name: "required input", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.MigrationInput = "absent" }},
		{name: "mapping field", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Mappings[3].Fields["bogus"] = "/bogus" }},
		{name: "observation identity", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Observations[0].RunID = "bad" }},
		{name: "migration", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "migration", func(document map[string]any) { document["contract_id"] = "other" })
		}},
		{name: "discovery", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "discovery", func(document map[string]any) { document["mode"] = "static" })
		}},
		{name: "flow shape", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, r, "flows", func(row map[string]any) { row["pages"] = "not-a-list" })
		}},
		{name: "test project", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, r, "tests", func(row map[string]any) { row["browserProject"] = "" })
		}},
		{name: "observation subject", edit: func(t *testing.T, r *BehaviorAdapterRequest) { r.Observations[0].Subject = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
			request.Mappings = append([]BehaviorAdapterMapping(nil), fixture.request.Mappings...)
			request.Mappings[3].Fields = map[string]string{}
			for key, value := range fixture.request.Mappings[3].Fields {
				request.Mappings[3].Fields[key] = value
			}
			request.Observations = append([]ObservationLink(nil), fixture.request.Observations...)
			if tc.edit != nil {
				tc.edit(t, &request)
			}
			raw := tc.raw
			if raw == nil {
				raw = behaviorAdapterRaw(t, request)
			}
			_, buildErr := BuildBehaviorAdapter(raw, tc.previous)
			report := CheckBehaviorAdapter(raw, tc.previous)
			if report.Accepted != (buildErr == nil) {
				t.Fatalf("accepted=%v build error=%v report=%+v", report.Accepted, buildErr, report)
			}
			if buildErr == nil {
				if len(report.Refusals) != 0 {
					t.Fatalf("accepted report lists refusals: %+v", report.Refusals)
				}
			} else {
				var refused *Error
				if !errors.As(buildErr, &refused) {
					t.Fatalf("build error is untyped: %v", buildErr)
				}
				first := report.Refusals[0]
				if first.State != "refused" || first.Code != refused.Code || first.Message != refused.Message {
					t.Fatalf("first refusal %+v, build refused %q", first, refused.Message)
				}
			}
			again := CheckBehaviorAdapter(raw, tc.previous)
			left, leftErr := Encode(report)
			right, rightErr := Encode(again)
			if leftErr != nil || rightErr != nil || !bytes.Equal(left, right) {
				t.Fatalf("check report is not deterministic: %v %v", leftErr, rightErr)
			}
			if bytes.Contains(left, []byte(legacyV1Member(t))) {
				t.Fatalf("check report vocabulary carries the legacy key: %s", left)
			}
		})
	}
}

// TestBehaviorAdapterCheckReportsEveryRefusal proves DCP-V1-044 lists refusals
// from independent stages at once and marks dependent stages not-evaluated.
func TestBehaviorAdapterCheckReportsEveryRefusal(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	request := fixture.request
	request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
	request.Mappings = append([]BehaviorAdapterMapping(nil), fixture.request.Mappings...)
	request.Schema = "unknown"
	behaviorAdapterRequestInput(&request, "flows").Anchor.Repository = strings.Repeat("e", 40)
	behaviorAdapterRequestInput(&request, "candidates").Anchor.Blob = "bad"
	request.Mappings[1].Fields = map[string]string{"id": "/variationKey"}
	candidateMapping := -1
	for index, mapping := range request.Mappings {
		if mapping.Input == "candidates" {
			candidateMapping = index
			fields := map[string]string{}
			for name, pointer := range mapping.Fields {
				if name != "id" {
					fields[name] = pointer
				}
			}
			request.Mappings[index].Fields = fields
		}
	}
	raw := behaviorAdapterRaw(t, request)
	if _, err := BuildBehaviorAdapter(raw, nil); err == nil || !strings.Contains(err.Error(), "invalid behavior adapter identity") {
		t.Fatalf("build should stop at the identity refusal: %v", err)
	}
	report := CheckBehaviorAdapter(raw, nil)
	if report.Accepted || report.Fallback != "full-relevant-suite" || len(report.Limitations) == 0 {
		t.Fatalf("report header: %+v", report)
	}
	refused := map[string]int{}
	blocked := map[string][]string{}
	order := []string{}
	for _, refusal := range report.Refusals {
		if refusal.State == "refused" {
			refused[refusal.Stage]++
		} else {
			blocked[refusal.Stage] = refusal.BlockedBy
		}
		if len(order) == 0 || order[len(order)-1] != refusal.Stage {
			order = append(order, refusal.Stage)
		}
	}
	if refused["identity"] != 1 || refused["inputs"] != 2 || refused["mappings"] != 2 {
		t.Fatalf("independent refusals missing: %+v", report.Refusals)
	}
	for stage, want := range map[string]string{"flows": "mappings", "variations": "mappings", "candidates": "mappings", "observations": "identity", "migration": "identity", "declarations": "flows"} {
		if len(blocked[stage]) == 0 || blocked[stage][0] != want {
			t.Fatalf("stage %s blocked_by=%v, want %s first: %+v", stage, blocked[stage], want, report.Refusals)
		}
	}
	if !strings.Contains(strings.Join(func() []string {
		messages := []string{}
		for _, refusal := range report.Refusals {
			if refusal.Stage == "mappings" && refusal.State == "not-evaluated" {
				messages = append(messages, refusal.Message)
			}
		}
		return messages
	}(), "\n"), "record checks for /mappings/0 not evaluated because input flows was refused") {
		t.Fatalf("mapping of a refused input is not reported as not-evaluated: %+v", report.Refusals)
	}
	mappingRefused := false
	for _, refusal := range report.Refusals {
		if refusal.Stage == "mappings" && refusal.State == "refused" && strings.Contains(refusal.Message, "required field mapping is missing: candidates.id") {
			mappingRefused = true
		}
	}
	if candidateMapping < 0 || !mappingRefused {
		t.Fatalf("mapping of a refused input was not validated as its own item: %+v", report.Refusals)
	}
	states := behaviorAdapterCheckStates(report)
	if states["discovery"].State != "passed" || states["tests"].State != "passed" || states["previous"].State != "not-applicable" {
		t.Fatalf("independent stages were not evaluated: %+v", report.Stages)
	}
	stageIndex := map[string]int{}
	for index, stage := range report.Stages {
		stageIndex[stage.Stage] = index
	}
	for index := 1; index < len(order); index++ {
		if stageIndex[order[index-1]] >= stageIndex[order[index]] {
			t.Fatalf("refusals are not in stage order: %v", order)
		}
	}
}

// TestBehaviorAdapterCheckIndependentRefusals proves DCP-V1-044 evaluates each
// item independently of the other items in its stage: Build's first refusal is
// unchanged, and the check also reports another item's refusal.
func TestBehaviorAdapterCheckIndependentRefusals(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	for _, tc := range []struct {
		name   string
		edit   func(*testing.T, *BehaviorAdapterRequest)
		stage  string
		wanted []string
	}{
		{name: "declaration field and duplicate identity", stage: "declarations", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			var flows map[string]any
			if err := jsonstd.Unmarshal([]byte(behaviorAdapterRequestInput(r, "flows").Document), &flows); err != nil {
				t.Fatal(err)
			}
			flowKey := flows["inventory"].(map[string]any)["items"].([]any)[0].(map[string]any)["flowKey"]
			behaviorAdapterEditRow(t, r, "candidates", func(row map[string]any) { row["candidateKey"] = flowKey })
			behaviorAdapterEditRow(t, r, "tests", func(row map[string]any) { row["browserProject"] = "" })
		}, wanted: []string{"mapped test project is invalid", "mapped behavior identity is duplicated"}},
		{name: "discovery project and duplicate execution", stage: "discovery", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "discovery", func(document map[string]any) {
				document["executions"] = []any{map[string]any{"id": "duplicate", "project": ""}, map[string]any{"id": "duplicate", "project": "chromium"}}
			})
		}, wanted: []string{"discovery project is missing", "discovery execution identity is duplicate or invalid"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
			request.Observations = append([]ObservationLink(nil), fixture.request.Observations...)
			tc.edit(t, &request)
			raw := behaviorAdapterRaw(t, request)
			_, buildErr := BuildBehaviorAdapter(raw, nil)
			var refused *Error
			if !errors.As(buildErr, &refused) || !strings.Contains(refused.Message, tc.wanted[0]) {
				t.Fatalf("build should refuse first with %q: %v", tc.wanted[0], buildErr)
			}
			report := CheckBehaviorAdapter(raw, nil)
			if report.Accepted || report.Refusals[0].Code != refused.Code || report.Refusals[0].Message != refused.Message {
				t.Fatalf("first refusal %+v, build refused %q", report.Refusals, refused.Message)
			}
			found := 0
			for _, want := range tc.wanted {
				for _, refusal := range report.Refusals {
					if refusal.Stage == tc.stage && refusal.State == "refused" && strings.Contains(refusal.Message, want) {
						found++
						break
					}
				}
			}
			if found != len(tc.wanted) {
				t.Fatalf("check omitted an independent refusal %v: %+v", tc.wanted, report.Refusals)
			}
		})
	}
}

// TestBehaviorAdapterCheckListsDependentChecks proves DCP-V1-044 lists the
// checks that depend on a refusal in the same stage as not-evaluated.
func TestBehaviorAdapterCheckListsDependentChecks(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	for _, tc := range []struct {
		name, stage, detail string
		edit                func(*testing.T, *BehaviorAdapterRequest)
	}{
		{name: "undecodable discovery", stage: "discovery", detail: "discovery executions not evaluated because the discovery record cannot be decoded", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditDocument(t, r, "discovery", func(document map[string]any) { document["unexpected"] = true })
		}},
		{name: "refused flow record", stage: "flows", detail: "remaining checks for flows record 0 not evaluated after behavior adapter input=flows field=/inventory/items/0/pages", edit: func(t *testing.T, r *BehaviorAdapterRequest) {
			behaviorAdapterEditRow(t, r, "flows", func(row map[string]any) { row["pages"] = "not-a-list" })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
			tc.edit(t, &request)
			report := CheckBehaviorAdapter(behaviorAdapterRaw(t, request), nil)
			for _, refusal := range report.Refusals {
				if refusal.Stage == tc.stage && refusal.State == "not-evaluated" && strings.HasPrefix(refusal.Message, tc.detail) && len(refusal.BlockedBy) == 1 && refusal.BlockedBy[0] == tc.stage {
					if behaviorAdapterCheckStates(report)[tc.stage].State != "refused" {
						t.Fatalf("stage %s should be refused: %+v", tc.stage, report.Stages)
					}
					return
				}
			}
			t.Fatalf("dependent check not listed as not-evaluated: %+v", report.Refusals)
		})
	}
}

// TestBehaviorAdapterCheckStopsAtFirstItemRefusal proves DCP-V1-044: within one
// item, evaluation stops at the item's first refusal and exactly one
// not-evaluated entry names the item's remaining checks.
func TestBehaviorAdapterCheckStopsAtFirstItemRefusal(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	for _, tc := range []struct {
		name, stage, want, absent string
		edit                      func(*testing.T, *BehaviorAdapterRequest) string
	}{
		{name: "input blob and path", stage: "inputs", want: "repository, revision or blob identity is invalid", absent: "../outside", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			input := behaviorAdapterRequestInput(r, "flows")
			input.Anchor.Blob = "bad"
			input.Anchor.Path = "../outside"
			for index := range r.Inputs {
				if r.Inputs[index].ID == "flows" {
					return "/inputs/" + strconv.Itoa(index)
				}
			}
			t.Fatal("flows input missing")
			return ""
		}},
		{name: "observation identity and source mapping", stage: "observations", want: "invalid behavior adapter observation identity", absent: "invalid behavior adapter observation source mapping", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			r.Observations[0].RunID = "bad"
			r.Observations[0].SourcePaths = map[string]string{"/repo/test.ts": "../outside"}
			return "/observations/0"
		}},
		{name: "receipt run identity", stage: "artifacts", want: "with the run identity digest", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			migration := behaviorAdapterRequestInput(r, r.MigrationInput)
			r.Observations[0].Input = migration.Anchor.Path
			r.Observations[0].InputRevision = migration.Anchor.Revision
			r.Observations[0].RunID = strings.Repeat("a", 64)
			return "observation " + r.Observations[0].ID
		}},
		{name: "empty discovery execution identity", stage: "discovery", want: "discovery execution identity is duplicate or invalid", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			behaviorAdapterEditDocument(t, r, "discovery", func(document map[string]any) {
				document["executions"] = []any{map[string]any{"id": "", "project": "chromium"}}
			})
			return "discovery execution 0"
		}},
		{name: "empty candidate identity", stage: "candidates", want: "candidate identity is duplicate or invalid", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			behaviorAdapterEditRow(t, r, "candidates", func(row map[string]any) { row["candidateKey"] = "" })
			return "candidates record 0"
		}},
		{name: "duplicate candidate identity", stage: "candidates", want: "candidate identity is duplicate or invalid", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			id := ""
			behaviorAdapterEditDocument(t, r, "candidates", func(document map[string]any) {
				items := document["inventory"].(map[string]any)["items"].([]any)
				id = items[0].(map[string]any)["candidateKey"].(string)
				document["inventory"].(map[string]any)["items"] = append(items, items[0])
			})
			return "candidate " + id
		}},
		{name: "empty test identity", stage: "tests", want: "test identity is duplicate or invalid", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			behaviorAdapterEditRow(t, r, "tests", func(row map[string]any) { row["testKey"] = "" })
			return "tests record 0"
		}},
		{name: "duplicate observation identity", stage: "observations", want: "duplicate behavior adapter observation identity", edit: func(t *testing.T, r *BehaviorAdapterRequest) string {
			r.Observations = append(r.Observations, r.Observations[0])
			return "/observations/" + strconv.Itoa(len(r.Observations)-1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			request.Inputs = append([]BehaviorAdapterInput(nil), fixture.request.Inputs...)
			request.Observations = append([]ObservationLink(nil), fixture.request.Observations...)
			item := tc.edit(t, &request)
			raw := behaviorAdapterRaw(t, request)
			_, buildErr := BuildBehaviorAdapter(raw, nil)
			var refused *Error
			if !errors.As(buildErr, &refused) || !strings.Contains(refused.Message, tc.want) {
				t.Fatalf("build should refuse with %q: %v", tc.want, buildErr)
			}
			report := CheckBehaviorAdapter(raw, nil)
			if report.Accepted || report.Refusals[0].Stage != tc.stage || report.Refusals[0].Message != refused.Message {
				t.Fatalf("first refusal %+v, build refused %q", report.Refusals, refused.Message)
			}
			refusals, entries := 0, 0
			for _, refusal := range report.Refusals {
				if refusal.Stage != tc.stage {
					continue
				}
				if refusal.State == "refused" {
					refusals++
					if tc.absent != "" && strings.Contains(refusal.Message, tc.absent) {
						t.Fatalf("check evaluated a later check of a refused item: %+v", report.Refusals)
					}
				} else if refusal.Message == "remaining checks for "+item+" not evaluated after "+refused.Message && len(refusal.BlockedBy) == 1 && refusal.BlockedBy[0] == tc.stage {
					entries++
				}
			}
			if refusals != 1 || entries != 1 {
				t.Fatalf("want one refusal and one item entry for %s, got %d and %d: %+v", item, refusals, entries, report.Refusals)
			}
		})
	}
}

// TestBehaviorAdapterBuildBoundMessage pins Build's bound refusal bytes to the
// message released before DCP-V1-044 split the bound checks.
func TestBehaviorAdapterBuildBoundMessage(t *testing.T) {
	fixture := behaviorAdapterFixture(t)
	for _, tc := range []struct {
		name string
		edit func(*BehaviorAdapterRequest)
	}{
		{name: "inputs", edit: func(r *BehaviorAdapterRequest) {
			r.Inputs = make([]BehaviorAdapterInput, MaxRecords+1)
		}},
		{name: "mappings", edit: func(r *BehaviorAdapterRequest) {
			r.Mappings = append([]BehaviorAdapterMapping(nil), r.Mappings[:3]...)
		}},
		{name: "observations", edit: func(r *BehaviorAdapterRequest) {
			r.Observations = make([]ObservationLink, MaxRecords+1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := fixture.request
			tc.edit(&request)
			_, err := BuildBehaviorAdapter(behaviorAdapterRaw(t, request), nil)
			var refused *Error
			if !errors.As(err, &refused) || refused.Code != "corpus-refused" || refused.Message != "behavior adapter bound exceeded" {
				t.Fatalf("bound refusal changed: %v", err)
			}
		})
	}
}

// TestBehaviorAdapterCheckBoundsReport proves DCP-V1-044: a refused count
// bound stops evaluation of the items it bounds, and the report's entries are
// capped with a terminal omission entry, so the report stays encodable.
func TestBehaviorAdapterCheckBoundsReport(t *testing.T) {
	raw := []byte(`{"mappings":[` + strings.TrimSuffix(strings.Repeat("{},", 10000), ",") + `]}`)
	report := CheckBehaviorAdapter(raw, nil)
	encoded, err := Encode(report)
	if err != nil || report.Accepted || len(encoded) > 64<<10 {
		t.Fatalf("mapping overflow report is unbounded: %v %d bytes", err, len(encoded))
	}
	entry := false
	for _, refusal := range report.Refusals {
		if refusal.Stage == "mappings" && refusal.State == "refused" {
			t.Fatalf("individual mappings evaluated after the count bound refused: %+v", refusal)
		}
		entry = entry || refusal.Stage == "mappings-bound" && refusal.State == "not-evaluated" && refusal.Message == "individual mappings not evaluated because the mappings count bound refused"
	}
	if !entry || behaviorAdapterCheckStates(report)["mappings"].State != "not-evaluated" {
		t.Fatalf("mapping overflow does not name the unevaluated mappings: %+v", report.Refusals)
	}

	fixture := behaviorAdapterFixture(t)
	request := fixture.request
	request.Observations = make([]ObservationLink, MaxRecords)
	raw = behaviorAdapterRaw(t, request)
	_, buildErr := BuildBehaviorAdapter(raw, nil)
	var refused *Error
	if !errors.As(buildErr, &refused) {
		t.Fatalf("build should refuse: %v", buildErr)
	}
	report = CheckBehaviorAdapter(raw, nil)
	encoded, err = Encode(report)
	last := report.Refusals[len(report.Refusals)-1]
	if err != nil || report.Accepted || report.Refusals[0].Message != refused.Message || len(report.Refusals) != 1025 || last.Message != "further entries omitted after 1024" || last.State != "not-evaluated" {
		t.Fatalf("report entries are not capped: %v entries=%d last=%+v", err, len(report.Refusals), last)
	}
}
