import SwiftUI

/// Letting another device in from this phone: the approval link it shared, then the same approval
/// as the web app. The pin is only ever typed here, read off the other screen.
struct ApproveView: View {
    private enum Step: Equatable {
        case paste, loading
        case asking(PendingLogin)
        case done, gone
    }

    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State private var step = Step.paste
    @State private var link = ""
    @State private var pin = ""
    @State private var error: String?
    @State private var busy = false

    var body: some View {
        Form {
            switch step {
            case .paste, .loading:
                Section {
                    HStack {
                        TextField(L("approve.link"), text: $link)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .disabled(step == .loading)
                            .onSubmit { read(link) }
                            .accessibilityIdentifier("approve-link")
                        PasteButton(payloadType: String.self) { strings in
                            Task { @MainActor in read(strings.first ?? "") }
                        }
                        .labelStyle(.titleOnly)
                        .buttonBorderShape(.capsule)
                        .disabled(step == .loading)
                    }
                    if step == .loading { ProgressView().frame(maxWidth: .infinity) }
                } header: {
                    Text(L("approve.pasteLead")).textCase(nil).font(.app(15)).foregroundStyle(Palette.label2)
                } footer: {
                    if let error {
                        Text(error).foregroundStyle(Palette.red).accessibilityIdentifier("approve-error")
                    }
                }
            case .asking(let request):
                asking(request)
            case .done, .gone:
                let done = step == .done
                VStack(spacing: 12) {
                    Text(L(done ? "approve.done" : "approve.gone")).font(.app(22, .bold))
                    Text(L(done ? "approve.doneLead" : "approve.goneLead")).font(.app(15)).foregroundStyle(Palette.label2)
                    Button(L("common.ok")) { dismiss() }
                        .buttonStyle(CapsuleButtonStyle(primary: true))
                        .padding(.top, 8)
                }
                .multilineTextAlignment(.center)
                .frame(maxWidth: .infinity)
                .padding(.vertical, 12)
                .listRowBackground(Color.clear)
            }
        }
        .navigationTitle(L("approve.title"))
        .navigationBarTitleDisplayMode(.inline)
    }

    private func asking(_ request: PendingLogin) -> some View {
        let email = model.user?.email ?? ""
        return VStack(spacing: 14) {
            Text(L("approve.ask")).font(.app(22, .bold))
            Text(request.device.isEmpty ? L("approve.leadUnknown", email) : L("approve.lead", request.device, email))
                .font(.app(15)).foregroundStyle(Palette.label2)
            Text(L("approve.pin")).font(.app(14)).foregroundStyle(Palette.label2).padding(.top, 8)
            TextField("", text: Binding(get: { pin }, set: { pin = String($0.filter(\.isNumber).prefix(4)); error = nil }))
                .keyboardType(.numberPad)
                .textContentType(.oneTimeCode)
                .font(.system(size: 32, weight: .semibold, design: .monospaced))
                .multilineTextAlignment(.center)
                .frame(width: 180)
                .padding(.vertical, 6)
                .overlay(alignment: .bottom) { Rectangle().fill(Palette.separator).frame(height: 1) }
                .accessibilityIdentifier("approve-pin")
            if let error {
                Text(error).font(.app(14)).foregroundStyle(Palette.red).accessibilityIdentifier("approve-error")
            }
            Text(L("approve.warning")).font(.app(13)).foregroundStyle(Palette.orange)
            Button(L("approve.allow")) { Task { await approve(request) } }
                .buttonStyle(CapsuleButtonStyle(primary: true))
                .disabled(pin.count != 4 || busy)
                .accessibilityIdentifier("approve-allow")
            Button(L("approve.refuse")) { Task { await refuse(request) } }
                .buttonStyle(.borderless)
                .foregroundStyle(Palette.tint)
                .disabled(busy)
                .accessibilityIdentifier("approve-refuse")
        }
        .multilineTextAlignment(.center)
        .frame(maxWidth: .infinity)
        .padding(.vertical, 12)
        .listRowBackground(Color.clear)
    }

    private func read(_ text: String) {
        link = text
        error = nil
        switch ApprovalLink.parse(text, server: model.server) {
        case .notALink:
            if !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { error = L("approve.notLink") }
        case .otherServer(let host):
            error = L("approve.otherServer", host)
        case .request(let id):
            step = .loading
            Task {
                do {
                    step = .asking(try await model.api.loginRequest(id))
                } catch {
                    step = .paste
                    failed(error)
                }
            }
        }
    }

    private func approve(_ request: PendingLogin) async {
        busy = true
        defer { busy = false }
        do {
            try await model.api.approveLoginRequest(request.id, pin: pin)
            step = .done
        } catch let failure as APIError where failure.status == 400 {
            // A wrong pin: the field clears for another look at the other screen.
            pin = ""
            error = L("approve.wrongPin")
        } catch {
            failed(error)
        }
    }

    private func refuse(_ request: PendingLogin) async {
        busy = true
        defer { busy = false }
        do {
            try await model.api.refuseLoginRequest(request.id)
            dismiss()
        } catch let failure as APIError where failure.status == 404 {
            dismiss()
        } catch {
            failed(error)
        }
    }

    /// A request that is unknown, expired, used, refused or out of pins is a 404, whichever call met it.
    private func failed(_ failure: any Error) {
        let failure = failure as? APIError
        if failure?.status == 404 { step = .gone } else { error = failure?.message ?? L("error.unknown") }
    }
}
