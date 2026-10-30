package intent_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func TestCALV0071_PolicyRepositories(t *testing.T) {
	docs := string(wire.Sum([]byte("/work/docs")))
	p, e := supervisionPolicy(t, "", `,"repositories":{"docs":{"pathSha256":"`+docs+`"},"site-2":{"pathSha256":"`+docs+`"}}`)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.Supervision.Repositories) != 2 || string(p.Supervision.Repositories["docs"]) != docs {
		t.Fatalf("repositories %+v", p.Supervision.Repositories)
	}
	if p, e = supervisionPolicy(t, ""); e != nil || p.Supervision.Repositories != nil {
		t.Fatalf("absent repositories %+v %v", p, e)
	}
	nine := []string{}
	for i := 0; i < 9; i++ {
		nine = append(nine, fmt.Sprintf(`"r%d":{"pathSha256":"%s"}`, i, docs))
	}
	for name, tail := range map[string]string{
		"empty":          `{}`,
		"over limit":     "{" + strings.Join(nine, ",") + "}",
		"uppercase name": `{"Docs":{"pathSha256":"` + docs + `"}}`,
		"leading digit":  `{"2docs":{"pathSha256":"` + docs + `"}}`,
		"too long":       `{"` + strings.Repeat("a", 33) + `":{"pathSha256":"` + docs + `"}}`,
		"unknown key":    `{"docs":{"checkout":"/work/docs","pathSha256":"` + docs + `"}}`,
		"missing digest": `{"docs":{}}`,
		"bad digest":     `{"docs":{"pathSha256":"nope"}}`,
		"not an object":  `["docs"]`,
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			if _, e := supervisionPolicy(t, "", `,"repositories":`+tail); e == nil {
				t.Fatalf("accepted %s", tail)
			}
		})
	}
	for name, ok := range map[string]bool{"a": true, "docs-2": true, strings.Repeat("a", 32): true, "": false, "-a": false, "a_b": false, "a/b": false, "a@b": false, ".queue": false} {
		if intent.ValidRepositoryName(name) != ok {
			t.Fatalf("name %q valid=%v", name, !ok)
		}
	}
}
