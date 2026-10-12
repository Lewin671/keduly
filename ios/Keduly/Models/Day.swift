import Foundation

/// A calendar date (`YYYY-MM-DD`) with no time and no zone, as the API sends it.
/// Arithmetic is calendar arithmetic, never a number of seconds.
struct Day: Hashable, Comparable, Codable, Sendable, CustomStringConvertible {
    let year: Int
    let month: Int
    let day: Int

    init(year: Int, month: Int, day: Int) {
        self.year = year
        self.month = month
        self.day = day
    }

    init?(_ string: String) {
        let parts = string.split(separator: "-", omittingEmptySubsequences: false)
        guard parts.count == 3, parts[0].count == 4, parts[1].count == 2, parts[2].count == 2,
              let y = Int(parts[0]), let m = Int(parts[1]), let d = Int(parts[2]),
              (1...12).contains(m), (1...31).contains(d) else { return nil }
        self.init(year: y, month: m, day: d)
    }

    /// The date the instant falls on in the given zone.
    init(_ date: Date, in zone: TimeZone) {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = zone
        let c = calendar.dateComponents([.year, .month, .day], from: date)
        self.init(year: c.year!, month: c.month!, day: c.day!)
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()
        let string = try container.decode(String.self)
        guard let day = Day(string) else {
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "not a date: \(string)")
        }
        self = day
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()
        try container.encode(description)
    }

    var description: String {
        String(format: "%04d-%02d-%02d", year, month, day)
    }

    static func < (a: Day, b: Day) -> Bool {
        (a.year, a.month, a.day) < (b.year, b.month, b.day)
    }

    /// Dates carry no zone, so their arithmetic runs on a fixed one.
    private static let utc: Calendar = {
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = TimeZone(identifier: "UTC")!
        return calendar
    }()

    private var utcNoon: Date {
        Day.utc.date(from: DateComponents(year: year, month: month, day: day, hour: 12))!
    }

    private init(utc date: Date) {
        let c = Day.utc.dateComponents([.year, .month, .day], from: date)
        self.init(year: c.year!, month: c.month!, day: c.day!)
    }

    func adding(days: Int) -> Day {
        Day(utc: Day.utc.date(byAdding: .day, value: days, to: utcNoon)!)
    }

    /// The same day of another month, clamped to that month's length.
    func adding(months: Int) -> Day {
        Day(utc: Day.utc.date(byAdding: .month, value: months, to: utcNoon)!)
    }

    /// Whole days from this date to the other; negative when the other is earlier.
    func days(until other: Day) -> Int {
        Day.utc.dateComponents([.day], from: utcNoon, to: other.utcNoon).day!
    }

    /// 0 for Monday … 6 for Sunday.
    var weekdayIndex: Int {
        (Day.utc.component(.weekday, from: utcNoon) + 5) % 7
    }

    var startOfWeek: Day { adding(days: -weekdayIndex) }
    var startOfMonth: Day { Day(year: year, month: month, day: 1) }
    var endOfMonth: Day { startOfMonth.adding(months: 1).adding(days: -1) }
    var daysInMonth: Int { endOfMonth.day }
}

/// The account's time zone: every wall-clock time shown or edited goes through it.
struct Zone: Equatable, Sendable {
    let timeZone: TimeZone
    let calendar: Calendar

    init(_ timeZone: TimeZone) {
        self.timeZone = timeZone
        var calendar = Calendar(identifier: .gregorian)
        calendar.timeZone = timeZone
        calendar.firstWeekday = 2
        calendar.locale = Locale(identifier: "zh_Hans")
        self.calendar = calendar
    }

    init(identifier: String) {
        self.init(TimeZone(identifier: identifier) ?? TimeZone(identifier: "UTC")!)
    }

    func day(of date: Date) -> Day { Day(date, in: timeZone) }

    /// The first moment of the day. Where a clock change skips midnight it is the first time that exists.
    func start(of day: Day) -> Date {
        let components = DateComponents(year: day.year, month: day.month, day: day.day)
        return calendar.date(from: components) ?? calendar.startOfDay(for: Date(timeIntervalSince1970: 0))
    }

    /// The instant of a wall-clock time on a day. `minutes` counts from midnight.
    func date(on day: Day, minutes: Int) -> Date {
        let components = DateComponents(year: day.year, month: day.month, day: day.day, hour: minutes / 60, minute: minutes % 60)
        return calendar.date(from: components) ?? start(of: day)
    }

    /// Wall-clock minutes since midnight.
    func minutes(of date: Date) -> Int {
        let c = calendar.dateComponents([.hour, .minute], from: date)
        return c.hour! * 60 + c.minute!
    }

    func hhmm(_ date: Date) -> String {
        Zone.hhmm(minutes(of: date))
    }

    static func hhmm(_ minutes: Int) -> String {
        String(format: "%02d:%02d", (minutes / 60) % 24, minutes % 60)
    }

    /// Minutes since midnight of an `HH:MM` string.
    static func minutes(ofClock clock: String) -> Int? {
        let parts = clock.split(separator: ":")
        guard parts.count == 2, let h = Int(parts[0]), let m = Int(parts[1]), (0..<24).contains(h), (0..<60).contains(m) else { return nil }
        return h * 60 + m
    }

    /// The part of a day that a stretch of time covers, in wall-clock minutes (0...1440), or nil when it misses the day.
    func span(start: Date, end: Date, on day: Day) -> (start: Int, end: Int)? {
        let dayStart = self.start(of: day)
        let dayEnd = self.start(of: day.adding(days: 1))
        guard start < dayEnd, end > dayStart || (start == end && start >= dayStart) else { return nil }
        let s = start <= dayStart ? 0 : minutes(of: start)
        let e = end >= dayEnd ? 1440 : minutes(of: end)
        return (s, max(s, e))
    }

    /// The days a stretch of time touches. An end exactly at midnight does not reach into that day.
    func days(from start: Date, to end: Date) -> ClosedRange<Day> {
        let first = day(of: start)
        let last = end > start ? day(of: end.addingTimeInterval(-1)) : first
        return first...max(first, last)
    }
}
