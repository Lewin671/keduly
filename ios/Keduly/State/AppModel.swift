import Foundation
import Observation

/// What the whole app shares: the session, the lists every screen needs (projects, counts), and
/// the focus timer. Each screen keeps its own data in memory and loads it again whenever
/// `dataVersion` changes: after a write, on returning to the foreground, and when the server
/// reports a new revision. Nothing is kept on disk except the session.
@MainActor
@Observable
final class AppModel {
    enum Phase: Equatable {
        /// Finding out whether there is a session, or failing to reach its server.
        case launching(failed: Bool)
        case signedOut
        case signedIn
    }

    private(set) var phase: Phase = .launching(failed: false)
    private(set) var user: User?
    private(set) var areas: [Area] = []
    private(set) var projects: [Project] = []
    private(set) var headings: [Heading] = []
    private(set) var counts = Counts(inbox: 0, today: 0, pending: 0)
    private(set) var focus: Focus?
    private(set) var clock: ServerClock?
    private(set) var zone = Zone(identifier: "UTC")
    private(set) var server: URL?
    /// Screens reload when this changes.
    private(set) var dataVersion = 0
    /// Items changed in place by ticking them: a ticked item stays where it is until the next reload.
    private(set) var itemOverrides: [String: Item] = [:]
    /// A line shown briefly over the page: a write that failed, or data that changed underneath.
    var notice: String?

    @ObservationIgnored let api: APIClient
    @ObservationIgnored private var revision: Int?
    @ObservationIgnored private var pollTask: Task<Void, Never>?
    @ObservationIgnored private var focusTask: Task<Void, Never>?
    @ObservationIgnored private var active = true

    init(api: APIClient) {
        self.api = api
    }

    // MARK: - Derived

    /// The server's time, as far as the last answer of `GET /focus` lets us estimate it.
    var serverNow: Date { clock?.now() ?? Date() }
    /// Today in the account's zone.
    var today: Day { zone.day(of: serverNow) }

    func project(_ id: String?) -> Project? {
        guard let id else { return nil }
        return projects.first { $0.id == id }
    }

    /// The item as the screen should show it: with a tick made on this visit applied.
    func current(_ item: Item) -> Item {
        itemOverrides[item.id] ?? item
    }

    // MARK: - Session

    func start() async {
        await api.setUnauthenticatedHandler { [weak self] in await self?.sessionEnded() }
        guard let server = await api.restore() else {
            phase = .signedOut
            return
        }
        self.server = server
        await enter()
    }

    /// Loads what every screen needs. Without it there is nothing to show, so a failure stays on the launch screen.
    func enter() async {
        phase = .launching(failed: false)
        do {
            async let boot = api.bootstrap()
            async let focus = api.focus()
            apply(try await boot)
            setFocus(try await focus)
            phase = .signedIn
            startPolling()
        } catch APIError.unauthenticated {
            sessionEnded()
        } catch {
            phase = .launching(failed: true)
        }
    }

    func signIn(server: URL, email: String, password: String, registerAs name: String?) async throws {
        await api.use(server: server)
        if let name {
            _ = try await api.register(email: email, password: password, name: name, timezone: TimeZone.current.identifier)
        } else {
            _ = try await api.login(email: email, password: password)
        }
        self.server = server
        await enter()
    }

    func signOut() async {
        try? await api.logout()
        await api.forget()
        sessionEnded()
    }

    /// A `401`, or signing out: the session and everything loaded with it are dropped.
    func sessionEnded() {
        pollTask?.cancel()
        focusTask?.cancel()
        user = nil
        areas = []
        projects = []
        headings = []
        counts = Counts(inbox: 0, today: 0, pending: 0)
        focus = nil
        clock = nil
        revision = nil
        itemOverrides = [:]
        notice = nil
        phase = .signedOut
    }

    // MARK: - Loading and change detection

    private func apply(_ boot: Bootstrap) {
        user = boot.user
        zone = Zone(identifier: boot.user.timezone)
        areas = boot.areas.sorted { $0.position < $1.position }
        projects = boot.projects.filter { !$0.archived }.sorted { $0.position < $1.position }
        headings = boot.headings
        counts = boot.counts
        revision = boot.revision
    }

    private func refreshShared() async {
        async let boot = try? api.bootstrap()
        async let focus = try? api.focus()
        if let boot = await boot { apply(boot) }
        if let focus = await focus { setFocus(focus) }
    }

    /// Makes every screen load its data again.
    func reloadAll() {
        itemOverrides = [:]
        dataVersion += 1
    }

    func setActive(_ active: Bool) {
        let returned = active && !self.active
        self.active = active
        guard returned, phase == .signedIn else { return }
        Task {
            await refreshShared()
            reloadAll()
        }
    }

    private func startPolling() {
        pollTask?.cancel()
        pollTask = Task { [weak self] in
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(20))
                guard let self, !Task.isCancelled else { return }
                if self.active { await self.poll() }
            }
        }
    }

    private func poll() async {
        async let counts = try? api.counts()
        async let focus = try? api.focus()
        if let focus = await focus { setFocus(focus) }
        guard let answer = await counts else { return }
        self.counts = answer.counts
        guard answer.revision != revision else { return }
        await refreshShared()
        reloadAll()
    }

    // MARK: - Writes

    /// Runs one write. It is never retried here. On success the shared lists are loaded again and,
    /// unless `reload` is false, so is every screen. On failure the user is told and the screens
    /// reload, because a request that got no answer may still have been carried out.
    /// With `inline`, a `400` is left for the caller to show next to its form.
    func write<T: Sendable>(reload: Bool = true, inline: Bool = false, _ operation: @Sendable (APIClient) async throws -> T) async -> Result<T, APIError> {
        do {
            let value = try await operation(api)
            await refreshShared()
            if reload { reloadAll() }
            return .success(value)
        } catch let error as APIError {
            if error == .unauthenticated { return .failure(error) }
            if !(inline && error.status == 400) { notice = error.message }
            if error.status != 400 {
                await refreshShared()
                reloadAll()
            }
            return .failure(error)
        } catch {
            return .failure(.network)
        }
    }

    /// Ticks or unticks an item. The row stays where it is, shown as done, so the tick can be taken back.
    func toggle(_ item: Item) async {
        let status: ItemStatus = item.isDone ? .open : .done
        let id = item.id
        if case .success(let updated) = await write(reload: false, { try await $0.updateItem(id, ItemPatch(status: .set(status))) }) {
            itemOverrides[id] = updated
        }
    }

    // MARK: - Focus

    private func setFocus(_ focus: Focus) {
        self.focus = focus
        clock = ServerClock(serverNow: focus.now)
        focusTask?.cancel()
        guard let session = focus.session, focus.state == .work || focus.state == .rest else { return }
        // The server decides when a tomato is earned: ask it again when the countdown reaches zero.
        let left = FocusMath.secondsLeft(end: session.end, now: focus.now)
        focusTask = Task { [weak self] in
            try? await Task.sleep(for: .seconds(left + 0.3))
            while !Task.isCancelled {
                guard let self else { return }
                if await self.refreshFocus() { return }
                try? await Task.sleep(for: .seconds(3))
            }
        }
    }

    /// Returns false when the server could not be asked.
    @discardableResult
    func refreshFocus() async -> Bool {
        guard let focus = try? await api.focus() else { return false }
        let ended = self.focus?.session?.id != focus.session?.id || self.focus?.state != focus.state
        setFocus(focus)
        if ended { reloadAll() }
        return true
    }

    /// Starts, stops or rests. The answer is the timer's new state.
    @discardableResult
    func focusAction(_ operation: @Sendable (APIClient) async throws -> Focus) async -> Bool {
        guard case .success(let focus) = await write(operation) else { return false }
        setFocus(focus)
        return true
    }
}
