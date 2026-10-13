import Foundation

// One method per endpoint the app uses, in the order of docs/api.md.

private struct Empty: Decodable, Sendable {}

extension APIClient {
    // MARK: Service and account

    func config() async throws -> ServerConfig {
        try await get("/config")
    }

    func login(email: String, password: String) async throws -> User {
        struct Body: Encodable, Sendable { let email, password: String }
        let data = try await perform("POST", "/auth/login", body: Self.encode(Body(email: email, password: password)), expectsSession: false)
        return try Self.decodeBody(data, key: "user")
    }

    func register(email: String, password: String, name: String, timezone: String) async throws -> User {
        struct Body: Encodable, Sendable { let email, password, name, timezone: String }
        let data = try await perform("POST", "/auth/register", body: Self.encode(Body(email: email, password: password, name: name, timezone: timezone)), expectsSession: false)
        return try Self.decodeBody(data, key: "user")
    }

    func logout() async throws {
        try await sendDiscarding("POST", "/auth/logout")
    }

    func updateMe(_ patch: UserPatch) async throws -> User {
        try await send("PATCH", "/me", body: patch, key: "user")
    }

    func bootstrap() async throws -> Bootstrap {
        try await get("/bootstrap")
    }

    func counts() async throws -> CountsResponse {
        try await get("/counts")
    }

    // MARK: Projects

    func createProject(name: String, color: ProjectColor?, areaId: String?, notes: String) async throws -> Project {
        struct Body: Encodable, Sendable {
            let name: String
            let color: ProjectColor?
            let areaId: String?
            let notes: String
        }
        return try await send("POST", "/projects", body: Body(name: name, color: color, areaId: areaId, notes: notes), key: "project")
    }

    func project(_ id: String) async throws -> ProjectDetail {
        try await get("/projects/\(id)")
    }

    func updateProject(_ id: String, _ patch: ProjectPatch) async throws -> Project {
        try await send("PATCH", "/projects/\(id)", body: patch, key: "project")
    }

    // MARK: Items

    struct ItemQuery: Sendable {
        var status: String?
        var projectId: String?
        var quadrant: QuadrantKey?
        var search: String?
        var limit: Int?
        var cursor: String?
    }

    func items(_ q: ItemQuery) async throws -> ItemPage {
        var query: [String: String] = [:]
        query["status"] = q.status
        query["project_id"] = q.projectId
        query["quadrant"] = q.quadrant?.rawValue
        query["q"] = q.search
        query["limit"] = q.limit.map(String.init)
        query["cursor"] = q.cursor
        return try await get("/items", query)
    }

    /// The first `count` items of a list, however many pages that takes: the server gives at most
    /// 200 at a time. `nextCursor` of the result continues after the last item returned.
    func items(_ q: ItemQuery, upTo count: Int) async throws -> ItemPage {
        var query = q
        query.cursor = nil
        var items: [Item] = []
        var total = 0
        var cursor: String?
        repeat {
            query.limit = min(Self.pageLimit, count - items.count)
            query.cursor = cursor
            let page = try await self.items(query)
            items += page.items
            total = page.total
            cursor = page.nextCursor
            if page.items.isEmpty { break }
        } while cursor != nil && items.count < count
        return ItemPage(items: items, nextCursor: cursor, total: total)
    }

    static let pageLimit = 200

    func item(_ id: String) async throws -> Item {
        try await get("/items/\(id)", key: "item")
    }

    func createItem(_ item: NewItem) async throws -> Item {
        try await send("POST", "/items", body: item, key: "item")
    }

    func updateItem(_ id: String, _ patch: ItemPatch) async throws -> Item {
        try await send("PATCH", "/items/\(id)", body: patch, key: "item")
    }

    func deleteItem(_ id: String) async throws {
        try await sendDiscarding("DELETE", "/items/\(id)")
    }

    func scheduleItem(_ id: String, start: Date, end: Date) async throws -> Item {
        struct Body: Encodable, Sendable { let start, end: Date }
        return try await send("POST", "/items/\(id)/schedule", body: Body(start: start, end: end), key: "item")
    }

    func unscheduleItem(_ id: String) async throws -> Item {
        try await send("DELETE", "/items/\(id)/schedule", key: "item")
    }

    // MARK: Views

    func today() async throws -> TodayResponse {
        try await get("/today")
    }

    func upcoming(from: Day, to: Day) async throws -> [UpcomingDay] {
        try await get("/upcoming", ["from": from.description, "to": to.description], key: "days")
    }

    func overview() async throws -> [OverviewProject] {
        try await get("/overview", key: "projects")
    }

    func matrix() async throws -> Quadrants {
        try await get("/matrix", key: "quadrants")
    }

    // MARK: Events

    func events(from: Date, to: Date) async throws -> [Event] {
        try await get("/events", ["from": from.formatted(.iso8601), "to": to.formatted(.iso8601)], key: "events")
    }

    func createEvent(_ event: EventPatch) async throws -> Event {
        try await send("POST", "/events", body: event, key: "event")
    }

    func updateEvent(_ id: String, _ patch: EventPatch) async throws -> Event {
        try await send("PATCH", "/events/\(id)", body: patch, key: "event")
    }

    func deleteEvent(_ id: String) async throws {
        try await sendDiscarding("DELETE", "/events/\(id)")
    }

    func freeSlots(on day: Day, minutes: Int) async throws -> [FreeSlot] {
        try await get("/free", ["date": day.description, "duration": String(minutes)], key: "slots")
    }

    // MARK: Focus

    func focus() async throws -> Focus {
        try await get("/focus", key: "focus")
    }

    /// A tomato on an item, or free focus, which may carry a title and a project of its own.
    func startFocus(itemId: String?, title: String? = nil, projectId: String? = nil) async throws -> Focus {
        struct Body: Encodable, Sendable {
            let itemId: String?
            let title: String?
            let projectId: String?
        }
        return try await send("POST", "/focus/start", body: Body(itemId: itemId, title: itemId == nil ? title : nil, projectId: itemId == nil ? projectId : nil), key: "focus")
    }

    func stopFocus() async throws -> Focus {
        try await send("POST", "/focus/stop", key: "focus")
    }

    func restFocus() async throws -> Focus {
        try await send("POST", "/focus/rest", key: "focus")
    }

    func focusSessions(from: Day, to: Day) async throws -> [FocusSession] {
        try await get("/focus/sessions", ["from": from.description, "to": to.description], key: "sessions")
    }

    func updateSession(_ id: String, _ patch: SessionPatch) async throws -> FocusSession {
        try await send("PATCH", "/focus/sessions/\(id)", body: patch, key: "session")
    }

    func focusStats() async throws -> FocusStats {
        try await get("/focus/stats")
    }

    // MARK: Suggestions and activity

    func pendingSuggestions() async throws -> [Suggestion] {
        try await get("/suggestions", ["status": "pending"], key: "suggestions")
    }

    func acceptSuggestion(_ id: String) async throws {
        try await sendDiscarding("POST", "/suggestions/\(id)/accept")
    }

    func rejectSuggestion(_ id: String) async throws {
        try await sendDiscarding("POST", "/suggestions/\(id)/reject")
    }

    func activity(cursor: String?) async throws -> ActivityPage {
        var query: [String: String] = [:]
        query["cursor"] = cursor
        return try await get("/activity", query)
    }

    func undoActivity(_ id: String) async throws -> Activity {
        try await send("POST", "/activity/\(id)/undo", key: "activity")
    }

    func redoActivity(_ id: String) async throws -> Activity {
        try await send("POST", "/activity/\(id)/redo", key: "activity")
    }

    /// Accepts every pending suggestion at once.
    func acceptAllSuggestions() async throws {
        try await sendDiscarding("POST", "/suggestions/accept-all")
    }
}
