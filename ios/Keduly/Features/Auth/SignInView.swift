import SwiftUI

/// The first screen: the server address, the email and the password. Registering is offered when
/// the server allows it.
struct SignInView: View {
    @Environment(AppModel.self) private var model
    @AppStorage("server") private var savedServer = ""
    @State private var server = ""
    @State private var email = ""
    @State private var password = ""
    @State private var name = ""
    @State private var registering = false
    @State private var canRegister = false
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        ScrollView {
            VStack(spacing: 0) {
                Image("Mark").resizable().frame(width: 64, height: 64)
                    .clipShape(RoundedRectangle(cornerRadius: 15, style: .continuous))
                    .padding(.top, 56)
                    .accessibilityHidden(true)
                Text("Keduly").font(.system(size: 28, weight: .bold)).padding(.top, 14)
                Text(registering ? L("auth.registerLead") : L("auth.loginLead")).font(.system(size: 15)).foregroundStyle(Palette.label2).padding(.top, 4)

                GroupCard {
                    field(L("auth.server"), last: false) {
                        TextField(L("auth.serverPlaceholder"), text: $server)
                            .keyboardType(.URL).textContentType(.URL).textInputAutocapitalization(.never).autocorrectionDisabled()
                            .accessibilityIdentifier("signin-server")
                    }
                    if registering {
                        field(L("auth.name"), last: false) {
                            TextField(L("auth.name"), text: $name).textContentType(.name).accessibilityIdentifier("signin-name")
                        }
                    }
                    field(L("auth.email"), last: false) {
                        TextField("me@example.com", text: $email)
                            .keyboardType(.emailAddress).textContentType(.username).textInputAutocapitalization(.never).autocorrectionDisabled()
                            .accessibilityIdentifier("signin-email")
                    }
                    field(L("auth.password"), last: true) {
                        SecureField(registering ? L("auth.passwordTip") : L("auth.password"), text: $password)
                            .textContentType(registering ? .newPassword : .password)
                            .submitLabel(.go)
                            .onSubmit { Task { await submit() } }
                            .accessibilityIdentifier("signin-password")
                    }
                }
                .padding(.top, 28)

                if let error {
                    Text(error).font(.system(size: 13)).foregroundStyle(Palette.red)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding(.horizontal, 16).padding(.top, 8)
                        .accessibilityIdentifier("signin-error")
                }

                Button(registering ? L("auth.register") : L("auth.login")) { Task { await submit() } }
                    .buttonStyle(CapsuleButtonStyle(primary: true))
                    .disabled(busy || server.isEmpty || email.isEmpty || password.isEmpty || (registering && name.isEmpty))
                    .padding(.top, 20)
                    .accessibilityIdentifier("signin-submit")

                if canRegister || registering {
                    Button(registering ? L("auth.toLogin") : L("auth.toRegister")) {
                        registering.toggle()
                        error = nil
                    }
                    .font(.system(size: 15))
                    .frame(minHeight: 44)
                    .padding(.top, 6)
                    .accessibilityIdentifier("signin-toggle")
                }
            }
            .padding(.horizontal, Metrics.pageMargin)
        }
        .background(Palette.grouped)
        .onAppear { if server.isEmpty { server = savedServer } }
        .task(id: server) {
            // Whether to offer registering depends on the server, once its address looks complete.
            try? await Task.sleep(for: .milliseconds(500))
            guard !Task.isCancelled, let url = try? ServerAddress.parse(server) else { return }
            await model.api.use(server: url)
            if let config = try? await model.api.config(), !Task.isCancelled {
                canRegister = config.registration == .open
            }
        }
    }

    private func field<Content: View>(_ title: String, last: Bool, @ViewBuilder content: () -> Content) -> some View {
        HStack(spacing: 12) {
            Text(title).font(.system(size: 17)).frame(width: 64, alignment: .leading)
            content().font(.system(size: 17))
        }
        .padding(.horizontal, 16)
        .frame(minHeight: 48)
        .overlay(alignment: .bottom) { if !last { Divider().padding(.leading, 16) } }
    }

    private func submit() async {
        guard !busy else { return }
        let url: URL
        do {
            url = try ServerAddress.parse(server)
        } catch {
            self.error = (error as? ServerAddress.Problem) == .insecure ? L("auth.serverInsecure") : L("auth.serverInvalid")
            return
        }
        busy = true
        defer { busy = false }
        do {
            try await model.signIn(server: url, email: email.trimmingCharacters(in: .whitespaces), password: password, registerAs: registering ? name.trimmingCharacters(in: .whitespaces) : nil)
            savedServer = url.absoluteString
        } catch APIError.unauthenticated {
            error = L("auth.wrong")
        } catch let failure as APIError {
            error = failure.status == 409 ? L("auth.taken") : failure.message
        } catch {
            self.error = L("error.unknown")
        }
    }
}

/// The server address as typed, made into a URL. Cleartext HTTP is for development: only debug builds take it.
enum ServerAddress {
    enum Problem: Error, Equatable { case invalid, insecure }

    static func parse(_ text: String, allowsHTTP: Bool = allowsCleartext) throws -> URL {
        var string = text.trimmingCharacters(in: .whitespacesAndNewlines)
        while string.hasSuffix("/") { string.removeLast() }
        guard !string.isEmpty else { throw Problem.invalid }
        if !string.contains("://") { string = "https://" + string }
        guard let components = URLComponents(string: string), let scheme = components.scheme?.lowercased(),
              let host = components.host, !host.isEmpty, components.query == nil, components.fragment == nil,
              let url = components.url else { throw Problem.invalid }
        switch scheme {
        case "https": return url
        case "http":
            guard allowsHTTP else { throw Problem.insecure }
            return url
        default: throw Problem.invalid
        }
    }

    static var allowsCleartext: Bool {
        #if DEBUG
        true
        #else
        false
        #endif
    }
}
