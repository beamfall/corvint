// swift-tools-version: 6.0

import PackageDescription

// Beamfall Apple shared foundation — the cross-app UI library (BeamfallDesignSystem)
// and client SDK (BeamfallKit), extracted with history from beamfall-apple during
// the ecosystem shared-UI reorg. The macOS Manager, iOS, and tvOS apps in
// beamfall-apple consume these library products; a dev-only Lab preview target is
// added separately. See:
//   beamfall-design-system/docs/plans/2026-06-14-ecosystem-shared-ui-reorg.md
let package = Package(
    name: "BeamfallAppleUI",
    defaultLocalization: "en",
    platforms: [
        .macOS(.v15),
        .tvOS(.v18),
        .iOS("26.0"),
    ],
    products: [
        .library(name: "BeamfallKit", targets: ["BeamfallKit"]),
        .library(name: "BeamfallDirectDistributionKit", targets: ["BeamfallDirectDistributionKit"]),
        .library(name: "BeamfallDesignSystem", targets: ["BeamfallDesignSystem"]),
        .library(name: "BeamfallVisualKit", targets: ["BeamfallVisualKit"]),
    ],
    dependencies: [
        // Snapshot regression gate for DesignSystem components (RV-APPLEUI-007).
        .package(
            url: "https://github.com/pointfreeco/swift-snapshot-testing",
            from: "1.17.0"
        )
    ],
    targets: [
        .target(
            name: "BeamfallKit",
            // BeamfallDesignSystem supplies the generated BeamfallTokens the shared
            // settings components (SettingsRow/SettingsSection, SETTINGS-2-KIT) render
            // against, so every Apple platform rides one token-backed component.
            dependencies: ["BeamfallDesignSystem"],
            resources: [.process("Resources")],
            linkerSettings: [
                .linkedFramework("Security", .when(platforms: [.macOS, .tvOS, .iOS]))
            ]
        ),
        .target(
            name: "BeamfallDirectDistributionKit",
            dependencies: ["BeamfallKit"]
        ),
        .executableTarget(
            name: "BeamfallKitLocalizationHost",
            dependencies: ["BeamfallKit"],
            resources: [.process("Resources")]
        ),
        .target(name: "BeamfallDesignSystem"),
        // Shared implementation layer for the production Visual runtime and
        // the dev-only Lab. It is intentionally not a product: consumers keep
        // importing BeamfallVisualKit while both in-repo surfaces execute one
        // post chain, uniform layout, and runtime policy implementation.
        .target(name: "BeamfallVisualCore"),
        // Shippable Production Visual Runtime (AV-7 slice 1). Extracts the
        // validated curated runtime + 3 launch visuals from the dev-only Lab into
        // a real library product the shipping apps can link. Shaders are compiled
        // out-of-band to Resources/default.metallib via
        // `script/build-shaders.sh BeamfallVisualKit`.
        .target(
            name: "BeamfallVisualKit",
            dependencies: ["BeamfallVisualCore"],
            exclude: ["Shaders"],
            resources: [
                .copy("Resources/default.metallib"),
                .process("Resources/PrivacyInfo.xcprivacy"),
            ]
        ),
        // Dev-only preview/visualization harness. Deliberately NOT in `products:`
        // so apps consuming the library products never pull it in. Depends on
        // BeamfallDesignSystem so it can preview shared UI elements. Shaders are
        // compiled out-of-band to Resources/default.metallib via script/build-shaders.sh.
        .executableTarget(
            name: "BeamfallAppleLab",
            dependencies: ["BeamfallDesignSystem", "BeamfallKit", "BeamfallVisualCore"],
            exclude: ["Shaders", "CONTEXT.md"],
            resources: [
                .copy("Resources/default.metallib")
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5)
            ]
        ),
        // Per-case and bundle-wide hang bounds (APPLEUI-TESTHANG-1). Load-time
        // constructors register an XCTest observer before the suite starts and arm
        // the independent outer bundle watchdog. Deliberately NOT a product: nothing
        // shippable links it.
        .target(
            name: "BeamfallTestWatchdog",
            path: "Tests/BeamfallTestWatchdog"
        ),
        .testTarget(
            name: "BeamfallAppleLabTests",
            dependencies: [
                "BeamfallAppleLab",
                "BeamfallTestWatchdog",
                .product(name: "SnapshotTesting", package: "swift-snapshot-testing"),
            ],
            swiftSettings: [
                .swiftLanguageMode(.v5)
            ]
        ),
        .testTarget(
            name: "BeamfallVisualKitTests",
            dependencies: ["BeamfallVisualKit", "BeamfallTestWatchdog"],
            // Vendored copy of the Core-owned visual-director conformance pack
            // (visual-director-pack.v1, VSYNC-5). Replayed natively against the
            // ported Director + SmartPreset evaluator by VisualDirectorPackTests.
            resources: [.copy("Fixtures/visual-director")]
        ),
        .testTarget(
            name: "BeamfallDirectDistributionKitTests",
            dependencies: ["BeamfallDirectDistributionKit", "BeamfallTestWatchdog"]
        ),
        // Source-introspection conformance for the shared BeamfallKit SDK
        // (localization key coverage, no hardcoded player strings). Moved here
        // from beamfall-apple so the checks live with the sources they inspect.
        .testTarget(
            name: "BeamfallKitTests",
            dependencies: ["BeamfallKit", "BeamfallKitLocalizationHost", "BeamfallTestWatchdog"]
        ),
        // Snapshot regression gate for representative DesignSystem components
        // (RV-APPLEUI-007). macOS-only — image snapshots render through
        // NSHostingView and are skipped on other platforms.
        .testTarget(
            name: "BeamfallDesignSystemTests",
            dependencies: [
                "BeamfallDesignSystem",
                "BeamfallTestWatchdog",
                .product(name: "SnapshotTesting", package: "swift-snapshot-testing"),
            ]
        ),
    ]
)
