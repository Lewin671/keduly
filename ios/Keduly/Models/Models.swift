import Foundation

// Objects of the HTTP API, as documented in docs/api.md. JSON keys are snake_case; the decoder
// converts them. Timestamps are `Date`, dates are `Day`, times of day stay `HH:MM` strings.

struct ChangeActor: Codable, Hashable, Sendable {
    enum Kind: String, Codable, Sendable { case user, agent, caldav, system }
    let kind: Kind
    let name: String
}

struct User: Codable, Hashable, Sendable {
    let id: String
    let email: String
    let name: String
    let timezone: String
    let timezoneAuto: Bool
    let workStart: String
    let workEnd: String
    let focusMinutes: Int
    let restMinutes: Int
    let longRestMinutes: Int
    let roundSize: Int
    let createdAt: Date
}

struct Area: Codable, Hashable, Identifiable, Sendable {
    let id: String
    let name: String
    let position: Int
}

enum ProjectColor: String, Codable, CaseIterable, Sendable {
    case blue, indigo, orange, teal, green, pink, purple, brown
}

struct Project: Codable, Hashable, Identifiable, Sendable {
    let id: String
    let areaId: String?
    let name: String
    let color: ProjectColor
    let notes: String
    let position: Int
    let archived: Bool
    let openCount: Int
    let doneCount: Int
    let createdAt: Date
    let updatedAt: Date
}

struct Heading: Codable, Hashable, Identifiable, Sendable {
    let id: String
    let projectId: String
    let name: String
    let position: Int
}

struct ItemBlock: Codable, Hashable, Sendable {
    let eventId: String
    let start: Date
    let end: Date
}

struct ItemSuggestion: Codable, Hashable, Sendable {
    let id: String
    let start: Date
    let end: Date
    let reason: String
    let actor: ChangeActor
}

/// Time spent on an item with the focus timer. A session still running is not counted.
struct ItemFocus: Codable, Hashable, Sendable {
    let tomatoes: Int
    let minutes: Int
}

enum ItemStatus: String, Codable, Sendable { case open, done }

struct Item: Codable, Hashable, Identifiable, Sendable {
    let id: String
    let projectId: String?
    let headingId: String?
    let title: String
    let notes: String
    let estimateMinutes: Int?
    let plannedDate: Day?
    let evening: Bool
    let dueDate: Day?
    let dueTime: String?
    let important: Bool
    let status: ItemStatus
    let completedAt: Date?
    let position: Int
    let block: ItemBlock?
    let suggestion: ItemSuggestion?
    let focus: ItemFocus
    let createdBy: ChangeActor
    let createdAt: Date
    let updatedAt: Date

    var isDone: Bool { status == .done }
}

enum EventStatus: String, Codable, Sendable { case confirmed, tentative, leaving }

struct Event: Codable, Hashable, Sendable {
    let id: String
    let projectId: String?
    let itemId: String?
    let title: String
    let notes: String
    let location: String
    let allDay: Bool
    let start: Date?
    let end: Date?
    let startDate: Day?
    let endDate: Day?
    let rrule: String?
    let recurring: Bool
    let instance: Date?
    let status: EventStatus
    let suggestionId: String?
    let itemDone: Bool?
    let readonly: Bool
    let createdBy: ChangeActor
    let updatedAt: Date

    /// Instances of a series share one `id`; an entry on the calendar is `id` plus `instance`.
    /// A pending move shows the same event twice, leaving and tentative, hence the status.
    var entryKey: String {
        "\(id)|\(instance.map { String($0.timeIntervalSince1970) } ?? "")|\(status.rawValue)|\(suggestionId ?? "")"
    }
}

struct FocusSession: Codable, Hashable, Identifiable, Sendable {
    enum Kind: String, Codable, Sendable { case work, rest }
    let id: String
    let kind: Kind
    let itemId: String?
    let projectId: String?
    let title: String
    let start: Date
    let end: Date
    let plannedMinutes: Int
    let completed: Bool
    let createdBy: ChangeActor
}

struct Focus: Codable, Hashable, Sendable {
    enum State: String, Codable, Sendable { case idle, work, over, rest }
    let state: State
    let session: FocusSession?
    let tomatoesToday: Int
    let minutesToday: Int
    let roundSize: Int
    let roundDone: Int
    let restMinutes: Int
    let now: Date
}

/// The fields a `create_item` suggestion carries for the item it would make.
struct SuggestedItem: Codable, Hashable, Sendable {
    let title: String?
    let projectId: String?
    let notes: String?
    let estimateMinutes: Int?
    let plannedDate: Day?
    let dueDate: Day?
    let important: Bool?
}

/// The fields a `create_event` suggestion carries for the event it would make.
struct SuggestedEvent: Codable, Hashable, Sendable {
    let title: String?
    let projectId: String?
    let allDay: Bool?
    let start: Date?
    let end: Date?
    let startDate: Day?
    let endDate: Day?
}

struct Suggestion: Codable, Hashable, Identifiable, Sendable {
    enum Status: String, Codable, Sendable { case pending, accepted, rejected }
    enum Kind: String, Codable, Sendable {
        case scheduleItem = "schedule_item"
        case createItem = "create_item"
        case createEvent = "create_event"
        case moveEvent = "move_event"
        case deleteEvent = "delete_event"
        case deleteItem = "delete_item"
    }
    let id: String
    let status: Status
    let kind: Kind
    let actor: ChangeActor
    let reason: String
    let title: String
    let itemId: String?
    let eventId: String?
    let start: Date?
    let end: Date?
    let item: SuggestedItem?
    let event: SuggestedEvent?
    let createdAt: Date
    let decidedAt: Date?
}

struct Activity: Codable, Hashable, Identifiable, Sendable {
    let id: String
    let actor: ChangeActor
    let action: String
    let summary: String
    let reason: String?
    let undoable: Bool
    let undone: Bool
    let createdAt: Date
}

struct Token: Codable, Hashable, Identifiable, Sendable {
    enum Kind: String, Codable, Sendable { case agent, caldav }
    enum Scope: String, Codable, Sendable { case read, write }
    let id: String
    let name: String
    let kind: Kind
    let scope: Scope
    let confirmDelete: Bool
    let lastUsedAt: Date?
    let createdAt: Date
}

struct Counts: Codable, Hashable, Sendable {
    let inbox: Int
    let today: Int
    let pending: Int
}

// MARK: - Responses that carry more than one object

struct ServerConfig: Codable, Sendable {
    enum Registration: String, Codable, Sendable { case open, closed }
    let registration: Registration
    let version: String
}

struct Bootstrap: Codable, Sendable {
    let user: User
    let areas: [Area]
    let projects: [Project]
    let headings: [Heading]
    let counts: Counts
    let revision: Int
}

struct CountsResponse: Codable, Sendable {
    let counts: Counts
    let revision: Int
}

struct ItemPage: Codable, Sendable {
    let items: [Item]
    let nextCursor: String?
    let total: Int
}

struct ProjectDetail: Codable, Sendable {
    let project: Project
    let headings: [Heading]
    let upcomingEvents: [Event]
    let unplannedCount: Int
    let focusWeekMinutes: Int
}

struct TodayResponse: Codable, Sendable {
    let date: Day
    let items: [Item]
    let overdue: [Item]
    let events: [Event]
    let freeMinutes: Int
    let unplannedMinutes: Int
}

struct UpcomingDay: Codable, Hashable, Sendable {
    let date: Day
    let events: [Event]
    let items: [Item]
}

struct OverviewProject: Codable, Sendable {
    let projectId: String
    let total: Int
    let items: [Item]
}

enum QuadrantKey: String, CaseIterable, Sendable {
    case `do`, plan, quick, later
}

struct Quadrant: Codable, Sendable {
    let total: Int
    let unplanned: Int
    let items: [Item]
}

struct Quadrants: Codable, Sendable {
    let `do`: Quadrant
    let plan: Quadrant
    let quick: Quadrant
    let later: Quadrant

    subscript(key: QuadrantKey) -> Quadrant {
        switch key {
        case .do: return self.do
        case .plan: return plan
        case .quick: return quick
        case .later: return later
        }
    }
}

struct FreeSlot: Codable, Hashable, Sendable {
    let start: Date
    let end: Date
}

struct FocusAmount: Codable, Hashable, Sendable {
    let projectId: String?
    let tomatoes: Int
    let minutes: Int
}

struct FocusDay: Codable, Hashable, Sendable {
    let date: Day
    let tomatoes: Int
    let minutes: Int
    let projects: [FocusAmount]
}

/// The last 7 days, today included.
struct FocusStats: Codable, Sendable {
    let days: [FocusDay]
    let projects: [FocusAmount]
    let tomatoes: Int
    let minutes: Int
    let streak: Int
}

struct ActivityPage: Codable, Sendable {
    let activities: [Activity]
    let nextCursor: String?
}
