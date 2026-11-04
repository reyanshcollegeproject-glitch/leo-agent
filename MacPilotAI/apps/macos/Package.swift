// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "MacPilot",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "MacPilot", targets: ["MacPilot"])
    ],
    targets: [
        .executableTarget(
            name: "MacPilot",
            path: "."
        )
    ]
)

