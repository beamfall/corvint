package snapshot

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0131_AttemptFromAnotherBuildRefusesUnsupportedVersion: an attempt
// record written by a newer build, with a new profile version and a member
// this build does not know, refuses UNSUPPORTED_VERSION before its closed
// keys are checked, at the top level and for the nested retry accounting.
// The same unknown member under this build's profile stays MALFORMED.
func TestCALV0131_AttemptFromAnotherBuildRefusesUnsupportedVersion(t *testing.T) {
	a := accountingAttempt()
	a.RetryAccounting = &RetryAccounting{Disposition: "NONE"}
	raw, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	edit := func(f func(o *wire.Object)) []byte {
		v, err := wire.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		f(v.Obj)
		return wire.EncodeFile(v)
	}
	for name, c := range map[string]struct {
		raw  []byte
		code string
	}{
		"newer attempt": {edit(func(o *wire.Object) {
			o.Set("profile", wire.String("taskman-attempt/1")).Set("futureFact", wire.String("added by a newer build"))
		}), wire.CodeUnsupportedVersion},
		"newer retry accounting": {edit(func(o *wire.Object) {
			x, _ := o.Get("retryAccounting")
			x.Obj.Set("profile", wire.String("taskman-retry-accounting/1")).Set("futureFact", wire.String("added by a newer build"))
		}), wire.CodeUnsupportedVersion},
		"same build, unknown member": {edit(func(o *wire.Object) {
			o.Set("futureFact", wire.String("not in this format"))
		}), wire.CodeMalformed},
	} {
		if _, err := DecodeAttempt(c.raw); wire.CodeOf(err) != c.code {
			t.Errorf("%s: got %v, want %s", name, err, c.code)
		}
	}
}
