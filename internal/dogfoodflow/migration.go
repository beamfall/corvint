package dogfoodflow

import (
	"sort"
	"strings"
)

// cemMigrationCommits are the commits that rewrote committed sealed CEMs to
// cem/1.0 in place (decision 0525 §3 step 6 and §5, DCW-V0-050): V1-1277's
// migration commit and any follow-up migration commit that converts maps a
// pre-cutover branch landed afterwards. Append each follow-up commit's full SHA
// in the change that lands it. From each one on it is the seal baseline: a
// sealed CEM it rewrote must keep the bytes it wrote. Tests point it at their
// own fixture commits.
var cemMigrationCommits = []string{
	"0b6f1bc245b44a8620c5e64f91e87f6d08da08d0",
	"3b86121f4ced8b90212eaab6644ffbb4c491710d", // #838 b4f3758d
}

// migrationRewrite is one sealed CEM a migration commit rewrote in place.
type migrationRewrite struct{ commit, before, after string }

// migratedSeals lists the sealed CEM paths that BASE..HEAD modifies only
// through a migration commit in the range: BASE holds exactly the bytes it
// rewrote and HEAD holds exactly the bytes it wrote. It also returns each path
// such a migration rewrote whose HEAD bytes are neither, for example a merge
// that restores the pre-migration tree: those are refused
// sealed-cem-migration-reverted even when the net BASE..HEAD diff hides them
// (DCW-V0-050). It reads listings, not one Git call per path.
func (f *flow) migratedSeals(base, target string) (map[string]bool, []string) {
	rewrites := map[string][]migrationRewrite{}
	for _, commit := range f.migrationsInRange(base, target) {
		for path, rewrite := range f.migrationRewrites(commit) {
			rewrites[path] = append(rewrites[path], rewrite)
		}
	}
	if len(rewrites) == 0 {
		return nil, nil
	}
	baseBlobs, targetBlobs := f.sealedBlobs(base), f.sealedBlobs(target)
	admitted, reverted := map[string]bool{}, []string{}
	for path, list := range rewrites {
		wrote, fromBase := false, false
		for _, rewrite := range list {
			wrote = wrote || targetBlobs[path] == rewrite.after
			fromBase = fromBase || baseBlobs[path] == rewrite.before
		}
		switch {
		case targetBlobs[path] != "" && !wrote:
			reverted = append(reverted, path)
		case wrote && fromBase:
			admitted[path] = true
		}
	}
	sort.Strings(reverted)
	return admitted, reverted
}

// migrationRewrites maps each sealed CEM path the migration commit modified to
// its blobs before and after.
func (f *flow) migrationRewrites(commit string) map[string]migrationRewrite {
	raw, status := f.git(true, "diff-tree", "-r", "--raw", "--no-renames", "--no-abbrev", "--no-commit-id", commit+"^", commit, "--", ".corvint/changes")
	if status != 0 {
		return nil
	}
	rewrites := map[string]migrationRewrite{}
	for _, line := range nonEmpty(strings.Split(raw, "\n")) {
		meta, path, _ := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if len(fields) == 5 && fields[4] == "M" && fields[2] != fields[3] && sealedCEMPath.FindStringSubmatch(path) != nil {
			rewrites[path] = migrationRewrite{commit: commit, before: fields[2], after: fields[3]}
		}
	}
	return rewrites
}

// sealedBlobs maps each regular file under .corvint/changes at rev to its blob.
func (f *flow) sealedBlobs(rev string) map[string]string {
	blobs := map[string]string{}
	listing, status := f.git(true, "ls-tree", "-r", "--full-tree", rev, "--", ".corvint/changes")
	if status != 0 {
		return blobs
	}
	for _, line := range nonEmpty(strings.Split(listing, "\n")) {
		meta, path, _ := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); len(fields) == 3 && fields[1] == "blob" {
			blobs[path] = fields[2]
		}
	}
	return blobs
}

// migratedConstituent reports a sealed CEM that target holds as a migration
// commit's rewrite of its bind commit's sidecar. Its bind commit's excluded
// sidecar still holds the source bytes, so the stable verifier cannot verify
// the rewrite there (CEM-V1-104); the batch must be split at the migration
// commit instead (DCW-V0-050).
func (f *flow) migratedConstituent(target, path, bindBlob, sealedBlob string) bool {
	if bindBlob == "" {
		return false
	}
	for _, commit := range cemMigrationCommits {
		if commit == "" || !f.gitSucceeds("merge-base", "--is-ancestor", commit, target) {
			continue
		}
		before, after := f.gitRevision(commit+"^:"+path), f.gitRevision(commit+":"+path)
		if before == bindBlob && after == sealedBlob && before != after {
			return true
		}
	}
	return false
}

// migrationNote names the migration commits in base..target for a NOTE line.
func (f *flow) migrationNote(base, target string) string {
	return strings.Join(f.migrationsInRange(base, target), ",")
}

// migrationsInRange lists the migration commits in base..target.
func (f *flow) migrationsInRange(base, target string) []string {
	inRange := []string{}
	for _, commit := range cemMigrationCommits {
		if commit != "" && f.gitSucceeds("merge-base", "--is-ancestor", commit, target) &&
			!f.gitSucceeds("merge-base", "--is-ancestor", commit, base) {
			inRange = append(inRange, commit)
		}
	}
	return inRange
}

// isMigrationCommit reports a migration commit itself when it only modifies
// sealed CEM paths, which batch-check then counts as bound (DCW-V0-050).
func (f *flow) isMigrationCommit(parent, commit string) bool {
	listed := false
	for _, migration := range cemMigrationCommits {
		listed = listed || (migration != "" && migration == commit)
	}
	if !listed {
		return false
	}
	changed, status := f.git(true, "diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-status", parent, commit)
	lines := nonEmpty(strings.Split(changed, "\n"))
	if status != 0 || len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		state, path, _ := strings.Cut(line, "\t")
		if state != "M" || sealedCEMPath.FindStringSubmatch(path) == nil {
			return false
		}
	}
	return true
}
