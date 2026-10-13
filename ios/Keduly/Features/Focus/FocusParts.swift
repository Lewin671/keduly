import SwiftUI

/// The dial, after Apple's Clock timer: one ring, large light figures, nothing else inside it.
struct Dial<Center: View>: View {
    let color: Color
    let fraction: Double
    @ViewBuilder var center: Center

    var body: some View {
        ZStack {
            Circle().stroke(Palette.fill, lineWidth: 9)
            Circle().trim(from: 0, to: max(0, min(1, fraction)))
                .stroke(color, style: StrokeStyle(lineWidth: 9, lineCap: .round))
                .rotationEffect(.degrees(-90))
                .animation(.linear(duration: 0.5), value: fraction)
            center
        }
        .padding(4.5)
        .frame(width: 272, height: 272)
        .dynamicTypeSize(...AppType.fixedLayoutLimit)
    }
}

struct DialFigures: View {
    let time: String
    let caption: String

    var body: some View {
        VStack(spacing: 4) {
            Text(time).font(.app(72, .ultraLight)).monospacedDigit().tracking(-2)
                .accessibilityIdentifier("dial-time")
            Text(caption).font(.app(13)).foregroundStyle(Palette.label2)
        }
    }
}

/// The round: one mark per tomato, filled as they are earned, and how far the long rest is.
struct RoundMarks: View {
    let focus: Focus

    var body: some View {
        HStack(spacing: 7) {
            ForEach(0..<max(1, focus.roundSize), id: \.self) { index in
                TomatoMark(filled: index < focus.roundDone, size: 22)
            }
            Text(text).font(.app(13)).foregroundStyle(Palette.label2).padding(.leading, 5)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel("\(focus.roundDone)/\(focus.roundSize) \(text)")
    }

    private var text: String {
        if focus.roundDone >= focus.roundSize { return L("focus.roundDone") }
        if focus.roundDone == 0 { return L("focus.roundSize", focus.roundSize) }
        return L("focus.roundLeft", focus.roundSize - focus.roundDone)
    }
}

/// What a tomato is on: a title, and a line that names the project and the tomatoes earned.
struct FocusSubject: Equatable {
    var title: String
    var projectId: String?
    var item: Item?
    var free: Bool
}

extension AppModel {
    /// The colour of the timer page: the subject's project, tomato red once earned, green while resting.
    func focusColor(_ state: Focus.State, projectId: String?) -> UIColor {
        switch state {
        case .over: return Palette.tomatoUI
        case .rest: return Palette.greenUI
        case .idle, .work: return Palette.projectUI(project(projectId)?.color)
        }
    }
}

/// Choosing what the next tomato is on: today's open items, important first, then free focus.
struct FocusPickerSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.dismiss) private var dismiss
    @State private var loader = Loader<TodayResponse>()

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 0) {
                    if loader.failed { ErrorLine(retry: load) }
                    ForEach(FocusPicker.candidates(loader.value)) { item in
                        Button {
                            router.focusPick = .item(item)
                            dismiss()
                        } label: {
                            row(title: item.title, important: item.important, detail: detail(item), selected: router.focusPick == .item(item))
                        }
                        .buttonStyle(.plain)
                        .accessibilityIdentifier("pick-\(item.title)")
                    }
                    Button {
                        router.focusPick = .free
                        dismiss()
                    } label: {
                        row(title: L("focus.free"), important: false, detail: L("focus.freeNote"), selected: router.focusPick == .free)
                    }
                    .buttonStyle(.plain)
                    .accessibilityIdentifier("pick-free")
                }
                .padding(.horizontal, Metrics.pageMargin)
            }
            .navigationTitle(L("focus.pick"))
            .navigationBarTitleDisplayMode(.inline)
        }
        .presentationDetents([.medium, .large])
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        await loader.load(model) { try await $0.today() }
    }

    private func detail(_ item: Item) -> String {
        var parts = [model.project(item.projectId)?.name ?? L("nav.inbox")]
        if let estimate = item.estimateMinutes { parts.append(Format.dur(estimate)) }
        if item.focus.tomatoes > 0 { parts.append(ItemText.earned(item, tomatoMinutes: model.user?.focusMinutes ?? 25)) }
        return parts.joined(separator: " · ")
    }

    private func row(title: String, important: Bool, detail: String, selected: Bool) -> some View {
        HStack(spacing: 10) {
            VStack(alignment: .leading, spacing: 2) {
                (Text(important ? "! " : "").foregroundStyle(Palette.orange).fontWeight(.heavy) + Text(title))
                    .font(.app(17)).lineLimit(2).multilineTextAlignment(.leading)
                Text(detail).font(.app(13)).foregroundStyle(Palette.label2).lineLimit(1)
            }
            Spacer(minLength: 0)
            if selected { Image(systemName: "checkmark").fontWeight(.semibold).foregroundStyle(Palette.tint) }
        }
        .padding(.vertical, 10)
        .frame(minHeight: 44)
        .overlay(alignment: .bottom) { Divider() }
        .contentShape(Rectangle())
    }
}

enum FocusPicker {
    /// Today's open items in the server's order, which puts the important ones first.
    static func candidates(_ today: TodayResponse?) -> [Item] {
        guard let today else { return [] }
        return (today.items + today.overdue).filter { !$0.isDone }
    }
}

/// Saying what a session was for: a project, then one of its items, or free focus with a title.
/// Each field applies at once. The time and the tomato never change, only where they count.
struct FilingSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    @State var session: FocusSession
    @State private var items: [Item] = []
    @State private var title = ""
    @State private var error: String?
    @FocusState private var editing: Bool

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Picker(L("field.project"), selection: Binding(
                        get: { session.projectId },
                        set: { value in Task { await setProject(value) } }
                    )) {
                        Text(L("stats.noProject")).tag(String?.none)
                        ForEach(model.projects) { Text($0.name).tag(String?.some($0.id)) }
                    }
                    .accessibilityIdentifier("filing-project")
                    Picker(L("session.item"), selection: Binding(
                        get: { session.itemId },
                        set: { value in Task { await apply(SessionPatch(itemId: Patch(value))) } }
                    )) {
                        Text(L("session.noItem")).tag(String?.none)
                        // The session's own item stays a choice even when it is done or belongs elsewhere.
                        if let id = session.itemId, !items.contains(where: { $0.id == id }) {
                            Text(session.title).tag(String?.some(id))
                        }
                        ForEach(items) { Text($0.title).tag(String?.some($0.id)) }
                    }
                    .accessibilityIdentifier("filing-item")
                    if session.itemId == nil {
                        TextField(L("session.what"), text: $title)
                            .focused($editing)
                            .submitLabel(.done)
                            .onSubmit { Task { await commitTitle() } }
                            .accessibilityIdentifier("filing-title")
                    }
                } footer: {
                    if let error { Text(error).foregroundStyle(Palette.red) }
                }
            }
            .navigationTitle(L("session.pick"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(L("common.done")) {
                        Task {
                            await commitTitle()
                            dismiss()
                        }
                    }
                    .fontWeight(.semibold)
                    .accessibilityIdentifier("filing-done")
                }
            }
        }
        .presentationDetents([.medium, .large])
        .onAppear { title = session.itemId == nil ? session.title : "" }
        .task(id: session.projectId) { await loadItems() }
    }

    /// The project's open items; without a project, today's.
    private func loadItems() async {
        if let project = session.projectId {
            items = (try? await model.api.items(.init(projectId: project), upTo: .max))?.items ?? []
        } else {
            items = FocusPicker.candidates(try? await model.api.today())
        }
    }

    /// Choosing another project takes the session off its item first: only free focus has a project of its own.
    private func setProject(_ project: String?) async {
        if session.itemId != nil {
            guard await apply(SessionPatch(itemId: .clear)) else { return }
        }
        await apply(SessionPatch(projectId: Patch(project)))
    }

    private func commitTitle() async {
        let text = title.trimmingCharacters(in: .whitespaces)
        guard session.itemId == nil, text != session.title else { return }
        await apply(SessionPatch(title: .set(text)))
    }

    @discardableResult
    private func apply(_ patch: SessionPatch) async -> Bool {
        let id = session.id
        switch await model.write(inline: true, { try await $0.updateSession(id, patch) }) {
        case .success(let updated):
            session = updated
            if updated.itemId == nil, !editing { title = updated.title }
            error = nil
            return true
        case .failure(let failure):
            error = failure.status == 400 ? failure.message : nil
            return false
        }
    }
}
