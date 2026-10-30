package gotest

import (
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"
)

const eventTime = `"Time":"2026-08-23T12:00:00.123456789-04:00"`

func TestDecodeExactActionMatrix(t *testing.T) {
	valid := []string{
		`{"Action":"build-output","ImportPath":"example/p","Output":"build\n"}`,
		`{"Action":"build-fail","ImportPath":"example/p"}`,
		`{"Action":"start","Package":"example/p",` + eventTime + `}`,
		`{"Action":"run","Package":"example/p","Test":"TestX",` + eventTime + `}`,
		`{"Action":"pause","Package":"example/p","Test":"TestX",` + eventTime + `}`,
		`{"Action":"cont","Package":"example/p","Test":"TestX",` + eventTime + `}`,
		`{"Action":"output","Package":"example/p","Output":"text",` + eventTime + `}`,
		`{"Action":"output","Package":"example/p","Test":"TestX","Output":"text","OutputType":"error",` + eventTime + `}`,
		`{"Action":"pass","Package":"example/p","Elapsed":0,` + eventTime + `}`,
		`{"Action":"pass","Package":"example/p","Test":"TestX","Elapsed":1.000000001,` + eventTime + `}`,
		`{"Action":"fail","Package":"example/p","Elapsed":0,"FailedBuild":"example/dep",` + eventTime + `}`,
		`{"Action":"skip","Package":"example/p",` + eventTime + `}`,
		`{"Action":"skip","Package":"example/p","Test":"TestX","Elapsed":0,` + eventTime + `}`,
		`{"Action":"bench","Package":"example/p","Test":"BenchmarkX",` + eventTime + `}`,
		`{"Action":"attr","Package":"example/p","Test":"TestX",` + eventTime + `}`,
		`{"Action":"attr","Package":"example/p","Test":"TestX","Key":"","Value":"",` + eventTime + `}`,
		`{"Action":"artifacts","Package":"example/p","Test":"TestX","Path":"/tmp/artifacts",` + eventTime + `}`,
	}
	for _, input := range valid {
		input := input
		t.Run(input, func(t *testing.T) {
			if _, err := decodeRunnerEvent([]byte(input), 1); err != nil {
				t.Fatalf("valid Go 1.27 event rejected: %v", err)
			}
		})
	}

	invalid := []struct {
		name  string
		input string
		want  Failure
	}{
		{"missing time", `{"Action":"start","Package":"p"}`, FailureMissingField},
		{"null time", `{"Action":"start","Package":"p","Time":null}`, FailureInvalidField},
		{"null package", `{"Action":"start","Package":null,` + eventTime + `}`, FailureInvalidField},
		{"run missing test", `{"Action":"run","Package":"p",` + eventTime + `}`, FailureMissingField},
		{"run null test", `{"Action":"run","Package":"p","Test":null,` + eventTime + `}`, FailureInvalidField},
		{"pass missing elapsed", `{"Action":"pass","Package":"p",` + eventTime + `}`, FailureMissingField},
		{"elapsed on run", `{"Action":"run","Package":"p","Test":"T","Elapsed":0,` + eventTime + `}`, FailureInvalidField},
		{"output missing output", `{"Action":"output","Package":"p",` + eventTime + `}`, FailureMissingField},
		{"output null output", `{"Action":"output","Package":"p","Output":null,` + eventTime + `}`, FailureInvalidField},
		{"unknown output type", `{"Action":"output","Package":"p","Output":"x","OutputType":"future",` + eventTime + `}`, FailureInvalidField},
		{"failed build on named fail", `{"Action":"fail","Package":"p","Test":"T","Elapsed":0,"FailedBuild":"dep",` + eventTime + `}`, FailureInvalidField},
		{"empty explicit test", `{"Action":"pass","Package":"p","Test":"","Elapsed":0,` + eventTime + `}`, FailureInvalidField},
		{"key on start", `{"Action":"start","Package":"p","Key":"",` + eventTime + `}`, FailureInvalidField},
		{"artifact missing path", `{"Action":"artifacts","Package":"p","Test":"T",` + eventTime + `}`, FailureMissingField},
		{"test field on build", `{"Action":"build-fail","ImportPath":"p","Test":""}`, FailureInvalidField},
		{"output on build fail", `{"Action":"build-fail","ImportPath":"p","Output":""}`, FailureInvalidField},
		{"unknown field", `{"Action":"start","Package":"p","Future":"x",` + eventTime + `}`, FailureUnknownField},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeRunnerEvent([]byte(tc.input), 1)
			var parseErr *Error
			if !errors.As(err, &parseErr) || parseErr.Failure != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestDecodeChoosesActionBeforeOptionalFields(t *testing.T) {
	_, err := decodeRunnerEvent([]byte(`{"Action":"future","Elapsed":[],"Future":null}`), 1)
	var parseErr *Error
	if !errors.As(err, &parseErr) || parseErr.Failure != FailureUnknownAction {
		t.Fatalf("got %v, want unknown action before optional-field interpretation", err)
	}
}

func TestStrictElapsedAndTimeLexing(t *testing.T) {
	elapsedValid := map[string]int64{
		"0": 0, "0.1": 100_000_000, "1.000000001": 1_000_000_001,
	}
	for raw, want := range elapsedValid {
		got, err := parseStrictElapsed(jsontext.Value(raw))
		if err != nil || got != want {
			t.Fatalf("elapsed %q: got %d, %v; want %d", raw, got, err, want)
		}
	}
	for _, raw := range []string{"-1", "+1", "01", "1.", ".1", "1e-3", "0.0000000000", "9223372037"} {
		if _, err := parseStrictElapsed(jsontext.Value(raw)); err == nil {
			t.Fatalf("invalid elapsed accepted: %q", raw)
		}
	}

	validTime := "2026-08-23T12:00:00.123456789-04:00"
	got, err := parseStrictTime(validTime)
	if err != nil || got.Format(timeLayout) != "2026-08-23T16:00:00.123456789Z" {
		t.Fatalf("valid timestamp: %v %v", got, err)
	}
	for _, raw := range []string{
		"2026-08-23T12:00:00", "2026-08-23T12:00:00z", "2026-08-23T12:00:00,1Z",
		"2026-08-23T12:00:00.1234567890Z", "2026-08-23T25:00:00Z",
		"2026-08-23T12:00:00+24:00", "2026-08-23T12:00:00+00:60",
	} {
		if _, err := parseStrictTime(raw); err == nil {
			t.Fatalf("invalid timestamp accepted: %q", raw)
		}
	}
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

func TestBoundedStreamingAggregation(t *testing.T) {
	base := bounds{lineBytes: 1024, eventBytes: 4096, outputBytes: 1024, packages: 8, tests: 8, events: 16}

	t.Run("stream does not retain events", func(t *testing.T) {
		var actions []string
		got, err := observe(strings.NewReader(lines(
			`{"Action":"start","Package":"p"}`,
			`{"Action":"pass","Package":"p","Elapsed":0}`,
		)), testConfig(t, 0, "p"), false, func(event Event) error {
			actions = append(actions, event.Action)
			return nil
		}, base)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Events) != 0 || got.EventCount != 2 || strings.Join(actions, ",") != "start,pass" {
			t.Fatalf("stream retained or lost events: %#v actions=%v", got, actions)
		}
	})

	tests := []struct {
		name   string
		input  string
		mutate func(*bounds)
		want   Failure
	}{
		{"event bytes", lines(`{"Action":"start","Package":"p"}`), func(b *bounds) { b.eventBytes = 8 }, FailureTooManyEventBytes},
		{"output bytes", lines(`{"Action":"start","Package":"p"}`, `{"Action":"output","Package":"p","Output":"12345"}`, `{"Action":"pass","Package":"p","Elapsed":0}`), func(b *bounds) { b.outputBytes = 4 }, FailureTooManyOutputBytes},
		{"events", lines(`{"Action":"start","Package":"p"}`, `{"Action":"pass","Package":"p","Elapsed":0}`), func(b *bounds) { b.events = 1 }, FailureTooManyEvents},
		{"packages", lines(`{"Action":"start","Package":"p"}`, `{"Action":"pass","Package":"p","Elapsed":0}`, `{"Action":"start","Package":"q"}`), func(b *bounds) { b.packages = 1 }, FailureConfig},
		{"tests", lines(`{"Action":"start","Package":"p"}`, `{"Action":"run","Package":"p","Test":"A"}`, `{"Action":"pass","Package":"p","Test":"A","Elapsed":0}`, `{"Action":"run","Package":"p","Test":"B"}`), func(b *bounds) { b.tests = 1 }, FailureTooManyTests},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limits := base
			tc.mutate(&limits)
			configPackages := []string{"p", "q"}
			_, err := observe(strings.NewReader(tc.input), testConfig(t, 0, configPackages...), false, nil, limits)
			var parseErr *Error
			if !errors.As(err, &parseErr) || parseErr.Failure != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestOfficialBenchmarkWithoutRunIsObservedButP0Incomplete(t *testing.T) {
	got, err := Observe(strings.NewReader(lines(
		`{"Action":"start","Package":"p"}`,
		`{"Action":"output","Package":"p","Test":"BenchmarkX","Output":"--- BENCH: BenchmarkX\n","OutputType":"frame"}`,
		`{"Action":"bench","Package":"p","Test":"BenchmarkX"}`,
		`{"Action":"pass","Package":"p","Elapsed":0}`,
	)), testConfig(t, 0, "p"))
	if err != nil {
		t.Fatal(err)
	}
	if got.TestCount != 1 || len(got.IncompleteReasons) != 1 || got.IncompleteReasons[0] != "BENCHMARK_ACTION" {
		t.Fatalf("benchmark semantics not retained honestly: %#v", got)
	}
}

func FuzzDecodeRunnerEvent(f *testing.F) {
	for _, seed := range []string{
		`{"Action":"start","Package":"p",` + eventTime + `}`,
		`{"Action":"attr","Package":"p","Test":"T",` + eventTime + `}`,
		`{"Action":"future","Elapsed":[]}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = decodeRunnerEvent([]byte(input), 1)
	})
}

func FuzzStrictElapsed(f *testing.F) {
	for _, seed := range []string{"0", "1.000000001", "1e-3", "-1"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseStrictElapsed(jsontext.Value(input))
	})
}

func FuzzStrictTime(f *testing.F) {
	for _, seed := range []string{"2026-08-23T12:00:00Z", "2026-08-23T12:00:00,1Z", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseStrictTime(input)
	})
}

func TestProductionBoundsAreFrozen(t *testing.T) {
	if MaxLineBytes != 1<<20 || MaxEventBytes != 16<<20 || MaxOutputBytes != 8<<20 ||
		MaxPackages != 4096 || MaxTests != 100000 || MaxEvents != 100000 {
		t.Fatalf("unexpected V0 bounds: %#v", v0Bounds)
	}
}
