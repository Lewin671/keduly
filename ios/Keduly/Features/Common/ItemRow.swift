import SwiftUI

/// One item in a list: a title, one line of facts, and on the right only when it will be done.
struct ItemRow: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let source: Item
    /// Left out on the project page, where every row would repeat it.
    var showProject = true
    /// Left out where the page already says the day (Today, Upcoming).
    var showPlan = true
    var last = false

    @Environment(\.dynamicTypeSize) private var typeSize

    private var item: Item { model.current(source) }

    var body: some View {
        HStack(alignment: .top, spacing: 1) {
            Checkbox(on: item.isDone, color: Palette.tint, label: L("item.complete", item.title)) {
                await model.toggle(item)
            }
            .padding(.leading, -11)
            Button { router.sheet = .item(item.id) } label: {
                VStack(alignment: .leading, spacing: 2) {
                    if typeSize.isAccessibilitySize {
                        // Large type: the title gets the whole width, and when it will be done its own line.
                        title
                        when
                    } else {
                        HStack(alignment: .top, spacing: 10) {
                            title
                            Spacer(minLength: 0)
                            when
                        }
                    }
                    facts
                }
                .padding(.vertical, 10)
                .frame(maxWidth: .infinity, alignment: .leading)
                .overlay(alignment: .bottom) { if !last { Divider() } }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityIdentifier("item-\(item.title)")
        }
    }

    private var title: some View {
        (Text(item.important ? "! " : "").foregroundStyle(Palette.orange).fontWeight(.heavy)
            + Text(item.title).foregroundStyle(item.isDone ? Palette.label2 : Palette.label).strikethrough(item.isDone))
            .font(.app(17))
            .lineLimit(2)
            .multilineTextAlignment(.leading)
    }

    // MARK: Right side

    @ViewBuilder private var when: some View {
        if item.isDone {
            EmptyView()
        } else if let focus = model.focus, focus.state == .work, let session = focus.session, session.itemId == item.id {
            TimelineView(.periodic(from: .now, by: 1)) { _ in
                let now = model.serverNow
                HStack(spacing: 4) {
                    MiniRing(color: Palette.tint, fraction: FocusMath.fractionLeft(end: session.end, plannedMinutes: session.plannedMinutes, now: now), size: 16)
                    Text(FocusMath.mmss(FocusMath.secondsLeft(end: session.end, now: now))).monospacedDigit()
                }
                .font(.app(13, .semibold))
                .foregroundStyle(Palette.tint)
            }
            .padding(.top, 2)
        } else if let suggestion = item.suggestion {
            PendingTag(text: L("item.pending", Format.whenLabel(suggestion.start, zone: model.zone, today: model.today))).padding(.top, 1)
        } else if let block = item.block {
            side(Format.whenLabel(block.start, zone: model.zone, today: model.today))
        } else if showPlan, let planned = item.plannedDate, planned != model.today {
            side(Format.dayLabel(planned, today: model.today))
        }
    }

    private func side(_ text: String) -> some View {
        Text(text).font(.app(13)).foregroundStyle(Palette.label2).lineLimit(1).fixedSize().padding(.top, 2)
    }

    // MARK: Facts

    /// Project, estimate, tomatoes or time spent, deadline. The project name gives way first.
    @ViewBuilder private var facts: some View {
        let project = model.project(item.projectId)
        let parts = factParts
        if showProject || !parts.isEmpty {
            // Large type: the project stands on a line of its own, so the facts after it stay whole.
            let stacked = typeSize.isAccessibilitySize && showProject && !parts.isEmpty
            let layout = stacked ? AnyLayout(VStackLayout(alignment: .leading, spacing: 2)) : AnyLayout(HStackLayout(spacing: 0))
            layout {
                if showProject {
                    HStack(spacing: 5) {
                        if let project { ColorDot(color: Palette.project(project.color)) }
                        Text(project?.name ?? L("nav.inbox")).lineLimit(1).truncationMode(.tail)
                    }
                    .layoutPriority(0)
                }
                HStack(spacing: 0) {
                    ForEach(Array(parts.enumerated()), id: \.offset) { index, part in
                        if (showProject && !stacked) || index > 0 { Text(" · ").foregroundStyle(Palette.label3).fixedSize() }
                        HStack(spacing: 2) {
                            if part.tomato { TomatoMark() }
                            Text(part.text).foregroundStyle(part.hot ? Palette.red : Palette.label2).lineLimit(1)
                        }
                        .fixedSize(horizontal: !stacked, vertical: false)
                    }
                }
                .layoutPriority(1)
            }
            .font(.app(13))
            .foregroundStyle(Palette.label2)
        }
    }

    private struct Fact {
        let text: String
        var tomato = false
        var hot = false
    }

    private var factParts: [Fact] {
        var out: [Fact] = []
        let tomatoMinutes = model.user?.focusMinutes ?? 25
        if item.isDone {
            if item.focus.minutes > 0 { out.append(Fact(text: L("focus.took", Format.span(item.focus.minutes)))) }
            if item.focus.tomatoes > 0 { out.append(Fact(text: String(item.focus.tomatoes), tomato: true)) }
            return out
        }
        if let estimate = item.estimateMinutes { out.append(Fact(text: Format.dur(estimate))) }
        if item.focus.tomatoes > 0 {
            let need = FocusMath.tomatoesNeeded(estimateMinutes: item.estimateMinutes, tomatoMinutes: tomatoMinutes)
            out.append(Fact(text: need > 0 ? "\(item.focus.tomatoes)/\(need)" : String(item.focus.tomatoes), tomato: true))
        } else if item.focus.minutes > 0 {
            out.append(Fact(text: L("focus.used", Format.span(item.focus.minutes))))
        }
        if let due = ItemText.due(item, today: model.today) { out.append(Fact(text: due.text, hot: due.hot)) }
        return out
    }
}

enum ItemText {
    /// The deadline as a row words it. Red when overdue or due today or tomorrow.
    static func due(_ item: Item, today: Day) -> (text: String, hot: Bool)? {
        guard let due = item.dueDate else { return nil }
        let days = today.days(until: due)
        if days < 0 { return (L("item.overdue", -days), true) }
        return (L("due.label", Format.dueLabel(date: due, time: item.dueTime, today: today)), days <= 1)
    }

    /// "已得 2/4", or just the count without an estimate.
    static func earned(_ item: Item, tomatoMinutes: Int) -> String {
        let need = FocusMath.tomatoesNeeded(estimateMinutes: item.estimateMinutes, tomatoMinutes: tomatoMinutes)
        return L("focus.earnedCount", need > 0 ? "\(item.focus.tomatoes)/\(need)" : String(item.focus.tomatoes))
    }
}

/// A plain list of rows.
struct ItemRows: View {
    let items: [Item]
    var showProject = true
    var showPlan = true

    var body: some View {
        VStack(spacing: 0) {
            ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
                ItemRow(source: item, showProject: showProject, showPlan: showPlan, last: index == items.count - 1)
            }
        }
    }
}
