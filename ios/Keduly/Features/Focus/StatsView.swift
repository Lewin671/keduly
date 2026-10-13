import SwiftUI

/// Focus statistics: four figures, a week of bars stacked by project colour, where the time went, the records.
struct StatsView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    private struct Content: Sendable {
        let stats: FocusStats
        let sessions: [FocusSession]
    }

    @State private var loader = Loader<Content>()
    /// How many days before today the records go back. Two days are shown up front.
    @State private var daysBack = 1

    private static let chartHeight: CGFloat = 150
    private static let maxDaysBack = 61

    var body: some View {
        Screen(failed: loader.failed, retry: load) {
            LargeTitle(L("focus.stats"))
            if let content = loader.value {
                if content.stats.minutes == 0 && content.sessions.isEmpty {
                    Blank(title: L("stats.empty"), text: L("stats.emptyTip"))
                } else {
                    figures(content.stats)
                    chart(content.stats)
                    SectionHeader(L("stats.where"))
                    ForEach(Array(content.stats.projects.enumerated()), id: \.offset) { _, amount in
                        projectRow(amount, top: content.stats.projects.map(\.minutes).max() ?? 0)
                    }
                    records(content.sessions)
                    if daysBack < Self.maxDaysBack {
                        MoreLink(title: L("stats.older"), indent: 0) {
                            daysBack = min(Self.maxDaysBack, daysBack + 14)
                            await load()
                        }
                    }
                }
            } else if !loader.failed {
                ProgressView().frame(maxWidth: .infinity).padding(.top, 60)
            }
        }
        .navigationTitle(L("focus.stats"))
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        let today = model.today, from = today.adding(days: -daysBack)
        await loader.load(model) { api in
            async let stats = api.focusStats()
            async let sessions = api.focusSessions(from: from, to: today)
            return Content(stats: try await stats, sessions: try await sessions)
        }
    }

    // MARK: Figures

    private func figures(_ stats: FocusStats) -> some View {
        HStack(alignment: .top) {
            figure(String(stats.days.last?.tomatoes ?? 0), L("stats.unitCount"), L("stats.today"))
            Spacer()
            figure(String(stats.tomatoes), L("stats.unitCount"), L("stats.week"))
            Spacer()
            figure(String(format: "%.1f", Double(stats.tomatoes) / Double(max(1, stats.days.count))), L("stats.unitCount"), L("stats.average"))
            Spacer()
            figure(String(stats.streak), L("stats.unitDays"), L("stats.streak"))
        }
        .padding(.top, 8)
        .padding(.bottom, 22)
    }

    private func figure(_ value: String, _ unit: String, _ caption: String) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            (Text(value).font(.app(28, .bold)) + Text(unit).font(.app(15, .semibold))).tracking(-0.4).lineLimit(1)
            Text(caption).font(.app(13)).foregroundStyle(Palette.label2)
        }
        .accessibilityElement(children: .combine)
    }

    // MARK: Chart

    private func chart(_ stats: FocusStats) -> some View {
        let scale = FocusMath.chartScale(stats.days.map(\.tomatoes))
        let unit = (Self.chartHeight - 20) / CGFloat(scale)
        let average = Double(stats.tomatoes) / Double(max(1, stats.days.count))
        return VStack(spacing: 0) {
            HStack(alignment: .bottom, spacing: 0) {
                ForEach(stats.days, id: \.date) { day in
                    VStack(spacing: 3) {
                        Text(day.tomatoes > 0 ? String(day.tomatoes) : "").font(.app(12, .semibold)).foregroundStyle(Palette.label2)
                        VStack(spacing: 0) {
                            ForEach(Array(day.projects.filter { $0.tomatoes > 0 }.reversed().enumerated()), id: \.offset) { _, amount in
                                Rectangle().fill(amountColor(amount)).frame(height: CGFloat(amount.tomatoes) * unit)
                            }
                        }
                        .frame(width: 26)
                        .clipShape(UnevenRoundedRectangle(topLeadingRadius: 6, topTrailingRadius: 6))
                    }
                    .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
                    .accessibilityElement(children: .ignore)
                    .accessibilityLabel("\(Format.dayLabel(day.date, today: model.today)) \(L("stats.tomatoes", day.tomatoes))")
                }
                Color.clear.frame(width: 34)
            }
            .frame(height: Self.chartHeight)
            .overlay(alignment: .bottom) {
                if average > 0 {
                    HStack(spacing: 6) {
                        Line().stroke(Palette.label3, style: StrokeStyle(lineWidth: 1, dash: [3, 3])).frame(height: 1)
                        Text(L("stats.average")).font(.app(11)).foregroundStyle(Palette.label2).frame(width: 28, alignment: .leading)
                    }
                    .offset(y: -CGFloat(average) * unit + 6)
                }
            }
            Divider()
            HStack(spacing: 0) {
                ForEach(stats.days, id: \.date) { day in
                    Text(Format.weekdayLetters[day.date.weekdayIndex])
                        .font(.system(size: 12, weight: day.date == model.today ? .semibold : .regular))
                        .foregroundStyle(day.date == model.today ? Palette.red : Palette.label2)
                        .frame(maxWidth: .infinity)
                }
                Color.clear.frame(width: 34, height: 1)
            }
            .padding(.top, 6)
        }
    }

    private struct Line: Shape {
        func path(in rect: CGRect) -> Path {
            var path = Path()
            path.move(to: CGPoint(x: rect.minX, y: rect.midY))
            path.addLine(to: CGPoint(x: rect.maxX, y: rect.midY))
            return path
        }
    }

    /// Time that belongs to no project is drawn in grey, so that it does not pass for one.
    private func amountColor(_ amount: FocusAmount) -> Color {
        model.project(amount.projectId).map { Palette.project($0.color) } ?? Palette.label3
    }

    private func projectRow(_ amount: FocusAmount, top: Int) -> some View {
        VStack(spacing: 6) {
            HStack(spacing: 5) {
                ColorDot(color: amountColor(amount))
                Text(model.project(amount.projectId)?.name ?? L("stats.noProject")).font(.app(17)).lineLimit(1)
                Spacer(minLength: 8)
                HStack(spacing: 2) {
                    TomatoMark()
                    Text("\(amount.tomatoes) · \(Format.span(amount.minutes))")
                }
                .font(.app(15)).foregroundStyle(Palette.label2).fixedSize()
            }
            GeometryReader { geo in
                ZStack(alignment: .leading) {
                    Capsule().fill(Palette.fill)
                    Capsule().fill(amountColor(amount)).frame(width: top > 0 ? geo.size.width * CGFloat(amount.minutes) / CGFloat(top) : 0)
                }
            }
            .frame(height: 4)
        }
        .padding(.vertical, 10)
        .accessibilityElement(children: .combine)
    }

    // MARK: Records

    /// Sessions that have ended, by the day they started on: newest day and newest session first.
    @ViewBuilder private func records(_ sessions: [FocusSession]) -> some View {
        let now = model.serverNow
        let ended = sessions.filter { $0.end <= now }.sorted { $0.start > $1.start }
        let days = Dictionary(grouping: ended) { model.zone.day(of: $0.start) }
        ForEach(days.keys.sorted(by: >), id: \.self) { day in
            let list = days[day] ?? []
            SectionHeader(Format.dayLabel(day, today: model.today), trailing: L("stats.tomatoes", list.filter(\.completed).count))
            ForEach(Array(list.enumerated()), id: \.element.id) { index, session in
                Button { router.sheet = .filing(session) } label: {
                    HStack(spacing: 10) {
                        Text("\(model.zone.hhmm(session.start))–\(model.zone.hhmm(session.end))")
                            .font(.app(15)).foregroundStyle(Palette.label2).monospacedDigit().frame(width: 96, alignment: .leading)
                        Text(session.title.isEmpty ? L("focus.free") : session.title).font(.app(17)).lineLimit(1)
                            .frame(maxWidth: .infinity, alignment: .leading)
                        if session.completed {
                            TomatoMark()
                        } else {
                            let minutes = Int(session.end.timeIntervalSince(session.start) / 60)
                            Text(L("stats.unfinished", Format.span(minutes))).font(.app(13)).foregroundStyle(Palette.label2).fixedSize()
                        }
                    }
                    .frame(minHeight: 44)
                    .overlay(alignment: .bottom) { if index < list.count - 1 { Divider() } }
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("record-\(index)")
            }
        }
    }
}
