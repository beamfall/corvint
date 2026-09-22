package protocol

import (
	"strings"
	"testing"
)

func TestMCPV0021ProtocolSelector(t *testing.T) {
	for _, args := range [][]string{
		{"--protocol-version"}, {"--protocol-version", "unknown"},
		{"--protocol-version", LegacyVersion, "--protocol-version", LegacyVersion},
		{"--version", "--protocol-version", LegacyVersion},
	} {
		if _, _, ok := ExtractVersionArgument(args); ok {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, version := range []string{Version, LegacyVersion} {
		rest, got, ok := ExtractVersionArgument([]string{"--root", "/repo", "--protocol-version", version})
		if !ok || got != version || strings.Join(rest, " ") != "--root /repo" {
			t.Fatalf("selector=%v,%s,%v", rest, got, ok)
		}
	}
}

func TestMCPV0022LegacyMetadata(t *testing.T) {
	for _, raw := range []string{
		`{"_meta":null}`, `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2025-11-25"}}`,
		`{"_meta":{"progressToken":null}}`, `{"_meta":{"progressToken":{}}}`,
	} {
		inbound, err := Decode([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":` + raw + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeLegacyRequestMeta(inbound.Params); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
