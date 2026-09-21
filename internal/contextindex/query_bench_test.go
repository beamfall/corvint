package contextindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkQueryBuildRepresentativeCorpus(b *testing.B) {
	for _, benchmark := range []struct {
		name  string
		build func(context.Context, string) (*Index, error)
	}{
		{"eager", Build},
		{"selective", func(ctx context.Context, root string) (*Index, error) { return BuildQuery(ctx, root, queryTaskFixture) }},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			root := benchmarkQueryRepository(b)
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				built, err := benchmark.build(context.Background(), root)
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(indexBlobBytes(built)), "blob-bytes/op")
			}
		})
	}
}

func BenchmarkQueryAuthorityStart(b *testing.B) {
	root := benchmarkQueryRepository(b)
	built, err := Build(context.Background(), root)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		receipt, err := QueryAuthorityStart(context.Background(), built, queryTaskFixture, 1)
		if err != nil {
			b.Fatal(err)
		}
		if receipt == nil {
			b.Fatal("nil receipt")
		}
	}
}

func benchmarkQueryRepository(b testing.TB) string {
	b.Helper()
	root := b.TempDir()
	benchmarkGit(b, root, "init", "-q")
	benchmarkGit(b, root, "config", "user.email", "corvint@example.test")
	benchmarkGit(b, root, "config", "user.name", "Corvint Test")
	for relative, content := range map[string]string{
		"AGENTS.md": "# Project instructions\n\nThe roadmap is the only active work queue.\n" +
			"Run `make orient`, then `script/context-packet.sh --ticket ID`.\n" +
			"Use `script/roadmap.sh` and run the required workflow gates.\n",
		"script/context-packet.sh": "#!/bin/sh\nexit 0\n",
		"script/roadmap.sh":        "#!/bin/sh\nexit 0\n",
	} {
		benchmarkWriteFile(b, root, relative, content)
	}
	for index := 0; index < 48; index++ {
		benchmarkWriteFile(b, root, fmt.Sprintf("internal/corpus/file%03d.go", index), "package corpus\n\nvar Payload = \""+strings.Repeat("x", 32*1024)+"\"\n")
	}
	benchmarkGit(b, root, "add", ".")
	benchmarkGit(b, root, "commit", "-qm", "representative query corpus")
	return root
}

func benchmarkGit(b testing.TB, root string, arguments ...string) {
	b.Helper()
	prefix := []string{"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false", "-c", "gc.auto=0", "-C", root}
	command := exec.Command("git", append(prefix, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		b.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func benchmarkWriteFile(b testing.TB, root, relative, content string) {
	b.Helper()
	file := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		b.Fatal(err)
	}
}

func indexBlobBytes(index *Index) int {
	total := 0
	for _, source := range index.Sources {
		total += len(source.Data)
	}
	return total
}
