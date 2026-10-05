package snapshot

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// legacyPriorSha256 is the sha256 of accountingAttempt() at generation 3 with
// two legacy prior generations, as encoded by origin/main ac818777 (before
// CAL-V0-079). It pins the N-1 bytes the new encoder must still write.
const legacyPriorSha256 = "b93e59bed07b195b8d006d1d3c459edc6932922d9590a755c240df8693629ac0"

func legacyPriorAttempt() *Attempt {
	a := accountingAttempt()
	a.Generation = "3"
	a.PriorGenerations = []PriorGeneration{{Generation: "1", Quiescence: "FENCED", ProvedSeq: "2"}, {Generation: "2", Quiescence: "PROVED", ProvedSeq: "4"}}
	return a
}

func ptr(s string) *string { return &s }

// CAL-V0-079: a legacy prior-generation entry decodes with no history and
// re-encodes byte-identical to the N-1 encoding.
func TestCALV0079_LegacyPriorGenerationsRoundTrip(t *testing.T) {
	t.Run("CAL-V0-079 LegacyPriorGenerationsRoundTrip", func(t *testing.T) {
		legacy, err := legacyPriorAttempt().Encode()
		if err != nil {
			t.Fatal(err)
		}
		if sum := sha256.Sum256(legacy); hex.EncodeToString(sum[:]) != legacyPriorSha256 {
			t.Fatalf("legacy encoding drifted from N-1: %s", legacy)
		}
		if strings.Contains(string(legacy), `"memberId"`) || strings.Contains(string(legacy), `"poolId"`) {
			t.Fatal("legacy entry gained history keys")
		}
		b, err := DecodeAttempt(legacy)
		if err != nil {
			t.Fatal(err)
		}
		for i, g := range b.PriorGenerations {
			if g.History != nil {
				t.Fatalf("entry %d: legacy history invented: %+v", i, g.History)
			}
		}
		again, err := b.Encode()
		if err != nil || !bytes.Equal(legacy, again) {
			t.Fatalf("legacy bytes changed: %v", err)
		}
	})
}

// CAL-V0-079: recorded history round-trips beside legacy entries; null records
// an observed absence of a stage or pool member.
func TestCALV0079_RecordedPriorGenerationsRoundTrip(t *testing.T) {
	t.Run("CAL-V0-079 RecordedPriorGenerationsRoundTrip", func(t *testing.T) {
		a := legacyPriorAttempt()
		a.Generation = "4"
		a.PriorGenerations = append(a.PriorGenerations, PriorGeneration{Generation: "3", Quiescence: "PROVED", ProvedSeq: "6", History: &GenerationHistory{Stage: ptr("implement"), PoolID: ptr("db"), MemberID: ptr("b")}})
		a.PriorGenerations[1].History = &GenerationHistory{}
		raw, err := a.Encode()
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`{"generation":"1","provedSeq":"2","quiescence":"FENCED"}`, `{"generation":"2","memberId":null,"poolId":null,"provedSeq":"4","quiescence":"PROVED","stage":null}`, `{"generation":"3","memberId":"b","poolId":"db","provedSeq":"6","quiescence":"PROVED","stage":"implement"}`} {
			if !strings.Contains(string(raw), want) {
				t.Fatalf("missing %s in %s", want, raw)
			}
		}
		b, err := DecodeAttempt(raw)
		if err != nil {
			t.Fatal(err)
		}
		if b.PriorGenerations[0].History != nil {
			t.Fatal("legacy entry gained history")
		}
		if h := b.PriorGenerations[1].History; h == nil || h.Stage != nil || h.PoolID != nil || h.MemberID != nil {
			t.Fatalf("null history: %+v", h)
		}
		if h := b.PriorGenerations[2].History; h == nil || *h.Stage != "implement" || *h.PoolID != "db" || *h.MemberID != "b" {
			t.Fatalf("recorded history: %+v", h)
		}
		again, err := b.Encode()
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatalf("recorded bytes changed: %v", err)
		}
	})
}

// CAL-V0-079: the closed decoder refuses partial, inconsistent or unknown
// history rather than guessing.
func TestCALV0079_MalformedPriorGenerationHistory(t *testing.T) {
	t.Run("CAL-V0-079 MalformedPriorGenerationHistory", func(t *testing.T) {
		legacy, err := legacyPriorAttempt().Encode()
		if err != nil {
			t.Fatal(err)
		}
		entry := `{"generation":"1","provedSeq":"2","quiescence":"FENCED"}`
		for name, bad := range map[string]string{
			"partial":       `{"generation":"1","provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
			"pool-no-stage": `{"generation":"1","memberId":"b","poolId":"db","provedSeq":"2","quiescence":"FENCED"}`,
			"member-null":   `{"generation":"1","memberId":null,"poolId":"db","provedSeq":"2","quiescence":"FENCED","stage":"implement"}`,
			"pool-null":     `{"generation":"1","memberId":"b","poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":null}`,
			"bad-stage":     `{"generation":"1","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":"deploy"}`,
			"unknown-key":   `{"author":"x","generation":"1","memberId":null,"poolId":null,"provedSeq":"2","quiescence":"FENCED","stage":null}`,
		} {
			raw := bytes.Replace(legacy, []byte(entry), []byte(bad), 1)
			if bytes.Equal(raw, legacy) {
				t.Fatalf("%s: fixture did not apply", name)
			}
			if _, err := DecodeAttempt(raw); err == nil {
				t.Fatalf("%s: accepted %s", name, bad)
			}
		}
	})
}
