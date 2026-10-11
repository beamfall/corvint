package appmap

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const navEventsData = "testdata/navigation-events"

// ANL-V0-037..044: the Node producer test drives synthetic instrumentation callbacks and pins the
// synthetic golden byte-for-byte.
func TestANLV0037ProducerNodeUnit(t *testing.T) {
	out, err := exec.Command("node", "--test", "navigation-events_test.cjs").CombinedOutput()
	if err != nil {
		t.Fatalf("navigation-events producer: %v\n%s", err, out)
	}
}

// ANL-V0-039: observe accepts the producer's synthetic golden and the files it wrote in a live run
// with trace 'retain-on-failure' (one file and the whole directory).
func TestANLV0039ObserveAcceptsProducerOutput(t *testing.T) {
	m := observeMap("")
	for _, name := range []string{"synthetic.json", "live-events/walk.json", "live-events"} {
		docs, raws, err := LoadNavigationEvents(filepath.Join(navEventsData, name))
		if err != nil {
			t.Fatal(name, err)
		}
		out, err := Observe(m, docs, raws, "abc", Options{Full: true})
		if err != nil {
			t.Fatal(name, err)
		}
		if strings.Contains(string(out), "token=zz") {
			t.Fatal("secret query value echoed", name)
		}
	}
	docs, raws, _ := LoadNavigationEvents(filepath.Join(navEventsData, "synthetic.json"))
	var obs Observation
	out, _ := Observe(m, docs, raws, "abc", Options{Full: true})
	if err := json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	var triggers []string
	for _, e := range obs.Traces[0].Edges {
		triggers = append(triggers, e.Trigger)
	}
	want := "click,click,UNKNOWN,history back,UNKNOWN,goto,redirect,redirect,click"
	if got := strings.Join(triggers, ","); got != want {
		t.Fatalf("triggers %s, want %s", got, want)
	}
}

// collapseRedirects folds each HTTP redirect chain to its final URL carrying the first hop's
// action: what main-frame snapshots can see of the same navigation (ANL-V0-022, ANL-V0-041).
func collapseRedirects(d NavigationEvents) NavigationEvents {
	out := d
	out.Events = nil
	for _, ev := range d.Events {
		if ev.Kind == "redirect" && len(out.Events) > 0 {
			out.Events[len(out.Events)-1].URL = ev.URL
			continue
		}
		out.Events = append(out.Events, ev)
	}
	return out
}

func observedPairs(t *testing.T, d NavigationEvents) ObservedTrace {
	t.Helper()
	raw, _ := json.Marshal(d)
	out, err := Observe(observeMap(""), []NavigationEvents{d}, [][]byte{raw}, "abc", Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	var obs Observation
	if err = json.Unmarshal(out, &obs); err != nil {
		t.Fatal(err)
	}
	return obs.Traces[0]
}

// ANL-V0-042: on the same live run, the producer's files and the trace zips give the same visits
// and edges; with an HTTP redirect they agree once the chain is collapsed, and only the producer
// shows the hop.
func TestANLV0042LiveParityWithTraceZip(t *testing.T) {
	for _, name := range []string{"walk", "redirect"} {
		docs, _, err := LoadNavigationEvents(filepath.Join(navEventsData, "live-events", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		events := docs[0]
		zipped, _, err := LoadTraceZip(filepath.Join(navEventsData, "live-traces", name+".zip"), events.TestKey)
		if err != nil {
			t.Fatal(err)
		}
		fromEvents, fromTrace := observedPairs(t, collapseRedirects(events)), observedPairs(t, zipped)
		if !reflect.DeepEqual(fromEvents.Visits, fromTrace.Visits) || !reflect.DeepEqual(fromEvents.Edges, fromTrace.Edges) {
			t.Fatalf("%s: producer %+v\ntrace %+v", name, fromEvents, fromTrace)
		}
		full := observedPairs(t, events)
		switch name {
		case "walk":
			if !reflect.DeepEqual(full.Visits, fromTrace.Visits) || !reflect.DeepEqual(full.Edges, fromTrace.Edges) || len(full.Edges) != 5 {
				t.Fatalf("walk without redirects differs: %+v", full)
			}
		case "redirect":
			if len(full.Visits) != 2 || !full.Visits[1].Redirect || full.Edges[0].Trigger != "redirect" || len(fromTrace.Visits) != 1 {
				t.Fatalf("redirect hop: producer %+v trace %+v", full, fromTrace)
			}
		}
	}
}
