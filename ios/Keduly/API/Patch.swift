import Foundation

/// One field of a `PATCH` body. The server reads an omitted field as "unchanged" and JSON `null`
/// as "clear it", so the two must not be confused the way an optional would confuse them.
enum Patch<Value: Encodable & Sendable>: Sendable {
    case unchanged
    case clear
    case set(Value)

    /// A value to store, or `nil` to clear the field.
    init(_ value: Value?) {
        self = value.map(Patch.set) ?? .clear
    }
}

extension Patch: Equatable where Value: Equatable {}

extension KeyedEncodingContainer {
    mutating func encode<Value>(_ patch: Patch<Value>, forKey key: Key) throws {
        switch patch {
        case .unchanged: break
        case .clear: try encodeNil(forKey: key)
        case .set(let value): try encode(value, forKey: key)
        }
    }
}

struct ItemPatch: Encodable, Sendable {
    var title: Patch<String> = .unchanged
    var notes: Patch<String> = .unchanged
    var projectId: Patch<String> = .unchanged
    var headingId: Patch<String> = .unchanged
    var estimateMinutes: Patch<Int> = .unchanged
    var plannedDate: Patch<Day> = .unchanged
    var evening: Patch<Bool> = .unchanged
    var dueDate: Patch<Day> = .unchanged
    var dueTime: Patch<String> = .unchanged
    var important: Patch<Bool> = .unchanged
    var status: Patch<ItemStatus> = .unchanged

    enum CodingKeys: String, CodingKey {
        case title, notes, projectId, headingId, estimateMinutes, plannedDate, evening, dueDate, dueTime, important, status
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(title, forKey: .title)
        try c.encode(notes, forKey: .notes)
        try c.encode(projectId, forKey: .projectId)
        try c.encode(headingId, forKey: .headingId)
        try c.encode(estimateMinutes, forKey: .estimateMinutes)
        try c.encode(plannedDate, forKey: .plannedDate)
        try c.encode(evening, forKey: .evening)
        try c.encode(dueDate, forKey: .dueDate)
        try c.encode(dueTime, forKey: .dueTime)
        try c.encode(important, forKey: .important)
        try c.encode(status, forKey: .status)
    }
}

/// The writable fields of an event. Creating sends what is set; changing sends what changed.
struct EventPatch: Encodable, Sendable {
    var title: Patch<String> = .unchanged
    var projectId: Patch<String> = .unchanged
    var notes: Patch<String> = .unchanged
    var location: Patch<String> = .unchanged
    var allDay: Patch<Bool> = .unchanged
    var start: Patch<Date> = .unchanged
    var end: Patch<Date> = .unchanged
    var startDate: Patch<Day> = .unchanged
    var endDate: Patch<Day> = .unchanged

    enum CodingKeys: String, CodingKey {
        case title, projectId, notes, location, allDay, start, end, startDate, endDate
    }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(title, forKey: .title)
        try c.encode(projectId, forKey: .projectId)
        try c.encode(notes, forKey: .notes)
        try c.encode(location, forKey: .location)
        try c.encode(allDay, forKey: .allDay)
        try c.encode(start, forKey: .start)
        try c.encode(end, forKey: .end)
        try c.encode(startDate, forKey: .startDate)
        try c.encode(endDate, forKey: .endDate)
    }
}

struct ProjectPatch: Encodable, Sendable {
    var name: Patch<String> = .unchanged
    var color: Patch<ProjectColor> = .unchanged
    var areaId: Patch<String> = .unchanged
    var notes: Patch<String> = .unchanged

    enum CodingKeys: String, CodingKey { case name, color, areaId, notes }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(name, forKey: .name)
        try c.encode(color, forKey: .color)
        try c.encode(areaId, forKey: .areaId)
        try c.encode(notes, forKey: .notes)
    }
}

/// What a focus session was for. `itemId` puts it on an item; `title` and `projectId` are for free focus only.
struct SessionPatch: Encodable, Sendable {
    var itemId: Patch<String> = .unchanged
    var title: Patch<String> = .unchanged
    var projectId: Patch<String> = .unchanged

    enum CodingKeys: String, CodingKey { case itemId, title, projectId }

    func encode(to encoder: Encoder) throws {
        var c = encoder.container(keyedBy: CodingKeys.self)
        try c.encode(itemId, forKey: .itemId)
        try c.encode(title, forKey: .title)
        try c.encode(projectId, forKey: .projectId)
    }
}

struct UserPatch: Encodable, Sendable {
    var name: String?
    var workStart: String?
    var workEnd: String?
    var focusMinutes: Int?
    var restMinutes: Int?
    var longRestMinutes: Int?
    var roundSize: Int?
}

struct NewItem: Encodable, Sendable {
    var title: String
    var projectId: String?
    var headingId: String?
    var estimateMinutes: Int?
    var plannedDate: Day?
    var dueDate: Day?
    var important: Bool?
}
