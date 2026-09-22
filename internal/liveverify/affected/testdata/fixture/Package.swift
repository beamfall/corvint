// swift-tools-version: 6.0

import PackageDescription

let package = Package(
    name: "AffectedFixture",
    targets: [
        .target(name: "Core", path: "swift/core"),
        .target(name: "Mid", dependencies: ["Core"], path: "swift/mid"),
        .target(name: "Leaf", dependencies: ["Mid"], path: "swift/leaf"),
        .target(name: "Solo", path: "swift/solo"),
        .testTarget(name: "CoreTests", dependencies: ["Core"], path: "swift/tests", sources: ["CoreTests.swift"]),
        .testTarget(name: "MidTests", dependencies: ["Mid"], path: "swift/tests", sources: ["MidTests.swift"]),
        .testTarget(name: "LeafTests", dependencies: ["Leaf"], path: "swift/tests", sources: ["LeafLegacyTests.swift", "LeafModernTests.swift"]),
        .testTarget(name: "SoloTests", dependencies: ["Solo"], path: "swift/tests", sources: ["SoloTests.swift"]),
    ]
)
