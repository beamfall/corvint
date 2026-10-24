package transaction

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0004_NoncanonicalTicketBytesStillRefused covers LGL-V0-004
// (proposed). loadTickets no longer parses a decoded canonical ticket a
// second time, because the decoding parse already proves canonical framing.
// A canonical ticket that differs from its encoding by a missing or extra
// trailing LF, insignificant whitespace, or an escape form is still refused
// as malformed, and the canonical bytes are accepted.
func TestLGLV0004_NoncanonicalTicketBytesStillRefused(t *testing.T) {
	base := paused(t)
	c := fixture.Ticket("T-1")
	raw := c.Encode()
	in := withTicket(t, base, c, raw)
	if got := Model(admin(Unpause, "canonical"), in); got.Kind != "Transaction" {
		t.Fatal("canonical ticket refused", got)
	}
	for name, corrupt := range map[string][]byte{
		"no-trailing-lf":    bytes.TrimSuffix(raw, []byte("\n")),
		"extra-trailing-lf": append(append([]byte{}, raw...), '\n'),
		"whitespace":        append([]byte("{ "), raw[1:]...),
		"escape-form":       bytes.Replace(raw, []byte(`"profile"`), []byte(`"\u0070rofile"`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(corrupt, raw) {
				t.Fatal("corruption left the bytes unchanged")
			}
			bad := in
			bad.CanonicalTickets = [][]byte{corrupt}
			got := Model(admin(Unpause, "noncanonical-"+name), bad)
			assertZero(t, got, "Refused")
			if got.Outcome.Outcome != mutation.OutcomeValidationFailed || !got.Outcome.HasCode(wire.CodeMalformed) {
				t.Fatal("noncanonical ticket accepted", got)
			}
		})
	}
}
