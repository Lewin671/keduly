import SwiftUI

/// Day by day from tomorrow: that day's events, then the items planned or due that day.
struct UpcomingView: View {
    @Environment(AppModel.self) private var model
    @State private var loader = Loader<[UpcomingDay]>()
    /// How many months past the current one were asked for.
    @State private var extraMonths = 0

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            LargeTitle(L("nav.upcoming"))
            if let days = loader.value {
                if days.isEmpty {
                    Blank(title: L("upcoming.empty"), text: L("upcoming.emptyTip"))
                }
                ForEach(days, id: \.date) { day in
                    SectionHeader(String(day.date.day), note: day.date == firstDay ? "\(L("day.tomorrow")) · \(Format.weekdayLong(day.date))" : Format.weekdayLong(day.date),
                                  trailing: day.date.month != model.today.month ? L("date.m", day.date.month) : nil)
                    VStack(spacing: 0) {
                        ForEach(day.events, id: \.entryKey) { EventLine(event: $0) }
                    }
                    .padding(.top, day.events.isEmpty ? 0 : 6)
                    ItemRows(items: day.items, showPlan: false)
                    HabitOccurrenceRows(records:day.habits)
                }
                let next = lastDay.adding(days: 1)
                MoreLink(title: L("upcoming.load", L("date.m", next.month)), indent: 0) {
                    extraMonths += 1
                    await load()
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.upcoming"))
        .task(id: model.dataVersion) { await load() }
    }

    private var firstDay: Day { model.today.adding(days: 1) }
    private var lastDay: Day { firstDay.startOfMonth.adding(months: extraMonths).endOfMonth }

    private func load() async {
        // One request per month: the server takes at most 62 days at a time.
        var ranges: [(Day, Day)] = []
        var from = firstDay
        while from <= lastDay {
            let to = min(from.endOfMonth, lastDay)
            ranges.append((from, to))
            from = to.adding(days: 1)
        }
        let chunks = ranges
        await loader.load(model) { api in
            var days: [UpcomingDay] = []
            for (from, to) in chunks { days += try await api.upcoming(from: from, to: to) }
            return days
        }
    }
}

/// Every open item, one section per project.
struct AllView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var loader = Loader<[OverviewProject]>()
    /// Projects for which more than the first five items were asked for, and how many.
    @State private var limits: [String: Int] = [:]
    @State private var more: [String: [Item]] = [:]

    private static let step = 15

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            LargeTitle(L("nav.all"))
            if let projects = loader.value {
                if projects.isEmpty {
                    Blank(title: L("all.empty"), text: L("all.emptyTip"))
                }
                ForEach(projects, id: \.projectId) { entry in
                    let project = model.project(entry.projectId)
                    let items = more[entry.projectId] ?? entry.items
                    Button { router.push(.project(entry.projectId)) } label: {
                        SectionHeader(project?.name ?? "", trailing: String(entry.total), color: Palette.project(project?.color))
                    }
                    .buttonStyle(.plain)
                    ItemRows(items: items, showProject: false)
                    if entry.total > items.count {
                        MoreLink(title: L("list.more", min(Self.step, entry.total - items.count), entry.total - items.count), indent: 34) {
                            limits[entry.projectId] = items.count + Self.step
                            await expand(entry.projectId)
                        }
                    }
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.all"))
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        await loader.load(model) { try await $0.overview() }
        for id in limits.keys { await expand(id) }
    }

    private func expand(_ id: String) async {
        guard let limit = limits[id] else { return }
        if let page = try? await model.api.items(.init(projectId: id), upTo: limit) {
            more[id] = page.items
        }
    }
}

/// Items not filed under a project yet.
struct InboxView: View {
    @Environment(AppModel.self) private var model
    @State private var list = PagedItems()

    private let query = APIClient.ItemQuery(projectId: "none")

    var body: some View {
        Screen(failed: list.failed, retry: load) {
            LargeTitle(L("nav.inbox"))
            if let items = list.items {
                if items.isEmpty {
                    Blank(title: L("inbox.empty"), text: L("inbox.emptyTip"))
                } else {
                    ItemRows(items: items, showProject: false)
                    if list.remaining > 0 {
                        MoreLink(title: L("list.more", min(60, list.remaining), list.remaining), indent: 34) {
                            await list.more(model, query, step: 60)
                        }
                    }
                }
            } else if !list.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.inbox"))
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        await list.reload(model, query, first: 20)
    }
}

/// Finished items by month, newest first. The next batch loads on reaching the end.
struct DoneView: View {
    @Environment(AppModel.self) private var model
    @State private var list = PagedItems()

    private let query = APIClient.ItemQuery(status: "done")

    var body: some View {
        Screen(failed: list.failed, retry: load) {
            LargeTitle(L("nav.done"), detail: list.items.map { _ in L("done.total", list.total) })
            if let items = list.items {
                if items.isEmpty {
                    Blank(title: L("done.empty"), text: L("done.emptyTip"))
                } else {
                    ForEach(months(items), id: \.title) { month in
                        SectionHeader(month.title, trailing: nil)
                        ItemRows(items: month.items)
                    }
                    if list.remaining > 0 {
                        ProgressView().frame(maxWidth: .infinity).padding(.vertical, 16)
                            .task { await list.more(model, query, step: 50) }
                    } else {
                        Text(L("done.end")).font(.app(13)).foregroundStyle(Palette.label3)
                            .frame(maxWidth: .infinity).padding(.vertical, 18)
                    }
                }
            } else if !list.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("nav.done"))
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        await list.reload(model, query, first: 50)
    }

    private func months(_ items: [Item]) -> [(title: String, items: [Item])] {
        var out: [(title: String, items: [Item])] = []
        let today = model.today
        for item in items {
            let day = model.zone.day(of: item.completedAt ?? item.updatedAt)
            let title = day.year == today.year && day.month == today.month ? L("done.thisMonth") : L("date.ym", day.year, day.month)
            if out.last?.title == title { out[out.count - 1].items.append(item) } else { out.append((title, [item])) }
        }
        return out
    }
}
