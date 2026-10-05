package wire

import (
	"errors"
	"sort"
	"strings"
	"testing"
)

// CAL-V0-078: every §11 code is classified, and only the documented
// contention codes are retryable.
func TestCALV0078_ClassificationCoversEveryCode(t *testing.T) {
	if len(retries) != len(Codes) {
		t.Fatalf("classification has %d entries, Codes has %d", len(retries), len(Codes))
	}
	for _, c := range Codes {
		r, ok := retries[c]
		if !ok {
			t.Errorf("code %s is unclassified", c)
			continue
		}
		if strings.TrimSpace(r.Condition) == "" {
			t.Errorf("code %s has no condition", c)
		}
	}
	for c := range retries {
		if !IsCode(c) {
			t.Errorf("classification names non-§11 code %s", c)
		}
	}
	var got []string
	for c, r := range retries {
		if r.Retryable {
			got = append(got, c)
		}
	}
	sort.Strings(got)
	want := []string{CodeLockTimeout, CodeRedoPending, CodeSnapshotMoved}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("retryable set = %v, want %v", got, want)
	}
}

// CAL-V0-078: fencing codes mean the attempt really lost and are never retryable.
func TestCALV0078_FencingNeverRetryable(t *testing.T) {
	for _, c := range []string{CodeFenced, CodeBootFenced, CodeSupervisorLost} {
		if RetryOf(c).Retryable {
			t.Errorf("fencing code %s is retryable", c)
		}
		if v, present := ResultRetryable(OutcomeRefused, []string{CodeLockTimeout, c}); !present || v {
			t.Errorf("result with %s and LOCK_TIMEOUT = (%t,%t), want (false,true)", c, v, present)
		}
	}
	if RetryOf("NOT_A_CODE").Retryable {
		t.Fatal("unknown code is retryable")
	}
}

func TestCALV0078_ResultRetryablePresence(t *testing.T) {
	for _, tc := range []struct {
		outcome        string
		codes          []string
		value, present bool
	}{
		{OutcomeOK, nil, false, false},
		{OutcomeOK, []string{CodeLockTimeout}, false, false},
		{OutcomeNotRun, nil, false, false},
		{OutcomeError, []string{CodeLockTimeout}, true, true},
		{OutcomeNotRun, []string{CodeSnapshotMoved, CodeRedoPending}, true, true},
		{OutcomeRefused, []string{CodeLockTimeout, CodeMalformed}, false, true},
		{OutcomeRefused, []string{CodeTicketHeld}, false, true},
	} {
		v, p := ResultRetryable(tc.outcome, tc.codes)
		if v != tc.value || p != tc.present {
			t.Errorf("ResultRetryable(%s,%v) = (%t,%t), want (%t,%t)", tc.outcome, tc.codes, v, p, tc.value, tc.present)
		}
	}
}

func TestCALV0078_EnvelopeRetryableMember(t *testing.T) {
	enc := func(r Result) string {
		t.Helper()
		b, err := r.Encode()
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	lock := enc(Result{Command: []string{"lease", "renew"}, Outcome: OutcomeError, Codes: []string{CodeLockTimeout}})
	if !strings.Contains(lock, `"profile":"taskman-command-result/0","retryable":true,"snapshot":null`) {
		t.Fatalf("LOCK_TIMEOUT envelope lacks retryable:true: %s", lock)
	}
	fenced := enc(Result{Command: []string{"lease", "renew"}, Outcome: OutcomeRefused, Codes: []string{CodeFenced}})
	if !strings.Contains(fenced, `"retryable":false`) {
		t.Fatalf("FENCED envelope lacks retryable:false: %s", fenced)
	}
	for _, r := range []Result{
		{Command: []string{"queue", "status"}, Outcome: OutcomeOK},
		{Command: []string{"attempt"}, Outcome: OutcomeNotRun},
	} {
		if s := enc(r); strings.Contains(s, "retryable") {
			t.Fatalf("OK or uncoded envelope gained retryable: %s", s)
		}
	}

	// A command whose effect a retry would repeat reports false; it round-trips.
	ran := enc(Result{Command: []string{"run"}, Outcome: OutcomeError, Codes: []string{CodeLockTimeout}, NotRetryable: true})
	if !strings.Contains(ran, `"retryable":false`) {
		t.Fatalf("NotRetryable envelope lacks retryable:false: %s", ran)
	}
	if res, err := DecodeResult([]byte(ran)); err != nil || !res.NotRetryable {
		t.Fatalf("NotRetryable round trip = %+v, %v", res, err)
	}
	if res, err := DecodeResult([]byte(lock)); err != nil || res.NotRetryable {
		t.Fatalf("retryable round trip = %+v, %v", res, err)
	}

	// Earlier bytes without the member still decode.
	legacy := strings.Replace(lock, `"retryable":true,`, "", 1)
	if _, err := DecodeResult([]byte(legacy)); err != nil {
		t.Fatalf("legacy coded envelope refused: %v", err)
	}
	for name, bad := range map[string]string{
		"true on non-retryable": strings.Replace(fenced, `"retryable":false`, `"retryable":true`, 1),
		"not bool":              strings.Replace(lock, `"retryable":true`, `"retryable":"true"`, 1),
		"on uncoded":            strings.Replace(enc(Result{Command: []string{"attempt"}, Outcome: OutcomeNotRun}), `"profile"`, `"retryable":false,"profile"`, 1),
		"on OK":                 strings.Replace(enc(Result{Command: []string{"queue", "status"}, Outcome: OutcomeOK}), `"profile"`, `"retryable":false,"profile"`, 1),
	} {
		if _, err := DecodeResult([]byte(bad)); CodeOf(err) != CodeMalformed {
			t.Errorf("%s: DecodeResult = %v, want MALFORMED", name, err)
		}
	}
}

// TestCALV0078_WithoutRetryKeepsCode: the mark that carries a non-retryable
// fact through an error keeps its code and text, leaves the original
// unmarked, and passes other errors through unchanged.
func TestCALV0078_WithoutRetryKeepsCode(t *testing.T) {
	original := Errorf(CodeLockTimeout, "lock", "held")
	marked := WithoutRetry(original)
	if CodeOf(marked) != CodeLockTimeout || marked.Error() != original.Error() || !RetryForbidden(marked) {
		t.Fatalf("marked %#v", marked)
	}
	if RetryForbidden(original) || original.NotRetryable {
		t.Fatal("marking mutated the original")
	}
	plain := errors.New("plain")
	if WithoutRetry(plain) != plain || RetryForbidden(plain) || WithoutRetry(nil) != nil || RetryForbidden(nil) {
		t.Fatal("non-wire errors changed")
	}
}
