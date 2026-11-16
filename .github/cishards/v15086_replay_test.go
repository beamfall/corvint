package cishards

import (
	"sort"
	"testing"
)

// TestV15086Run42408095814FullReplay replays the package times of pull-request run 42408095814
// (V1-5086), whose sixteen -p 4 shards ran from 397 s to 842 s, through the full sixteen-shard
// plan at the parallelism they ran with: the planned max/min wall ratio is at most 1.5 and the
// longest shard at most the 400 s budget. The run's as-run units cannot meet the budget, since
// single units ran 842 s (internal/tasks/cli, two slices) and 799 s (typedreach, whole); the
// fixture slices every unit above the budget into the fewest test slices that each stay within
// it, as tools/ci-test-slices generate would from that run's own top-level test times.
func TestV15086Run42408095814FullReplay(t *testing.T) {
	universe, ms, splits := replayFixture(t, run42408095814)
	if len(universe) != 458 || len(splits) != 7 {
		t.Fatalf("fixture has %d packages, %d split", len(universe), len(splits))
	}
	allowed := []string{}
	for p := range splits {
		allowed = append(allowed, p)
	}
	sort.Strings(allowed)
	costs, allow, slices := costsFile(ms), allowFile(allowed...), slicesFile(splits)
	const shards, parallel = 16, 4
	plan, err := PlanFromParallel(universe, shards, parallel, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	requireComplete(t, universe, plan)
	hi, lo := spread(walls(plan, ms, parallel))
	old, err := PlanFrom(universe, shards, costs, allow, slices)
	if err != nil {
		t.Fatal(err)
	}
	oldHi, oldLo := spread(walls(old, ms, parallel))
	t.Logf("full -p %d: wall %.1f..%.1f s (max/min %.3f); least-loaded %.1f..%.1f s (max/min %.3f)",
		parallel, lo/1000, hi/1000, hi/lo, oldLo/1000, oldHi/1000, oldHi/oldLo)
	if hi > 1.5*lo {
		t.Fatalf("max/min %.3f above 1.5", hi/lo)
	}
	if hi > DefaultWallBudgetMs {
		t.Fatalf("max shard wall %.1f s above the %d ms budget", hi/1000, DefaultWallBudgetMs)
	}
	if hi > oldHi || hi/lo > oldHi/oldLo {
		t.Fatalf("wall placement (%.1f s, %.3f) no better than least-loaded (%.1f s, %.3f)", hi/1000, hi/lo, oldHi/1000, oldHi/oldLo)
	}
}

// run42408095814 is the per-package elapsed milliseconds of the go-product-shard jobs of
// https://github.com/beamfall/corvint/actions/runs/42408095814 (pull_request, head
// e83883d44875a00c0fa2784596d2d85ca2a0f75a, sixteen shards at -p 4), read from the jobs' go test
// JSON output and floored at 1 ms: "path ms" per package, "path ms,ms,..." per split package.
// internal/liveverify/affected keeps its three as-run slices. Each unit that ran above the 400 s
// budget is replaced by the generator's slicing of its package (AFP-V0-041: contiguous runs of
// the sorted top-level tests weighted by their measured times, each slice the package's elapsed
// times its share) at the fewest slices that each stay within 400 s: cmd/corvint 3,
// internal/tasks/store 4, internal/tasks/cli 4, internal/doccorpus 2, typedreach 3 and
// internal/analyzerkotlinandroid 2. Those six slicings are derived from the run, not measured;
// the committed allow-list splits only cmd/corvint and internal/liveverify/affected of them.
const run42408095814 = "" +
	"benchmarks 15697; benchmarks/dogfood-measure 31751; benchmarks/dogfood-workers 1209; " +
	"benchmarks/runner 1331; benchmarks/selfuse-batch 183861; " +
	"cmd/corvint 364866,341165,363540; cmd/corvint-analyzer-c-jni 1718; " +
	"cmd/corvint-analyzer-dotnet 1148; cmd/corvint-analyzer-go 6314; " +
	"cmd/corvint-analyzer-html-css 40171; cmd/corvint-analyzer-java 1361; " +
	"cmd/corvint-analyzer-js 18816; cmd/corvint-analyzer-kotlin-android 1236; " +
	"cmd/corvint-analyzer-objective-c 1408; cmd/corvint-analyzer-python 43960; " +
	"cmd/corvint-analyzer-ruby 1077; cmd/corvint-analyzer-rust 1655; " +
	"cmd/corvint-analyzer-shader 18145; cmd/corvint-analyzer-shell 17126; " +
	"cmd/corvint-analyzer-sql-native 4200; cmd/corvint-analyzer-structured-data 10032; " +
	"cmd/corvint-analyzer-swift 1808; cmd/corvint-behavior-falsify 1032; " +
	"cmd/corvint-cem-candidate 1341; cmd/corvint-cem-experiments 1033; cmd/corvint-cli 1; " +
	"cmd/corvint-companion-release 1097; cmd/corvint-console 1355; " +
	"cmd/corvint-corpus-http 6102; cmd/corvint-corpus-mcp 145689; " +
	"cmd/corvint-corpus-parity 1676; cmd/corvint-corpus-republish 2735; " +
	"cmd/corvint-dashboard-snapshot 4021; cmd/corvint-docs-mcp 2602; cmd/corvint-gateway 1040; " +
	"cmd/corvint-go-test-provider 182706; cmd/corvint-go-toolchain-receipt 1024; " +
	"cmd/corvint-intake 1025; cmd/corvint-js-test-provider 12412; cmd/corvint-lsp 2998; " +
	"cmd/corvint-mcp 1; cmd/corvint-playwright-minimize 1061; " +
	"cmd/corvint-postmerge-connect 3077; cmd/corvint-postmerge-host-launcher 1344; " +
	"cmd/corvint-postmerge-metrics 2008; cmd/corvint-postmerge-workflow 1070; " +
	"cmd/corvint-public-release-check 1045; cmd/corvint-readiness-record 1060; " +
	"cmd/corvint-release-candidate 1; cmd/corvint-release-install 1; " +
	"cmd/corvint-remote-provider 1021; cmd/corvint-ruby-test-provider 1184; " +
	"cmd/corvint-tasks 1; cmd/corvint-test 96239; cmd/corvint-test-runner 31961; " +
	"cmd/corvint-test-validity-mcp 1890; cmd/corvint-tests-accept 1069; " +
	"cmd/corvint-update 1034; cmd/corvint-web-flows 1079; cmd/corvint-work-queue 2546; " +
	"conformance 1021; conformance/cli-errors-v0 262415; conformance/cli-parity-v0 194853; " +
	"conformance/dashboard-snapshot-v0 18854; conformance/frontier-v0 1095; " +
	"conformance/go-live-test-v0 267144; conformance/harness-event-v0 1043; " +
	"conformance/host-lifecycle-v1 4550; conformance/human-documentation-compiler-v0 3013; " +
	"conformance/mcp-2026-07-28 19135; conformance/mcp-test-validity-v0 60346; " +
	"conformance/ocm-v0 9924; conformance/perf-v0 1073; conformance/release-artifact-v0 66068; " +
	"conformance/release-artifact-v0/archive-verifier 1066; " +
	"conformance/release-artifact-v0/archivebuild 1268; " +
	"conformance/release-artifact-v0/archiveverify 1983; " +
	"conformance/release-artifact-v0/archivewire 1; conformance/use-cases-v0 2128; " +
	"conformance/work-queue-v0 29732; conformance/work-queue-v0/cmd/cli-adapter 1; " +
	"conformance/work-queue-v0/cmd/cli-prose 1; conformance/work-queue-v0/cmd/cli-replay 1; " +
	"conformance/work-queue-v0/cmd/cli-scope 1; examples/evidence-provider/v0 1034; " +
	"examples/evidence-provider/v0/check 1068; " +
	"examples/evidence-provider/v0/conformance 39731; experimental/analyzers/sqlnative 16744; " +
	"integrations/testfixture 1; internal/analyzercap 7694; internal/analyzerdotnet 1395; " +
	"internal/analyzerenvelope 1025; internal/analyzerexec 2406; internal/analyzergo 6838; " +
	"internal/analyzerhtmlcss 7601; internal/analyzerjs 6995; " +
	"internal/analyzerkotlinandroid 208238,200838; internal/analyzernativebridge 166086; " +
	"internal/analyzerpython 4486; internal/analyzerruby 1689; internal/analyzerrust 1418; " +
	"internal/analyzershader 8459; internal/analyzershell 11549; " +
	"internal/analyzerstructured 2032; internal/analyzerswift 67894; internal/appflows 152220; " +
	"internal/appmap 338393; internal/archivepath 1024; internal/attest 1520; " +
	"internal/authorityevent 1035; internal/authoritystore 2450; " +
	"internal/behaviorfalsify 159155; internal/behaviorfalsify/cli 1; internal/betarung 1092; " +
	"internal/breakagemap 41239; internal/browsercov 1631; " +
	"internal/browsercov/cmd/browsercov 1; internal/capcatalog 2682; " +
	"internal/cem/cemcode 1020; internal/cem/cli 26690; internal/cem/corpusbinding 109530; " +
	"internal/cem/coverprofile 1; internal/cem/gitauth 14250; internal/cem/gitrun 5260; " +
	"internal/cem/mdreport 1; internal/cem/patch 1716; internal/cem/publish 1161; " +
	"internal/cem/sim 1043; internal/cem/stablefixture 1; internal/cem/verify 77155; " +
	"internal/cem/wire 6946; internal/cem/witnesspaths 1080; internal/cem/workflow 46436; " +
	"internal/cemcandidate 7256; internal/cemchain 128872; internal/cemdiscriminate 1072; " +
	"internal/changee2e 190611; internal/changewitness 1064; internal/cliapp 1044; " +
	"internal/cliconformance 1; internal/clierror 1051; internal/cliprofile 1896; " +
	"internal/clitree 1047; internal/clitree/clitreetest 1; internal/command/breakage 1230; " +
	"internal/command/flows 142830; internal/compactionkernel 1062; " +
	"internal/companioncli 1022; internal/companionrelease 117430; " +
	"internal/companionrelease/currentproof 38447; internal/console 19444; " +
	"internal/contextindex 302706; internal/contextsummary 1; internal/corpusindex 264937; " +
	"internal/corpusrepublish 5883; internal/corpusserve 2649; " +
	"internal/criterionexperiment 358973; internal/dashboard/adapters 5033; " +
	"internal/dashboard/authority 1030; internal/dashboard/model 63957; " +
	"internal/dashboard/repository 18779; internal/dashboard/roadmap 1915; " +
	"internal/dashboard/source 2650; internal/delta 50866; internal/depsource 2886; " +
	"internal/diagnostic 34144; internal/disagree 1064; internal/doccitations 1100; " +
	"internal/doccompiler 11353; internal/doccorpus 329790,328619; " +
	"internal/doccorpus/selfcorpus 274972; internal/docmaintain 11694; " +
	"internal/docviews 230203; internal/dogfoodflow 193157; internal/dogfoodocm 1492; " +
	"internal/dogfoodoperation 1066; internal/evalrepo 3823; internal/evidencereach 1244; " +
	"internal/execfile 4043; internal/extevidence 221810; internal/extmutate 9401; " +
	"internal/flowcoverage 9008; internal/flowcoverage/testfixture 1; internal/flowdocs 41408; " +
	"internal/frictionmetric 1174; internal/frontier 1033; internal/frontiernext 1; " +
	"internal/frontiernextrepo 1034; internal/frontierrepo 2751; internal/funclink 1433; " +
	"internal/gateagreement 1025; internal/gateselect 5862; internal/genesis 2100; " +
	"internal/gitancestry 1054; internal/gitancestry/ancestrytest 1; internal/gitnotes 22617; " +
	"internal/gitprobe 1037; internal/gitstatus 4095; internal/gokernel 5588; " +
	"internal/goplsclient 2006; internal/gotestrun 2821; internal/groupreap 24194; " +
	"internal/intake 1700; internal/jobinputs 27496; internal/jslinecov 3598; " +
	"internal/jsonend 1067; internal/jstestprovider 36507; " +
	"internal/jstestprovider/externallive 1409; internal/jstestprovider/loopbackhttp 1; " +
	"internal/langplugin 3780; internal/langplugin/decl 1; internal/langplugin/dialect 1054; " +
	"internal/linecov 97511; internal/linecov/v8 1038; " +
	"internal/liveverify/affected 69131,108933,150945; " +
	"internal/liveverify/affected/cfamily 1111; internal/liveverify/affected/dotnet 1100; " +
	"internal/liveverify/affected/golang 97181; " +
	"internal/liveverify/affected/golang/typedreach 259517,267618,272135; " +
	"internal/liveverify/affected/java 1360; internal/liveverify/affected/java/javalex 1; " +
	"internal/liveverify/affected/kotlin 1172; internal/liveverify/affected/languages 1082; " +
	"internal/liveverify/affected/python 1384; internal/liveverify/affected/ruby 5031; " +
	"internal/liveverify/affected/rust 1234; internal/liveverify/affected/shell 4052; " +
	"internal/liveverify/affected/swift 1313; internal/liveverify/affected/typescript 53738; " +
	"internal/liveverify/godiscovery 5287; internal/liveverify/gorunner 42179; " +
	"internal/liveverify/gotest 2012; internal/liveverify/jsresolve 1067; " +
	"internal/liveverify/mutate 17838; internal/liveverify/parentverify 1824; " +
	"internal/liveverify/processidentity 1; internal/liveverify/provider 216443; " +
	"internal/liveverify/provider/interlock 1333; internal/liveverify/pymutate 1043; " +
	"internal/liveverify/pyresolve 1600; internal/liveverify/rbmutate 5098; " +
	"internal/liveverify/rbmutate/minirspec 1; internal/liveverify/session 184603; " +
	"internal/localauthority 1219; internal/localcompletion 168727; internal/lrf 1031; " +
	"internal/lrfrepo 217319; internal/lspevidence 2232; internal/lspprovider 5803; " +
	"internal/lspsnapshot 1056; internal/lspstdio 75658; internal/mcp/bridge 273147; " +
	"internal/mcp/cli 55758; internal/mcp/corpusbridge 49167; internal/mcp/docsbridge 9636; " +
	"internal/mcp/protocol 1092; internal/mcp/rootbind 1; internal/mcp/server 3119; " +
	"internal/mcp/tasksbridge 28056; internal/mcp/tasksread 1087; " +
	"internal/mcp/testvaliditybridge 1610; internal/mcp/toolgroup 1; " +
	"internal/migrationratchet 1371; internal/modelgateway 202; internal/nativelinecov 1559; " +
	"internal/necessity 2338; internal/obscorpus 1494; internal/observations 3447; " +
	"internal/observedstale 1236; internal/opencodequalification 214838; " +
	"internal/outcomecal 1053; internal/perfbudget 126632; " +
	"internal/perfbudget/cmd/perf-budget 8055; internal/plansnapshot 1357; " +
	"internal/playwrightminimize 1432; internal/playwrightminimize/cli 1; " +
	"internal/playwrightruns 2220; internal/postmergeconnector 5779; " +
	"internal/postmergehost 90014; internal/postmergemetrics 7158; " +
	"internal/postmergeproof 23676; internal/postmergeproof/procfs 1226; " +
	"internal/postmergeworkflow 177520; internal/procgroup 105902; internal/procprofile 3386; " +
	"internal/projectpath 1106; internal/projectprofile 1022; internal/pythongrammar 1027; " +
	"internal/pythonsyntax 1066; internal/receiptbundle 9555; internal/releasecandidate 5855; " +
	"internal/releasegate 109812; internal/remoteprovider 1455; internal/repoenvelope 1026; " +
	"internal/repro 1040; internal/requestpath 1186; internal/revieworder 1183; " +
	"internal/rootalias 1; internal/rubyautoload 1034; internal/rubydialect 1034; " +
	"internal/rubylinecov 11946; internal/rubyprofile 3255; internal/rubytestprovider 7669; " +
	"internal/runhygiene 1; internal/runidentity 3285; internal/runtimeenv 1023; " +
	"internal/scopelease 6361; internal/secretscreen 79616; internal/semescalate 2030; " +
	"internal/sessiontest 45894; internal/sharedpath 11196; internal/skhproof 1187; " +
	"internal/skillexport 1074; internal/slotlearn 1825; internal/specindex 27080; " +
	"internal/stepnegation 1456; internal/stepverify 35864; internal/stepverify/safeopen 1066; " +
	"internal/swiftimpact 1554; internal/taskbarrier 1331; internal/taskman 116664; " +
	"internal/taskprojection 1034; internal/tasks 41684; internal/tasks/archive 25834; " +
	"internal/tasks/authority 249738; internal/tasks/barrierwarning 1107; " +
	"internal/tasks/cli 394917,390084,394494,394449; internal/tasks/criterionbinding 1275; " +
	"internal/tasks/defects 1898; internal/tasks/dispatch 148750; " +
	"internal/tasks/evidencestore 1064; internal/tasks/fixture 1086; " +
	"internal/tasks/importer 1113; internal/tasks/intent 63734; internal/tasks/journal 18346; " +
	"internal/tasks/lifecycle 1039; internal/tasks/mutation 2922; " +
	"internal/tasks/obligation 1224; internal/tasks/release 1171; " +
	"internal/tasks/safeopen 1086; internal/tasks/scale 131198; internal/tasks/scopes 1643; " +
	"internal/tasks/service 22346; internal/tasks/similar 32565; " +
	"internal/tasks/snapshot 14013; internal/tasks/store 366904,376445,384930,385403; " +
	"internal/tasks/supervisor 26874; internal/tasks/testhooks 1036; " +
	"internal/tasks/ticket 2140; internal/tasks/transaction 252644; internal/tasks/wire 3216; " +
	"internal/tasks/worktrail 1051; internal/tcq 2656; internal/testacceptance 129209; " +
	"internal/testacceptance/cli 1; internal/testevidence 1329; internal/testplan 17151; " +
	"internal/testplatform 1123; internal/testreuse 60466; internal/testrunner 146521; " +
	"internal/testrunner/cli 1; internal/testrunner/dynamic 38295; " +
	"internal/testrunner/mobile 7622; internal/testrunner/native 20592; " +
	"internal/testrunner/platform 14288; internal/testrunner/registry 1148; " +
	"internal/testrunner/sql 1205; internal/testsource 29654; internal/testsupport 4565; " +
	"internal/testvalidity 1082; internal/testvaliditydoc 2402; " +
	"internal/testvaliditydoc/externalorigins 1238; internal/toolchainreceipt 1054; " +
	"internal/touchsurprise 2830; internal/trace 48510; internal/tracemigraterepo 18639; " +
	"internal/tracerecordrepo 45496; internal/tracerepopaths 1; internal/unplannedread 3380; " +
	"internal/untrackedallowance 1059; internal/update 66252; internal/upgradechanges 14149; " +
	"internal/versionline 1051; internal/webdialect 1037; internal/witness 30549; " +
	"internal/witnesscollapse 4453; internal/witnesspaths/datareaders 1; " +
	"internal/worklistadapter 1; internal/workqueue 9304; internal/worksource 6267; " +
	"internal/workspace 1072; internal/worktreeimpact 23078; internal/wp3codec 1083; " +
	"protocol/integrations 1091; script/spec-coverage-audit 1080; " +
	"tools/affected-zero-unknown-replay 2368; tools/beamfall-shadow 82805; " +
	"tools/beamfall-shadow/fdexec 1; tools/beamfall-shadow/fixtures/matrix/go 1; " +
	"tools/beamfall-shadow/wrapper 1947; tools/capabilities-check 114300; " +
	"tools/cem-citation-baseline 15277; tools/cem-release-suites 15242; " +
	"tools/cem-stable-regen 124508; tools/cem-structural-closure 1483; tools/cem-trial 59455; " +
	"tools/ci-flake-report 3918; tools/ci-forward-row 51885; tools/ci-nightly-signal 1367; " +
	"tools/ci-reuse-plan 43478; tools/ci-shard-costs 1396; tools/ci-test-slices 4289; " +
	"tools/cli-discovery-gen 9417; tools/compat-trial 39203; " +
	"tools/compat-trial/build-fixtures 23126; tools/compat-trial/fixtures/new 1; " +
	"tools/compat-trial/fixtures/old 1; tools/conflict-markers-check 1067; " +
	"tools/corvint-pr-tests 85198; tools/cw-trial 26735; tools/declaration-compat-check 1688; " +
	"tools/docs-blinded-task 1; tools/docs-ci-plan 15761; tools/dogfood-guard-hunks 1; " +
	"tools/dogfood-inherit-citations 4302; tools/fixture-maintenance-check 12026; " +
	"tools/flow-guide-check 30713; tools/gate-affected-select 5036; tools/gate-ledger 77891; " +
	"tools/gotest-evidence 1438; tools/heading-nav-bench 1333; tools/import-ratchet 3101; " +
	"tools/linecov 16933; tools/live-test-run 1062; tools/native-hook-observer 1419; " +
	"tools/paired-trial 1122; tools/pi-tui-fixture 1037; tools/plan-delta 1053; " +
	"tools/plugin-dirs 1042; tools/py-signal-lint 1034; tools/qualify-opencode 1; " +
	"tools/requirement-id-collisions 2844; tools/retrieval-bench 13253; " +
	"tools/run-measure 3765; tools/selection-baseline 20704; tools/shell-test-select 1078; " +
	"tools/str-parity 3134; tools/tasks-profiles-check 1071; tools/test-commands-check 26617; " +
	"tools/test-commands-run 1102; tools/test-exec-writes-check 11623; " +
	"tools/test-read-scopes-check 111536; tools/test-shards 50500; tools/tiers-replay 1571; " +
	"tools/typed-reach-measure 1; tools/unbounded-readers 1876; tools/verify-syntax-check 1; " +
	"tools/walk-skipdir-check 1143; "
