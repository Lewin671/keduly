import SwiftUI

/// Settings: the name, the working hours, the focus lengths, and signing out. Each change is saved as it is made.
struct SettingsView: View {
    @Environment(AppModel.self) private var model
    @State private var name = ""
    @State private var error: String?
    @State private var saving = false
    @State private var confirmingSignOut = false
    @FocusState private var editingName: Bool

    var body: some View {
        Form {
            if let user = model.user {
                Section {
                    LabeledContent(L("account.name")) {
                        TextField(L("account.name"), text: $name)
                            .multilineTextAlignment(.trailing)
                            .focused($editingName)
                            .submitLabel(.done)
                            .onSubmit { Task { await saveName() } }
                            .accessibilityIdentifier("settings-name")
                    }
                    LabeledContent(L("account.email"), value: user.email)
                    LabeledContent(L("account.timezone"), value: user.timezone)
                    LabeledContent(L("account.server"), value: model.server?.host() ?? "")
                } header: {
                    Text(L("account.title"))
                } footer: {
                    if let error { Text(error).foregroundStyle(Palette.red) }
                }

                Section {
                    clock(L("account.workStart"), user.workStart) { UserPatch(workStart: $0) }
                    clock(L("account.workEnd"), user.workEnd) { UserPatch(workEnd: $0) }
                } header: {
                    Text(L("account.hours"))
                } footer: {
                    Text(L("account.hoursNote"))
                }

                Section {
                    stepper(L("focusSettings.tomato"), user.focusMinutes, 1...180, L("focusSettings.minutes"), id: "focus-minutes") { UserPatch(focusMinutes: $0) }
                    stepper(L("focusSettings.rest"), user.restMinutes, 1...60, L("focusSettings.minutes"), id: "rest-minutes") { UserPatch(restMinutes: $0) }
                    stepper(L("focusSettings.longRest"), user.longRestMinutes, 1...120, L("focusSettings.minutes"), id: "long-rest-minutes") { UserPatch(longRestMinutes: $0) }
                    stepper(L("focusSettings.round"), user.roundSize, 2...12, L("stats.unitCount"), id: "round-size") { UserPatch(roundSize: $0) }
                } header: {
                    Text(L("focusSettings.title"))
                } footer: {
                    Text(L("focusSettings.footer"))
                }

                Section {
                    Button(L("account.signOut"), role: .destructive) { confirmingSignOut = true }
                        .accessibilityIdentifier("sign-out")
                }
            }
        }
        .navigationTitle(L("settings.title"))
        .navigationBarTitleDisplayMode(.large)
        .onAppear { name = model.user?.name ?? "" }
        .onChange(of: editingName) { _, editing in
            if !editing { Task { await saveName() } }
        }
        .confirmationDialog(L("account.signOutAsk"), isPresented: $confirmingSignOut, titleVisibility: .visible) {
            Button(L("account.signOut"), role: .destructive) { Task { await model.signOut() } }
            Button(L("common.cancel"), role: .cancel) {}
        }
    }

    /// One save at a time: a control reads the value the server last confirmed, so a second
    /// change made before the first one returned would start from the old value.
    private func save(_ patch: UserPatch) async {
        guard !saving else { return }
        saving = true
        defer { saving = false }
        switch await model.write(inline: true, { try await $0.updateMe(patch) }) {
        case .success: error = nil
        case .failure(let failure): error = failure.status == 400 ? failure.message : nil
        }
    }

    private func saveName() async {
        // Another setting may be on its way; the name waits for it instead of being dropped.
        while saving { try? await Task.sleep(for: .milliseconds(50)) }
        let text = name.trimmingCharacters(in: .whitespaces)
        guard let user = model.user, text != user.name else { return }
        if text.isEmpty { name = user.name } else { await save(UserPatch(name: text)) }
    }

    /// A time of day in the account's zone.
    private func clock(_ title: String, _ value: String, _ patch: @escaping (String) -> UserPatch) -> some View {
        let day = model.today
        return DatePicker(title, selection: Binding(
            get: { model.zone.date(on: day, minutes: Zone.minutes(ofClock: value) ?? 0) },
            set: { date in Task { await save(patch(model.zone.hhmm(date))) } }
        ), displayedComponents: .hourAndMinute)
        .disabled(saving)
    }

    private func stepper(_ title: String, _ value: Int, _ range: ClosedRange<Int>, _ unit: String, id: String, _ patch: @escaping (Int) -> UserPatch) -> some View {
        Stepper(value: Binding(get: { value }, set: { new in Task { await save(patch(new)) } }), in: range) {
            HStack {
                Text(title)
                Spacer()
                Text("\(value) \(unit)").foregroundStyle(Palette.label2).monospacedDigit()
            }
        }
        .disabled(saving)
        .accessibilityIdentifier(id)
    }
}
