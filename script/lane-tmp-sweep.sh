#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# ASD-V0-2194: operator-only sweep of stale Corvint test scratch. Dry run unless --apply.
set -euo pipefail
apply=no
root=${TMPDIR:-/tmp}
while (($#)); do
  case $1 in
    --root) [[ $# -gt 1 ]] || { echo 'lane-tmp-sweep: --root needs a directory' >&2; exit 2; }; root=$2; shift 2 ;;
    --apply) apply=yes; shift ;;
    --help) echo 'usage: script/lane-tmp-sweep.sh [--root DIR] [--apply]'
      echo 'List owned Corvint test directories idle for over 24 hours; --apply removes them.'
      echo 'Example: script/lane-tmp-sweep.sh --root /tmp'; exit 0 ;;
    *) echo "lane-tmp-sweep: unknown argument $1" >&2; exit 2 ;;
  esac
done
while [[ $root == ?*/ ]]; do root=${root%/}; done
[[ -d $root && ! -L $root ]] || { echo 'lane-tmp-sweep: root must be a real directory' >&2; exit 2; }
root=$(cd -P "$root" && pwd)
[[ $root != / ]] || { echo 'lane-tmp-sweep: refusing /' >&2; exit 2; }
exec perl - "$root" "$apply" <<'PERL'
use strict;
use warnings;
use File::Find ();
use File::Path ();
use Fcntl qw(:mode);
my ($root, $apply) = @ARGV;
my $cutoff = time - 24 * 60 * 60;
# No generic test*, go-build*, cache, slot, or arbitrary corvint-* prefix is admitted.
# heavy-run.* is exclusively heavy.sh's private per-job TMPDIR.
sub admitted { $_[0] =~ /\A(?:corvint-[A-Za-z0-9_-]+-test\.[A-Za-z0-9_]{6}|heavy-run\.[A-Za-z0-9_]{6})\z/ }
sub stale {
    my ($dir) = @_;
    my $ok = 1;
    local $SIG{__WARN__} = sub { $ok = 0 };
    eval {
        File::Find::find({ no_chdir => 1, wanted => sub {
            my @s = lstat($File::Find::name);
            # Never follow or remove symlink trees, foreign-owned content, or fresh content.
            $ok = 0 if !@s || S_ISLNK($s[2]) || $s[4] != $< || $s[9] >= $cutoff;
            $ok = 0 if !-r _ || (-d _ && !-x _);
        } }, $dir);
    };
    return $ok && !$@;
}
sub processes {
    open(my $ps, '-|', 'ps', '-ax', '-o', 'command=') or return;
    local $/;
    my $args = <$ps>;
    close($ps) or return;
    # lsof supplies open files and cwd, including jobs that don't name scratch in their argv.
    open(my $ls, '-|', 'lsof', '-nP', '-a', '-u', "$<", '-Fn') or return;
    my $files = <$ls>;
    # lsof exits 1 for no open files; an empty result is uncertain, so keep everything.
    close($ls) or return;
    return unless defined($files) && length($files);
    return ($args // '') . "\n" . $files;
}
opendir(my $dh, $root) or die "lane-tmp-sweep: cannot read $root: $!\n";
my @names = sort grep { admitted($_) } readdir($dh);
closedir($dh);
my $active = processes();
my ($removed, $kept) = (0, 0);
for my $name (@names) {
    my $dir = "$root/$name";
    my @s = lstat($dir);
    next unless @s && S_ISDIR($s[2]) && !S_ISLNK($s[2]) && $s[4] == $<;
    my $live = defined($active) ? index($active, $dir) >= 0 : 1;
    if ($live || !stale($dir)) { print "KEEP $dir\n"; $kept++; next }
    if ($apply eq 'no') { print "WOULD-REMOVE $dir\n"; next }
    # Recheck recency and process use at the deletion boundary. A process starting after this
    # check is outside the observation; run only on idle lane hosts.
    my $now = processes();
    if (!defined($now) || index($now, $dir) >= 0 || !stale($dir)) { print "KEEP $dir\n"; $kept++; next }
    File::Find::find({ no_chdir => 1, wanted => sub {
        chmod((lstat($_))[2] & 07777 | 0700, $_) if -d $_ && !-l $_;
    } }, $dir);
    File::Path::remove_tree($dir, {error => \my $errors});
    die "lane-tmp-sweep: could not remove $dir\n" if @$errors || -e $dir;
    print "REMOVED $dir\n";
    $removed++;
}
print "SUMMARY applied=$apply removed=$removed kept=$kept\n";
PERL
