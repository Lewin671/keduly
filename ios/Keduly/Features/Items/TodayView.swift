import SwiftUI

/// Today: whether the day fits, the day's events as a few quiet lines, then what to do.
struct TodayView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var loader = Loader<TodayResponse>()

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            let day = loader.value?.date ?? model.today
            LargeTitle(L("nav.today"), detail: "\(L("date.md", day.month, day.day)) \(Format.weekdayLong(day))")
            if let today = loader.value {
                if today.habits.isEmpty && today.items.isEmpty && today.overdue.isEmpty && today.events.isEmpty {
                    Blank(title: L("today.empty"), text: L("today.emptyTip"))
                } else {
                    content(today)
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.today"))
        .toolbar { ToolbarItem(placement: .topBarTrailing) { BellButton() } }
        .task(id: model.dataVersion) { await load() }
        .task {
            // The free time is counted from now, so the sentence goes stale by itself: ask again every minute.
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(60))
                if !Task.isCancelled { await load() }
            }
        }
    }

    private func load() async {
        await loader.load(model) { try await $0.today() }
    }

    @ViewBuilder private func content(_ today: TodayResponse) -> some View {
        summary(TodayBudget(today, workEnd: model.user?.workEnd ?? ""))
            .padding(.top, 2)
            .padding(.bottom, 12)
            .accessibilityIdentifier("today-summary")

        VStack(spacing: 0) {
            ForEach(today.events.prefix(5), id: \.entryKey) { EventLine(event: $0) }
        }
        if today.events.count > 5 {
            Button {
                router.calendarDay = today.date
                router.calendarMode = .day
                router.tab = .calendar
            } label: {
                Text(L("list.moreEvents", today.events.count - 5)).font(.app(15)).foregroundStyle(Palette.tint)
                    .frame(maxWidth: .infinity, minHeight: 36, alignment: .leading)
                    .padding(.leading, 55)
                    .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
        }

        if !today.habits.isEmpty { SectionHeader(L("habit.heading")); HabitOccurrenceRows(records:today.habits) }

        // An item that is past its deadline stands under 逾期 even when it was planned for today.
        let late = today.overdue + today.items.filter { item in !item.isDone && (item.dueDate.map { $0 < today.date } ?? false) }
        let rest = today.items.filter { item in !late.contains { $0.id == item.id } }
        let evening = rest.filter(\.evening)
        let daytime = rest.filter { !$0.evening }

        if !late.isEmpty {
            SectionHeader(L("today.overdue"), trailing: String(late.count), color: Palette.red)
            ItemRows(items: late, showPlan: false)
        }
        SectionHeader(L("today.items"), trailing: String(daytime.filter { !$0.isDone }.count))
        if daytime.isEmpty {
            Text(L("matrix.none")).font(.app(15)).foregroundStyle(Palette.label3).padding(.leading, 34).padding(.top, 12)
        } else {
            ItemRows(items: daytime, showPlan: false)
        }
        if !evening.isEmpty {
            SectionHeader(title: L("when.tonight"), trailing: String(evening.filter { !$0.isDone }.count)) {
                Image(systemName: "moon.fill").font(.app(15)).foregroundStyle(Color(uiColor: Palette.indigoUI))
            }
            ItemRows(items: evening, showPlan: false)
        }
    }

    private func summary(_ budget: TodayBudget) -> Text {
        budget.parts.reduce(Text("")) { text, part in
            switch part {
            case .plain(let s): return text + Text(s).foregroundStyle(Palette.label2)
            case .strong(let s): return text + Text(s).foregroundStyle(Palette.label).fontWeight(.semibold)
            case .warning(let s): return text + Text(s).foregroundStyle(Palette.red).fontWeight(.semibold)
            }
        }
        .font(.app(15))
    }
}
