// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MetaMuse",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "MetaMuse", targets: ["MetaMuse"])
    ],
    targets: [
        .executableTarget(
            name: "MetaMuse",
            path: "Sources/MetaMuse"
        )
    ]
)
