// swift-tools-version:5.7
import PackageDescription

/// GoWind UBA iOS/macOS 埋点 SDK（uba-swift）。
/// 核心仅依赖 Foundation（URLSession），iOS / macOS / tvOS / watchOS 通用。
let package = Package(
    name: "UbaSDK",
    platforms: [
        .iOS(.v13), .macOS(.v10_15), .tvOS(.v13), .watchOS(.v6),
    ],
    products: [
        .library(name: "UbaSDK", targets: ["UbaSDK"]),
    ],
    targets: [
        .target(name: "UbaSDK", path: "Sources/UbaSDK"),
        .testTarget(name: "UbaSDKTests", dependencies: ["UbaSDK"], path: "Tests/UbaSDKTests"),
    ]
)
