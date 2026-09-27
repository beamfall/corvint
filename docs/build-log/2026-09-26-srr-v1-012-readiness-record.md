## 2026-09-26 SRR-V1-012: the `corvint-readiness-record` command

`cmd/corvint-readiness-record` implements the SRR-V1-012 shape that decision 0422 accepted. Build
mode writes the canonical record to a new `-output` file. Verify mode re-verifies an existing record
and writes nothing. The command runs no gate and never tags, signs, publishes or promotes.

Three details sit inside the accepted shape:

- The evidence file has one row per line: `ROW`, `STATUS`, `PATH`, `DECISION` and `REASON`,
  separated by tabs, with absent values left empty. A relative `PATH` resolves against the evidence
  file's directory. A malformed line or a duplicated row is refused.
- Build mode writes a temporary file beside the output and hard-links it into place. An existing
  output is refused and left unchanged, and a partial record never appears at the output path.
- Verify mode rebuilds the rows from the evidence file and refuses a record whose rows differ.
  `VerifyReadinessRecord` rechecks every recorded digest, but it cannot tell a FAIL log relabelled
  as PASS from a real PASS, because the digest is the same. The test builds that relabelled record,
  shows that the package verifier admits it, and shows that the command's verifier refuses it.

An independent review reproduced two defects, and both are fixed here. First, a CRLF evidence file
left a carriage return in the reason, and that counted as an explanation. A NOT_RUN or FALLBACK row
with no decision and no real reason was then admitted. The parser now reads CRLF as LF, and a
whitespace-only reason explains nothing. Second, `-output` could point inside the candidate. The
record then broke the candidate's closed checksum inventory, so every later candidate check failed.
Build mode now refuses an output inside the candidate or the source root (SRR-V1-011). It compares
by file identity, so a symlink or case alias cannot hide the overlap. Supplying one store flag
without the other is now a usage error (exit 2).

Two further reviews reproduced more gaps, also fixed here. The output guard cleaned the path as
text before following symlinks, so a working directory reached through a symlink into the source
root, or an absolute `link/../FILE`, still wrote inside it. The temporary file was placed in the
textually cleaned directory, so `link/../FILE` leaving the source root still wrote the whole record
into it first. Build now resolves the output directory once to an absolute, symlink-free path,
resolving each component, `..` included, in order from the working directory's resolved path. The
guard checks that directory by file identity, and both the temporary file and the link are written
there. The path does not grow while the guard climbs, so a deep directory is accepted while its
resolved absolute path fits the platform path limit. The reason rule caught only whitespace, so a
zero-width space, a NUL, a terminal escape or a Hangul filler still explained a row, and invalid
UTF-8 built a record its own verify refused. Every reason must now be valid UTF-8 without a control,
format, separator, private-use, noncharacter or default-ignorable code point, and only a reason with
a letter or digit explains. A record built from a CRLF file before CRLF was read as LF no longer
verifies; no such record was ever published.

A fourth review reproduced three more gaps, one overclaim, two undocumented refusals and two test
gaps, all fixed here. The resolver follows up to 255 symlinks but the kernel far fewer, so a longer
chain wrote where the reported path could not reach. Build now refuses unless the directory as
spelled opens the resolved directory. The guard and the write were separate path lookups, so a
component replaced by a symlink during the build redirected the write into a root. Build now opens
the directory before the guard and creates, links and removes the temporary file through that
handle. The reason rule missed the variation selectors that the spec's default-ignorable rule
covers; they are now refused, U+FE0F included. The spec no longer claims that a mount alias of a
directory below a root (bind mount, `subst` drive, network mount) is caught. It now states two
refusals by design: joiners, soft hyphens and direction marks in a reason, and an output directory
through a Windows junction, which Go reports as irregular. A final junction, which the resolver left
in place, is now refused as well. New tests cover a relative output through a link and `..`, a
64-link chain and a directory swapped during the build. The no-write check now watches the
directory's mtime, so it also holds when tests run as root.

The round-4 review, the fifth, reproduced five gaps, all fixed here. A relative evidence `PATH` was
joined with lexical cleaning, so a `..` after a symlinked directory named a different file than the
kernel opens. It is now appended as spelled, and build and verify read it the same way. The
fourth-review claim that a final junction is refused by the identity check was wrong: Go's resolver
refuses every junction on the output directory with ENOTDIR, which Windows words as a missing path.
The error now names the directory as spelled and its possible causes. An output name that is empty,
`.`, `..` or too long for its temporary name is refused before the build, and a failed link is
worded by its cause. Two package variables, in the style of `goToolchainProbe`, let tests swap a
path between resolving and opening and choose the temporary name; the new tests kill the three
surviving mutants of the two identity checks and the exclusive temporary create. An ancestor swapped
for a link into a root and back between the open and the parent comparison is not detected, and the
spec now lists it as a non-goal: the guard stops an operator from naming an output inside a root, the
swap needs a concurrent writer that controls that ancestor, `os.Root` cannot open a handle's parent,
and a handle descent from the volume root would refuse ancestors that are searchable but not
readable.

The round-5 review, the sixth, reproduced one medium gap and four low ones, all fixed here. One
ancestor swap was enough to bypass the guard; no swap back was needed. After the open, the lexical
parents of the resolved path pass through the new link, so the walk never met the root, and a
record landed in the source root's `.git/objects`. Build now resolves the directory again after the
open, checks that settled path against the handle, and walks its parents. The non-goal is narrowed
to a concurrent writer that controls an ancestor of a root or a directory inside one. The claim that
a symlink or case alias of a root cannot hide the overlap was tested only through macOS's `/var`
link; a new test spells the source root through a symlink and, on a case-insensitive filesystem, in
upper case, and kills a lexical-prefix mutant on Linux too. The command's flag-to-mode wiring had no
success-path test; two package variables now let a test run both modes through `run` and check each
flag's destination. A row named in an error is quoted, and an empty `-verify` is a usage error
instead of build mode.

No release has used the record yet. Its first use is the `1.0.0-rc.1` candidate (V1-0018 AC2,
V1-0020 AC3).
