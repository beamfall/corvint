package frictionmetric

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func evaluate(t *testing.T, record string) map[string]any {
	t.Helper()
	report, err := Evaluate(strings.NewReader(record))
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func sessionMetric(t *testing.T, report map[string]any, index int, name string) map[string]any {
	t.Helper()
	return report["sessions"].([]any)[index].(map[string]any)["metrics"].(map[string]any)[name].(map[string]any)
}

func TestURE996MetricDefinitions(t *testing.T) {
	report := evaluate(t, `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","relevant":["b"],"events":[
		{"kind":"read","ts_ms":100,"path":"a","planned":true},
		{"kind":"read","ts_ms":200,"path":"a","planned":false},
		{"kind":"edit","ts_ms":300,"path":"a","before":"x1","after":"x2"},
		{"kind":"read","ts_ms":400,"path":"a","planned":true},
		{"kind":"command","ts_ms":500,"command":"t","exit":2},
		{"kind":"command","ts_ms":600,"command":"t","exit":0},
		{"kind":"command","ts_ms":700,"command":"t","exit":0},
		{"kind":"edit","ts_ms":800,"path":"a","before":"x2","after":"x1"},
		{"kind":"read","ts_ms":1100,"path":"b","planned":true}]}]}`)
	want := map[string]int64{"rereads": 1, "unplanned_reads": 1, "retried_commands": 1, "reverted_edits": 1, firstRelevantRead: 1000}
	for name, value := range want {
		got := sessionMetric(t, report, 0, name)
		if got["status"] != observed || got["value"] != value {
			t.Errorf("%s = %v, want %d", name, got, value)
		}
	}
	arm := report["arms"].(map[string]any)["default"].(map[string]any)["rereads"].(map[string]any)
	if arm["sum"] != int64(1) || arm["sessions"] != 1 {
		t.Errorf("arm total = %v", arm)
	}
	if encoded := strings.Join(fieldStrings(report), " "); strings.Contains(encoded, "x1") || strings.Contains(encoded, `"a"`) {
		t.Errorf("report leaks a path or digest: %s", encoded)
	}
}

func fieldStrings(value any) []string {
	switch typed := value.(type) {
	case map[string]any:
		var out []string
		for key, item := range typed {
			out = append(out, strconv.Quote(key))
			out = append(out, fieldStrings(item)...)
		}
		return out
	case []any:
		var out []string
		for _, item := range typed {
			out = append(out, fieldStrings(item)...)
		}
		return out
	case string:
		return []string{strconv.Quote(typed)}
	}
	return nil
}

func TestURE996MissingInputIsNotObserved(t *testing.T) {
	report := evaluate(t, `{"profile":"corvint-friction-sessions/0","sessions":[
		{"id":"empty","arm":"a","events":[]},
		{"id":"sparse","arm":"b","relevant":["z"],"events":[
			{"kind":"read","path":"a"},
			{"kind":"command","command":"t"},
			{"kind":"edit","path":"a","after":"x"}]},
		{"id":"unreached","arm":"b","relevant":["z"],"events":[{"kind":"read","ts_ms":1,"path":"a","planned":true}]}]}`)
	for _, name := range Metrics {
		if got := sessionMetric(t, report, 0, name); got["status"] != notObserved || got["reason"] != "no-events" {
			t.Errorf("empty %s = %v", name, got)
		}
	}
	reasons := map[string]string{"unplanned_reads": "read-without-planned", "retried_commands": "command-without-exit", "reverted_edits": "edit-without-digests", firstRelevantRead: "event-without-ts"}
	for name, reason := range reasons {
		if got := sessionMetric(t, report, 1, name); got["status"] != notObserved || got["reason"] != reason || got["value"] != nil {
			t.Errorf("sparse %s = %v, want NOT_OBSERVED %s", name, got, reason)
		}
	}
	if got := sessionMetric(t, report, 2, firstRelevantRead); got["status"] != notReached {
		t.Errorf("unreached = %v", got)
	}
	if got := report["arms"].(map[string]any)["a"].(map[string]any)["rereads"].(map[string]any); got["status"] != notObserved {
		t.Errorf("arm with no observed session = %v, want NOT_OBSERVED", got)
	}
	if got := report["benchmark"].(map[string]any); got["status"] != notObserved {
		t.Errorf("unlabelled benchmark = %v", got)
	}
}

func TestURE996RefusesMalformedRecords(t *testing.T) {
	for name, record := range map[string]string{
		"profile":   `{"profile":"x","sessions":[{"id":"s","events":[]}]}`,
		"unknown":   `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[]}],"extra":1}`,
		"trailing":  `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[]}]} {}`,
		"no-id":     `{"profile":"corvint-friction-sessions/0","sessions":[{"events":[]}]}`,
		"duplicate": `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[]},{"id":"s","events":[]}]}`,
		"kind":      `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[{"kind":"write","path":"a"}]}]}`,
		"label":     `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","events":[{"kind":"read","path":"a","labels":["slow"]}]}]}`,
		"unsorted":  `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","relevant":["b","a"],"events":[]}]}`,
		"none":      `{"profile":"corvint-friction-sessions/0","sessions":[]}`,
		"oversized": `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"` + strings.Repeat("s", maxDocumentBytes) + `","events":[]}]}`,
	} {
		if _, err := Evaluate(strings.NewReader(record)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestURE998BenchmarkPrecisionRecall(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "benchmark.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	report, err := Evaluate(file)
	if err != nil {
		t.Fatal(err)
	}
	benchmark := report["benchmark"].(map[string]any)
	if benchmark["labelled_sessions"] != 5 {
		t.Fatalf("benchmark = %v", benchmark)
	}
	want := map[string][5]any{
		"rereads":          {2, 1, 0, 2.0 / 3.0, 1.0},
		"unplanned_reads":  {4, 0, 0, 1.0, 1.0},
		"retried_commands": {2, 0, 1, 1.0, 2.0 / 3.0},
		"reverted_edits":   {1, 0, 1, 1.0, 0.5},
		firstRelevantRead:  {5, 0, 0, 1.0, 1.0},
	}
	metrics := benchmark["metrics"].(map[string]any)
	for name, expected := range want {
		got := metrics[name].(map[string]any)
		actual := [5]any{got["true_positive"], got["false_positive"], got["false_negative"], got["precision"], got["recall"]}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s = %v, want %v", name, actual, expected)
		}
	}
	empty := evaluate(t, `{"profile":"corvint-friction-sessions/0","sessions":[{"id":"s","labelled":true,"events":[{"kind":"read","path":"a","planned":true}]}]}`)
	precision := empty["benchmark"].(map[string]any)["metrics"].(map[string]any)["rereads"].(map[string]any)["precision"].(map[string]any)
	if precision["status"] != notObserved {
		t.Errorf("zero-denominator precision = %v", precision)
	}
}
