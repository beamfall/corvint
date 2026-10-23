package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// sameFreshness reports whether two overlays describe the same observed state.
func sameFreshness(a, b FreshnessOverlay) bool {
	return a.State == b.State && a.Observation() == b.Observation() && reflect.DeepEqual(a.Limitations, b.Limitations)
}

// TestDCPV1465PrimaryObservationMatchesFreshRead: freshness that takes the observation started
// at the read's start gives the overlay and receipt a freshness observing the checkout itself
// gives, and uses the started observation rather than a new one.
func TestDCPV1465PrimaryObservationMatchesFreshRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, _ := builtFixture(t)
	want, err := LiveFreshness(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	read, cancel := context.WithCancel(ctx)
	defer cancel()
	read, join := WithPrimaryObservation(read, root)
	got, err := LiveFreshness(read, root, a)
	cancel()
	join()
	if err != nil {
		t.Fatal(err)
	}
	if !sameFreshness(got, want) {
		t.Fatalf("prefetched freshness %+v, observed %+v", got, want)
	}
	if p := read.Value(primaryObservationKey{}).(*primaryObservation); p.take() {
		t.Fatal("freshness did not use the started observation")
	}
	request := Request{Operation: "search", Query: "Count"}
	wantReceipt, err := QueryLive(ctx, a, request, want)
	if err != nil {
		t.Fatal(err)
	}
	gotReceipt, err := QueryLive(ctx, a, request, got)
	if err != nil || !reflect.DeepEqual(gotReceipt, wantReceipt) {
		t.Fatalf("prefetched receipt differs: %v", err)
	}
}

// TestDCPV1466PrimaryObservationUsedOnce: the started observation answers one freshness of its
// own root; a later freshness under the same context observes the checkout again and sees a
// commit that landed in between, and a freshness of another root never takes it.
func TestDCPV1466PrimaryObservationUsedOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root, a, _ := builtFixture(t)
	read, cancel := context.WithCancel(ctx)
	defer cancel()
	read, join := WithPrimaryObservation(read, root)
	defer join()
	defer cancel()
	first, err := LiveFreshness(read, root, a)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "src/view.ts"))
	if err != nil {
		t.Fatal(err)
	}
	commitFiles(t, root, map[string]string{"src/view.ts": insertLines(string(original), 20, "const padding = 1;\n")})
	second, err := LiveFreshness(read, root, a)
	if err != nil {
		t.Fatal(err)
	}
	want, err := LiveFreshness(ctx, root, a)
	if err != nil {
		t.Fatal(err)
	}
	if second.Observation() == first.Observation() || !sameFreshness(second, want) {
		t.Fatalf("second freshness reused the started observation: first %q second %q want %q", first.Observation(), second.Observation(), want.Observation())
	}

	other, cancelOther := context.WithCancel(ctx)
	defer cancelOther()
	other, joinOther := WithPrimaryObservation(other, t.TempDir())
	defer joinOther()
	defer cancelOther()
	if _, err := LiveFreshness(other, root, a); err != nil {
		t.Fatal(err)
	}
	if p := other.Value(primaryObservationKey{}).(*primaryObservation); !p.take() {
		t.Fatal("freshness of another root took the started observation")
	}
}

// TestDCPV1467CancelledPrimaryObservationJoins: a read that ends before any freshness, as a
// refused read does, cancels the started observation and its join returns once the observation
// has ended; nothing it started outlives the read. Run under -race.
func TestDCPV1467CancelledPrimaryObservationJoins(t *testing.T) {
	t.Parallel()
	root, _, _ := builtFixture(t)
	for _, early := range []bool{true, false} {
		read, cancel := context.WithCancel(context.Background())
		read, join := WithPrimaryObservation(read, root)
		p := read.Value(primaryObservationKey{}).(*primaryObservation)
		if !early {
			<-p.done
		}
		cancel()
		joined := make(chan struct{})
		go func() {
			join()
			close(joined)
		}()
		select {
		case <-joined:
		case <-time.After(time.Minute):
			t.Fatalf("early=%v: join did not return after cancel", early)
		}
		select {
		case <-p.done:
		default:
			t.Fatalf("early=%v: join returned before the observation ended", early)
		}
		if early && p.err == nil && p.live == nil {
			t.Fatal("cancelled observation ended with neither a result nor an error")
		}
	}
}
