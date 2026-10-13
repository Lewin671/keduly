import SwiftUI

/// The claim secret stays in memory; only the public approval link can be shared.
struct DeviceLoginRequest: Decodable, Sendable {
    struct Request: Decodable, Sendable {
        let id: String
        let expiresAt: Date
    }
    let request: Request
    let pin: String
    let secret: String

    func approvalURL(server: URL) -> URL? {
        var parts = URLComponents(url: server, resolvingAgainstBaseURL: false)
        parts?.path = "/"
        parts?.query = nil
        parts?.fragment = "approve=\(request.id)"
        return parts?.url
    }
}

struct DeviceLoginClaim: Decodable, Sendable {
    let status: String
    let user: User?
}

struct DeviceAuthorizationView: View {
    let server: URL
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @Environment(\.scenePhase) private var scenePhase
    @Environment(\.openURL) private var openURL
    @AppStorage("server") private var savedServer = ""
    @State private var request: DeviceLoginRequest?
    @State private var error: String?
    @State private var attempt = 0

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 20) {
                    Text(L("auth.authorizationLead")).font(.app(22, .bold))
                    Text(L("auth.authorizationHelp")).font(.app(15))
                        .foregroundStyle(Palette.label2)
                    if let request, let url = request.approvalURL(server: server) {
                        Text(request.pin).font(.system(size: 36, weight: .semibold, design: .monospaced))
                            .accessibilityIdentifier("authorization-pin")
                        if error == nil {
                            Text(L("auth.authorizationWaiting")).font(.app(14)).foregroundStyle(Palette.label2)
                            Button(L("auth.authorizationOpen")) { openURL(url) }
                                .buttonStyle(CapsuleButtonStyle(primary: true))
                            ShareLink(L("auth.authorizationShare"), item: url)
                            Text(url.absoluteString).font(.app(12)).foregroundStyle(Palette.label2)
                                .textSelection(.enabled)
                                .accessibilityIdentifier("authorization-link")
                        }
                    } else if error == nil {
                        ProgressView()
                    }
                    if let error {
                        Text(error).font(.app(15)).foregroundStyle(Palette.red)
                            .accessibilityIdentifier("authorization-error")
                        Button(L("auth.authorizationRetry")) { attempt += 1 }
                    }
                }
                .multilineTextAlignment(.center)
                .padding(24).padding(.top, 24)
            }
            .background(Palette.grouped.ignoresSafeArea())
            .navigationTitle(L("auth.authorizationTitle"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button(L("auth.authorizationCancel")) { dismiss() }
                }
            }
        }
        .task(id: attempt) { await authorize() }
    }

    @MainActor
    private func authorize() async {
        request = nil
        error = nil
        do {
            await model.api.use(server: server)
            let current = try await model.api.startLoginRequest()
            try Task.checkCancellation()
            request = current
            while Date() < current.request.expiresAt {
                try await Task.sleep(for: .seconds(2))
                guard scenePhase == .active else { continue }
                guard Date() < current.request.expiresAt else { break }
                if try await model.api.claimLoginRequest(current) {
                    try Task.checkCancellation()
                    savedServer = server.absoluteString
                    try await model.finishDeviceSignIn(server: server)
                    dismiss()
                    return
                }
            }
            error = L("auth.authorizationExpired")
        } catch is CancellationError {
            // Closing the sheet stops claiming. The server expires the unclaimed request.
        } catch let failure as APIError {
            error = failure.status == 404 ? L("auth.authorizationExpired") : failure.message
        } catch {
            self.error = L("error.unknown")
        }
    }
}
