// corvint-corpus-republish is an experimental trusted-local companion. It
// performs no publication and reads no credential environment variables.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Beamfall/corvint/internal/corpusrepublish"
	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/postmergeconnector"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: corvint-corpus-republish build|queue --trusted-local --root ROOT --request FILE --policy FILE --expected-policy-sha256 SHA")
	}
	mode := args[0]
	if mode != "build" && mode != "queue" {
		return fmt.Errorf("republish command unavailable")
	}
	fs := flag.NewFlagSet("corvint-corpus-republish", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "immutable Git repository containing both source and documentation commits")
	docsRoot := fs.String("documentation-root", "", "protected documentation checkout (defaults to root)")
	request := fs.String("request", "", "closed author request JSON")
	policy := fs.String("policy", "", "independently host-owned approval policy JSON")
	expected := fs.String("expected-policy-sha256", "", "host-configured policy byte digest")
	trusted := fs.Bool("trusted-local", false, "host prerequisite: policy and mutable outputs isolated from author environment")
	prevResult := fs.String("previous-result", "", "operator-pinned previous Result")
	prevArtifact := fs.String("previous-artifact", "", "operator-pinned previous corpus artifact")
	prevSidecar := fs.String("previous-sidecar", "", "operator-pinned previous incremental sidecar")
	output := fs.String("output-dir", "", "existing private fresh build-output directory")
	pending := fs.String("pending", "", "private single-writer pending state path")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || !*trusted || *root == "" || *request == "" || *policy == "" || *expected == "" {
		return fmt.Errorf("republish explicit trusted-local host invocation required")
	}
	if mode == "build" && (*output == "" || *pending != "") || mode == "queue" && (*pending == "" || *output != "") {
		return fmt.Errorf("republish command output flags invalid")
	}
	var err error
	*root, err = filepath.Abs(*root)
	if err != nil {
		return err
	}
	*root, err = filepath.EvalSymlinks(*root)
	if err != nil {
		return err
	}
	if *docsRoot == "" {
		*docsRoot = *root
	}
	*docsRoot, err = filepath.Abs(*docsRoot)
	if err != nil {
		return err
	}
	*docsRoot, err = filepath.EvalSymlinks(*docsRoot)
	if err != nil {
		return err
	}
	paths := []string{*request, *policy}
	for _, p := range []string{*prevResult, *prevArtifact, *prevSidecar} {
		if p != "" {
			paths = append(paths, p)
		}
	}
	for i, p := range paths {
		paths[i], err = filepath.Abs(p)
		if err != nil {
			return err
		}
	}
	// Host-owned policy and previous artifacts must be outside both author
	// checkouts and Git metadata. This checks layout, not OS/forge authentication.
	hostProtected := []string{paths[0]}
	for _, r := range []string{*root, *docsRoot} {
		gp, e := postmergeconnector.ProtectedGitPaths(ctx, r)
		if e != nil {
			return e
		}
		hostProtected = append(hostProtected, r)
		hostProtected = append(hostProtected, gp...)
	}
	for _, p := range paths[1:] {
		if err := postmergeconnector.CheckDestination(p, *root, hostProtected); err != nil {
			return err
		}
	}
	requestRaw, err := postmergeconnector.ReadFile(paths[0])
	if err != nil {
		return err
	}
	policyRaw, err := postmergeconnector.ReadFile(paths[1])
	if err != nil {
		return err
	}
	read := func(name string, large bool) ([]byte, error) {
		if name == "" {
			return nil, nil
		}
		p, e := filepath.Abs(name)
		if e != nil {
			return nil, e
		}
		if large {
			return doccorpus.ReadCorpusFile(filepath.Dir(p), filepath.Base(p))
		}
		return postmergeconnector.ReadFile(p)
	}
	priorResult, err := read(*prevResult, false)
	if err != nil {
		return err
	}
	priorArtifact, err := read(*prevArtifact, true)
	if err != nil {
		return err
	}
	priorSidecar, err := read(*prevSidecar, true)
	if err != nil {
		return err
	}
	built, err := corpusrepublish.Build(ctx, *root, requestRaw, policyRaw, *expected, priorResult, priorArtifact, priorSidecar)
	if err != nil {
		return err
	}
	if mode == "queue" {
		var admitted corpusrepublish.Policy
		if err := corpusrepublish.Decode(policyRaw, &admitted); err != nil {
			return err
		}
		state, noop, err := postmergeconnector.SaveRepublishPending(ctx, built.Plan, *pending, *root, paths, []string{*root, *docsRoot}, admitted.ExpectedPendingGeneration, admitted.ExpectedPendingSHA256)
		if err != nil {
			return err
		}
		return emit(stdout, struct {
			Profile    string                     `json:"profile"`
			Generation uint64                     `json:"generation"`
			Noop       bool                       `json:"noop"`
			Request    postmergeconnector.Request `json:"request"`
		}{postmergeconnector.RepublishPendingProfile, state.Generation, noop, state.Plan.Request})
	}
	protected := append([]string{}, paths...)
	for _, r := range []string{*root, *docsRoot} {
		p, e := postmergeconnector.ProtectedGitPaths(ctx, r)
		if e != nil {
			return e
		}
		protected = append(protected, r)
		protected = append(protected, p...)
	}
	type file struct {
		name string
		data []byte
	}
	files := []file{{"result.json", built.ResultBytes}, {"corpus.json", built.Corpus}, {"index.json", built.Index}, {"consumer.json", built.ConsumerBytes}}
	planRaw, err := corpusrepublish.Encode(built.Plan)
	if err != nil {
		return err
	}
	files = append(files, file{"request.json", planRaw})
	if len(built.Sidecar) > 0 {
		files = append(files, file{"incremental.json", built.Sidecar})
	}
	// Validate the entire destination set before the first output. Files are fresh
	// exclusive creations; a build never overwrites prior artifacts or inputs.
	for _, f := range files {
		name := filepath.Join(*output, f.name)
		if err := postmergeconnector.CheckDestination(name, *root, protected); err != nil {
			return err
		}
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			return fmt.Errorf("republish output must be fresh")
		}
	}
	for _, f := range files {
		if err := saveFresh(ctx, filepath.Join(*output, f.name), f.data); err != nil {
			return err
		}
	}
	return emit(stdout, struct {
		Profile      string                     `json:"profile"`
		ResultSHA256 string                     `json:"result_sha256"`
		Stats        doccorpus.IncrementalStats `json:"incremental_stats"`
	}{corpusrepublish.ResultProfile, doccorpus.Digest(built.ResultBytes), built.Stats})
}
func saveFresh(ctx context.Context, name string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(name), ".republish-build-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	n, err := f.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Link(tmp, name)
}
func emit(w io.Writer, v any) error {
	raw, err := corpusrepublish.Encode(v)
	if err != nil {
		return err
	}
	n, err := w.Write(raw)
	if err == nil && n != len(raw) {
		err = io.ErrShortWrite
	}
	return err
}
