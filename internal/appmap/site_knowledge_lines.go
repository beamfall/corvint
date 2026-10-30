package appmap

import "context"

// Bases of a screen the site-knowledge IR gives one test-body line (CCE-V0-310).
const (
	SKLineBasisVisit = "site-knowledge-visit"
	SKLineBasisStep  = "site-knowledge-step-screen"
)

// SKLine names one line of a test file in a root ("" is --root).
type SKLine struct {
	Repo, Path string
	Line       int
}

// SKLineScreen is the screen the site-knowledge IR places one test-body line on, with its basis.
type SKLineScreen struct{ Screen, Basis string }

// LineScreens reads the site-knowledge IR of the declared sources at the map revision
// (SKH-V0-020, SKH-V0-189) and returns, for each line of a test body holding a visit or an
// assertion, the screen the IR places it on (CCE-V0-310). A visit binds to the screen its URL
// resolves to; an assertion binds to the screen of the last visit, or of the last action run
// through a page object joined to a screen (SKH-V0-121, SKH-V0-185), before it. An unresolved
// visit, an unresolved call or an unparsed step forgets the screen, so nothing after it binds
// until a later step names one again. A line two tests place on different screens is omitted.
func LineScreens(ctx context.Context, m *Map, sources []string, o Options) (map[SKLine]SKLineScreen, error) {
	k, _, _, err := readSiteKnowledge(ctx, m, sources, o)
	if err != nil {
		return nil, err
	}
	idx := newScreenIndex(m.HashPrefix, m.BasePath, m.Screens)
	out, conflict := map[SKLine]SKLineScreen{}, map[SKLine]bool{}
	put := func(at SKLine, v SKLineScreen) {
		if conflict[at] {
			return
		}
		if prev, ok := out[at]; ok && prev != v {
			delete(out, at)
			conflict[at] = true
			return
		}
		out[at] = v
	}
	for _, t := range k.Tests {
		screen := ""
		for _, s := range t.Steps {
			// Only a step written in this test's own body is a harvestable line of its file.
			body := len(s.Via) == 0 && s.Anchor.Path == t.Anchor.Path && s.Anchor.Repo == t.Anchor.Repo
			at := SKLine{Repo: s.Anchor.Repo, Path: s.Anchor.Path, Line: s.Anchor.Start}
			switch {
			case s.Kind == StepVisit:
				screen, _ = skVisitScreen(idx, s)
				if screen != "" && body {
					put(at, SKLineScreen{Screen: screen, Basis: SKLineBasisVisit})
				}
			case s.Kind == StepAssert:
				if screen != "" && body {
					put(at, SKLineScreen{Screen: screen, Basis: SKLineBasisStep})
				}
			case s.Kind == StepWaitFor:
			case k.stepScreen(s) != "":
				screen = k.stepScreen(s)
			case s.Kind == StepOpaque && (s.Reason != OpaqueDynamicValue || s.Call == StepVisit):
				screen = ""
			}
		}
	}
	return out, nil
}
