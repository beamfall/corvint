package contextindex

import (
	"encoding/json"
	"testing"
)

func contextExclusionWire(t *testing.T, value any) []byte {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}
