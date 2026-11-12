// SPDX-License-Identifier: Apache-2.0

package appflows

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// FSV-V1-006 (V1-5410): the writer emits the highest supported version and
// the same build's reader reads back the written declaration as that
// version; a declaration the reader would refuse is never written.
func TestFSVV1006WriterAndReaderAgree_V15410(t *testing.T) {
	for _, d := range []FlowsDeclaration{
		{Dir: "flows"},
		{Dir: "flows", Kinds: []KindRule{{Path: ".", Kind: affected.TestKindUnit}, {Path: "test/integration", Kind: affected.TestKindIntegration}}},
		{Dir: "e2e/flows", PlaywrightConfig: "playwright.config.ts", PlaywrightDiscovery: ".corvint/playwright-discovery.json"},
	} {
		body, err := EncodeFlowsDeclaration(d)
		if err != nil {
			t.Fatalf("%+v: %v", d, err)
		}
		if want := `"profile":"` + FlowsProfile(FlowsWriteVersion()) + `"`; !strings.Contains(string(body), want) {
			t.Fatalf("%+v: %s lacks %s", d, body, want)
		}
		read, err := readFlowsBody(t, string(body))
		if err != nil || read == nil {
			t.Fatalf("%+v: the reader refused the written file %s: %v", d, body, err)
		}
		d.Version = FlowsWriteVersion()
		if !reflect.DeepEqual(*read, d) {
			t.Fatalf("round trip:\nwrote %+v\nread  %+v", d, *read)
		}
	}
	for _, d := range []FlowsDeclaration{
		{Dir: "../out"},
		{Dir: "flows", Kinds: []KindRule{{Path: "lib", Kind: affected.TestKindE2E}}},
		{Dir: "flows", PlaywrightConfig: "playwright.config.ts"},
		{Dir: "flows", Version: FlowsWriteVersion() + 1},
	} {
		if body, err := EncodeFlowsDeclaration(d); err == nil {
			t.Fatalf("%+v: wrote %s", d, body)
		}
	}
	if FlowsProfile(FlowsWriteVersion()) != FlowsProfilePrefix+strconv.Itoa(FlowsSupportedVersions()[len(FlowsSupportedVersions())-1]) {
		t.Fatalf("write profile %s is not the highest supported", FlowsProfile(FlowsWriteVersion()))
	}
}
