// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestSTRV08301ManifestCarriesItsProducer_STRV08301: Build records the producer the caller
// states, the member round-trips through the canonical encoding and the strict reader, a
// manifest without one (an older or unstamped writer) still reads and keeps its bytes, and the
// two differ in digest, so a shard cannot match a manifest another binary wrote.
func TestSTRV08301ManifestCarriesItsProducer_STRV08301(t *testing.T) {
	producer := &Producer{Commit: strings.Repeat("a", 40), CommitState: "clean"}
	with, err := Build(Input{Declared: Declared{}, ShardCount: 1, Producer: producer})
	if err != nil {
		t.Fatal(err)
	}
	without, err := Build(Input{Declared: Declared{}, ShardCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(with.Producer, producer) || without.Producer != nil {
		t.Fatalf("producer %+v / %+v", with.Producer, without.Producer)
	}
	dir := t.TempDir()
	for name, m := range map[string]Manifest{"with": with, "without": without} {
		data, err := Encode(m)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"producer"`) != (m.Producer != nil) {
			t.Fatalf("%s: encoded %s", name, data)
		}
		path := filepath.Join(dir, name+".json")
		if err := WriteFile(path, data); err != nil {
			t.Fatal(err)
		}
		read, _, err := ReadManifest(path)
		if err != nil || !reflect.DeepEqual(read.Producer, m.Producer) {
			t.Fatalf("%s: read %v %+v", name, err, read.Producer)
		}
	}
	withDigest, _ := Digest(with)
	withoutDigest, _ := Digest(without)
	if withDigest == withoutDigest {
		t.Fatal("the producer is not part of the manifest digest")
	}
}
