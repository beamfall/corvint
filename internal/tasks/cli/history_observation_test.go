package cli

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// CAL-V0-096: attempt show renders a legacy prior generation's absent history
// as NOT_OBSERVED and leaves recorded values, including null, untouched.
func TestCALV0096_HistoryObservation(t *testing.T) {
	t.Run("CAL-V0-096 HistoryObservation", func(t *testing.T) {
		raw := `{"priorGenerations":[{"generation":"1","provedSeq":"2","quiescence":"FENCED"},{"generation":"2","memberId":"NOT_OBSERVED","poolId":"db","provedSeq":"4","quiescence":"PROVED","stage":"implement"},{"generation":"3","memberId":null,"poolId":null,"provedSeq":"6","quiescence":"PROVED","stage":null}]}`
		v, err := wire.Parse([]byte(raw + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		addHistoryObservation(v.Obj)
		want := `{"priorGenerations":[{"generation":"1","history":"NOT_OBSERVED","memberId":"NOT_OBSERVED","poolId":"NOT_OBSERVED","provedSeq":"2","quiescence":"FENCED","stage":"NOT_OBSERVED"},{"generation":"2","history":"RECORDED","memberId":"NOT_OBSERVED","poolId":"db","provedSeq":"4","quiescence":"PROVED","stage":"implement"},{"generation":"3","history":"RECORDED","memberId":null,"poolId":null,"provedSeq":"6","quiescence":"PROVED","stage":null}]}`
		if got := string(wire.Encode(v)); got != want {
			t.Fatalf("got  %s\nwant %s", got, want)
		}
	})
}
