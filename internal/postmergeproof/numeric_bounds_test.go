package postmergeproof

import (
	"fmt"
	"testing"
)

// PMR-V2-006: preserve each accepted interval and reject narrowing.
func TestV13179_DescendantIntervalExact(t *testing.T) {
	for _, value := range []int64{-1, 0, 20, 2147483647, 2147483648, 4294967296, 9223372036854775807} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"scope":"session","interval_ms":%d,"processes":[],"absent":true,"failures":[],"limitations":[]}`, value))
			node, err := parseJSONTree(raw, true)
			if err != nil {
				t.Fatal(err)
			}
			facts, _, err := descendantFactsV2(node)
			valid := value >= 0 && value <= int64(^uint(0)>>1)
			if (err == nil) != valid || (err == nil && int64(facts.IntervalMS) != value) {
				t.Fatalf("interval %d: %+v, %v; valid=%v", value, facts, err, valid)
			}
		})
	}
	t.Run("native reference admission", func(t *testing.T) {
		f := newProcWorld(t).fixture(t, fixtureSpec{name: "numeric-interval"})
		if _, err := f.verifyRaw(); err != nil {
			t.Fatalf("unmodified fixture: %v", err)
		}
		// A negative interval exercises the same malformed-observation refusal on
		// every host. The table above separately covers the native-width ceiling.
		f.cleanupRun()["descendants"].(map[string]any)["interval_ms"] = -1
		f.storeNative()
		_, err := f.verifyRaw()
		expectRefusal(t, err, "REJECTED", "process-native-reference-invalid")
	})
}
