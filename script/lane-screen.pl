#!/usr/bin/env perl
# SPDX-License-Identifier: Apache-2.0
# Local dispatch evidence only; no Tasks, shell evaluation, network or state writes in screen.
use strict;
use warnings;
use JSON::PP;
use Getopt::Long qw(GetOptionsFromArray);
use Cwd qw(abs_path);

sub usage {
    print "usage: script/lane-dispatch.sh screen|refs --charter FILE [--repo DIR] [--base REF]\n",
          "  screen: [--tickets FILE] [--ticket ID]... [--open-charter FILE]... [--override ID]...\n",
          "  refs: [--host-repo PATH_OR_REMOTE]... [--publish]\n",
          "Example: script/lane-dispatch.sh screen --charter lane.json --tickets tickets.json\n",
          "Charter JSON: tickets, packages, pins [{path,commit}], idBlocks [{prefix,start,end}],\n",
          "  branches [branch], hosts [{repo}]; status=closed excludes an open charter.\n",
          "Markdown: Tickets:, Package:, Pin: PATH\@SHA, ID-block: PREFIX START END,\n",
          "  Branch: NAME and Host-repo: PATH_OR_REMOTE lines. Repeat lines as needed.\n",
          "Open charters default to REPO/.corvint/lane-charters/*.{json,md}.\n",
          "Findings exit 1; bad/unavailable inputs exit 2. Overrides retain every finding.\n";
}
sub refuse { print STDERR "lane-screen: REFUSE $_[0]; fix: inspect --help and supply readable, valid inputs\n"; exit 2 }
my $mode = shift @ARGV // '';
if (grep { $_ eq '--help' || $_ eq '-h' } @ARGV) { usage(); exit 0 }
my ($repo, $base, $charter, $publish) = ('.', 'origin/main', '', 0);
$ENV{GIT_NO_LAZY_FETCH}=1;
$ENV{GIT_OPTIONAL_LOCKS}=0;
my (@open, @override, @ticket, @hosts, @ticket_files);
GetOptionsFromArray(\@ARGV, 'repo=s'=>\$repo, 'base=s'=>\$base, 'charter=s'=>\$charter,
    'tickets=s@'=>\@ticket_files, 'open-charter=s@'=>\@open, 'override=s@'=>\@override,
    'ticket=s@'=>\@ticket, 'host-repo=s@'=>\@hosts, 'publish'=>\$publish) && !@ARGV
    or refuse('bad arguments');
$mode =~ /^(screen|refs)$/ or refuse('unknown command');
sub git_try {
    my (@args) = @_;
    open my $fh, '-|', 'git', '-C', $repo, @args or refuse('cannot start git');
    local $/; my $s = <$fh> // ''; my $ok = close $fh;
    return ($ok, $s);
}
sub git { my ($ok,$s)=git_try(@_); $ok or refuse("git @_ failed"); return $s }
sub scalar_text { defined($_[0]) && !ref($_[0]) && $_[0] !~ /[\x00-\x1f\x7f]/ }
sub read_file {
    my ($path)=@_;
    -f $path && -s $path <= 4*1024*1024 or refuse("unreadable/oversized file $path");
    open my $fh, '<', $path or refuse("cannot read $path"); local $/; return <$fh> // '';
}
sub read_charter {
    my ($path)=@_; my $s=read_file($path); my $c;
    if ($s =~ /^\s*\{/) {
        $c=eval { decode_json($s) }; ref($c) eq 'HASH' or refuse("bad charter JSON $path");
    } else {
        $c={};
        for (split /\n/, $s) {
            if (/^\s*(?:[-*]\s*)?Tickets?:\s*(.*)$/i) { push @{$c->{tickets}}, $1 =~ /\bV[0-9]+-[0-9]+\b/g }
            if (/^\s*(?:[-*]\s*)?Package:\s*`?([^\s`]+)`?\s*$/i) { push @{$c->{packages}}, $1 }
            if (/^\s*(?:[-*]\s*)?Pin:\s*`?([^\s\@`]+)\@([a-f0-9]{7,40})`?\s*$/i) { push @{$c->{pins}}, {path=>$1,commit=>$2} }
            if (/^\s*(?:[-*]\s*)?ID-block:\s*([A-Z0-9-]+)\s+(\d+)\s+(\d+)\s*$/i) { push @{$c->{idBlocks}}, {prefix=>$1,start=>$2,end=>$3} }
            if (/^\s*(?:[-*]\s*)?Branch:\s*`?([^\s`]+)`?\s*$/i) { push @{$c->{branches}}, $1 }
            if (/^\s*(?:[-*]\s*)?Host-repo:\s*(.+?)\s*$/i) { push @{$c->{hosts}}, {repo=>$1} }
            $c->{status}=lc($1) if /^\s*Status:\s*(open|closed)\s*$/i;
        }
    }
    for my $key (qw(tickets packages pins idBlocks branches hosts)) {
        $c->{$key} //= [];
        ref($c->{$key}) eq 'ARRAY' or refuse("$path $key is not an array");
    }
    for my $key (qw(tickets packages branches)) {
        scalar_text($_) && length($_) or refuse("$path invalid $key member") for @{$c->{$key}};
    }
    for (@{$c->{tickets}}) { /^V\d+-\d+$/ or refuse("$path invalid charter ticket $_") }
    for my $p (@{$c->{pins}}) {
        ref($p) eq 'HASH' && scalar_text($p->{path}) && length($p->{path}) &&
          ($p->{commit}//'') =~ /^[a-f0-9]{7,40}$/ or refuse("$path invalid pin");
    }
    for my $b (@{$c->{idBlocks}}) {
        ref($b) eq 'HASH' && ($b->{prefix}//'') =~ /^[A-Z][A-Z0-9-]+$/ &&
          ($b->{start}//'') =~ /^\d+$/ && ($b->{end}//'') =~ /^\d+$/ &&
          $b->{end} >= $b->{start} && $b->{end}-$b->{start} < 256 or refuse("$path invalid ID block");
    }
    for (@{$c->{hosts}}) { ref($_) eq 'HASH' && scalar_text($_->{repo}) && length($_->{repo}) or refuse("$path invalid host") }
    return $c;
}
my $sha=git('rev-parse','--verify',"$base^{commit}"); chomp $sha;
my $c=length($charter) ? read_charter($charter) : {};
if ($mode eq 'refs') {
    length($charter) or refuse('refs requires --charter');
    @hosts=(@hosts, map { $_->{repo} } @{$c->{hosts}});
    if (!@{$c->{branches}}) { print "lane-screen: HOST_REFS none declared\n"; exit 0 }
    @hosts or refuse('refs requires host repositories for declared branches');
    my @refs;
    for my $b (@{$c->{branches}}) {
        git('check-ref-format',"refs/heads/$b");
        my ($ok,$oid)=git_try('rev-parse','--verify',"refs/heads/$b^{commit}");
        if (!$ok) { ($ok,$oid)=git_try('rev-parse','--verify',"refs/remotes/origin/$b^{commit}") }
        $ok or refuse("missing source branch $b"); chomp $oid; push @refs, [$b,$oid];
    }
    # Publish explicit, non-forced refspecs only. Verify every host after publication.
    sub publish_refs {
        my ($host,$refs)=@_;
        $host !~ /^-/ && scalar_text($host) or refuse('invalid host repository');
        git('push','--', $host, map { "$_->[1]:refs/heads/$_->[0]" } @$refs);
    }
    my $bad=0;
    for my $host (@hosts) {
        scalar_text($host) && $host !~ /^-/ or refuse('invalid host repository');
        publish_refs($host,\@refs) if $publish;
        my $out=git('ls-remote','--refs','--', $host, map { "refs/heads/$_->[0]" } @refs);
        my %found=map { my ($oid,$ref)=split /\s+/; ($ref=>$oid) } grep { length } split /\n/, $out;
        for my $r (@refs) {
            my ($b,$oid)=@$r; my $got=$found{"refs/heads/$b"}//'MISSING';
            if ($got ne $oid) { print "lane-screen: HOST_REF host=$host branch=$b expected=$oid actual=$got\n"; $bad=1 }
            else { print "lane-screen: HOST_REF_OK host=$host branch=$b commit=$oid\n" }
        }
    }
    exit $bad;
}
$publish && refuse('--publish is only valid with refs');
my %wanted=map { /\b(V\d+-\d+)$/ ? ($1=>1) : refuse("invalid ticket $_") } (@ticket,@{$c->{tickets}//[]});
my %body;
for my $tickets (@ticket_files) {
    my $data=eval { decode_json(read_file($tickets)) }; $@ and refuse("bad tickets JSON $tickets");
    $data=$data->{items} if ref($data) eq 'HASH' && ($data->{outcome}//'') eq 'OK';
    ref($data) eq 'ARRAY' or refuse('tickets must be an array or OK ticket-show result');
    for (@$data) {
        ref($_) eq 'HASH' && ($_->{ticketId}//'') =~ /\b(V\d+-\d+)$/ or refuse('invalid ticket record');
        my $id=$1; $wanted{$id}=1;
        my $r=$_->{record}//{}; my $o=$_->{operatorNote}//{};
        ref($r) eq 'HASH' && ref($o) eq 'HASH' && !ref($r->{body}) && !ref($o->{text}) or refuse('invalid ticket body or operator note');
        $body{$id}=($r->{body}//'').' '.($o->{text}//'');
    }
}
%wanted or refuse('screen needs tickets in charter, --ticket or --tickets');
my %waive=map { /^(V\d+-\d+)$/ ? ($1=>1) : refuse("invalid override $_") } @override;
my $bad=0;
sub finding {
    my ($kind,$id,$detail,$charter_wide)=@_;
    my $over=$charter_wide ? 0 : ($waive{$id}//0);
    print "lane-screen: $kind ticket=$id $detail", ($over ? ' OVERRIDDEN' : ''), "\n";
    $bad=1 unless $over;
}
my @charters=@open;
my $registry="$repo/.corvint/lane-charters";
if (-d $registry) {
    opendir my $dh,$registry or refuse("cannot read charter registry $registry");
    push @charters,map { "$registry/$_" } sort grep { /\.(json|md)$/ && -f "$registry/$_" } readdir $dh;
    closedir $dh;
}
my %seen;
my @others;
for my $f (@charters) {
    next if $seen{abs_path($f)//$f}++ || (length($charter) && (abs_path($f)//$f) eq (abs_path($charter)//$charter));
    my $other=read_charter($f); next if ($other->{status}//'open') eq 'closed';
    push @others, [$f,$other];
    for my $id (@{$other->{tickets}}) { finding('CLAIMED',$id,"charter=$f") if $wanted{$id} }
}
# Subjects are references to delivered work, not proof that every AC was completed.
my ($main_ok,$main)=git_try('rev-parse','--verify','--quiet','refs/remotes/origin/main^{commit}');
chomp $main; $main=$sha unless $main_ok;
for my $line (split /\n/, git('log','--format=%H%x09%s',$sha,$main)) {
    my ($oid,$subject)=split /\t/, $line, 2;
    my %ids=map { $_=>1 } $subject =~ /\b(V\d+-\d+)\b/g;
    for my $id (sort keys %ids) { finding('DELIVERED',$id,"commit=$oid subject=$subject") if $wanted{$id} }
}
my @open_refs;
my %named_branches=map { $_=>1 } (@{$c->{branches}//[]},map { @{$_->[1]{branches}} } @others);
for my $ref (split /\n/, git('for-each-ref','--format=%(refname)','refs/heads','refs/remotes/origin')) {
    (my $name=$ref) =~ s{^refs/(heads|remotes/origin)/}{};
    next unless $named_branches{$name} || $ref =~ m{/(?:[^/]+/)*(?:lane-|batch-)[^/]+$};
    my $log=git('log','--format=%H%x09%s',"$main..$ref");
    push @open_refs,$ref if length($log);
    for my $line (split /\n/, $log) {
        my ($oid,$subject)=split /\t/,$line,2;
        my %ids=map { $_=>1 } $subject =~ /\b(V\d+-\d+)\b/g;
        for my $id (sort keys %ids) { finding('CLAIMED',$id,"branch=$ref commit=$oid subject=$subject") if $wanted{$id} }
    }
}
for my $id (sort keys %body) {
    my $s=$body{$id};
    while ($s =~ /\bduplicat(?:e[sd]?|ing)\s+(?:of\s+)?(V\d+-\d+)([^\n]*)/ig) {
        finding('DUPLICATE',$id,"of=$1 evidence=$2");
    }
    while ($s =~ /(?:\bBASE\s*[=:]\s*|\bSource ref:\s*([^\s\@]+)\@)([a-f0-9]{7,40})/g) {
        my ($ref,$pin)=($1,$2); my ($ok,$now)=git_try('rev-parse','--verify',defined($ref)?"$ref^{commit}":$sha);
        chomp $now;
        finding('STALE_PIN',$id,"pin=$pin expected=$now") if !$ok || index($now,$pin)!=0;
    }
}
my $owner=join(',',sort keys %wanted);
for my $p (@{$c->{packages}//[]}) {
    $p =~ s{^\./}{}; $p =~ s{/$}{};
    my ($ok,$out)=git_try('ls-tree',"$sha:$p");
    finding('STALE_PACKAGE',$owner,"package=$p base=$sha (no Go package)",1) unless $ok && $out =~ /\b[^\s\/]+\.go$/m;
}
for my $p (@{$c->{pins}//[]}) {
    my ($pin_exists)=git_try('cat-file','-e',"$p->{commit}:$p->{path}");
    my ($base_exists)=git_try('cat-file','-e',"$sha:$p->{path}");
    my ($ok,$out)=git_try('diff','--name-only', $p->{commit}, $sha,'--',$p->{path});
    finding('STALE_PIN',$owner,"path=$p->{path} pin=$p->{commit} base=$sha",1) if !$pin_exists || !$base_exists || !$ok || length($out);
}
my $index='';
if (@{$c->{idBlocks}//[]}) { $index=git('show',"$sha:docs/specs/REQUIREMENTS.tsv") }
my %requested_id;
for my $b (@{$c->{idBlocks}//[]}) {
    for my $n ($b->{start}..$b->{end}) {
        my $id=sprintf '%s-%03d',$b->{prefix},$n;
        finding('COLLIDING_ID',$owner,"id=$id overlaps another block in $charter",1) if $requested_id{$id}++;
        finding('COLLIDING_ID',$owner,"id=$id base=$sha",1) if $index =~ /^\Q$id\E\t/m;
    }
    for my $o (@others) {
        for my $other (@{$o->[1]{idBlocks}}) {
            finding('COLLIDING_ID',$owner,"prefix=$b->{prefix} range=$b->{start}..$b->{end} charter=$o->[0]",1)
              if $b->{prefix} eq $other->{prefix} && $b->{start} <= $other->{end} && $other->{start} <= $b->{end};
        }
    }
}
if (%requested_id) {
    for my $ref (@open_refs) {
        my $rows=git('show',"$ref:docs/specs/REQUIREMENTS.tsv");
        for my $id ($rows =~ /^([^\t\n]+)\t/gm) {
            finding('COLLIDING_ID',$owner,"id=$id branch=$ref",1) if $requested_id{$id};
        }
    }
}
print "lane-screen: PASS base=$sha tickets=",scalar(keys %wanted),"\n" unless $bad;
exit $bad;
