## 2026-09-26 V1-0388 EAF-V0-013: a FIFO or device ignore or attributes file is refused before status

Git opens an in-tree `.gitignore` or `.gitattributes` without `O_NONBLOCK`. When one of them is a
FIFO, `git status` waits for a writer until the caller's Git deadline expires, and the refusal
names the deadline instead of the file. A mutant run confirmed this for a root `.gitignore` and for
a `.gitattributes` in a tracked subdirectory: without the check, both cases hang for the test's
full 20-second context.

Decision: before status runs, `gitstatus` lists the ignore and attributes files in the root and in
the directory of every index entry, and inspects each with `Lstat` through `os.Root`. A FIFO or a
device refuses with the existing class `metadata-unreadable` and the reason `worktree file <name>
is a FIFO` (`is not a regular file` for a device). The closed class set of decision 0383 is
unchanged, so no amendment is needed; a FIFO here is an unreadable input, the same class as an
irregular `info/exclude`. A symlink and a directory are admitted: Git does not follow a symlinked in-tree
ignore or attributes file, and it skips a directory.

The entry paths come from the index bytes status already reads. A small decoder handles index
versions 2 to 4 and SHA-1 or SHA-256 object IDs, choosing the size that frames every entry. An
index that frames with neither size lists only the root pair, and Git still validates the index.

Set aside: a full worktree walk, which would descend ignored trees; an extra `git ls-files`
process, which would spend the process budget and could disagree with the status memo; and a new
reason class, which needs a 0383 amendment for no new caller distinction.

Residual: an untracked directory is not listed, so a FIFO ignore file there is still bounded only
by the Git deadline. Platforms other than darwin and linux refuse as before (`errPlatform`).

Evidence: `TestStatusRefusesBlockingWorktreeInputsPromptly` (root `.gitignore` and nested
`.gitattributes`; refused well inside 10 s and status never runs), `TestStatusAdmitsSymlinkedWorktreeInputs`,
`TestWorktreeInputsListEveryIndexDirectory` (index v3 and v4, SHA-1 and SHA-256, an
intent-to-add entry, an unframed index) and `TestStandaloneBuildRefusesFIFOIgnoreFilePromptly`
(`repository-probe-failed`, `metadata-unreadable`, before `gitDeadline`). Rollback: revert the
change; a FIFO ignore or attributes file then hangs status until the Git deadline, as before.
