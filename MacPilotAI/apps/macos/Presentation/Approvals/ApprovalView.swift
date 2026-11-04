import SwiftUI

struct ApprovalView: View {
    let approval: ApprovalItem
    let onResolve: (Bool) -> Void

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            // Header
            HStack(spacing: 8) {
                Image(systemName: "exclamationmark.shield.fill")
                    .foregroundColor(Theme.warningOrange)
                    .font(.system(size: 16))

                Text("Security Approval Required")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundColor(Theme.textWhite)

                Spacer()

                Text(approval.severity.uppercased())
                    .font(.system(size: 10, weight: .heavy))
                    .foregroundColor(Theme.warningOrange)
                    .padding(.horizontal, 6)
                    .padding(.vertical, 2)
                    .background(Theme.warningOrange.opacity(0.15))
                    .cornerRadius(4)
            }

            // Reason
            Text(approval.reason)
                .font(.system(size: 12))
                .foregroundColor(Theme.textSecondary)

            // Command Box
            Text(approval.command)
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundColor(Theme.textWhite)
                .padding(10)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.bgBlack)
                .cornerRadius(6)
                .overlay(
                    RoundedRectangle(cornerRadius: 6)
                        .stroke(Theme.borderSubtle, lineWidth: 1)
                )

            // Action Buttons
            HStack(spacing: 12) {
                Button(action: {
                    onResolve(false)
                }) {
                    Text("Deny Action")
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundColor(Theme.textWhite)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 7)
                        .background(Theme.alertRed.opacity(0.8))
                        .cornerRadius(6)
                }
                .buttonStyle(.plain)

                Button(action: {
                    onResolve(true)
                }) {
                    Text("Approve & Execute")
                        .font(.system(size: 12, weight: .semibold))
                        .foregroundColor(.white)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 7)
                        .background(Theme.appleBlue)
                        .cornerRadius(6)
                }
                .buttonStyle(.plain)
            }
        }
        .padding(16)
        .background(Theme.cardCharcoal)
        .cornerRadius(12)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Theme.warningOrange.opacity(0.4), lineWidth: 1)
        )
    }
}

