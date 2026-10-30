// Root build for the Beamfall Android shared-UI repo. Holds three modules:
//   :kit  — the SDK (pure data/network/storage; no Compose), extracted from beamfall-android.
//   :ui   — the shared Compose UI library (theme/tokens scaffold; design-system impl deferred).
//   :lab  — a dev-only harness app for previewing :ui in isolation (never shipped).
// Plugin versions mirror beamfall-android's root build so :kit behaves identically.
plugins {
    id("com.android.application") version "9.2.1" apply false
    id("com.android.library") version "9.2.1" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.4.0" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "2.4.0" apply false
    id("io.github.takahirom.roborazzi") version "1.68.0" apply false
    // TEST-ANDROIDUI-5: Kover coverage floor (ADR-0121 §4). Applied to :ui; 0.9.x has stable AGP
    // integration so the debug unit-test variant is correctly measured.
    id("org.jetbrains.kotlinx.kover") version "0.9.8" apply false
}

val hardwareKeystoreShipGate = tasks.register<Exec>("hardwareKeystoreShipGate") {
    group = "verification"
    description = "Blocks migration-sensitive publishing without real-device evidence"
    workingDir(rootProject.projectDir)
    commandLine(
        "python3",
        "script/check-hardware-keystore-ship-gate.py",
        "--release-tag",
        "v${libs.versions.beamfall.ui.get()}",
    )
}

// Group coordinate so the consuming monorepo (beamfall-android) can resolve these modules
// from a Gradle composite build (`includeBuild`) via dependency substitution
// (e.g. "com.beamfall.android-ui:kit" -> the included :kit project).
subprojects {
    group = "com.beamfall.android-ui"
    pluginManager.withPlugin("maven-publish") {
        tasks.withType<org.gradle.api.publish.maven.tasks.PublishToMavenRepository>().configureEach {
            dependsOn(hardwareKeystoreShipGate)
        }
    }
}
