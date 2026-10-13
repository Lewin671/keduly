import SwiftUI

/// The focus tab when nothing covers it: the dial, what the tomato will be on, the round, and the
/// main button fixed above the tab bar.
struct FocusView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    @State private var today = Loader<TodayResponse>()

    var body: some View {
        VStack(spacing: 0) {
            if today.failed { ErrorLine(retry: load).padding(.horizontal, Metrics.pageMargin) }
            if let focus = model.focus {
                page(focus)
            } else {
                Spacer()
                ProgressView()
                Spacer()
            }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.background)
        .navigationTitle(L("mode.focus"))
        .inlineTitle()
        .toolbar {
            ToolbarItemGroup(placement: .topBarTrailing) {
                Button { router.push(.stats) } label: { Image(systemName: "chart.bar.fill") }
                    .accessibilityLabel(L("focus.stats"))
                    .accessibilityIdentifier("stats")
                BellButton()
            }
        }
        .task(id: model.dataVersion) { await load() }
    }

    private func load() async {
        await today.load(model) { try await $0.today() }
        await model.refreshFocus()
    }

    /// The item the next tomato goes to: the one picked, as long as it is still open.
    private var picked: Item? {
        let candidates = FocusPicker.candidates(today.value)
        switch router.focusPick {
        case .free: return nil
        case .automatic: return candidates.first
        case .item(let item):
            let current = model.current(candidates.first { $0.id == item.id } ?? item)
            return current.isDone ? candidates.first : current
        }
    }

    @ViewBuilder private func page(_ focus: Focus) -> some View {
        let running = focus.state != .idle
        let item = running ? nil : picked
        let minutes = model.user?.focusMinutes ?? 25
        let color = Color(uiColor: model.focusColor(focus.state, projectId: running ? focus.session?.projectId : item?.projectId))
        VStack(spacing: 0) {
            Spacer(minLength: 0)
            Text(L("focus.nth", focus.tomatoesToday + (focus.state == .over ? 0 : 1)))
                .font(.app(15, .semibold)).foregroundStyle(Palette.label2)
            if running, let session = focus.session {
                TimelineView(.periodic(from: .now, by: 0.5)) { _ in
                    let now = model.serverNow
                    Dial(color: color, fraction: focus.state == .over ? 1 : FocusMath.fractionLeft(end: session.end, plannedMinutes: session.plannedMinutes, now: now)) {
                        DialFigures(time: focus.state == .over ? "00:00" : FocusMath.mmss(FocusMath.secondsLeft(end: session.end, now: now)),
                                    caption: focus.state == .over ? L("focus.timeUp") : L("focus.until", model.zone.hhmm(session.end)))
                    }
                }
                .padding(.top, 18)
                subject(title: focus.state == .rest ? L("focus.resting") : session.title.isEmpty ? L("focus.free") : session.title,
                        detail: model.project(session.projectId)?.name, color: model.project(session.projectId).map { Palette.project($0.color) }, chevron: false)
                    .padding(.top, 22)
            } else {
                Dial(color: color, fraction: 1) {
                    DialFigures(time: FocusMath.mmss(Double(minutes * 60)), caption: L("dur.minutes", minutes))
                }
                .padding(.top, 18)
                Button { router.sheet = .focusPicker } label: {
                    if let item {
                        let project = model.project(item.projectId)
                        subject(title: item.title, detail: project?.name ?? L("nav.inbox"),
                                suffix: item.focus.tomatoes > 0 || item.estimateMinutes != nil ? ItemText.earned(item, tomatoMinutes: minutes) : nil,
                                color: project.map { Palette.project($0.color) }, chevron: true)
                    } else {
                        subject(title: L("focus.free"), detail: L("focus.freeNote"), color: nil, chevron: true)
                    }
                }
                .buttonStyle(.plain)
                .padding(.top, 22)
                .accessibilityIdentifier("focus-pick")
            }
            RoundMarks(focus: focus).padding(.top, 18)
            Spacer(minLength: 0)
            if running {
                Button(L("focus.back")) { withAnimation { router.timerExpanded = true } }
                    .buttonStyle(CapsuleButtonStyle(primary: true))
                    .accessibilityIdentifier("focus-back")
            } else {
                AsyncButton(L("focus.start")) {
                    let id = item?.id
                    if await model.focusAction({ try await $0.startFocus(itemId: id) }) {
                        withAnimation { router.timerExpanded = true }
                    }
                }
                .buttonStyle(CapsuleButtonStyle(primary: true))
                .accessibilityIdentifier("focus-start")
            }
            Text(focus.tomatoesToday > 0 || focus.minutesToday > 0 ? L("focus.todayTotal", focus.tomatoesToday, Format.span(focus.minutesToday)) : L("stats.noStreak"))
                .font(.app(13)).foregroundStyle(Palette.label2)
                .padding(.top, 12)
                .padding(.bottom, 10)
        }
        .padding(.horizontal, Metrics.pageMargin)
    }

    /// The detail names the project, which gives way first, and then what must stay readable.
    private func subject(title: String, detail: String?, suffix: String? = nil, color: Color?, chevron: Bool) -> some View {
        VStack(spacing: 3) {
            (Text(title) + (chevron ? Text("  ") + Text(Image(systemName: "chevron.down")).font(.app(12, .bold)).foregroundStyle(Palette.label3) : Text("")))
                .font(.app(20, .semibold)).tracking(-0.4)
                .lineLimit(2).multilineTextAlignment(.center)
            if let detail, !detail.isEmpty {
                HStack(spacing: 5) {
                    if let color { ColorDot(color: color) }
                    Text(detail).lineLimit(1)
                    if let suffix { Text("· \(suffix)").lineLimit(1).fixedSize() }
                }
                .font(.app(15)).foregroundStyle(Palette.label2)
            }
        }
        .padding(.horizontal, 6)
        .frame(minHeight: 44)
        .contentShape(Rectangle())
    }
}

/// The timer while it runs: the whole screen, tinted with the project's colour, no tab bar.
/// Also what a tomato that ran out looks like (做完了 / 再来一个 / 休息), and the rest, in green.
struct TimerPage: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let focus: Focus
    let session: FocusSession
    @State private var item = Loader<Item>()

    var body: some View {
        let colorUI = model.focusColor(focus.state, projectId: session.projectId)
        let color = Color(uiColor: colorUI)
        VStack(spacing: 0) {
            HStack {
                Button { withAnimation { router.timerExpanded = false } } label: {
                    HStack(spacing: 3) {
                        Image(systemName: "chevron.down").font(.app(13, .bold))
                        Text(L("focus.collapse")).font(.app(15, .medium))
                    }
                    .foregroundStyle(Palette.label)
                    .padding(.leading, 11)
                    .padding(.trailing, 14)
                    .frame(height: 36)
                    .background(Palette.fill, in: Capsule())
                    .frame(minHeight: 44)
                    .contentShape(Rectangle())
                }
                .buttonStyle(.plain)
                .accessibilityIdentifier("timer-collapse")
                Spacer()
            }
            .padding(.leading, 16)

            Spacer(minLength: 0)
            Text(label).font(.app(15, .semibold)).foregroundStyle(Palette.label2)
                .accessibilityIdentifier("timer-label")
            dial(color).padding(.top, 18)
            subject.padding(.top, 22)
            RoundMarks(focus: focus).padding(.top, 18)
            Spacer(minLength: 0)

            HStack(spacing: 10) { actions }
                .padding(.horizontal, Metrics.pageMargin)
            Text(focus.state == .work ? L("focus.noPause") : " ")
                .font(.app(13)).foregroundStyle(Palette.label2)
                .padding(.top, 12)
                .padding(.bottom, 16)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Palette.wash(colorUI, 0.09).ignoresSafeArea())
        .task(id: ItemKey(id: session.itemId, version: model.dataVersion)) {
            guard let id = session.itemId else { return }
            await item.load(model) { try await $0.item(id) }
        }
    }

    private struct ItemKey: Equatable {
        let id: String?
        let version: Int
    }

    private var label: String {
        switch focus.state {
        case .over: return L("focus.earned", focus.tomatoesToday)
        case .rest: return focus.roundDone >= focus.roundSize ? L("focus.longRest") : L("focus.rest")
        case .idle, .work: return L("focus.nth", focus.tomatoesToday + 1)
        }
    }

    @ViewBuilder private func dial(_ color: Color) -> some View {
        if focus.state == .over {
            Dial(color: color, fraction: 1) { TomatoMark(size: 92) }
        } else {
            TimelineView(.periodic(from: .now, by: 0.5)) { _ in
                let now = model.serverNow
                Dial(color: color, fraction: FocusMath.fractionLeft(end: session.end, plannedMinutes: session.plannedMinutes, now: now)) {
                    DialFigures(time: FocusMath.mmss(FocusMath.secondsLeft(end: session.end, now: now)), caption: L("focus.until", model.zone.hhmm(session.end)))
                }
            }
        }
    }

    @ViewBuilder private var subject: some View {
        let project = model.project(session.projectId)
        let free = session.itemId == nil && focus.state != .rest
        VStack(spacing: 3) {
            Text(focus.state == .rest ? L("focus.resting") : session.title.isEmpty ? L("focus.free") : session.title)
                .font(.app(20, .semibold)).tracking(-0.4)
                .lineLimit(2).multilineTextAlignment(.center)
            if focus.state != .rest {
                HStack(spacing: 5) {
                    if let project { ColorDot(color: Palette.project(project.color)) }
                    if session.itemId != nil {
                        Text(project?.name ?? L("nav.inbox")).foregroundStyle(Palette.label2).lineLimit(1)
                        if let item = item.value {
                            Text("· \(ItemText.earned(item, tomatoMinutes: model.user?.focusMinutes ?? 25))").foregroundStyle(Palette.label2).lineLimit(1).fixedSize()
                        }
                    } else {
                        Text(project?.name ?? L("focus.freeNote")).foregroundStyle(Palette.label2).lineLimit(1)
                    }
                    if free {
                        // The moment a tomato is over is when one knows what it was.
                        Button(session.title.isEmpty && session.projectId == nil ? L("focus.file") : L("focus.refile")) {
                            router.sheet = .filing(session)
                        }
                        .frame(minHeight: 44)
                        .accessibilityIdentifier("timer-file")
                    }
                }
                .font(.app(15))
            }
        }
        .padding(.horizontal, 26)
    }

    /// A tomato on an item can end with the item being done; free focus has nothing to tick.
    private var canFinish: Bool {
        session.itemId != nil && item.value?.isDone != true
    }

    @ViewBuilder private var actions: some View {
        switch focus.state {
        case .idle:
            EmptyView()
        case .work:
            AsyncButton(L("focus.giveUp")) { await model.focusAction { try await $0.stopFocus() } }
                .buttonStyle(CapsuleButtonStyle())
                .accessibilityIdentifier("timer-giveup")
            if canFinish { finishButton(primary: true) }
        case .over:
            if canFinish { finishButton(primary: false) }
            AsyncButton(L("focus.again")) {
                let session = session
                await model.focusAction { try await $0.startFocus(itemId: session.itemId, title: session.title, projectId: session.projectId) }
            }
            .buttonStyle(CapsuleButtonStyle())
            .accessibilityIdentifier("timer-again")
            AsyncButton(focus.roundDone >= focus.roundSize ? L("focus.takeLongRest", focus.restMinutes) : L("focus.takeRest", focus.restMinutes)) {
                await model.focusAction { try await $0.restFocus() }
            }
            .buttonStyle(CapsuleButtonStyle(primary: true))
            .accessibilityIdentifier("timer-rest")
        case .rest:
            AsyncButton(L("focus.skipRest")) { await model.focusAction { try await $0.stopFocus() } }
                .buttonStyle(CapsuleButtonStyle())
                .accessibilityIdentifier("timer-skip")
        }
    }

    private func finishButton(primary: Bool) -> some View {
        AsyncButton(L("focus.finish")) {
            guard let id = session.itemId else { return }
            // Ticking the item ends its timer and keeps the minutes.
            _ = await model.write { try await $0.updateItem(id, ItemPatch(status: .set(.done))) }
        }
        .buttonStyle(CapsuleButtonStyle(primary: primary))
        .accessibilityIdentifier("timer-finish")
    }
}

/// Away from the timer page: the ring, the time left and the item, above the tab bar.
struct TimerStrip: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router
    let focus: Focus
    let session: FocusSession

    var body: some View {
        let color = Color(uiColor: model.focusColor(focus.state, projectId: session.projectId))
        Button { withAnimation { router.timerExpanded = true } } label: {
            HStack(spacing: 9) {
                if focus.state == .over {
                    TomatoMark(size: 20)
                    Text(L("focus.timeUp")).font(.app(15, .semibold))
                } else {
                    TimelineView(.periodic(from: .now, by: 1)) { _ in
                        let now = model.serverNow
                        HStack(spacing: 9) {
                            MiniRing(color: color, fraction: FocusMath.fractionLeft(end: session.end, plannedMinutes: session.plannedMinutes, now: now))
                            Text(FocusMath.mmss(FocusMath.secondsLeft(end: session.end, now: now))).font(.app(15, .semibold)).monospacedDigit()
                        }
                    }
                }
                Text(focus.state == .rest ? L("focus.resting") : session.title.isEmpty ? L("focus.free") : session.title)
                    .font(.app(15)).foregroundStyle(Palette.label2).lineLimit(1)
                    .frame(maxWidth: .infinity, alignment: .leading)
                Image(systemName: "chevron.right").font(.app(13, .semibold)).foregroundStyle(Palette.label3)
            }
            .foregroundStyle(Palette.label)
            .padding(.leading, 13)
            .padding(.trailing, 14)
            .frame(height: 48)
            .background(.regularMaterial, in: Capsule())
            .overlay(Capsule().strokeBorder(Palette.label.opacity(0.06), lineWidth: 0.5))
            .shadow(color: .black.opacity(0.14), radius: 14, y: 8)
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .accessibilityIdentifier("timer-strip")
    }
}
