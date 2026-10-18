#if DEBUG
import Foundation

/// Development only: environment variables that sign in and open a given screen, so that the app
/// can be looked at screen by screen from the command line. None of this exists in a release build.
///
///     KEDULY_SERVER, KEDULY_EMAIL, KEDULY_PASSWORD   sign in with these instead of the kept session
///     KEDULY_SCREEN                                  lists, matrix, today, upcoming, all, inbox, done, project, item,
///                                                    new-item, month, day, new-event, focus, timer, picker, stats,
///                                                    activity, settings, approve
@MainActor
enum DebugLaunch {
    /// Returns true when it took over starting the app.
    static func run(_ model: AppModel, _ router: Router) async -> Bool {
        let env = ProcessInfo.processInfo.environment
        guard let server = env["KEDULY_SERVER"].flatMap({ try? ServerAddress.parse($0) }),
              let email = env["KEDULY_EMAIL"], let password = env["KEDULY_PASSWORD"] else {
            guard let screen = env["KEDULY_SCREEN"] else { return false }
            await model.start()
            await open(screen, model, router)
            return true
        }
        await model.api.setUnauthenticatedHandler { [weak model] in await model?.sessionEnded() }
        await model.api.forget()
        do {
            try await model.signIn(server: server, email: email, password: password, registerAs: nil)
        } catch {
            model.sessionEnded()
            return true
        }
        if let screen = env["KEDULY_SCREEN"] { await open(screen, model, router) }
        return true
    }

    private static func open(_ screen: String, _ model: AppModel, _ router: Router) async {
        guard model.phase == .signedIn else { return }
        switch screen {
        case "lists": router.itemsPath = []
        case "matrix": router.itemsPath = [.matrix]
        case "today": router.itemsPath = [.today]
        case "upcoming": router.itemsPath = [.upcoming]
        case "all": router.itemsPath = [.all]
        case "inbox": router.itemsPath = [.inbox]
        case "done": router.itemsPath = [.done]
        case "activity": router.itemsPath = [.activity]
        case "settings": router.itemsPath = [.settings]
        case "approve": router.itemsPath = [.settings, .approve]
        case "project":
            if let project = model.projects.first { router.itemsPath = [.project(project.id)] }
        case "item":
            // The item with a pending suggestion, which is what the item sheet of the design shows.
            router.itemsPath = [.today]
            if let today = try? await model.api.today(), let item = today.items.first(where: { $0.suggestion != nil }) ?? today.items.first {
                router.sheet = .item(item.id)
            }
        case "new-item":
            router.itemsPath = [.today]
            router.sheet = .newItem(NewItemContext(today: true))
        case "month":
            router.tab = .calendar
            router.calendarMode = .month
        case "day":
            router.tab = .calendar
            router.calendarMode = .day
        case "new-event":
            router.tab = .calendar
            router.sheet = .newEvent(model.today)
        case "focus": router.tab = .focus
        case "timer":
            router.tab = .focus
            router.timerExpanded = true
        case "picker":
            router.tab = .focus
            router.sheet = .focusPicker
        case "stats":
            router.tab = .focus
            router.focusPath = [.stats]
        default: break
        }
    }
}
#endif
