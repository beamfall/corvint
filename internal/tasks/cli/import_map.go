package cli

import (
	"path/filepath"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// importMapCommand runs `corvint-tasks import-map update` (CTS-V0-020..022):
// the only writer of intent/import-map.json after init. It needs an OWNER
// binding and the digest of the map the caller read.
func importMapCommand(env Env, args []string) *wire.Result {
	cmd := []string{"import-map", "update"}
	if len(args) == 0 || args[0] != "update" {
		return usage([]string{"import-map"}, "import-map needs the verb update")
	}
	args = args[1:]
	role, request, expected, file := "OWNER", "", "", ""
	values := map[string]*string{"--role": &role, "--request-id": &request, "--expected-import-map-sha256": &expected, "--file": &file}
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		key := args[i]
		destination, ok := values[key]
		if !ok || seen[key] || i+1 >= len(args) {
			return usage(cmd, "import-map update requires --request-id ID --expected-import-map-sha256 SHA256 --file PATH and optional --role OWNER; duplicate and unknown flags refuse")
		}
		seen[key] = true
		i++
		*destination = args[i]
	}
	if !seen["--request-id"] || !seen["--expected-import-map-sha256"] || !seen["--file"] {
		return usage(cmd, "import-map update requires --request-id ID --expected-import-map-sha256 SHA256 --file PATH")
	}
	if _, err := mutation.ParseRequestID("requestId", request); err != nil {
		return errorResult(cmd, err)
	}
	digest, err := wire.ParseDigest("expectedImportMapSha256", expected)
	if err != nil {
		return errorResult(cmd, err)
	}
	actor, err := initActor(role)
	if err != nil {
		return errorResult(cmd, err)
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(env.Cwd, file)
	}
	raw, err := intent.ReadFile(file, wire.MaxImportMapBytes)
	if err != nil {
		return errorResult(cmd, err)
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	observed, _, err := writerProbe(nil, repo.StateDir)
	if err != nil {
		return errorResult(cmd, err)
	}
	now, err := wire.ParseTimestamp("recordedAt", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return errorResult(cmd, err)
	}
	report, err := store.ImportMapUpdate(writerContext(), repo, actor, store.ImportMapRequest{QueueID: observed.Head.QueueID.Raw, RequestID: request, ExpectedImportMapSha256: digest, ImportMap: raw}, now)
	if err != nil {
		return errorResult(cmd, err)
	}
	res := mutateResult(cmd, report)
	res.Items[0].Obj.Set("newImportMapSha256", wire.String(string(wire.Sum(raw))))
	return res
}
