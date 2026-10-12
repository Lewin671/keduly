import SwiftUI

/// The three tabs under one floating bar, the sheets, and the timer page that covers them all.
struct MainView: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    var body: some View {
        @Bindable var router = router
        ZStack {
            Group {
                switch router.tab {
                case .items:
                    NavigationStack(path: $router.itemsPath) {
                        ListsRootView().tabBarInset().navigationDestination(for: Route.self) { RouteView(route: $0).tabBarInset() }
                    }
                case .calendar:
                    NavigationStack(path: $router.calendarPath) {
                        CalendarView().tabBarInset().navigationDestination(for: Route.self) { RouteView(route: $0).tabBarInset() }
                    }
                case .focus:
                    NavigationStack(path: $router.focusPath) {
                        FocusView().tabBarInset().navigationDestination(for: Route.self) { RouteView(route: $0).tabBarInset() }
                    }
                }
            }
            .overlay(alignment: .bottom) { BottomBar() }

            if router.timerExpanded, let focus = model.focus, focus.state != .idle, let session = focus.session {
                TimerPage(focus: focus, session: session)
                    .transition(.move(edge: .bottom))
                    .zIndex(1)
            }
        }
        .overlay(alignment: .top) { NoticeBanner() }
        .animation(.default, value: model.notice)
        .sheet(item: $router.sheet) { SheetView(route: $0) }
        .onChange(of: model.focus?.state) { _, state in
            // Nothing is running any more: the timer page has nothing to show.
            if state == .idle || state == nil { router.timerExpanded = false }
        }
    }
}

private struct RouteView: View {
    let route: Route

    var body: some View {
        switch route {
        case .matrix: MatrixView()
        case .today: TodayView()
        case .upcoming: UpcomingView()
        case .all: AllView()
        case .inbox: InboxView()
        case .done: DoneView()
        case .project(let id): ProjectView(id: id)
        case .stats: StatsView()
        case .activity: ActivityView()
        case .settings: SettingsView()
        }
    }
}

private struct SheetView: View {
    @Environment(AppModel.self) private var model
    let route: SheetRoute

    var body: some View {
        Group {
            switch route {
            case .item(let id): ItemSheet(id: id)
            case .newItem(let context): NewItemSheet(context: context)
            case .newEvent(let day): EventSheet(event: nil, day: day)
            case .event(let event): EventSheet(event: event, day: model.today)
            case .suggestion(let id): SuggestionSheet(id: id)
            case .entries(let events, let day): EntriesSheet(events: events, day: day)
            case .newProject: ProjectEditor(project: nil)
            case .editProject(let project): ProjectEditor(project: project)
            case .filing(let session): FilingSheet(session: session)
            case .focusPicker: FocusPickerSheet()
            }
        }
        .tint(Palette.tint)
    }
}

/// Keeps a page's content clear of the floating bar, and of the timer strip when it shows.
private struct TabBarInset: ViewModifier {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    func body(content: Content) -> some View {
        let strip = model.focus.map { $0.state != .idle } == true && !router.timerExpanded
        content.safeAreaInset(edge: .bottom, spacing: 0) {
            Color.clear.frame(height: Metrics.tabBarHeight + 8 + (strip ? 58 : 0))
        }
    }
}

private extension View {
    func tabBarInset() -> some View { modifier(TabBarInset()) }
}

/// The floating capsule tab bar with the round add button beside it, and above them the running timer.
private struct BottomBar: View {
    @Environment(AppModel.self) private var model
    @Environment(Router.self) private var router

    var body: some View {
        VStack(spacing: 10) {
            if let focus = model.focus, focus.state != .idle, let session = focus.session, !router.timerExpanded {
                TimerStrip(focus: focus, session: session)
            }
            HStack(spacing: 0) {
                HStack(spacing: 0) {
                    tab(.items, "checkmark.circle", L("mode.items"))
                    tab(.calendar, "calendar", L("mode.cal"))
                    tab(.focus, "timer", L("mode.focus"))
                }
                .padding(4)
                .background(.regularMaterial, in: Capsule())
                .overlay(Capsule().strokeBorder(Palette.label.opacity(0.06), lineWidth: 0.5))
                .shadow(color: .black.opacity(0.14), radius: 14, y: 8)
                Spacer()
                Button(action: add) {
                    Image(systemName: "plus").font(.system(size: 24, weight: .semibold)).foregroundStyle(Palette.onTint)
                        .frame(width: 62, height: 62)
                        .background(Palette.tint, in: Circle())
                        .shadow(color: Palette.tint.opacity(0.38), radius: 11, y: 8)
                }
                .buttonStyle(.plain)
                .accessibilityLabel(router.tab == .calendar ? L("event.new") : L("item.new"))
                .accessibilityIdentifier("add")
            }
        }
        .padding(.horizontal, Metrics.pageMargin)
        .padding(.top, 8)
        .padding(.bottom, -8)
        .ignoresSafeArea(.keyboard)
    }

    private func tab(_ tab: AppTab, _ symbol: String, _ title: String) -> some View {
        let on = router.tab == tab
        return Button {
            // The focus tab leads straight to the timer while one is running.
            if tab == .focus, model.focus.map({ $0.state != .idle }) == true, router.tab != .focus {
                router.select(tab)
                withAnimation { router.timerExpanded = true }
            } else {
                router.select(tab)
            }
        } label: {
            VStack(spacing: 2) {
                Image(systemName: symbol).font(.system(size: 21, weight: .medium)).frame(height: 25)
                Text(title).font(.system(size: 10, weight: .semibold))
            }
            .foregroundStyle(on ? Palette.tint : Palette.label2)
            .frame(width: 82, height: 54)
            .background(on ? Palette.fill : Color.clear, in: Capsule())
            .contentShape(Capsule())
        }
        .buttonStyle(.plain)
        .accessibilityAddTraits(on ? .isSelected : [])
        .accessibilityIdentifier("tab-\(title)")
    }

    /// In the calendar the button makes an event, elsewhere an item, which starts where it was made.
    private func add() {
        switch router.tab {
        case .calendar:
            router.sheet = .newEvent(router.calendarDay ?? model.today)
        case .items, .focus:
            var context = NewItemContext()
            if case .project(let id) = router.top { context.projectId = id }
            if router.top == .today { context.today = true }
            router.sheet = .newItem(context)
        }
    }
}
