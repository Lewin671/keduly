import SwiftUI

/// An item in full: title, notes, the agent's suggestion, then one grouped list of its fields.
/// Every change is sent as it is made.
struct ItemSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @Environment(\.dismiss) private var dismiss
    let id: String

    @State private var loader = Loader<Item>()
    @State private var title = ""
    @State private var notes = ""
    @State private var error: String?
    @State private var confirmingDelete = false
    @FocusState private var focus: Field?

    private enum Field { case title, notes }

    enum Page: Hashable { case when, due, estimate, project }

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    if loader.failed { ErrorLine(retry: load) }
                    if let item = loader.value {
                        content(item)
                    } else if !loader.failed {
                        ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.bottom, 24)
            }
            .background(Palette.grouped)
            .navigationTitle(L("item.detail"))
            .inlineTitle()
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button(L("common.done")) {
                        Task {
                            await commitText()
                            dismiss()
                        }
                    }
                    .fontWeight(.semibold)
                    .accessibilityIdentifier("item-done")
                }
            }
            .navigationDestination(for: Page.self) { page in
                ItemPageHost(loader: loader) { item in
                    switch page {
                    case .when: WhenPage(item: item, update: update, reload: load)
                    case .due: DuePage(item: item, update: update)
                    case .estimate: EstimatePage(item: item, update: update)
                    case .project: ProjectPickPage(item: item, update: update)
                    }
                }
            }
        }
        .presentationDetents([.large])
        .presentationDragIndicator(.visible)
        .task(id: model.dataVersion) { await load() }
        .onChange(of: focus) { old, _ in
            if old != nil { Task { await commitText() } }
        }
    }

    private func load() async {
        let id = id
        await loader.load(model) { try await $0.item(id) }
        if let item = loader.value, focus == nil {
            title = item.title
            notes = item.notes
        }
    }

    /// Sends one change. A `400` is shown under the title, in the server's words.
    private func update(_ patch: ItemPatch) async {
        let id = id
        switch await model.write(inline: true, { try await $0.updateItem(id, patch) }) {
        case .success(let item):
            loader.value = item
            error = nil
        case .failure(let failure):
            error = failure.status == 400 ? failure.message : nil
        }
    }

    private func commitText() async {
        guard let item = loader.value else { return }
        var patch = ItemPatch()
        let newTitle = title.trimmingCharacters(in: .whitespacesAndNewlines)
        if newTitle.isEmpty { title = item.title } else if newTitle != item.title { patch.title = .set(newTitle) }
        if notes != item.notes { patch.notes = .set(notes) }
        guard patch.title != .unchanged || patch.notes != .unchanged else { return }
        await update(patch)
    }

    @ViewBuilder private func content(_ item: Item) -> some View {
        let project = model.project(item.projectId)
        HStack(alignment: .top, spacing: 1) {
            Checkbox(on: item.isDone, label: L("item.complete", item.title)) {
                await update(ItemPatch(status: .set(item.isDone ? .open : .done)))
            }
            .padding(.leading, -11)
            .padding(.top, -7)
            VStack(alignment: .leading, spacing: 4) {
                TextField(L("item.title"), text: $title, axis: .vertical)
                    .font(.system(size: 22, weight: .bold))
                    .focused($focus, equals: .title)
                    .accessibilityIdentifier("item-title")
                TextField(L("item.notes"), text: $notes, axis: .vertical)
                    .font(.system(size: 15))
                    .foregroundStyle(Palette.label2)
                    .focused($focus, equals: .notes)
                    .accessibilityIdentifier("item-notes")
            }
        }
        .padding(.top, 6)
        .padding(.horizontal, 4)

        if let error {
            Text(error).font(.system(size: 13)).foregroundStyle(Palette.red).padding(.top, 8).padding(.horizontal, 4)
        }

        if let suggestion = item.suggestion {
            SuggestionCard(
                title: L("suggest.scheduleTitle", Format.rangeLabel(start: suggestion.start, end: suggestion.end, zone: model.zone, today: model.today)),
                reason: suggestion.reason, actor: suggestion.actor.name, id: suggestion.id
            )
            .padding(.top, 14)
        }

        GroupCard {
            NavigationLink(value: Page.when) {
                GroupRow(title: L("field.time"), value: whenText(item), minHeight: 46) { rowIcon("calendar") }
            }
            .accessibilityIdentifier("row-when")
            NavigationLink(value: Page.due) {
                let due = dueText(item)
                GroupRow(title: L("field.due"), value: due?.text ?? L("common.none"), valueColor: due?.hot == true ? Palette.red : Palette.label2, minHeight: 46) { rowIcon("flag") }
            }
            .accessibilityIdentifier("row-due")
            NavigationLink(value: Page.estimate) {
                GroupRow(title: L("field.estimate"), value: estimateText(item), minHeight: 46) { rowIcon("clock") }
            }
            .accessibilityIdentifier("row-estimate")
            NavigationLink(value: Page.project) {
                GroupRow(title: L("field.project"), minHeight: 46) { rowIcon("circle.circle") } accessory: {
                    HStack(spacing: 5) {
                        if let project { ColorDot(color: Palette.project(project.color)) }
                        Text(project?.name ?? L("nav.inbox")).font(.system(size: 17)).foregroundStyle(Palette.label2).lineLimit(1)
                    }
                }
            }
            .accessibilityIdentifier("row-project")
            GroupRow(title: L("item.important"), chevron: false, last: true, minHeight: 46) { rowIcon("exclamationmark") } accessory: {
                Toggle(L("item.important"), isOn: Binding(
                    get: { item.important },
                    set: { value in Task { await update(ItemPatch(important: .set(value))) } }
                ))
                .labelsHidden()
                .tint(Palette.green)
                .accessibilityIdentifier("item-important")
            }
        }
        .buttonStyle(.plain)
        .padding(.top, 14)

        HStack(spacing: 10) {
            if !item.isDone {
                let running = model.focus?.session?.itemId == item.id && model.focus?.state != .idle
                AsyncButton {
                    if !running {
                        await commitText()
                        guard await model.focusAction({ try await $0.startFocus(itemId: item.id) }) else { return }
                    }
                    router.timerExpanded = true
                    dismiss()
                } label: {
                    Label(running ? L("focus.back") : L("focus.start"), systemImage: "play.fill").labelStyle(.titleAndIcon)
                }
                .buttonStyle(CapsuleButtonStyle(primary: true))
                .accessibilityIdentifier("item-focus")
            }
            Menu {
                Button(L("common.delete"), systemImage: "trash", role: .destructive) { confirmingDelete = true }
            } label: {
                Image(systemName: "ellipsis").font(.system(size: 20, weight: .semibold)).foregroundStyle(Palette.label)
                    .frame(width: 50, height: 50)
                    .background(Palette.fill, in: Circle())
            }
            .accessibilityLabel(L("common.more"))
            .accessibilityIdentifier("item-more")
            if item.isDone { Spacer() }
        }
        .padding(.top, 16)
        .confirmationDialog(L("item.deleteAsk"), isPresented: $confirmingDelete, titleVisibility: .visible) {
            Button(L("common.delete"), role: .destructive) {
                Task {
                    let id = id
                    if case .success = await model.write({ try await $0.deleteItem(id) }) { dismiss() }
                }
            }
            Button(L("common.cancel"), role: .cancel) {}
        }
    }

    private func rowIcon(_ symbol: String) -> some View {
        Image(systemName: symbol).font(.system(size: 17)).foregroundStyle(Palette.label2)
    }

    private func whenText(_ item: Item) -> String {
        if let suggestion = item.suggestion { return L("item.pending", Format.whenLabel(suggestion.start, zone: model.zone, today: model.today)) }
        if let block = item.block { return Format.slotLabel(block.start, zone: model.zone, today: model.today) }
        if let planned = item.plannedDate {
            return item.evening && planned == model.today ? L("when.tonight") : Format.dayLabel(planned, today: model.today)
        }
        return L("when.none")
    }

    private func dueText(_ item: Item) -> (text: String, hot: Bool)? {
        guard let due = item.dueDate else { return nil }
        let days = model.today.days(until: due)
        if days < 0 { return (L("item.overdue", -days), true) }
        return (Format.dueLabel(date: due, time: item.dueTime, today: model.today), days <= 1)
    }

    private func estimateText(_ item: Item) -> String {
        var parts: [String] = []
        if let estimate = item.estimateMinutes { parts.append(Format.dur(estimate)) }
        if item.focus.minutes > 0 { parts.append(L("focus.used", Format.span(item.focus.minutes))) }
        return parts.isEmpty ? L("common.none") : parts.joined(separator: " · ")
    }
}

/// What an agent proposes, with its reason, and the two answers.
struct SuggestionCard: View {
    @Environment(AppModel.self) private var model
    let title: String
    let reason: String
    let actor: String
    let id: String
    var onDecided: () -> Void = {}

    var body: some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(title).font(.system(size: 15, weight: .semibold))
            if !reason.isEmpty { Text(reason).font(.system(size: 13)).foregroundStyle(Palette.label2) }
            Text(actor).font(.system(size: 13)).foregroundStyle(Palette.label2)
            HStack(spacing: 8) {
                AsyncButton(L("suggest.reject")) {
                    if case .success = await model.write({ try await $0.rejectSuggestion(id) }) { onDecided() }
                }
                .buttonStyle(CapsuleButtonStyle(height: 36))
                .accessibilityIdentifier("suggestion-reject")
                AsyncButton(L("suggest.accept")) {
                    if case .success = await model.write({ try await $0.acceptSuggestion(id) }) { onDecided() }
                }
                .buttonStyle(CapsuleButtonStyle(primary: true, height: 36))
                .accessibilityIdentifier("suggestion-accept")
            }
            .padding(.top, 8)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Palette.cell, in: RoundedRectangle(cornerRadius: 14, style: .continuous))
        .overlay(RoundedRectangle(cornerRadius: 14, style: .continuous).strokeBorder(Palette.label3, style: StrokeStyle(lineWidth: 1, dash: [4, 3])))
    }
}

// MARK: - The pages behind the rows

/// Keeps a pushed page showing the item as it is now, not as it was when the page was pushed.
private struct ItemPageHost<Content: View>: View {
    let loader: Loader<Item>
    @ViewBuilder let content: (Item) -> Content

    var body: some View {
        if let item = loader.value { content(item) }
    }
}

/// 时间: the day the item is meant for, or a time block on the calendar.
struct WhenPage: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let item: Item
    let update: (ItemPatch) async -> Void
    let reload: () async -> Void

    @State private var day: Day?
    @State private var startMinutes = 9 * 60
    @State private var duration = 30
    @State private var slots: [FreeSlot] = []
    @State private var busy = false

    private var chosenDay: Day { day ?? model.today }

    var body: some View {
        Form {
            Section(L("when.dateOnly")) {
                planButton(L("day.today"), model.today, evening: false)
                planButton(L("when.tonight"), model.today, evening: true)
                planButton(L("day.tomorrow"), model.today.adding(days: 1), evening: false)
            }
            Section {
                DatePicker(L("field.date"), selection: dayBinding, displayedComponents: .date)
                DatePicker(L("field.start"), selection: timeBinding, displayedComponents: .hourAndMinute)
                Picker(L("field.duration"), selection: $duration) {
                    ForEach(durations, id: \.self) { Text(Format.dur($0)).tag($0) }
                }
                AsyncButton(L("when.schedule")) {
                    await schedule(model.zone.date(on: chosenDay, minutes: startMinutes))
                }
                .disabled(busy)
                .accessibilityIdentifier("when-schedule")
                AsyncButton(L("when.planDay", Format.dayLabel(chosenDay, today: model.today))) {
                    await update(ItemPatch(plannedDate: .set(chosenDay), evening: .set(false)))
                    dismiss()
                }
            } header: {
                Text(L("when.schedule"))
            }
            if !slots.isEmpty {
                Section(L("when.free")) {
                    ForEach(slots, id: \.self) { slot in
                        AsyncButton {
                            await schedule(slot.start)
                        } label: {
                            Text("\(model.zone.hhmm(slot.start))–\(model.zone.hhmm(slot.end))").foregroundStyle(Palette.label)
                        }
                    }
                }
            }
            if item.block != nil || item.plannedDate != nil {
                Section {
                    AsyncButton(L("when.clear"), role: .destructive) {
                        if item.block != nil {
                            let id = item.id
                            _ = await model.write { try await $0.unscheduleItem(id) }
                        }
                        await update(ItemPatch(plannedDate: .clear, evening: .set(false)))
                        dismiss()
                    }
                    .accessibilityIdentifier("when-clear")
                }
            }
        }
        .navigationTitle(L("when.title"))
        .navigationBarTitleDisplayMode(.inline)
        .onAppear {
            if let block = item.block {
                day = model.zone.day(of: block.start)
                startMinutes = model.zone.minutes(of: block.start)
                duration = max(5, Int(block.end.timeIntervalSince(block.start) / 60))
            } else {
                day = item.plannedDate.map { max($0, model.today) } ?? model.today
                duration = item.estimateMinutes ?? 30
                // The next half hour, for a block that starts today.
                let now = model.zone.minutes(of: model.serverNow)
                startMinutes = min(23 * 60, (now / 30 + 1) * 30)
            }
        }
        .task(id: SlotKey(day: chosenDay, duration: duration)) {
            let day = chosenDay, duration = duration
            slots = (try? await model.api.freeSlots(on: day, minutes: duration)) ?? []
        }
    }

    private struct SlotKey: Equatable {
        let day: Day
        let duration: Int
    }

    private var durations: [Int] {
        Array(Set(Estimates.presets + [duration])).sorted()
    }

    private var dayBinding: Binding<Date> {
        Binding(get: { model.zone.date(on: chosenDay, minutes: 12 * 60) }, set: { day = model.zone.day(of: $0) })
    }

    private var timeBinding: Binding<Date> {
        Binding(get: { model.zone.date(on: chosenDay, minutes: startMinutes) }, set: { startMinutes = model.zone.minutes(of: $0) })
    }

    private func planButton(_ title: String, _ day: Day, evening: Bool) -> some View {
        AsyncButton {
            await update(ItemPatch(plannedDate: .set(day), evening: .set(evening)))
            dismiss()
        } label: {
            HStack {
                Text(title).foregroundStyle(Palette.label)
                Spacer()
                if item.block == nil, item.plannedDate == day, item.evening == evening {
                    Image(systemName: "checkmark").fontWeight(.semibold)
                }
            }
        }
    }

    private func schedule(_ start: Date) async {
        busy = true
        defer { busy = false }
        let id = item.id
        // Calendar arithmetic: a block that crosses a clock change still lasts its minutes.
        let end = model.zone.calendar.date(byAdding: .minute, value: duration, to: start) ?? start
        if case .success = await model.write({ try await $0.scheduleItem(id, start: start, end: end) }) {
            await reload()
            dismiss()
        }
    }
}

/// 截止: a date, and optionally a time.
struct DuePage: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let item: Item
    let update: (ItemPatch) async -> Void

    var body: some View {
        Form {
            Section {
                quick(L("day.today"), model.today)
                quick(L("day.tomorrow"), model.today.adding(days: 1))
            }
            Section {
                DatePicker(L("due.date"), selection: Binding(
                    get: { model.zone.date(on: item.dueDate ?? model.today, minutes: 12 * 60) },
                    set: { date in Task { await update(ItemPatch(dueDate: .set(model.zone.day(of: date)))) } }
                ), displayedComponents: .date)
                .datePickerStyle(.graphical)
            }
            if let due = item.dueDate {
                Section {
                    Toggle(L("due.withTime"), isOn: Binding(
                        get: { item.dueTime != nil },
                        set: { on in Task { await update(ItemPatch(dueTime: on ? .set(model.user?.workEnd ?? "18:00") : .clear)) } }
                    ))
                    if let time = item.dueTime, let minutes = Zone.minutes(ofClock: time) {
                        DatePicker(L("due.title"), selection: Binding(
                            get: { model.zone.date(on: due, minutes: minutes) },
                            set: { date in Task { await update(ItemPatch(dueTime: .set(model.zone.hhmm(date)))) } }
                        ), displayedComponents: .hourAndMinute)
                    }
                }
                Section {
                    AsyncButton(L("common.clear"), role: .destructive) {
                        await update(ItemPatch(dueDate: .clear, dueTime: .clear))
                        dismiss()
                    }
                    .accessibilityIdentifier("due-clear")
                }
            }
        }
        .navigationTitle(L("due.title"))
        .navigationBarTitleDisplayMode(.inline)
    }

    private func quick(_ title: String, _ day: Day) -> some View {
        AsyncButton {
            await update(ItemPatch(dueDate: .set(day)))
            dismiss()
        } label: {
            HStack {
                Text(title).foregroundStyle(Palette.label)
                Spacer()
                if item.dueDate == day { Image(systemName: "checkmark").fontWeight(.semibold) }
            }
        }
        .accessibilityIdentifier("due-\(title)")
    }
}

/// 预计: how long the item should take.
struct EstimatePage: View {
    @Environment(\.dismiss) private var dismiss
    let item: Item
    let update: (ItemPatch) async -> Void
    @State private var custom = ""

    var body: some View {
        Form {
            Section {
                ForEach(Estimates.presets, id: \.self) { minutes in
                    AsyncButton {
                        await update(ItemPatch(estimateMinutes: .set(minutes)))
                        dismiss()
                    } label: {
                        HStack {
                            Text(Format.dur(minutes)).foregroundStyle(Palette.label)
                            Spacer()
                            if item.estimateMinutes == minutes { Image(systemName: "checkmark").fontWeight(.semibold) }
                        }
                    }
                    .accessibilityIdentifier("estimate-\(minutes)")
                }
            }
            Section {
                HStack {
                    TextField(L("estimate.custom"), text: $custom).keyboardType(.numberPad)
                    AsyncButton(L("common.ok")) {
                        guard let minutes = Int(custom), minutes > 0 else { return }
                        await update(ItemPatch(estimateMinutes: .set(minutes)))
                        dismiss()
                    }
                    .disabled(Int(custom) == nil)
                }
            }
            if item.estimateMinutes != nil {
                Section {
                    AsyncButton(L("common.clear"), role: .destructive) {
                        await update(ItemPatch(estimateMinutes: .clear))
                        dismiss()
                    }
                }
            }
        }
        .navigationTitle(L("estimate.title"))
        .navigationBarTitleDisplayMode(.inline)
    }
}

/// 项目: which project the item belongs to; none is the inbox.
struct ProjectPickPage: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let item: Item
    let update: (ItemPatch) async -> Void

    var body: some View {
        Form {
            row(nil, L("nav.inbox"))
            ForEach(model.projects) { row($0, $0.name) }
        }
        .navigationTitle(L("project.pick"))
        .navigationBarTitleDisplayMode(.inline)
    }

    private func row(_ project: Project?, _ name: String) -> some View {
        AsyncButton {
            // A heading belongs to a project, so moving the item leaves its heading behind.
            await update(ItemPatch(projectId: Patch(project?.id), headingId: .clear))
            dismiss()
        } label: {
            HStack(spacing: 8) {
                if let project { ColorDot(color: Palette.project(project.color), size: 10) } else { Image(systemName: "tray").foregroundStyle(Palette.label2) }
                Text(name).foregroundStyle(Palette.label)
                Spacer()
                if item.projectId == project?.id { Image(systemName: "checkmark").fontWeight(.semibold) }
            }
        }
        .accessibilityIdentifier("pick-project-\(name)")
    }
}
