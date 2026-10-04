package extevidence

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"testing"
)

func TestCapturedSelectionParityAndMutation(t *testing.T) {
	t.Run("DLT-V0-005 captured selection parity", testCapturedSelectionParityAndMutation)
}
func testCapturedSelectionParityAndMutation(t *testing.T) {
	p := newPair(t)
	ctx := context.Background()
	for _, name := range []string{"positive-same-repository", "positive-cross-repository", "stale-pinned-blob", "unbound-test-repository"} {
		t.Run(name, func(t *testing.T) {
			c := pathCase(t, name)
			source := fixtureRecord(t, p, pathFixtures, c)
			raw, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			captured, err := CaptureRecord(raw)
			if err != nil {
				t.Fatal(err)
			}
			input := selectionInput(c)
			input.CheckoutStatus = checkoutStatus["clean"]
			legacy := Selection(ctx, p.app.root, p.app.head, []string{source}, bindCase(p, c), input)
			result := SelectionCaptured(ctx, p.app.root, p.app.head, []CapturedRecord{captured}, bindCase(p, c), input)
			if legacy["state"] != result.Selection["state"] || !reflect.DeepEqual(selectedTests(legacy), selectedTests(result.Selection)) {
				t.Fatalf("selection parity %v %v", legacy, result)
			}
			for i := range raw {
				raw[i] = 'x'
			}
			if err := os.WriteFile(source, raw, 0600); err != nil {
				t.Fatal(err)
			}
			again := SelectionCaptured(ctx, p.app.root, p.app.head, []CapturedRecord{captured}, bindCase(p, c), input)
			if !reflect.DeepEqual(result, again) {
				t.Fatal("capture changed after caller/file mutation")
			}
		})
	}
}
func TestCapturedAssertsDistinctFromVerifies(t *testing.T) {
	t.Run("DLT-V0-006 asserts distinct from verifies", testCapturedAssertsDistinctFromVerifies)
}
func testCapturedAssertsDistinctFromVerifies(t *testing.T) {
	p := newPair(t)
	c := pathCase(t, "positive-same-repository")
	source := fixtureRecord(t, p, pathFixtures, c)
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{"verifies", "asserts"} {
		data := bytes.ReplaceAll(raw, []byte(`"verifies"`), []byte(`"`+relation+`"`))
		captured, err := CaptureRecord(data)
		if err != nil {
			t.Fatal(err)
		}
		got := SelectionCaptured(context.Background(), p.app.root, p.app.head, []CapturedRecord{captured}, nil, selectionInput(c))
		want := 0
		if relation == "asserts" {
			want = 1
		}
		if len(got.Assertions) != want {
			t.Fatalf("%s: %+v", relation, got)
		}
	}
}
func TestCapturedFailuresDoNotNarrow(t *testing.T) {
	t.Run("DLT-V0-005 failed capture never narrows", testCapturedFailuresDoNotNarrow)
}
func testCapturedFailuresDoNotNarrow(t *testing.T) {
	p := newPair(t)
	input := SelectionInput{Changed: []string{"pkg/main.go"}, Profile: ProfileStrict, Limit: 20}
	got := SelectionCaptured(context.Background(), p.app.root, p.app.head, []CapturedRecord{FailedCapture()}, nil, input)
	if got.Selection["state"] == SelectionNarrow || len(got.Assertions) > 0 {
		t.Fatal("failure narrowed")
	}
	if _, err := CaptureRecord(make([]byte, MaxRecordBytes+1)); err == nil {
		t.Fatal("oversized capture admitted")
	}
}
