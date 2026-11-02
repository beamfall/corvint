package opencodequalification

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAHI620InspectorWitnessNamesPreciseCause pins the cause a timed-out
// native UI witness reports, from fixture frames and witness logs.
func TestAHI620InspectorWitnessNamesPreciseCause(t *testing.T) {
	previous := frameWitnessTimeout
	frameWitnessTimeout = 50 * time.Millisecond
	t.Cleanup(func() { frameWitnessTimeout = previous })
	sidebar := []string{"Corvint", "Work", "Evidence", "0/0 completed", "0 draft", "0 held", "0 archived"}
	screen := `{"width":80,"height":24,"lines":[[{"text":"Corvint"}],[{"text":"Work"}],[{"text":"unavailable: Tasks unavailable"}],[{"text":"Evidence"}]]}`
	tests := []struct {
		name, screen, log string
		absent            []string
		width             int
		want              []string
		reject            []string
	}{
		{name: "no frame, plugin not loaded", want: []string{"no native frame captured (the witness plugin never logged loaded)"}, reject: []string{"lacks"}},
		{name: "no frame, plugin loaded", log: `{"stage":"loaded","version":"2.2.0"}` + "\n", want: []string{"no native frame captured (the witness plugin logged loaded)"}},
		{
			name: "sidebar strings missing", screen: screen,
			log:    `{"stage":"loaded","version":"2.2.0"}` + "\n" + `{"error":"Error: session create refused"}` + "\n" + "not json\n",
			want:   []string{"frame 80x24", `lacks "0/0 completed", "0 draft", "0 held", "0 archived"`, `witness plugin error "Error: session create refused"`},
			reject: []string{`"Corvint"`, `"Evidence"`, "expected width", "still shows"},
		},
		{name: "width and forbidden string", screen: screen, absent: []string{"Tasks unavailable", "Reading Corvint Tasks"}, width: 72, want: []string{"expected width 72", `still shows "Tasks unavailable"`}, reject: []string{"Reading Corvint Tasks", "witness plugin error"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if test.screen != "" {
				if err := os.WriteFile(filepath.Join(root, "screen.json"), []byte(test.screen), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if test.log != "" {
				if err := os.WriteFile(filepath.Join(root, "witness.jsonl"), []byte(test.log), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := waitFrame(context.Background(), root, "sidebar", sidebar, test.absent, test.width)
			if err == nil {
				t.Fatal("waitFrame succeeded on a frame that lacks the sidebar witness")
			}
			got := err.Error()
			for _, want := range append([]string{"native UI witness missing: sidebar: ", "frame saved to " + root + "/failed-frame.txt"}, test.want...) {
				if !strings.Contains(got, want) {
					t.Errorf("error %q lacks %q", got, want)
				}
			}
			for _, reject := range test.reject {
				if strings.Contains(got, reject) {
					t.Errorf("error %q names %q", got, reject)
				}
			}
			if _, statErr := os.Stat(filepath.Join(root, "failed-frame.txt")); statErr != nil {
				t.Errorf("failed frame not saved: %v", statErr)
			}
		})
	}
}
