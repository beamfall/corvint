package appmap

import (
	"encoding/json"
	"net/url"
	"os"
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
		// The live trace is kept as its one zip entry's text, so the fixture stays bindable; the
		// zip is rebuilt here with the entry name Playwright wrote.
		trace, err := os.ReadFile(filepath.Join(navEventsData, "live-traces", name+".trace"))
		if err != nil {
			t.Fatal(err)
		}
		zipped, _, err := LoadTraceZip(traceZip(t, map[string]string{"0-trace.trace": string(trace)}), events.TestKey)
		if err != nil || len(zipped) != 1 {
			t.Fatal(len(zipped), err)
		}
		fromEvents, fromTrace := observedPairs(t, collapseRedirects(events)), observedPairs(t, zipped[0])
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

// ANL-V0-044: the producer's redaction leaves nothing observe's redactURL would still remove, so
// observe reads the same decoded URL and resolves the same screen from the producer's file as from
// the raw commit, and no secret value reaches the file.
func TestANLV0044ProducerRedactionMatchesObserve(t *testing.T) {
	raws := []string{
		"https://u:p@x.test/a?token=sekret1&x=1#t",
		"http://h/p?a=b&code=sekret2&v=abcdefghijklmnopqrstuvwxyz0123",
		"http://h/p?a=1;token=sekret3&b=2&q=%zz",
		"http://h/#/r?session=sekret4&q=1",
		"http://h/#!/home?%2574oken=sekret5&a=1",
		"http://h/#/r?q=%zz&token=sekret7",
		"http://h/items/7#tab",
		"http://h/search?q=hello+world&page=2",
		"http://h/p?to%E2%84%AAen=sekret8&x=%C3%A9",
		"http://h/#!/home?%C5%BFecret=sekret9",
		"http://h/#!/home?tab=1;ignored=2&token=sekret10",
		"http://h/#!/home?q=%25zz&b=3",
		"http://h/#!/home?q=%FF&b=3",
		"http://h/p?q=%FF&b=3",
		"http://h/a%zz?x=1",
		"http://h/caf\u00e9?q=\u00e9#/r\u00e9?x=\u00e9&code=sekret11",
		"http://h/#!/%68ome?x=1",
		"http://h/#!/home?x=%41",
		"http://h/#!/%68ome?b=2&a=1",
		"http://h/#!/%68ome?token=sekret12",
		"http://h/#!/%68ome?b=2&token=sekret13&a=1",
		"http://h/#!/%68ome?q=%zz",
		"http://h/#!/home?q=%zz&token=sekret14",
		"http://u:p@h/a%zz?token=sekret15#!/home?x=1",
		"http://h/?x=1#!/home?q=%zz&token=sekret17",
		"http://h/?#!/home?q=%zz",
		"http://h/#/r%3Fsession%3Dsekret6&b=%2526",
		"http://h/#/r%3Ftoken%3Dsekret16%zz",
		"http://h/cb#token_type=bearer&access_token=sekret18",
		"http://h/cb#state=1&id_token=sekret19" + strings.Repeat("z", 30),
		"http://h/cb#sekret20" + strings.Repeat("z", 30),
		"http://h/cb#a=1;b=sekret21",
		"http://h/cb#access_token=sekret22%zz",
		"http://h/#!/home",
		"http://h/#!/r=1",
	}
	// Unchanged fragments keep their encoding, which observe's literal route match depends on.
	unchanged := map[string]bool{"http://h/#!/%68ome?x=1": true, "http://h/#!/home?x=%41": true, "http://h/#!/%68ome?b=2&a=1": true, "http://h/#!/home": true, "http://h/#!/r=1": true}
	// A route with an encoded '?' cannot keep its bytes once its query is redacted: withheld.
	withheld := map[string]bool{"http://h/#/r%3Fsession%3Dsekret6&b=%2526": true, "http://h/#/r%3Ftoken%3Dsekret16%zz": true, "http://h/cb#access_token=sekret22%zz": true}
	in, _ := json.Marshal(raws)
	script := `const {redactURL} = require('./navigation-events.cjs'); console.log(JSON.stringify(JSON.parse(process.argv[1]).map(redactURL)))`
	out, err := exec.Command("node", "-e", script, string(in)).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var produced []*string
	if err := json.Unmarshal(out, &produced); err != nil || len(produced) != len(raws) {
		t.Fatalf("producer output %s: %v", out, err)
	}
	var rawDoc, producedDoc []NavigationEvent
	for i, raw := range raws {
		if withheld[raw] != (produced[i] == nil) {
			t.Errorf("%s: withheld %v, want %v", raw, produced[i] == nil, withheld[raw])
			continue
		}
		if produced[i] == nil {
			continue
		}
		p := *produced[i]
		if unchanged[raw] && p != raw {
			t.Errorf("%s: unchanged fragment rewritten as %s", raw, p)
		}
		if strings.Contains(p, "sekret") {
			t.Errorf("%s: secret written as %s", raw, p)
		}
		if got, want := decodedURL(redactURL(p)), decodedURL(redactURL(raw)); got != want {
			t.Errorf("%s: observe reads %s from the producer, %s from the raw URL", raw, got, want)
		}
		rawDoc = append(rawDoc, NavigationEvent{Kind: "navigate", URL: raw})
		producedDoc = append(producedDoc, NavigationEvent{Kind: "navigate", URL: p})
	}
	// Observation level: under a "#!" hash prefix both documents place every visit on the same
	// screen with the same status; /%68ome stays unmapped in both, and an unparseable URL still
	// resolves its route.
	m := observeMap("#!")
	fromRaw, _ := observeOnce(t, m, NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: rawDoc}, Options{})
	fromProduced, _ := observeOnce(t, m, NavigationEvents{Schema: NavigationEventsSchema, TestKey: "t", Events: producedDoc}, Options{})
	home := 0
	for i, v := range fromRaw.Traces[0].Visits {
		w := fromProduced.Traces[0].Visits[i]
		if v.Screen != w.Screen || v.Status != w.Status {
			t.Errorf("%s: raw visit %s/%s, producer visit %s/%s", rawDoc[i].URL, v.Screen, v.Status, w.Screen, w.Status)
		}
		if w.Screen == screenID("shop", "home") {
			home++
		}
	}
	if home == 0 {
		t.Fatal("no visit resolved /home; the comparison is vacuous")
	}
}

// decodedURL is a URL with its fragment decoded, the form observe's redactURL compares by.
func decodedURL(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return s
	}
	frag := u.Fragment
	u.Fragment, u.RawFragment = "", ""
	return u.String() + "#" + frag
}
