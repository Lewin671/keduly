import SwiftUI

/// A new event, or an existing one to change or delete. Instances of a recurring event are read-only.
struct EventSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    /// The event to change, or nil for a new one on `day`.
    let event: Event?
    let day: Day

    @State private var title = ""
    @State private var allDay = false
    @State private var start = Date()
    @State private var end = Date()
    @State private var startDay = Day(year: 2000, month: 1, day: 1)
    @State private var endDay = Day(year: 2000, month: 1, day: 1)
    @State private var projectId: String?
    @State private var location = ""
    @State private var notes = ""
    @State private var error: String?
    @State private var busy = false
    @State private var confirmingDelete = false
    @State private var ready = false

    private var readonly: Bool { event?.readonly == true }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField(L("event.title"), text: $title).accessibilityIdentifier("event-title")
                    TextField(L("event.location"), text: $location)
                } footer: {
                    if let error { Text(error).foregroundStyle(Palette.red) } else if readonly { Text(L("event.recurringNote")) }
                }
                Section {
                    Toggle(L("cal.allDay"), isOn: $allDay).accessibilityIdentifier("event-allday")
                    if allDay {
                        DatePicker(L("field.startDate"), selection: dayBinding($startDay), displayedComponents: .date)
                        DatePicker(L("field.endDate"), selection: dayBinding($endDay), in: model.zone.date(on: startDay, minutes: 0)..., displayedComponents: .date)
                    } else {
                        DatePicker(L("field.start"), selection: $start)
                        DatePicker(L("field.end"), selection: $end, in: start...)
                    }
                }
                Section {
                    Picker(L("field.project"), selection: $projectId) {
                        Text(L("project.none")).tag(String?.none)
                        ForEach(model.projects) { Text($0.name).tag(String?.some($0.id)) }
                    }
                    TextField(L("item.notes"), text: $notes, axis: .vertical).lineLimit(1...6)
                }
                if event != nil, !readonly {
                    Section {
                        Button(L("event.delete"), role: .destructive) { confirmingDelete = true }
                            .accessibilityIdentifier("event-delete")
                    }
                }
            }
            .disabled(readonly)
            .navigationTitle(event == nil ? L("event.new") : L("event.edit"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button(readonly ? L("common.done") : L("common.cancel")) { dismiss() } }
                if !readonly {
                    ToolbarItem(placement: .confirmationAction) {
                        Button(event == nil ? L("common.add") : L("common.save")) { Task { await save() } }
                            .disabled(busy || title.trimmingCharacters(in: .whitespaces).isEmpty)
                            .accessibilityIdentifier("event-save")
                    }
                }
            }
            .confirmationDialog(L("event.deleteAsk"), isPresented: $confirmingDelete, titleVisibility: .visible) {
                Button(L("common.delete"), role: .destructive) {
                    Task {
                        guard let id = event?.id else { return }
                        if case .success = await model.write({ try await $0.deleteEvent(id) }) { dismiss() }
                    }
                }
                Button(L("common.cancel"), role: .cancel) {}
            }
        }
        .presentationDetents([.large])
        .onAppear(perform: prepare)
        .onChange(of: start) { old, new in
            // Moving the start keeps the length.
            guard ready else { return }
            end = new.addingTimeInterval(max(300, end.timeIntervalSince(old)))
        }
        .onChange(of: startDay) { old, new in
            guard ready else { return }
            endDay = new.adding(days: max(0, old.days(until: endDay)))
        }
    }

    private func dayBinding(_ day: Binding<Day>) -> Binding<Date> {
        Binding(get: { model.zone.date(on: day.wrappedValue, minutes: 12 * 60) }, set: { day.wrappedValue = model.zone.day(of: $0) })
    }

    private func prepare() {
        guard !ready else { return }
        if let event {
            title = event.title
            allDay = event.allDay
            projectId = event.projectId
            location = event.location
            notes = event.notes
        }
        if let event, let s = event.start, let e = event.end {
            start = s
            end = e
            startDay = model.zone.day(of: s)
            endDay = startDay
        } else if let event, let first = event.startDate, let last = event.endDate {
            startDay = first
            endDay = last
            start = model.zone.date(on: first, minutes: 9 * 60)
            end = model.zone.date(on: first, minutes: 10 * 60)
        } else {
            // A new event: the next full hour on the selected day, one hour long.
            let now = model.zone.minutes(of: model.serverNow)
            let minutes = day == model.today ? min(23 * 60, (now / 60 + 1) * 60) : 9 * 60
            start = model.zone.date(on: day, minutes: minutes)
            end = model.zone.calendar.date(byAdding: .hour, value: 1, to: start) ?? start
            startDay = day
            endDay = day
        }
        // Let the values settle before changes to the start begin to move the end.
        DispatchQueue.main.async { ready = true }
    }

    private func save() async {
        busy = true
        defer { busy = false }
        let name = title.trimmingCharacters(in: .whitespaces)
        var patch = EventPatch()
        if let event {
            if name != event.title { patch.title = .set(name) }
            if projectId != event.projectId { patch.projectId = Patch(projectId) }
            if location != event.location { patch.location = .set(location) }
            if notes != event.notes { patch.notes = .set(notes) }
            if allDay != event.allDay { patch.allDay = .set(allDay) }
            if allDay {
                if allDay != event.allDay || startDay != event.startDate || endDay != event.endDate {
                    patch.startDate = .set(startDay)
                    patch.endDate = .set(endDay)
                }
            } else if allDay != event.allDay || start != event.start || end != event.end {
                patch.start = .set(start)
                patch.end = .set(end)
            }
        } else {
            patch.title = .set(name)
            if let projectId { patch.projectId = .set(projectId) }
            if !location.isEmpty { patch.location = .set(location) }
            if !notes.isEmpty { patch.notes = .set(notes) }
            if allDay {
                patch.allDay = .set(true)
                patch.startDate = .set(startDay)
                patch.endDate = .set(endDay)
            } else {
                patch.start = .set(start)
                patch.end = .set(end)
            }
        }
        let body = patch, id = event?.id
        let result = await model.write(inline: true) { api in
            if let id { return try await api.updateEvent(id, body) }
            return try await api.createEvent(body)
        }
        switch result {
        case .success: dismiss()
        case .failure(let failure): error = failure.status == 400 ? failure.message : nil
        }
    }
}

/// A tentative or leaving entry of the calendar: what the agent proposes, why, and the two answers.
struct SuggestionSheet: View {
    @Environment(AppModel.self) private var model
    @Environment(\.dismiss) private var dismiss
    let id: String
    @State private var loader = Loader<[Suggestion]>()

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 12) {
                    if loader.failed { ErrorLine(retry: load) }
                    if let suggestions = loader.value {
                        if let suggestion = suggestions.first(where: { $0.id == id }) {
                            Text(suggestion.title).font(.system(size: 22, weight: .bold))
                            SuggestionCard(title: SuggestionText.what(suggestion, zone: model.zone, today: model.today), reason: suggestion.reason,
                                           actor: "\(suggestion.actor.name) · \(Format.stamp(suggestion.createdAt, zone: model.zone, today: model.today))",
                                           id: suggestion.id) { dismiss() }
                        } else {
                            Blank(title: L("error.not_found"), text: "", top: 40)
                        }
                    } else if !loader.failed {
                        ProgressView().frame(maxWidth: .infinity).padding(.top, 40)
                    }
                }
                .padding(.horizontal, 16)
                .padding(.top, 8)
            }
            .background(Palette.grouped)
            .navigationTitle(L("suggest.title"))
            .navigationBarTitleDisplayMode(.inline)
            .toolbar { ToolbarItem(placement: .cancellationAction) { Button(L("common.close")) { dismiss() } } }
        }
        .presentationDetents([.medium, .large])
        .task { await load() }
    }

    private func load() async {
        await loader.load(model) { try await $0.pendingSuggestions() }
    }
}

enum SuggestionText {
    /// What a suggestion would do, in a few words.
    static func what(_ s: Suggestion, zone: Zone, today: Day) -> String {
        func slot(_ start: Date?, _ end: Date?) -> String {
            guard let start, let end else { return "" }
            return Format.rangeLabel(start: start, end: end, zone: zone, today: today)
        }
        switch s.kind {
        case .scheduleItem: return L("suggest.scheduleTitle", slot(s.start, s.end))
        case .createItem: return s.start == nil ? L("suggest.createItem") : L("suggest.createItemAt", slot(s.start, s.end))
        case .createEvent:
            if let event = s.event, event.allDay == true, let day = event.startDate {
                return L("suggest.createEvent", Format.dayLabel(day, today: today))
            }
            return L("suggest.createEvent", slot(s.event?.start ?? s.start, s.event?.end ?? s.end))
        case .moveEvent: return L("suggest.move", slot(s.start, s.end))
        case .deleteEvent: return L("suggest.deleteEvent")
        case .deleteItem: return L("suggest.deleteItem")
        }
    }
}
