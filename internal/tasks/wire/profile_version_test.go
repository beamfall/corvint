package wire

import (
	"fmt"
	"testing"
)

// TestCALV0131_ProfileVersionRefusesOnlyAnotherVersion: the early profile
// check refuses only the wanted profile at another version, and leaves every
// other shape to the decoder's own closed checks.
func TestCALV0131_ProfileVersionRefusesOnlyAnotherVersion(t *testing.T) {
	const want = "taskman-attempt/0"
	for raw, code := range map[string]string{
		`{"profile":"taskman-attempt/1","futureFact":1}`: CodeUnsupportedVersion,
		`{"profile":"taskman-attempt/0","futureFact":1}`: "",
		`{"profile":"taskman-ticket/0"}`:                 "",
		`{"profile":7}`:                                  "",
		`{"Profile":"taskman-attempt/1"}`:                "",
		`{"other":"taskman-attempt/1"}`:                  "",
		`["taskman-attempt/1"]`:                          "",
	} {
		if got := CodeOf(RawProfileVersion("/profile", []byte(raw), want)); got != code {
			t.Errorf("raw %s: %q, want %q", raw, got, code)
		}
		v, err := Parse([]byte(raw + "\n"))
		if err != nil {
			continue
		}
		if got := CodeOf(ProfileVersion("/profile", v, want)); got != code {
			t.Errorf("value %s: %q, want %q", raw, got, code)
		}
		r := NewReader(v, "/")
		r.Profile(want)
		if got := CodeOf(r.Err()); got != code {
			t.Errorf("reader %s: %q, want %q", raw, got, code)
		}
	}
}

// TestCALV0131_CodeOfUnwraps: wrapping a coded refusal anywhere keeps its
// code and its retry mark; only an uncoded error reports MALFORMED.
func TestCALV0131_CodeOfUnwraps(t *testing.T) {
	coded := Errorf(CodeUnsupportedVersion, "/profile", "newer")
	wrapped := fmt.Errorf("verify: %w", fmt.Errorf("decode: %w", coded))
	if got := CodeOf(wrapped); got != CodeUnsupportedVersion {
		t.Fatalf("wrapped code %q, want %s", got, CodeUnsupportedVersion)
	}
	if got := CodeOf(fmt.Errorf("plain")); got != CodeMalformed {
		t.Fatalf("uncoded error %q, want %s", got, CodeMalformed)
	}
	if CodeOf(nil) != "" {
		t.Fatal("nil error has a code")
	}
	if !RetryForbidden(fmt.Errorf("x: %w", WithoutRetry(coded))) {
		t.Fatal("wrapped retry mark lost")
	}
	if got := CodeOf(WithoutRetry(wrapped)); got != CodeUnsupportedVersion || !RetryForbidden(WithoutRetry(wrapped)) {
		t.Fatalf("WithoutRetry over a wrap: %q", got)
	}
}

// TestCALV0131_FirstRefusalSticks: once a reader has refused the profile
// version, the closed-key check and later field reads, including a wrapped
// refusal adopted from a nested decoder, never replace that first refusal.
func TestCALV0131_FirstRefusalSticks(t *testing.T) {
	v, err := Parse([]byte(`{"futureFact":"1","profile":"taskman-attempt/1"}` + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewReader(v, "")
	if got := CodeOf(r.Profile("taskman-attempt/0")); got != CodeUnsupportedVersion {
		t.Fatalf("Profile returned %q", got)
	}
	r.Closed("profile", "ticketId")
	_ = r.Field("ticketId").String()
	r.adopt(fmt.Errorf("nested: %w", Errorf(CodeMalformed, "/x", "later")))
	r.Fail(CodeMalformed, "later")
	if got := CodeOf(r.Err()); got != CodeUnsupportedVersion {
		t.Fatalf("first refusal replaced by %q", got)
	}
}
