pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
}

val androidUiSource =
    providers
        .gradleProperty("beamfallAndroidUiSource")
        .orElse(providers.environmentVariable("BEAMFALL_ANDROID_UI_SOURCE"))
        .orElse("auto")
        .get()
require(androidUiSource in setOf("auto", "composite", "package")) {
    "beamfallAndroidUiSource must be one of: auto, composite, package"
}

val androidUiComposite =
    listOfNotNull(
        System.getenv("BEAMFALL_ANDROID_UI_COMPOSITE")?.let(::file),
        file("../beamfall-android-ui"),
        file("../../../beamfall-android-ui"),
    ).firstOrNull { it.resolve("settings.gradle.kts").isFile }
val useAndroidUiComposite =
    when (androidUiSource) {
        "auto" -> {
            androidUiComposite != null
        }

        "composite" -> {
            require(androidUiComposite != null) {
                "beamfallAndroidUiSource=composite requested, but no sibling beamfall-android-ui checkout was found"
            }
            true
        }

        else -> {
            false
        }
    }

val githubPackagesUser =
    providers
        .environmentVariable("GITHUB_ACTOR")
        .orElse(providers.gradleProperty("gpr.user"))
val githubPackagesKey =
    providers
        .environmentVariable("GITHUB_TOKEN")
        .orElse(providers.gradleProperty("gpr.key"))
if (!useAndroidUiComposite && (!githubPackagesUser.isPresent || !githubPackagesKey.isPresent)) {
    error(
        "Resolving com.beamfall.android-ui:kit from GitHub Packages requires " +
            "GITHUB_ACTOR/GITHUB_TOKEN or gpr.user/gpr.key. " +
            "Use -PbeamfallAndroidUiSource=composite with a sibling beamfall-android-ui checkout " +
            "for local source substitution.",
    )
}

dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        google()
        mavenCentral()
        // Shared Beamfall Android SDK (:kit) is published to GitHub Packages from
        // ../beamfall-android-ui as com.beamfall.android-ui:kit. Resolving it needs a
        // token with read:packages — GITHUB_ACTOR/GITHUB_TOKEN env or gpr.user/gpr.key.
        maven {
            name = "GitHubPackages"
            url = uri("https://maven.pkg.github.com/Beamfall/android-ui")
            credentials {
                username = githubPackagesUser.orNull
                password = githubPackagesKey.orNull
            }
        }
    }
}

rootProject.name = "BeamfallAndroid"

if (useAndroidUiComposite) {
    includeBuild(androidUiComposite!!) {
        dependencySubstitution {
            substitute(module("com.beamfall.android-ui:kit")).using(project(":kit"))
            substitute(module("com.beamfall.android-ui:ui")).using(project(":ui"))
        }
    }
}

include(":app-tv")
include(":app-mobile")
include(":benchmark-tv")
include(":benchmark-mobile")
// TECHAUDIT-5: baseline-profile generator modules (androidx.baselineprofile "com.android.test"
// producer side); generateBaselineProfile writes into the corresponding app module's
// src/main/baselineProfiles/baseline-prof.txt.
include(":baselineprofile-tv")
include(":baselineprofile-mobile")
// AND-2: in-house LGPL-only FFmpeg audio extension + RenderersFactory (ADR-0017 §3).
include(":decoder-ffmpeg")
