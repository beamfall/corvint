package gotest

import (
	"strings"
	"testing"
)

func TestCanonicalFactBodyExact(t *testing.T) {
	empty := digestString("")
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "test start",
			input: `{"Action":"start","Package":"example/p","Time":"2026-08-23T12:00:00-04:00"}`,
			want:  `{"artifactPathSha256":null,"attribute":null,"durationNanoseconds":null,"factKind":"TEST","failedBuild":null,"output":{"byteCount":"0","outputType":null,"sha256":"` + empty + `","text":null,"truncated":false},"package":"example/p","parentTestId":null,"rawAction":"start","sourceAnchor":null,"test":null,"timeRaw":"2026-08-23T12:00:00-04:00","timeUtc":"2026-08-23T16:00:00Z"}`,
		},
		{
			name:  "test output",
			input: `{"OutputType":"frame","Output":"hello","Test":"TestX","Package":"example/p","Time":"2026-08-23T12:00:00Z","Action":"output"}`,
			want:  `{"artifactPathSha256":null,"attribute":null,"durationNanoseconds":null,"factKind":"TEST","failedBuild":null,"output":{"byteCount":"5","outputType":"frame","sha256":"` + digestString("hello") + `","text":null,"truncated":false},"package":"example/p","parentTestId":null,"rawAction":"output","sourceAnchor":null,"test":"TestX","timeRaw":"2026-08-23T12:00:00Z","timeUtc":"2026-08-23T12:00:00Z"}`,
		},
		{
			name:  "attribute",
			input: `{"Action":"attr","Package":"example/p","Test":"TestX","Key":"owner","Value":"corvint","Time":"2026-08-23T12:00:00Z"}`,
			want:  `{"artifactPathSha256":null,"attribute":{"keySha256":"` + digestString("owner") + `","valueSha256":"` + digestString("corvint") + `"},"durationNanoseconds":null,"factKind":"TEST","failedBuild":null,"output":{"byteCount":"0","outputType":null,"sha256":"` + empty + `","text":null,"truncated":false},"package":"example/p","parentTestId":null,"rawAction":"attr","sourceAnchor":null,"test":"TestX","timeRaw":"2026-08-23T12:00:00Z","timeUtc":"2026-08-23T12:00:00Z"}`,
		},
		{
			name:  "build fail",
			input: `{"Action":"build-fail","ImportPath":"example/p"}`,
			want:  `{"factKind":"BUILD","importPath":"example/p","output":{"byteCount":"0","sha256":"` + empty + `","text":null,"truncated":false},"rawAction":"build-fail"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decoded, err := decodeRunnerEvent([]byte(tc.input), 1)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decoded.event.CanonicalFactBody()
			if err != nil || string(got) != tc.want {
				t.Fatalf("canonical body mismatch\ngot:  %s\nwant: %s\nerr: %v", got, tc.want, err)
			}
		})
	}
}

func TestCanonicalFactBodyHasNoRawIdentityAndIsOrderIndependent(t *testing.T) {
	left, err := decodeRunnerEvent([]byte(`{"Action":"start","Package":"p","Time":"2026-08-23T12:00:00Z"}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	right, err := decodeRunnerEvent([]byte(`{"Time":"2026-08-23T12:00:00Z","Package":"p","Action":"start"}`), 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := left.event.CanonicalFactBody()
	b, _ := right.event.CanonicalFactBody()
	if string(a) != string(b) || strings.Contains(string(a), "identity") || strings.Contains(string(a), "sequence") {
		t.Fatalf("fact body depends on raw representation or contains pre-WEI identity: %s / %s", a, b)
	}
}

func FuzzCanonicalFactBody(f *testing.F) {
	f.Add(`{"Action":"start","Package":"p","Time":"2026-08-23T12:00:00Z"}`)
	f.Fuzz(func(t *testing.T, raw string) {
		decoded, err := decodeRunnerEvent([]byte(raw), 1)
		if err != nil {
			return
		}
		_, _ = decoded.event.CanonicalFactBody()
	})
}
