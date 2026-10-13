import SwiftUI

/// A strip of the week's seven days over one day's time axis.
struct DayView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let day: Day
    let index: CalendarIndex

    static let hourHeight: CGFloat = 58
    /// The height a short event keeps so that its title stays readable.
    static let minHeight: CGFloat = 24

    var body: some View {
        let entries = index.entries(on: day)
        VStack(spacing: 0) {
            strip
                .padding(.horizontal, Metrics.pageMargin - 8)
                .contentShape(Rectangle())
                .gesture(DragGesture(minimumDistance: 30).onEnded { value in
                    guard abs(value.translation.width) > abs(value.translation.height) else { return }
                    withAnimation { router.calendarDay = day.adding(days: value.translation.width < 0 ? 7 : -7) }
                })
            ForEach(entries.filter(\.allDay), id: \.entryKey) { event in
                allDayRow(event)
            }
            Divider()
            ScrollViewReader { proxy in
                ScrollView {
                    timeline(entries.filter { !$0.allDay })
                        .padding(.top, 10)
                        .padding(.trailing, 12)
                        .padding(.bottom, 24)
                }
                .onAppear { proxy.scrollTo("open", anchor: .top) }
            }
        }
    }

    // MARK: Week strip

    private var strip: some View {
        HStack(spacing: 0) {
            ForEach(0..<7, id: \.self) { offset in
                let cell = day.startOfWeek.adding(days: offset)
                let today = cell == model.today, selected = cell == day
                Button { router.calendarDay = cell } label: {
                    VStack(spacing: 3) {
                        Text(Format.weekdayLetters[offset]).font(.system(size: 11, weight: .semibold)).foregroundStyle(Palette.label3)
                        Text(String(cell.day))
                            .font(.system(size: 19, weight: today || selected ? .semibold : .regular))
                            .foregroundStyle(selected ? (today ? Color.white : Palette.background) : today ? Palette.red : Palette.label)
                            .frame(width: 34, height: 34)
                            .background(selected ? (today ? Palette.red : Palette.label) : Color.clear, in: Circle())
                    }
                    .frame(maxWidth: .infinity)
                    .padding(.bottom, 6)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityLabel("\(L("date.md", cell.month, cell.day)) \(Format.weekdayLong(cell))")
                .accessibilityAddTraits(selected ? .isSelected : [])
                .accessibilityIdentifier("strip-\(cell.day)")
            }
        }
    }

    private func allDayRow(_ event: Event) -> some View {
        let color = Palette.projectUI(model.project(event.projectId)?.color)
        return VStack(spacing: 0) {
            Divider()
            HStack(spacing: 6) {
                Text(L("cal.allDay")).font(.system(size: 11)).foregroundStyle(Palette.label2).frame(width: 44, alignment: .trailing)
                Button { router.open(event) } label: {
                    Text(event.title).font(.system(size: 13, weight: .semibold)).lineLimit(1)
                        .strikethrough(event.status == .leaving)
                        .foregroundStyle(Palette.ink(color, 0.62))
                        .padding(.horizontal, 9)
                        .frame(maxWidth: .infinity, minHeight: 24, alignment: .leading)
                        .background(Palette.wash(color, 0.2), in: RoundedRectangle(cornerRadius: 6, style: .continuous))
                }
                .buttonStyle(.plain)
            }
            .padding(.leading, 12)
            .padding(.trailing, Metrics.pageMargin)
            .padding(.vertical, 6)
        }
    }

    // MARK: Time axis

    private func timeline(_ events: [Event]) -> some View {
        let h = Self.hourHeight
        let spans: [(event: Event, start: Int, end: Int)] = events.compactMap { event in
            guard let start = event.start, let end = event.end, let span = index.zone.span(start: start, end: end, on: day) else { return nil }
            return (event, span.start, span.end)
        }
        let blocks = DayLayout.layout(spans.map { DayLayout.Span(start: $0.start, end: $0.end) }, minMinutes: Int((Self.minHeight / h * 60).rounded(.up)))
        return ZStack(alignment: .topLeading) {
            VStack(spacing: 0) {
                ForEach(0..<24, id: \.self) { hour in
                    ZStack(alignment: .topLeading) {
                        Rectangle().fill(Palette.separator).frame(height: 0.5).padding(.leading, 58)
                        if hour > 0 {
                            Text(Zone.hhmm(hour * 60)).font(.system(size: 11, weight: .medium)).foregroundStyle(Palette.label2)
                                .frame(width: 50, alignment: .trailing)
                                .offset(y: -7)
                        }
                    }
                    .frame(height: h, alignment: .top)
                    .id(hour)
                }
            }
            // Where the axis opens: just above 07:00, so that no hour label is cut in half.
            VStack(spacing: 0) {
                Color.clear.frame(height: 7 * h - 16)
                Color.clear.frame(height: 1).id("open")
            }
            .frame(width: 1)
            .accessibilityHidden(true)
            GeometryReader { geo in
                let width = geo.size.width - 60
                ForEach(Array(blocks.enumerated()), id: \.offset) { _, block in
                    let lane = width / CGFloat(block.lanes)
                    let height = CGFloat(block.end - block.start) / 60 * h - 2
                    Group {
                        switch block.content {
                        case .entry(let i):
                            EventBlock(event: spans[i].event, day: day, height: height, lanes: block.lanes)
                        case .more(let indices):
                            MoreBlock(count: indices.count, start: block.start, end: block.end) {
                                router.sheet = .entries(indices.map { spans[$0].event }, day)
                            }
                        }
                    }
                    .frame(width: lane - 2, height: height)
                    .offset(x: 60 + CGFloat(block.lane) * lane + 1, y: CGFloat(block.start) / 60 * h + 1)
                }
            }
            if day == model.today {
                TimelineView(.everyMinute) { _ in
                    let minutes = model.zone.minutes(of: model.serverNow)
                    NowLine(label: model.zone.hhmm(model.serverNow)).offset(y: CGFloat(minutes) / 60 * h - 9)
                }
            }
        }
        .frame(height: 24 * h)
    }
}

/// The current time: a red line with the time in a red capsule.
private struct NowLine: View {
    let label: String

    var body: some View {
        HStack(spacing: 0) {
            Text(label).font(.system(size: 11, weight: .semibold)).foregroundStyle(.white).monospacedDigit()
                .frame(width: 44, height: 18)
                .background(Palette.red, in: Capsule())
                .padding(.leading, 6)
            Circle().fill(Palette.red).frame(width: 9, height: 9).padding(.leading, 2)
            Rectangle().fill(Palette.red).frame(height: 1.5).padding(.leading, -5)
        }
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

/// An event or a time block on the time axis, tinted with its project's colour.
private struct EventBlock: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let event: Event
    let day: Day
    let height: CGFloat
    let lanes: Int

    var body: some View {
        let color = Palette.projectUI(model.project(event.projectId)?.color)
        let tentative = event.status == .tentative, leaving = event.status == .leaving
        let task = event.itemId != nil && !tentative
        let short = height < 36
        Button { router.open(event) } label: {
            Group {
                if short {
                    HStack(spacing: 6) {
                        titleText.lineLimit(1)
                        timeText.layoutPriority(-1)
                    }
                } else {
                    VStack(alignment: .leading, spacing: 0) {
                        titleText.lineLimit(height > 56 ? 2 : 1)
                        timeText
                    }
                    .padding(.top, 4)
                }
            }
            .padding(.leading, task ? 30 : 10)
            .padding(.trailing, 6)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: short ? .leading : .topLeading)
            .foregroundStyle(Palette.ink(color))
            .background(Palette.wash(color, tentative ? 0.05 : task ? 0.11 : 0.18))
            .overlay(alignment: .leading) {
                if !tentative && !task {
                    RoundedRectangle(cornerRadius: 2).fill(Color(uiColor: color)).frame(width: 3).padding(.vertical, 3).padding(.leading, 3)
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
            .overlay {
                if tentative {
                    RoundedRectangle(cornerRadius: 7, style: .continuous)
                        .strokeBorder(Palette.wash(color, 0.62), style: StrokeStyle(lineWidth: 1, dash: [4, 3]))
                }
            }
            .opacity(leaving ? 0.45 : 1)
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("block-\(event.title)")
        .overlay(alignment: short ? .leading : .topLeading) {
            if task, let item = event.itemId {
                let done = event.itemDone == true
                Checkbox(on: done, color: Color(uiColor: color), size: 15, label: L("item.complete", event.title)) {
                    _ = await model.write { try await $0.updateItem(item, ItemPatch(status: .set(done ? .open : .done))) }
                }
                .frame(width: 30, height: min(height, 30))
            }
        }
    }

    private var titleText: some View {
        Text(event.title)
            .font(.system(size: 13, weight: event.status == .tentative ? .medium : .semibold))
            .strikethrough(event.status == .leaving || event.itemDone == true)
            .multilineTextAlignment(.leading)
    }

    private var timeText: some View {
        Text(time).font(.system(size: 11.5)).opacity(0.8).lineLimit(1)
    }

    /// The whole range, or only the start where three lanes leave no room.
    private var time: String {
        guard let start = event.start, let end = event.end else { return "" }
        var text = model.zone.hhmm(start)
        if lanes <= 2 || height < 36 {
            let nextDay = model.zone.day(of: end.addingTimeInterval(-1)) > model.zone.day(of: start)
            text += "–\(nextDay ? L("cal.nextDay") : "")\(model.zone.hhmm(end))"
        }
        if event.status == .tentative { text += L("cal.pendingMark") }
        return text
    }
}

/// Everything from the third lane on, folded into one block.
private struct MoreBlock: View {
    let count: Int
    let start: Int
    let end: Int
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            VStack(alignment: .leading, spacing: 0) {
                Text(L("cal.plus", count)).font(.system(size: 13, weight: .semibold)).lineLimit(1)
                Text("\(Zone.hhmm(start))–\(Zone.hhmm(end))").font(.system(size: 11.5)).opacity(0.8).lineLimit(1).minimumScaleFactor(0.8)
            }
            .padding(.leading, 7)
            .padding(.trailing, 4)
            .padding(.top, 4)
            .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
            .foregroundStyle(Palette.label2)
            .background(Palette.fill, in: RoundedRectangle(cornerRadius: 7, style: .continuous))
            .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("block-more")
    }
}
