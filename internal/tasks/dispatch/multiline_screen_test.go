//go:build darwin || linux

package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/obligation"
)

// multilineSecrets returns synthetic secrets a model wrapped across lines: a
// PEM private-key block and a token broken by a newline, each between lines
// that must survive, and the fragments that must not.
func multilineSecrets(t *testing.T) (text string, fragments []string) {
	t.Helper()
	// Assembled at run time so no key header appears in the source.
	begin, end := "-----BEGIN "+"RSA PRIVATE"+" KEY-----", "-----END "+"RSA PRIVATE"+" KEY-----"
	body1, body2 := strings.Repeat("Zq9x/Pk7+", 7), strings.Repeat("Wm4+Yt8/", 5)
	token1, token2 := "ghp_"+strings.Repeat("Zq9x", 3), strings.Repeat("Hv5k", 4)
	for _, frag := range []string{body1, body2, token1, token2} {
		if obligation.MatchesSecretScreen(frag) {
			t.Fatalf("precondition: fragment %q matches on its own", frag)
		}
	}
	if !obligation.MatchesSecretScreen(token1+token2) || !obligation.MatchesSecretScreen(begin+"\n"+body1+"\n"+end) {
		t.Fatal("precondition: the whole secrets match")
	}
	text = strings.Join([]string{"intro", begin, body1, "  " + body2 + "\r", end, "middle", token1, token2, "-- done"}, "\n")
	return text, []string{"Zq9x/Pk7", "Wm4+Yt8", "ghp_", "Hv5k"}
}

// TestATRV0169_ScreenRedactsSecretsWrappedAcrossLines shows that the
// salvage note and the carry-over record replace every line holding part of
// a PEM block or of a token broken by a newline, and keep the lines around
// them (V1-1792).
func TestATRV0169_ScreenRedactsSecretsWrappedAcrossLines(t *testing.T) {
	text, fragments := multilineSecrets(t)
	want := strings.Join([]string{"intro", redactedLine, redactedLine, redactedLine, redactedLine, "middle", redactedLine, redactedLine, "-- done"}, "\n")
	if got, n := screen(text); got != want || n != 6 {
		t.Fatalf("screen redacted %d lines:\n%s", n, got)
	}

	t.Run("salvage", func(t *testing.T) {
		d, _ := openCtx(t, &fakeQueue{}, testConfig(t, "exit 0"))
		w := &Worker{ID: "w-wrap", Ticket: "t1", Role: "impl", KillReason: "CONTEXT"}
		ev := EvidenceDir(d.dir, w.ID)
		if err := os.MkdirAll(ev, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ev, "a.log"), []byte(text+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		n := d.buildHandoffNote(w, d.Now(), "")
		if len(n.Files) != 1 || n.Files[0].Text != want+"\n" || n.RedactedLines != 6 {
			t.Fatalf("salvage note %+v", n)
		}
		for _, frag := range fragments {
			if strings.Contains(string(n.encode()), frag) {
				t.Fatalf("salvage note kept %q", frag)
			}
		}
	})

	t.Run("carry-over", func(t *testing.T) {
		c := testConfig(t, "exit 0")
		c.Roles[0].CarryOver = &CarryOver{}
		d, now := openCtx(t, &fakeQueue{}, c)
		key := "ticket:a:q:t1"
		noteWith(t, d, &Worker{ID: "w1", Role: "impl", Key: key, Ticket: key}, *now, SessionNoProgress, text)
		rec, err := ReadCarryOver(CarryOverPath(d.dir, key))
		if err != nil || len(rec.Entries) != 1 || rec.Entries[0].RedactedLines != 6 {
			t.Fatalf("record %+v %v", rec, err)
		}
		block, _ := d.carryOverFor(c.Roles[0].CarryOver, key, testFP(d, key))
		for _, frag := range fragments {
			if strings.Contains(rec.Entries[0].Text, frag) || strings.Contains(block, frag) {
				t.Fatalf("carry-over kept %q:\n%s", frag, rec.Entries[0].Text)
			}
		}
	})
}
