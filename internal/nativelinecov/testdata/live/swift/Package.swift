// swift-tools-version: 6.0
// SPDX-License-Identifier: Apache-2.0
import PackageDescription
let package = Package(name: "CovliveProbe", products: [.library(name: "Logic", targets: ["Logic"])], targets: [.target(name: "Logic"), .testTarget(name: "LogicTests", dependencies: ["Logic"])])
