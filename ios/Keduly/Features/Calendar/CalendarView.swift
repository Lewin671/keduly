import SwiftUI

/// The calendar tab: a month grid over the selected day's agenda, or a week strip over one day's time axis.
struct CalendarView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var loader = Loader<[Event]>()

    private var day: Day { router.calendarDay ?? model.today }

    /// The days whose events are on screen: the weeks of the month, or the week of the day.
    private var range: ClosedRange<Day> {
        switch router.calendarMode {
        case .month: return day.startOfMonth.startOfWeek...day.endOfMonth.startOfWeek.adding(days: 6)
        case .day: return day.startOfWeek...day.startOfWeek.adding(days: 6)
        }
    }

    private struct LoadKey: Equatable {
        let range: ClosedRange<Day>
        let version: Int
    }

    var body: some View {
        @Bindable var router = router
        let index = CalendarIndex(events: loader.value ?? [], zone: model.zone)
        VStack(alignment: .leading, spacing: 0) {
            VStack(alignment: .leading, spacing: 0) {
                if loader.failed { ErrorLine(retry: load) }
                LargeTitle(L("date.m", day.month), detail: L("date.y", day.year))
                Picker(L("view.label"), selection: $router.calendarMode) {
                    Text(L("view.day")).tag(CalendarMode.day)
                    Text(L("view.month")).tag(CalendarMode.month)
                }
                .pickerStyle(.segmented)
                .padding(.top, 4)
                .padding(.bottom, 12)
                .accessibilityIdentifier("calendar-mode")
            }
            .padding(.horizontal, Metrics.pageMargin)

            switch router.calendarMode {
            case .month:
                MonthView(day: day, index: index)
            case .day:
                DayView(day: day, index: index)
            }
        }
        .background(Palette.background)
        .navigationTitle(L("mode.cal"))
        .inlineTitle()
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                Button(L("day.today")) { withAnimation { router.calendarDay = model.today } }
                    .foregroundStyle(Palette.red)
                    .accessibilityIdentifier("calendar-today")
                BellButton()
            }
        }
        .task(id: LoadKey(range: range, version: model.dataVersion)) { await load() }
    }

    private func load() async {
        let from = model.zone.start(of: range.lowerBound)
        let to = model.zone.start(of: range.upperBound.adding(days: 1))
        await loader.load(model) { try await $0.events(from: from, to: to) }
    }
}

/// The loaded events, by the day they fall on in the account's zone.
struct CalendarIndex {
    let zone: Zone
    private let byDay: [Day: [Event]]

    init(events: [Event], zone: Zone) {
        self.zone = zone
        var byDay: [Day: [Event]] = [:]
        for event in events {
            let days: ClosedRange<Day>
            if event.allDay, let first = event.startDate, let last = event.endDate {
                // `end_date` is inclusive.
                days = first...max(first, last)
            } else if let start = event.start, let end = event.end {
                days = zone.days(from: start, to: end)
            } else {
                continue
            }
            var day = days.lowerBound
            while day <= days.upperBound {
                byDay[day, default: []].append(event)
                day = day.adding(days: 1)
            }
        }
        self.byDay = byDay
    }

    /// A day's entries: all-day ones first, then by start.
    func entries(on day: Day) -> [Event] {
        (byDay[day] ?? []).sorted { a, b in
            if a.allDay != b.allDay { return a.allDay }
            return (a.start ?? .distantPast, a.title) < (b.start ?? .distantPast, b.title)
        }
    }
}

// MARK: - Month

struct MonthView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let day: Day
    let index: CalendarIndex

    var body: some View {
        let entries = index.entries(on: day)
        ScrollView {
            VStack(alignment: .leading, spacing: 0) {
                HStack(spacing: 0) {
                    ForEach(Format.weekdayLetters, id: \.self) { letter in
                        Text(letter).font(.app(11, .semibold)).foregroundStyle(Palette.label3).frame(maxWidth: .infinity)
                    }
                }
                .padding(.bottom, 4)
                grid
                    .contentShape(Rectangle())
                    .gesture(DragGesture(minimumDistance: 30).onEnded { value in
                        guard abs(value.translation.width) > abs(value.translation.height) else { return }
                        withAnimation { router.calendarDay = day.adding(months: value.translation.width < 0 ? 1 : -1) }
                    })
            }
            .padding(.horizontal, Metrics.pageMargin - 8)
            .dynamicTypeSize(...AppType.fixedLayoutLimit)

            Divider().padding(.top, 6)
            HStack(alignment: .firstTextBaseline, spacing: 8) {
                Text(L("date.md", day.month, day.day)).font(.app(17, .semibold))
                Text(Format.weekdayLong(day)).font(.app(15)).foregroundStyle(Palette.label2)
                Spacer()
                if !entries.isEmpty { Text(L("cal.count", entries.count)).font(.app(13)).foregroundStyle(Palette.label2) }
            }
            .padding(.horizontal, Metrics.pageMargin)
            .padding(.top, 11)
            .padding(.bottom, 9)

            VStack(alignment: .leading, spacing: 0) {
                if entries.isEmpty {
                    Text(L("cal.none")).font(.app(15)).foregroundStyle(Palette.label2).padding(.vertical, 14)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                ForEach(Array(entries.enumerated()), id: \.element.entryKey) { offset, event in
                    AgendaRow(event: event, day: day, last: offset == entries.count - 1)
                }
            }
            .padding(.horizontal, Metrics.pageMargin)
            .padding(.bottom, 24)
        }
    }

    private var grid: some View {
        let first = day.startOfMonth
        let cells: [Day?] = Array(repeating: nil, count: first.weekdayIndex) + (0..<day.daysInMonth).map { first.adding(days: $0) }
        return LazyVGrid(columns: Array(repeating: GridItem(.flexible(), spacing: 0), count: 7), spacing: 0) {
            ForEach(Array(cells.enumerated()), id: \.offset) { _, cell in
                if let cell {
                    Button { router.calendarDay = cell } label: { dayCell(cell) }
                        .buttonStyle(.plain)
                        .accessibilityLabel(L("date.md", cell.month, cell.day))
                        .accessibilityAddTraits(cell == day ? .isSelected : [])
                        .accessibilityIdentifier("day-\(cell.day)")
                } else {
                    Color.clear.frame(height: 50)
                }
            }
        }
    }

    private func dayCell(_ cell: Day) -> some View {
        let today = cell == model.today, selected = cell == day
        // Up to three dots in project colours; a hollow one is tentative. An event about to move away has none.
        let dots = index.entries(on: cell).filter { $0.status != .leaving }.prefix(3)
        return VStack(spacing: 3) {
            Text(String(cell.day))
                .font(.system(size: 19, weight: today || selected ? .semibold : .regular))
                .foregroundStyle(selected ? (today ? Color.white : Palette.background) : today ? Palette.red : cell.weekdayIndex >= 5 ? Palette.label2 : Palette.label)
                .frame(width: 34, height: 34)
                .background(selected ? (today ? Palette.red : Palette.label) : Color.clear, in: Circle())
            HStack(spacing: 3) {
                ForEach(Array(dots.enumerated()), id: \.offset) { _, event in
                    let color = Palette.project(model.project(event.projectId)?.color)
                    if event.status == .tentative {
                        Circle().strokeBorder(color, lineWidth: 1.3).frame(width: 6, height: 6)
                    } else {
                        Circle().fill(color).frame(width: 6, height: 6)
                    }
                }
            }
            .frame(height: 6)
        }
        .padding(.top, 3)
        .frame(maxWidth: .infinity, minHeight: 50, alignment: .top)
        .contentShape(Rectangle())
    }
}

/// One entry in a day's list: start and end, a colour bar, the title and where it belongs.
struct AgendaRow: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let event: Event
    let day: Day
    var last = false

    var body: some View {
        let project = model.project(event.projectId)
        let color = Palette.project(project?.color)
        Button { router.open(event) } label: {
            HStack(alignment: .top, spacing: 11) {
                VStack(alignment: .leading, spacing: 0) {
                    if event.allDay {
                        Text(L("cal.allDay")).foregroundStyle(Palette.label)
                    } else if let start = event.start, let end = event.end {
                        Text(model.zone.day(of: start) < day ? Zone.hhmm(0) : model.zone.hhmm(start)).foregroundStyle(Palette.label)
                        Text(model.zone.hhmm(end)).foregroundStyle(Palette.label2)
                    }
                }
                .font(.app(13))
                .monospacedDigit()
                .fixedSize()
                .frame(minWidth: 43, alignment: .leading)
                .padding(.top, 7)
                Group {
                    if event.status == .tentative { DashedBar(color: color) } else { RoundedRectangle(cornerRadius: 2).fill(color) }
                }
                .frame(width: 4)
                .padding(.vertical, 7)
                VStack(alignment: .leading, spacing: 0) {
                    Text(event.title).font(.app(17)).foregroundStyle(Palette.label).lineLimit(1).strikethrough(event.status == .leaving)
                    Text(detail(project)).font(.app(13)).foregroundStyle(Palette.label2).lineLimit(1)
                }
                .padding(.top, 6)
                .padding(.bottom, 7)
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .bottom) { if !last { Divider() } }
            }
            .frame(minHeight: 50)
            .opacity(event.status == .leaving ? 0.5 : 1)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("agenda-\(event.title)")
    }

    private func detail(_ project: Project?) -> String {
        var parts = [project?.name ?? L("project.none")]
        if event.allDay, let first = event.startDate, let last = event.endDate, first != last {
            parts.append("\(L("date.md", first.month, first.day))–\(first.month == last.month ? L("date.d", last.day) : L("date.md", last.month, last.day))")
        }
        if !event.location.isEmpty { parts.append(event.location) }
        if event.status == .tentative { parts.append(L("cal.pending")) }
        if event.status == .leaving { parts.append(L("cal.leaving")) }
        if let end = event.end, model.zone.day(of: end.addingTimeInterval(-1)) > day {
            parts.append(L("cal.untilNext", model.zone.hhmm(end)))
        }
        return parts.joined(separator: " · ")
    }
}

/// The entries folded into a "+N" block of the day view.
struct EntriesSheet: View {
    let events: [Event]
    let day: Day

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 0) {
                    ForEach(Array(events.enumerated()), id: \.element.entryKey) { offset, event in
                        AgendaRow(event: event, day: day, last: offset == events.count - 1)
                    }
                }
                .padding(.horizontal, Metrics.pageMargin)
            }
            .navigationTitle(L("cal.count", events.count))
            .navigationBarTitleDisplayMode(.inline)
        }
        .presentationDetents([.medium, .large])
    }
}
