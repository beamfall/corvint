#!/usr/bin/env perl
# SPDX-License-Identifier: Apache-2.0
# ASD-V0-2200..2203. Real Git, fixture-only tickets/hosts/binaries; no live Tasks or host access.
use strict;
use warnings;
use Test::More;
use File::Temp qw(tempdir);
use File::Path qw(make_path remove_tree);
use Cwd qw(abs_path);
use JSON::PP;
my $script=abs_path('script');
delete @ENV{grep { /^GIT_/ } keys %ENV};
$ENV{LC_ALL}='C';
my $tmp=tempdir('lane-screen-test.XXXXXX',TMPDIR=>1,CLEANUP=>1);
my $repo="$tmp/repo";
make_path("$repo/pkg/live","$repo/docs/specs");
sub write_file { my ($f,$s)=@_; open my $h,'>',$f or die "$f: $!"; print $h $s; close $h or die $! }
sub json { write_file($_[0],encode_json($_[1])) }
sub run {
    my (@argv)=@_;
    my $pid=open my $h,'-|'; defined($pid) or die $!;
    if (!$pid) { open STDERR,'>&',STDOUT or die $!; exec @argv; die "$argv[0]: $!" }
    local $/; my $out=<$h>//''; close $h; return ($?>>8,$out);
}
sub g { my ($rc,$s)=run('git','-C',$repo,@_); $rc==0 or die "git @_: $s"; chomp $s; return $s }
sub commit { g('add','.'); g('-c','user.name=fixture','-c','user.email=fixture@local','commit','-qm',$_[0]); return g('rev-parse','HEAD') }
g('init','-q');
write_file("$repo/go.mod","module github.com/Beamfall/corvint\n");
write_file("$repo/pkg/live/live.go","package live\n");
write_file("$repo/docs/specs/REQUIREMENTS.tsv","ASD-V0-2209\tfixture.md\toccupied\n");
my $old=commit('fixture base');
write_file("$repo/pkg/live/live.go","package live\nconst New = true\n");
commit('fix: V1-1534 delivered');
write_file("$repo/note","one\n"); commit('fix: V1-2149 AC1 delivered');
write_file("$repo/note","two\n"); commit('fix: V1-1672 delivered');
write_file("$repo/note","three\n"); my $duplicate=commit('fix: V1-2511 delivered (original f33bb4971)');
my $base=g('rev-parse','HEAD'); g('update-ref','refs/remotes/origin/main',$base);
g('branch','lane-other');
g('checkout','-q','lane-other');
write_file("$repo/docs/specs/REQUIREMENTS.tsv","ASD-V0-2209\tfixture.md\toccupied\nASD-V0-2208\tfixture.md\tin flight\n");
write_file("$repo/note","in flight\n"); commit('fix: V1-9999 in flight, V1-99990 lookalike');
g('branch','batch-flight'); g('checkout','-q','--detach',$base);
my $charter="$tmp/charter.json"; my $other="$tmp/other.json";
json($charter,{tickets=>['V1-9998'],packages=>['pkg/live']});
json($other,{tickets=>['V1-9998','V1-9999']});
sub screen { run('bash',"$script/lane-dispatch.sh",'screen','--repo',$repo,'--base',$base,'--charter',$charter,@_) }
my ($rc,$out)=screen('--open-charter',$other);
is($rc,1,'ASD-V0-2200 duplicate open charters refuse'); like($out,qr/CLAIMED ticket=V1-9998 charter=\Q$other\E/,'names charter');
($rc,$out)=screen('--open-charter',$other,'--override','V1-9998');
is($rc,0,'explicit ticket override admits'); like($out,qr/OVERRIDDEN/,'override retains finding');
($rc,$out)=screen('--open-charter',$other,'--override','V1-1000');
is($rc,1,'unrelated override does not admit the claim');
make_path("$repo/.corvint/lane-charters");
json("$repo/.corvint/lane-charters/open.json",{tickets=>['V1-9998']});
($rc,$out)=screen(); is($rc,1,'registered open charter discovered without explicit input');
unlink "$repo/.corvint/lane-charters/open.json" or die $!;
json($charter,{tickets=>['V1-9999','V1-999']});
($rc,$out)=screen(); is($rc,1,'unmerged branch subjects refuse');
like($out,qr/CLAIMED ticket=V1-9999 branch=refs\/heads\/lane-other commit=/,'names lane commit');
like($out,qr/branch=refs\/heads\/batch-flight/,'lists every named branch');
unlike($out,qr/ticket=V1-999\s/,'whole ticket token, no prefix match');
json($charter,{tickets=>['V1-9998'],idBlocks=>[{prefix=>'ASD-V0',start=>2208,end=>2208}]});
($rc,$out)=screen(); is($rc,1,'open branch ID collision refuses');
like($out,qr/COLLIDING_ID .*id=ASD-V0-2208 branch=refs\/heads\/lane-other/,'names unmerged branch ID evidence');
json($other,{tickets=>['V1-9998'],status=>'closed'});
json($charter,{tickets=>['V1-9998']});
($rc,$out)=screen('--open-charter',$other); is($rc,0,'closed charter excluded');
my $tickets="$tmp/tickets.json";
json($tickets,[map { {ticketId=>$_,record=>{body=>$_ eq 'V1-3048' ? "Duplicate of V1-2511 (f33bb4971, fixture commit $duplicate)" : $_ eq 'V1-2886' ? "BASE=$old" : ''}} }
    qw(V1-1534 V1-2149 V1-1672 V1-3048 V1-2886)]);
json($charter,{tickets=>['V1-9998'],packages=>['pkg/removed'],pins=>[{path=>'pkg/live/live.go',commit=>$old}],idBlocks=>[{prefix=>'ASD-V0',start=>2209,end=>2209}]});
($rc,$out)=screen('--tickets',$tickets); is($rc,1,'ASD-V0-2201 combined regression screen refuses');
like($out,qr/DELIVERED ticket=$_ commit=/,"delivered $_") for qw(V1-1534 V1-2149 V1-1672);
like($out,qr/V1-2149 AC1 delivered/,'partial AC delivery remains explicit');
like($out,qr/DUPLICATE ticket=V1-3048 of=V1-2511.*f33bb4971/,'duplicate with retained original pin');
like($out,qr/STALE_PIN ticket=V1-2886/,'stale ticket pin');
like($out,qr/STALE_PIN ticket=.*path=pkg\/live\/live.go/,'stale charter content pin');
like($out,qr/STALE_PACKAGE .*package=pkg\/removed/,'nonexistent charter package');
like($out,qr/COLLIDING_ID .*id=ASD-V0-2209/,'occupied requirement block');
my $state=g('status','--porcelain','--untracked-files=all').g('show-ref');
($rc,$out)=screen('--tickets',$tickets);
is(g('status','--porcelain','--untracked-files=all').g('show-ref'),$state,'screen leaves repository and refs unchanged');
json($charter,{tickets=>['V1-1534']});
($rc,$out)=screen('--base',$old); is($rc,1,'old dispatch pin still screens delivered work on current origin/main');
like($out,qr/DELIVERED ticket=V1-1534/,'newer main delivery retained');
json($charter,{tickets=>['V1-9998'],idBlocks=>[{prefix=>'ASD-V0',start=>2200,end=>2202}]});
json($other,{tickets=>[],idBlocks=>[{prefix=>'ASD-V0',start=>2202,end=>2204}]});
($rc,$out)=screen('--open-charter',$other); like($out,qr/COLLIDING_ID .*charter=/,'open charter block overlaps');
write_file("$tmp/charter.md","Tickets: V1-9998\nPackage: pkg/live\nID-block: ASD-V0 2210 2211\n");
($rc,$out)=run('bash',"$script/lane-dispatch.sh",'screen','--repo',$repo,'--base',$base,'--charter',"$tmp/charter.md");
is($rc,0,'documented Markdown charter works');
# Review fix: a recognized Markdown field whose value does not parse refuses (exit 2).
for my $bad ('Pin: go.mod@bogus','ID-block: ASD-V0 nope 2209','Tickets: none','Package: pkg/a pkg/b',
             'Branch: two words','- Host-repo:','Status: done') {
    write_file("$tmp/bad.md","Tickets: V1-9998\n$bad\n");
    ($rc,$out)=run('bash',"$script/lane-dispatch.sh",'screen','--repo',$repo,'--base',$base,'--charter',"$tmp/bad.md");
    is($rc,2,"malformed Markdown '$bad' refuses"); like($out,qr/REFUSE .*malformed Markdown/,"names malformed '$bad'");
}
json($charter,{tickets=>['V1-9998'],pins=>[{path=>'x',commit=>'bogus'}]});
($rc,$out)=screen(); is($rc,2,'malformed pin refuses instead of passing');
json($charter,{tickets=>['V1-9998'],pins=>[{path=>'missing.go',commit=>$base}]});
($rc,$out)=screen(); is($rc,1,'missing path at both pin and base is stale');
json($charter,{tickets=>['V1-9998'],packages=>['pkg/removed']});
($rc,$out)=screen('--override','V1-9998'); is($rc,1,'ticket override cannot conceal a stale charter package');
json($tickets,[{ticketId=>'V1-9998',record=>'bad'}]);
($rc,$out)=screen('--tickets',$tickets); is($rc,2,'malformed ticket body refuses with usage status');

# ASD-V0-2202: local bare repositories stand in for host clones. The two named branches
# are both published to both hosts; later movement is refused rather than force-pushed.
my @hosts=("$tmp/host one.git","$tmp/host-two.git");
for (@hosts) { my ($r,$s)=run('git','init','--bare','-q',$_); $r==0 or die $s }
json($charter,{tickets=>['V1-9998'],branches=>['lane-other','batch-flight'],hosts=>[map {{repo=>$_}} @hosts]});
sub refs { run('bash',"$script/lane-dispatch.sh",'refs','--repo',$repo,'--base',$base,'--charter',$charter,@_) }
($rc,$out)=refs(); is($rc,1,'missing host refs refuse');
is(scalar(()=$out =~ /actual=MISSING/g),4,'every missing branch on every host listed');
($rc,$out)=refs('--publish'); is($rc,0,'publish named refs to all hosts');
is(scalar(()=$out =~ /HOST_REF_OK/g),4,'verify all four host refs');
($rc,$out)=refs(); is($rc,0,'dispatch host check passes after publication');
g('branch','-f','lane-other',$old);
($rc,$out)=refs(); is($rc,1,'existing wrong host tip refuses');
($rc,$out)=refs('--publish'); is($rc,2,'publication never force rewinds a host');
g('branch','-f','lane-other',g('rev-parse','batch-flight'));
# Review fix: relative host paths resolve from the invocation directory, never beneath --repo;
# a configured remote name of --repo still passes as a name. A decoy under --repo must stay empty.
for ("$tmp/rel-host.git","$repo/rel-host.git") { my ($r,$s)=run('git','init','--bare','-q',$_); $r==0 or die $s }
json("$tmp/relhost.json",{tickets=>['V1-9998'],branches=>['lane-other','batch-flight']});
write_file("$tmp/relhost.md","Tickets: V1-9998\nBranch: lane-other\nHost-repo: rel-host.git\n");
sub refs_in { run('sh','-c','cd "$1" && shift && exec "$@"','sh',$tmp,'bash',"$script/lane-dispatch.sh",'refs','--repo',$repo,'--base',$base,@_) }
($rc,$out)=refs_in('--charter',"$tmp/relhost.json",'--host-repo','rel-host.git','--publish');
is($rc,0,'relative --host-repo publishes from the invocation directory');
like($out,qr/HOST_REF_OK host=\Q$tmp\E\/rel-host\.git branch=lane-other/,'resolved host path named');
my ($lr,$lo)=run('git','--git-dir',"$tmp/rel-host.git",'show-ref'); like($lo,qr/refs\/heads\/batch-flight/,'invocation-relative host received refs');
($lr,$lo)=run('git','--git-dir',"$repo/rel-host.git",'show-ref'); is($lo,'','decoy beneath --repo untouched');
($rc,$out)=refs_in('--charter',"$tmp/relhost.md"); is($rc,0,'relative charter Host-repo verifies from the invocation directory');
g('remote','add','fixture-host',$hosts[0]);
($rc,$out)=refs_in('--charter',"$tmp/relhost.json",'--host-repo','fixture-host');
is($rc,0,'configured remote name still resolves through --repo'); like($out,qr/HOST_REF_OK host=fixture-host /,'remote name retained');
g('remote','remove','fixture-host'); remove_tree("$repo/rel-host.git");

# Full dispatch path uses only a fixture ticket reader, never the real Tasks runtime.
my $reader="$tmp/tasks";
write_file($reader,"#!/bin/sh\nprintf '%s\\n' '{\"outcome\":\"OK\",\"items\":[{\"ticketId\":\"V1-9998\",\"record\":{\"body\":\"new work\"}}]}'\n"); chmod 0755,$reader;
local $ENV{LANE_DISPATCH_TASKS}=$reader;
sub inputs { run('bash',"$script/lane-dispatch.sh",'inputs','--lane','fixture','--repo',$repo,'--base',$base,'--date','2026-11-03','--out',$_[0],'--charter',$charter,@_[1..$#_],'V1-9998') }
json($other,{tickets=>['V1-9998']});
($rc,$out)=inputs("$tmp/blocked",'--open-charter',$other);
is($rc,2,'dispatch itself runs open charter screen'); ok(!-e "$tmp/blocked/lane.env",'no dispatch receipt on refusal');
($rc,$out)=inputs("$tmp/dispatch"); is($rc,0,'full dispatch screens tickets and checks host refs'); ok(-f "$tmp/dispatch/lane.env",'receipt emitted only after checks');
my ($drop,$drop_out)=run('git','--git-dir',$hosts[0],'update-ref','-d','refs/heads/batch-flight'); $drop==0 or die $drop_out;
($rc,$out)=inputs("$tmp/missing-ref"); is($rc,2,'dispatch refuses a missing named branch on host');
ok(!-e "$tmp/missing-ref/lane.env",'missing host ref prevents dispatch receipt');
($rc,$out)=inputs("$tmp/publish-dispatch",'--publish-refs'); is($rc,0,'dispatch publishes missing refs and completes');
like($out,qr/HOST_REF_OK/,'dispatch retains host ref verification');
write_file($reader,"#!/bin/sh\nprintf '%s\\n' '{\"outcome\":\"OK\",\"items\":[{\"ticketId\":\"V1-9998\",\"record\":{\"body\":\"Duplicate of V1-2511 (f33bb4971)\"}}]}'\n");
($rc,$out)=inputs("$tmp/body-blocked"); is($rc,2,'dispatch screens fixture ticket bodies'); like($out,qr/DUPLICATE ticket=V1-9998/,'runtime body finding retained');

# ASD-V0-2203: even a pinned-old lane must refuse a host tool stale against origin/main.
my $binary="$tmp/corvint";
write_file($binary,"#!/bin/sh\necho 'Corvint fixture (build 1, commit $old)'\n"); chmod 0755,$binary;
($rc,$out)=run('bash',"$script/lane-binary-check.sh",'--corvint',$binary,'--against','origin/main',$repo);
is($rc,1,'fake stale binary refused against origin/main'); like($out,qr/STALE.*build 1.*Go source paths differ/,'both builds and source difference retained');
($rc,$out)=run('bash',"$script/lane-binary-check.sh",'--corvint',$binary,'--against',$old,$repo);
is($rc,0,'same old binary is fresh against pinned old base');
make_path("$tmp/bin"); write_file("$tmp/bin/corvint","#!/bin/sh\necho 'Corvint fixture (build 1, commit $old)'\n"); chmod 0755,"$tmp/bin/corvint";
local $ENV{PATH}="$tmp/bin:$ENV{PATH}";
($rc,$out)=run('bash',"$script/lane-setup.sh",'--repo',$repo,'--dir',"$tmp/lane",'--branch','lane-start','--base',$old);
is($rc,2,'lane start refuses stale host despite old pin'); ok(!-e "$tmp/lane",'refusal precedes lane creation');
like($out,qr/host-binary-not-current/,'actionable lane start error');
write_file("$tmp/bin/corvint","#!/bin/sh\necho 'Corvint fixture (build 5, commit $base)'\n");
($rc,$out)=run('bash',"$script/lane-setup.sh",'--repo',$repo,'--dir',"$tmp/fresh-lane",'--branch','lane-start-fresh','--base',$base);
is($rc,0,'fresh binary permits actual lane setup'); ok(-d "$tmp/fresh-lane",'fresh lane created');
done_testing();
