import SwiftUI
import WebKit
import Combine

struct VNCDesktopWebView: NSViewRepresentable {
    let url: URL

    func makeNSView(context: Context) -> WKWebView {
        let config = WKWebViewConfiguration()
        config.preferences.javaScriptCanOpenWindowsAutomatically = true
        let webView = WKWebView(frame: .zero, configuration: config)
        webView.setValue(false, forKey: "drawsBackground") // Transparent background
        let request = URLRequest(url: url)
        webView.load(request)
        return webView
    }

    func updateNSView(_ nsView: WKWebView, context: Context) {
        // Preserved active connection
    }
}

struct AIComputerView: View {
    @EnvironmentObject var env: AppEnvironment
    @State private var computerStatus: ComputerStatusModel?
    @State private var metrics: ComputerMetricsModel?
    @State private var isLoading: Bool = false
    @State private var isAIControl: Bool = true
    @State private var showSettings: Bool = false
    @State private var snapshotName: String = ""
    @State private var snapshotAlert: String? = nil

    private let vncURL = URL(string: "http://127.0.0.1:8089/desktop/vnc_lite.html?path=ws/vnc&scale=true&autoconnect=true")!
    let timer = Timer.publish(every: 2.0, on: .main, in: .common).autoconnect()

    var body: some View {
        VStack(spacing: 0) {
            // MARK: - 1. Top Header Bar
            headerBar

            Divider().background(Theme.borderSubtle)

            // MARK: - 2. Live Desktop / Canvas Viewport
            ZStack {
                Theme.bgBlack.ignoresSafeArea()

                if let status = computerStatus, status.computer.status == "RUNNING" {
                    VNCDesktopWebView(url: vncURL)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                        .padding(6)
                } else if let status = computerStatus, status.computer.status == "STARTING" {
                    bootingView
                } else {
                    stoppedPlaceholderView
                }

                if showSettings {
                    settingsOverlayView
                }
            }

            Divider().background(Theme.borderSubtle)

            // MARK: - 3. Telemetry & Metrics Footer Bar
            footerBar
        }
        .frame(minWidth: 840, minHeight: 560)
        .background(Theme.cardCharcoal)
        .onAppear {
            fetchStatus()
        }
        .onReceive(timer) { _ in
            fetchStatus()
        }
    }

    // MARK: - Header Bar
    private var headerBar: some View {
        HStack(spacing: 12) {
            // Computer Info
            HStack(spacing: 8) {
                Image(systemName: "display")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundColor(Theme.appleBlue)

                VStack(alignment: .leading, spacing: 1) {
                    Text(computerStatus?.computer.name ?? "Ubuntu 24.04 ARM64")
                        .font(.system(size: 13, weight: .bold))
                        .foregroundColor(Theme.textWhite)

                    Text(computerStatus?.computer.os ?? "Ubuntu 24.04 LTS (ARM64)")
                        .font(.system(size: 10, weight: .medium))
                        .foregroundColor(Theme.textMuted)
                }
            }

            // Running Status Indicator
            HStack(spacing: 5) {
                Circle()
                    .fill(statusColor)
                    .frame(width: 8, height: 8)

                Text(computerStatus?.computer.status ?? "STOPPED")
                    .font(.system(size: 10, weight: .heavy))
                    .foregroundColor(statusColor)
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 4)
            .background(statusColor.opacity(0.12))
            .cornerRadius(8)

            Spacer()

            // AI vs Manual Control Mode Switch
            HStack(spacing: 2) {
                Button(action: { setControlMode(ai: true) }) {
                    HStack(spacing: 4) {
                        Image(systemName: "bolt.fill")
                            .font(.system(size: 9))
                        Text("AI Control")
                            .font(.system(size: 10, weight: .bold))
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .foregroundColor(isAIControl ? .white : Theme.textMuted)
                    .background(isAIControl ? Theme.appleBlue : Color.clear)
                    .cornerRadius(6)
                }
                .buttonStyle(.plain)

                Button(action: { setControlMode(ai: false) }) {
                    HStack(spacing: 4) {
                        Image(systemName: "hand.point.up.left.fill")
                            .font(.system(size: 9))
                        Text("Manual Control")
                            .font(.system(size: 10, weight: .bold))
                    }
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .foregroundColor(!isAIControl ? .white : Theme.textMuted)
                    .background(!isAIControl ? Color.purple : Color.clear)
                    .cornerRadius(6)
                }
                .buttonStyle(.plain)
            }
            .padding(2)
            .background(Theme.bgBlack)
            .cornerRadius(8)

            // Start, Stop, Restart Controls
            HStack(spacing: 6) {
                if computerStatus?.computer.status != "RUNNING" {
                    Button(action: { startComputer() }) {
                        HStack(spacing: 4) {
                            Image(systemName: "play.fill")
                                .font(.system(size: 9, weight: .bold))
                            Text("Start")
                                .font(.system(size: 10, weight: .bold))
                        }
                        .foregroundColor(.white)
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background(Color.green)
                        .cornerRadius(6)
                    }
                    .buttonStyle(.plain)
                    .disabled(isLoading)
                } else {
                    Button(action: { stopComputer() }) {
                        HStack(spacing: 4) {
                            Image(systemName: "stop.fill")
                                .font(.system(size: 9, weight: .bold))
                            Text("Stop")
                                .font(.system(size: 10, weight: .bold))
                        }
                        .foregroundColor(.white)
                        .padding(.horizontal, 10)
                        .padding(.vertical, 5)
                        .background(Color.red)
                        .cornerRadius(6)
                    }
                    .buttonStyle(.plain)
                    .disabled(isLoading)

                    Button(action: { restartComputer() }) {
                        Image(systemName: "arrow.clockwise")
                            .font(.system(size: 10, weight: .bold))
                            .foregroundColor(.white)
                            .padding(6)
                            .background(Color.orange)
                            .cornerRadius(6)
                    }
                    .buttonStyle(.plain)
                    .help("Restart Virtual Computer")
                    .disabled(isLoading)
                }

                Button(action: { showSettings.toggle() }) {
                    Image(systemName: "gearshape.fill")
                        .font(.system(size: 11, weight: .semibold))
                        .foregroundColor(showSettings ? Theme.appleBlue : Theme.textMuted)
                        .padding(6)
                        .background(Theme.bgBlack)
                        .cornerRadius(6)
                }
                .buttonStyle(.plain)
                .help("Computer Settings")
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 10)
        .background(Theme.cardCharcoal)
    }

    // MARK: - Footer Telemetry Bar
    private var footerBar: some View {
        HStack(spacing: 20) {
            // CPU Metric
            metricGauge(
                title: "CPU",
                value: String(format: "%.1f%%", metrics?.cpuPercent ?? 8.4),
                percent: (metrics?.cpuPercent ?? 8.4) / 100.0,
                color: .blue
            )

            // RAM Metric
            metricGauge(
                title: "RAM",
                value: String(format: "%.1f / 4.0 GB", (metrics?.memoryUsedMb ?? 1240.0) / 1024.0),
                percent: (metrics?.memoryUsedMb ?? 1240.0) / 4096.0,
                color: .cyan
            )

            // Disk Metric
            metricGauge(
                title: "Disk",
                value: String(format: "%.1f / 20.0 GB", (metrics?.diskUsedMb ?? 3840.0) / 1024.0),
                percent: (metrics?.diskUsedMb ?? 3840.0) / 20480.0,
                color: .purple
            )

            Spacer()

            // Display Resolution & VNC Status
            HStack(spacing: 8) {
                Text("1920 × 1080 (HVF)")
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundColor(Theme.textMuted)

                HStack(spacing: 4) {
                    Circle()
                        .fill(computerStatus?.agentConnected == true ? Color.green : Color.orange)
                        .frame(width: 6, height: 6)
                    Text(computerStatus?.agentConnected == true ? "Agent Online" : "Agent Standby")
                        .font(.system(size: 9, weight: .medium))
                        .foregroundColor(Theme.textMuted)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 8)
        .background(Theme.cardCharcoal)
    }

    private func metricGauge(title: String, value: String, percent: Double, color: Color) -> some View {
        HStack(spacing: 6) {
            Text(title)
                .font(.system(size: 10, weight: .bold))
                .foregroundColor(Theme.textMuted)

            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 3)
                    .fill(Color.gray.opacity(0.2))
                    .frame(width: 50, height: 6)

                RoundedRectangle(cornerRadius: 3)
                    .fill(color)
                    .frame(width: max(6, min(50, CGFloat(percent) * 50)), height: 6)
            }

            Text(value)
                .font(.system(size: 10, weight: .semibold))
                .foregroundColor(Theme.textWhite)
        }
    }

    // MARK: - State Placeholders

    private var stoppedPlaceholderView: some View {
        VStack(spacing: 16) {
            ZStack {
                Circle()
                    .fill(Theme.appleBlue.opacity(0.12))
                    .frame(width: 72, height: 72)
                Image(systemName: "desktopcomputer")
                    .font(.system(size: 32, weight: .bold))
                    .foregroundColor(Theme.appleBlue)
            }

            VStack(spacing: 6) {
                Text("Dedicated Ubuntu ARM64 Virtual Computer")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundColor(Theme.textWhite)

                Text("Persistent disk storage • XFCE Desktop • Chromium • Go • Python • Node.js")
                    .font(.system(size: 12))
                    .foregroundColor(Theme.textMuted)
            }

            Button(action: { startComputer() }) {
                HStack(spacing: 6) {
                    Image(systemName: "bolt.fill")
                    Text("Start Virtual Computer")
                        .font(.system(size: 13, weight: .heavy))
                }
                .foregroundColor(.white)
                .padding(.horizontal, 18)
                .padding(.vertical, 10)
                .background(Theme.appleBlue)
                .cornerRadius(10)
            }
            .buttonStyle(.plain)
            .disabled(isLoading)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private var bootingView: some View {
        VStack(spacing: 14) {
            ProgressView()
                .scaleEffect(1.2)
                .progressViewStyle(CircularProgressViewStyle(tint: Theme.appleBlue))

            Text("Booting Ubuntu ARM64 & Initializing Guest Agent...")
                .font(.system(size: 13, weight: .semibold))
                .foregroundColor(Theme.textWhite)

            Text("Hardware acceleration: Apple Silicon HVF")
                .font(.system(size: 11))
                .foregroundColor(Theme.textMuted)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    // MARK: - Settings Overlay Drawer
    private var settingsOverlayView: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                Text("Virtual Computer Configuration")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundColor(Theme.textWhite)
                Spacer()
                Button(action: { showSettings = false }) {
                    Image(systemName: "xmark.circle.fill")
                        .font(.system(size: 14))
                        .foregroundColor(Theme.textMuted)
                }
                .buttonStyle(.plain)
            }

            Divider().background(Theme.borderSubtle)

            VStack(alignment: .leading, spacing: 8) {
                settingRow(label: "Architecture", value: "ARM64 (Apple Silicon Host)")
                settingRow(label: "Virtualization Engine", value: "QEMU with -accel hvf")
                settingRow(label: "CPU Cores", value: "\(computerStatus?.computer.cpuCores ?? 4) vCPUs")
                settingRow(label: "Memory (RAM)", value: "\(computerStatus?.computer.memoryMb ?? 4096) MB")
                settingRow(label: "Persistent Disk", value: "\(computerStatus?.computer.diskSizeGb ?? 20) GB (Sparse qcow2)")
                settingRow(label: "Display Resolution", value: "1920 × 1080 (virtio-gpu-pci)")
                settingRow(label: "VNC Streaming Port", value: "5900 (Proxy on /ws/vnc)")
                settingRow(label: "Guest Agent Port", value: "8090 (Bearer Auth)")
            }

            Divider().background(Theme.borderSubtle)

            // Snapshot Management
            Text("Snapshots & Recovery")
                .font(.system(size: 12, weight: .bold))
                .foregroundColor(Theme.textWhite)

            HStack {
                TextField("Snapshot label (e.g. dev-checkpoint-1)", text: $snapshotName)
                    .textFieldStyle(PlainTextFieldStyle())
                    .padding(6)
                    .background(Theme.bgBlack)
                    .cornerRadius(6)

                Button("Take Snapshot") {
                    createSnapshot()
                }
                .font(.system(size: 11, weight: .bold))
                .padding(.horizontal, 10)
                .padding(.vertical, 6)
                .background(Theme.appleBlue)
                .foregroundColor(.white)
                .cornerRadius(6)
            }

            Spacer()
        }
        .padding(18)
        .frame(width: 380)
        .background(Theme.cardCharcoal)
        .overlay(
            Rectangle()
                .stroke(Theme.borderSubtle, lineWidth: 1)
        )
        .frame(maxWidth: .infinity, alignment: .trailing)
        .transition(.move(edge: .trailing))
    }

    private func settingRow(label: String, value: String) -> some View {
        HStack {
            Text(label)
                .font(.system(size: 11))
                .foregroundColor(Theme.textMuted)
            Spacer()
            Text(value)
                .font(.system(size: 11, weight: .semibold))
                .foregroundColor(Theme.textWhite)
        }
    }

    private var statusColor: Color {
        switch computerStatus?.computer.status {
        case "RUNNING": return .green
        case "STARTING": return .yellow
        default: return .gray
        }
    }

    // MARK: - Actions

    private func fetchStatus() {
        Task {
            if let s = try? await env.agentClient.getComputerStatus() {
                self.computerStatus = s
                self.isAIControl = (s.computer.controlMode == "ai")
            }
            if let m = try? await env.agentClient.getComputerMetrics() {
                self.metrics = m
            }
        }
    }

    private func startComputer() {
        isLoading = true
        Task {
            try? await env.agentClient.startComputer()
            fetchStatus()
            isLoading = false
        }
    }

    private func stopComputer() {
        isLoading = true
        Task {
            try? await env.agentClient.stopComputer()
            fetchStatus()
            isLoading = false
        }
    }

    private func restartComputer() {
        isLoading = true
        Task {
            try? await env.agentClient.restartComputer()
            fetchStatus()
            isLoading = false
        }
    }

    private func setControlMode(ai: Bool) {
        isAIControl = ai
        Task {
            try? await env.agentClient.setComputerMode(mode: ai ? "ai" : "manual")
        }
    }

    private func createSnapshot() {
        let name = snapshotName.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !name.isEmpty else { return }
        Task {
            let encoded = name.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed) ?? name
            let url = URL(string: "http://127.0.0.1:8089/api/v1/computers/snapshots?id=ubuntu-primary&name=\(encoded)")!
            var req = URLRequest(url: url)
            req.httpMethod = "POST"
            _ = try? await URLSession.shared.data(for: req)
            snapshotName = ""
        }
    }
}
