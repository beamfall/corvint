plugins {
    id("com.android.application") version "9.2.1" apply false
    id("com.android.library") version "9.2.1" apply false
    id("com.android.test") version "9.2.1" apply false
    // TECHAUDIT-5: generates the checked-in startup baseline profile (see :baselineprofile-mobile /
    // :baselineprofile-tv) and, when applied to an app module, wires profile consumption.
    alias(libs.plugins.baselineprofile) apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.0" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "2.4.0" apply false
    alias(libs.plugins.roborazzi) apply false
    // TEST-ANDROID-2 / ADR-0121 §4: Kover for JVM unit-test coverage (line floor gate).
    alias(libs.plugins.kover) apply false
    // TECHAUDIT-4: Spotless drives the ktlint Kotlin formatter as a gate. Applied at the root so a
    // single `spotlessCheck`/`spotlessApply` covers every module's Kotlin sources + the .kts scripts.
    alias(libs.plugins.spotless)
}

// ── ANDROID-RELEASE-VERSION-SOURCE-1 / ANDTV-STORE-READINESS-1: authoritative release version ──
// Single source of truth for both app modules' versionCode/versionName, so neither restates a
// literal (see app-mobile/build.gradle.kts, app-tv/build.gradle.kts). Both are read from the
// checked-in gradle/version.properties. versionCode used to be `git rev-list --count HEAD`, which
// regresses on a hotfix branch or a behind-main worktree and pinned nothing to what Play had already
// accepted; it is now a committed literal guarded by lastPublishedVersionCode in the same file —
// configuration fails outright when the build would not strictly exceed the last published code,
// so a regressing artifact cannot be produced. The producing commit's full SHA is still exposed so
// a built artifact can name the source it came from (providers.exec keeps that configuration-cache
// compatible).
val releaseVersionProperties =
    java.util.Properties().apply {
        file("gradle/version.properties").inputStream().use { load(it) }
    }

fun versionProperty(key: String): String =
    releaseVersionProperties.getProperty(key)
        ?: throw GradleException("gradle/version.properties is missing $key")

val releaseVersionName: String = versionProperty("versionName")
val releaseVersionCode: Int = versionProperty("versionCode").trim().toInt()
val lastPublishedVersionCode: Int = versionProperty("lastPublishedVersionCode").trim().toInt()
if (releaseVersionCode <= lastPublishedVersionCode) {
    throw GradleException(
        "versionCode $releaseVersionCode does not exceed lastPublishedVersionCode " +
            "$lastPublishedVersionCode (gradle/version.properties) — Play rejects any upload at or " +
            "below the highest accepted code. Bump versionCode above the last published value.",
    )
}
val releaseCommitSha: String =
    providers
        .exec { commandLine("git", "rev-parse", "HEAD") }
        .standardOutput.asText
        .get()
        .trim()

extra["releaseVersionName"] = releaseVersionName
extra["releaseVersionCode"] = releaseVersionCode
extra["releaseCommitSha"] = releaseCommitSha

// ── TECHAUDIT-4: Spotless + ktlint Kotlin formatting gate ──────────────────────────────────────
// `spotlessCheck` (wired into gateCheck below) fails RED on any misformatted Kotlin; `spotlessApply`
// rewrites in place. ktlint version is pinned in the version catalog for reproducibility.
spotless {
    kotlin {
        target("**/*.kt")
        targetExclude("**/build/**")
        ktlint(libs.versions.ktlint.get())
    }
    kotlinGradle {
        target("**/*.gradle.kts")
        targetExclude("**/build/**")
        ktlint(libs.versions.ktlint.get())
    }
}

tasks.register("fastGate") {
    // :kit's unit tests now live in the ../beamfall-android-ui repo (published as
    // com.beamfall.android-ui:kit) and are no longer wired into this gate — that repo runs them in
    // its own CI. Wiring them back in needs a `workflow`-scoped CI change and is out of scope here.
    // This gate covers the app shells: unit tests + lint for both, plus the l10n/vocab gates.
    dependsOn(
        // TECHAUDIT-4: Spotless + ktlint Kotlin formatting gate — fails RED on any misformatted
        // Kotlin source or Gradle script. `./gradlew spotlessApply` rewrites in place.
        "spotlessCheck",
        // TESTAUDIT-ANDROID-NIGHTLY-1: protects deterministic nightly ownership. The scheduled
        // graph keeps the threshold self-test but never requires producer-only benchmark output.
        "nightlyTaskGraphTest",
        ":app-tv:testDebugUnitTest",
        ":app-mobile:testDebugUnitTest",
        // TEST-ANDROID-2 / ADR-0121 §4: independent Kover line/branch coverage ratchets.
        "koverFloorGateTest",
        "koverFloorGate",
        // ANTEST-3 (android-ui-0001 Decision 2, ADR-0100 + android-ui ADR-0001): the UI-state screenshot catalog —
        // including the forced large-text / reduce-motion / high-contrast a11y variants — is a
        // BLOCKING PR gate, not nightly-only. The screenshot test stays excluded from the bare
        // testDebugUnitTest run (ORG-33's light-gate intent), but verifyUiCatalog re-includes it via
        // the Roborazzi verify task (see app-*/build.gradle.kts), so this stays a single extra render
        // rather than doubling the unit-test cost. nightlyCheck still also runs it.
        ":app-tv:verifyUiCatalog",
        ":app-mobile:verifyUiCatalog",
        ":app-tv:lintDebug",
        ":app-mobile:lintDebug",
        "l10nGate",
        "vocabGateTest",
        "vocabGateExtractorTest",
        "vocabSourceGate",
        "googlePlayMetadataGateTest",
        "googlePlayMetadataGate",
        // AND-11 TV app-quality release gate (ADR-0017 §6 / ADR-0022 §8): the *Test variants prove
        // each gate goes RED on a violating fixture before the gate itself runs over the real tree.
        "tvQualityGateTest",
        "tvQualityGate",
        // ANDTV-STORE-READINESS-1: Play store readiness — leanback required="true" in the manifest
        // source AND in the release APK's badging (self-test proves RED first), plus release-variant
        // lint so R8/shrinker-only findings are not masked by the debug-only lint run.
        "tvStoreReadinessGateTest",
        "tvStoreReadinessGate",
        ":app-tv:lintRelease",
        "platformGateTest",
        "platformGate",
        "androidShellGateTest",
        "androidShellGate",
        // RV-AND-003 Play acquisition-firewall gate (ADR-0050): self-test proves RED on a forbidden
        // surface, then the gate scans the Play module source + assembled APK for acquisition-adjacent
        // surfaces. The ADR-0017 Play-billing paywall is legitimate and is NOT a firewall breach.
        "firewallGateTest",
        "firewallSourceGate",
        "tokenDriftGateTest",
        "tokenDriftGate",
        // AND-2 FFmpeg audio-extension license gate (ADR-0017 §3): the GPLv3 prebuilt is prohibited
        // in this proprietary client. The *Test proves the gate goes RED on every GPL-taint class
        // before the gate scans the real :decoder-ffmpeg recipe/jniLibs/manifest.
        "licenseGateTest",
        "licenseGate",
        "gateSplitContractTest",
    )
}

tasks.register("deviceGate") {
    // REVSWEEP-SEC-3: real AndroidKeyStore coverage requires an emulator.
    dependsOn(":app-mobile:connectedDebugAndroidTest")
}

tasks.register("gateCheck") {
    dependsOn("fastGate", "deviceGate")
}

tasks.register("nightlyCheck") {
    dependsOn(
        "gateCheck",
        ":app-tv:verifyUiCatalog",
        ":app-mobile:verifyUiCatalog",
        "vocabGate",
        "firewallGate",
        // AUDIT-ANDROID-10 / TEST-ANDROID-4 (ADR-0121 §2): deterministic self-test for the
        // macrobenchmark startup-regression gate. Real benchmark JSON is parsed only after the
        // producer tasks in nightly-macrobench.yml; nightlyCheck does not produce that output.
        "benchmarkThresholdGateTest",
    )
}

tasks.register("releaseSmoke") {
    dependsOn(
        ":app-tv:assembleRelease",
        ":app-tv:bundleRelease",
        ":app-mobile:assembleRelease",
        ":app-mobile:bundleRelease",
    )
}

tasks.register<Exec>("vocabGate") {
    dependsOn(":app-tv:assembleDebug", ":app-mobile:assembleDebug")
    commandLine("bash", "script/vocab_gate.sh")
}

tasks.register<Exec>("vocabSourceGate") {
    environment("BEAMFALL_APK_STRINGS_SCAN", "0")
    commandLine("bash", "script/vocab_gate.sh")
}

tasks.register<Exec>("vocabGateTest") {
    commandLine("bash", "script/test/vocab_gate_binary_test.sh")
}

tasks.register<Exec>("vocabGateExtractorTest") {
    commandLine("bash", "script/vocab_gate_test.sh")
}

tasks.register<Exec>("pinnedCoreLiveEnrollmentCheckTest") {
    commandLine("bash", "script/test/pinned_core_live_enrollment_check_test.sh")
}

tasks.register<Exec>("androidDeviceCiContractTest") {
    commandLine("bash", "script/test/android_device_ci_contract_test.sh")
}

tasks.register<Exec>("pinnedCoreLiveEnrollmentCheck") {
    commandLine("bash", "script/pinned_core_live_enrollment_check.sh")
}

tasks.register<Exec>("l10nGate") {
    commandLine("bash", "script/l10n_gate.sh")
}

// GCR-011-C-ANDROID-METADATA (ADR-0034 §2-3, ADR-0091): the Google Play listing corpus is a
// release artifact, not app source, but shipping a TODO/placeholder or an over-length field to
// Play Console is a real release defect — same gate+gateTest self-proving pattern as the other
// static release gates below.
tasks.register<Exec>("googlePlayMetadataGate") {
    commandLine("bash", "script/google_play_metadata_gate.sh")
}

tasks.register<Exec>("googlePlayMetadataGateTest") {
    commandLine("bash", "script/test/google_play_metadata_gate_test.sh")
}

// ── AND-11: TV app-quality release gate (ADR-0017 §6 TV quality checklist + ADR-0022 §8 floor) ──
// Static-analysis gates over manifest + source (no device), each paired with a *Test that proves it
// goes RED on a violating fixture — a gate that cannot fail isn't a gate (ticket Verify clause).
tasks.register<Exec>("tvQualityGate") {
    // D-pad reachability + leanback banner + no touch-only affordances over app-tv.
    commandLine("bash", "script/tv_quality_gate.sh")
}

tasks.register<Exec>("tvQualityGateTest") {
    commandLine("bash", "script/test/tv_quality_gate_test.sh")
}

// ANDTV-STORE-READINESS-1: Play store readiness checks (leanback required in source + release
// badging). The gate dumps the release APK with aapt2, so it depends on the release assembly; the
// self-test runs on fixtures only.
tasks.register<Exec>("tvStoreReadinessGate") {
    dependsOn(":app-tv:assembleRelease")
    commandLine("bash", "script/tv_quality_gate.sh", "--store-readiness")
}

tasks.register<Exec>("tvStoreReadinessGateTest") {
    commandLine("bash", "script/tv_quality_gate.sh", "--store-readiness-self-test")
}

tasks.register<Exec>("platformGate") {
    // minSdk 29 floor (both modules) + adaptive window-size-class shell (no stretched-phone tablet UI).
    commandLine("bash", "script/platform_gate.sh")
}

tasks.register<Exec>("platformGateTest") {
    commandLine("bash", "script/test/platform_gate_test.sh")
}

tasks.register<Exec>("androidShellGate") {
    commandLine("bash", "script/android_shell_gate.sh")
}

tasks.register<Exec>("androidShellGateTest") {
    commandLine("bash", "script/test/android_shell_gate_test.sh")
}

// ── RV-AND-003: Play acquisition-firewall gate (ADR-0050 / ADR-0002 §qBittorrent / ADR-0006 §5) ──
// The Play binary must not reference content-acquisition/sourcing surfaces. Billing (ADR-0017) is
// legitimate and deliberately not in the banned set. The *Test proves the gate goes RED on a fixture.
tasks.register<Exec>("firewallGate") {
    dependsOn(":app-mobile:assembleDebug")
    commandLine("bash", "script/firewall_gate.sh")
}

tasks.register<Exec>("firewallSourceGate") {
    environment("BEAMFALL_APK_STRINGS_SCAN", "0")
    commandLine("bash", "script/firewall_gate.sh")
}

tasks.register<Exec>("firewallGateTest") {
    commandLine("bash", "script/test/firewall_gate_test.sh")
}

// ── RV-AND-009 / Arch F3: token drift gate ────────────────────────────────────────────────────
// Consumer-side drift catch for generated design tokens. Asserts each app theme role with a
// canonical counterpart aliases BeamfallTokens.Theme.Cinema from :ui instead of local hex copies.
// The *Test proves the gate goes RED on an injected palette drift before it runs over the real tree.
tasks.register<Exec>("tokenDriftGate") {
    commandLine("bash", "script/token_drift_gate.sh")
}

tasks.register<Exec>("tokenDriftGateTest") {
    commandLine("bash", "script/test/token_drift_gate_test.sh")
}

// ── AND-2: FFmpeg audio-extension license-compliance gate (ADR-0017 §3) ─────────────────────────
// The self-built FFmpeg audio extension must be LGPL-only; the GPLv3 prebuilt is prohibited. The
// gate fails RED on any GPL configure flag, GPL-only decoder, missing LGPL guard flag, GPL `.so`,
// or GPL license manifest in :decoder-ffmpeg. The *Test proves it goes RED on each before it runs.
tasks.register<Exec>("licenseGate") {
    commandLine("bash", "script/license_gate.sh")
}

tasks.register<Exec>("licenseGateTest") {
    commandLine("bash", "script/test/license_gate_test.sh")
}

// ── TESTAUDIT-ANDROID-COVERAGE-1: independent Kover coverage ratchets ─────────────────────────
// Each app's lines and branches must independently hold their measured baseline. Lower overrides
// require the explicit KOVER_ALLOW_DECREASE=1 escape hatch enforced by the script.
tasks.register<Exec>("koverFloorGate") {
    dependsOn(":app-mobile:koverXmlReportDebug", ":app-tv:koverXmlReportDebug")
    commandLine("bash", "script/kover_floor_gate.sh")
}

tasks.register<Exec>("koverFloorGateTest") {
    commandLine("bash", "script/test/kover_floor_gate_test.sh")
}

// ── AUDIT-ANDROID-10 / TEST-ANDROID-4: macrobenchmark startup-regression gate (ADR-0121 §2) ────
// Fails if any macrobenchmark JSON output under build/outputs/ reports a startupMs median above
// the threshold in script/check_benchmark_thresholds.sh. The producer-only macrobenchmark workflow
// invokes this parser after its device runs; the *Test is safe in deterministic non-producer gates.
tasks.register<Exec>("benchmarkThresholdGate") {
    commandLine("bash", "script/check_benchmark_thresholds.sh")
}

tasks.register<Exec>("benchmarkThresholdGateTest") {
    commandLine("bash", "script/test/check_benchmark_thresholds_test.sh")
}

tasks.register<Exec>("nightlyTaskGraphTest") {
    commandLine("bash", "script/test/nightly_task_graph_test.sh")
}

tasks.register<Exec>("gateSplitContractTest") {
    dependsOn("shellTests")
    commandLine("python3", "script/test/gate_split_contract_test.py")
}

val registeredShellTests =
    tasks.withType<Exec>().matching {
        it.commandLine.any { argument ->
            argument.startsWith("script/test/") && argument.endsWith(".sh")
        }
    }

tasks.register("shellTestDiscovery") {
    doLast {
        val discovered =
            fileTree("script/test") { include("*.sh") }
                .files
                .map { it.relativeTo(projectDir).path.replace(java.io.File.separatorChar, '/') }
                .toSet()
        val registered =
            registeredShellTests
                .flatMap { it.commandLine }
                .filter { it.startsWith("script/test/") && it.endsWith(".sh") }
                .toSet()
        check(discovered == registered) {
            "shell test registration drift: unregistered=${discovered - registered}; missing=${registered - discovered}"
        }
    }
}

tasks.register("shellTests") {
    dependsOn(registeredShellTests, "shellTestDiscovery")
}
