package wire

import "testing"

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
		r := NewReader(v, "/").Profile(want)
		if got := CodeOf(r.Err()); got != code {
			t.Errorf("reader %s: %q, want %q", raw, got, code)
		}
	}
}
