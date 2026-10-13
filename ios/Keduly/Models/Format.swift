import Foundation

/// A string from the localisation resources, with `%@` and `%d` filled in.
func L(_ key: String, _ args: CVarArg...) -> String {
    let format = Bundle.main.localizedString(forKey: key, value: nil, table: nil)
    return args.isEmpty ? format : String(format: format, arguments: args)
}

/// How dates, times and amounts are worded in the interface.
enum Format {
    /// A length of time: "45 分钟", "2 小时", "1.5 小时".
    static func dur(_ minutes: Int) -> String {
        if minutes < 60 { return L("dur.minutes", minutes) }
        if minutes % 60 == 0 { return L("dur.hours", String(minutes / 60)) }
        return L("dur.hours", String(format: "%.1f", Double(minutes) / 60))
    }

    /// Time spent, to the minute: "50 分钟", "2 小时", "2 小时 15 分钟". Under a minute counts as one.
    static func span(_ minutes: Int) -> String {
        let m = max(1, minutes)
        if m < 60 { return L("dur.minutes", m) }
        return m % 60 == 0 ? L("dur.hours", String(m / 60)) : L("dur.hoursMinutes", m / 60, m % 60)
    }

    /// Counts above 99 are not spelled out.
    static func cap(_ n: Int) -> String {
        n > 99 ? "99+" : String(n)
    }

    static func weekdayShort(_ day: Day) -> String {
        L("day.short").split(separator: ",").map(String.init)[day.weekdayIndex]
    }

    static func weekdayLong(_ day: Day) -> String {
        L("day.long").split(separator: ",").map(String.init)[day.weekdayIndex]
    }

    static var weekdayLetters: [String] {
        L("day.letters").split(separator: ",").map(String.init)
    }

    static func monthDay(_ day: Day, today: Day) -> String {
        day.year == today.year ? L("date.md", day.month, day.day) : L("date.ymd", day.year, day.month, day.day)
    }

    /// "今天", "明天", "昨天", the weekday within the current week, otherwise the date.
    static func dayLabel(_ day: Day, today: Day) -> String {
        if day == today { return L("day.today") }
        if day == today.adding(days: 1) { return L("day.tomorrow") }
        if day == today.adding(days: -1) { return L("day.yesterday") }
        if day.startOfWeek == today.startOfWeek { return weekdayShort(day) }
        return monthDay(day, today: today)
    }

    /// Where a time block sits, as short as a row needs: "15:30", "明天 09:00", "10月21日".
    static func whenLabel(_ start: Date, zone: Zone, today: Day) -> String {
        let day = zone.day(of: start)
        if day == today { return zone.hhmm(start) }
        if day == today.adding(days: 1) || day.startOfWeek == today.startOfWeek { return "\(dayLabel(day, today: today)) \(zone.hhmm(start))" }
        return monthDay(day, today: today)
    }

    /// A day and a time in full: "今天 14:00", "周三 10:30", "10月21日 10:00".
    static func slotLabel(_ start: Date, zone: Zone, today: Day) -> String {
        "\(dayLabel(zone.day(of: start), today: today)) \(zone.hhmm(start))"
    }

    /// A stretch of time: "今天 14:00–15:30"; the end names its day when it is another one.
    static func rangeLabel(start: Date, end: Date, zone: Zone, today: Day) -> String {
        let first = zone.day(of: start), last = zone.day(of: end)
        let tail = first == last ? zone.hhmm(end) : "\(dayLabel(last, today: today)) \(zone.hhmm(end))"
        return "\(slotLabel(start, zone: zone, today: today))–\(tail)"
    }

    static func dueLabel(date: Day, time: String?, today: Day) -> String {
        let day = dayLabel(date, today: today)
        return time.map { "\(day) \($0)" } ?? day
    }

    /// When something happened, for the activity log: "今天 07:58", "昨天 21:14", "10月11日 20:30".
    static func stamp(_ date: Date, zone: Zone, today: Day) -> String {
        let day = zone.day(of: date)
        let label = day == today ? L("day.today") : day == today.adding(days: -1) ? L("day.yesterday") : monthDay(day, today: today)
        return "\(label) \(zone.hhmm(date))"
    }
}

/// The sentence at the top of Today: how much free time is left and whether the unplanned items fit.
struct TodayBudget: Equatable, Sendable {
    /// Free minutes within working hours, counted from now.
    let freeMinutes: Int
    /// Work left on the unplanned items.
    let unplannedMinutes: Int
    /// Open daytime items with neither a time block nor a pending suggestion.
    let unplanned: Int
    /// How many of those are marked important.
    let important: Int
    /// End of the working day, `HH:MM`.
    let workEnd: String

    enum Part: Equatable, Sendable {
        case plain(String)
        case strong(String)
        case warning(String)
    }

    init(freeMinutes: Int, unplannedMinutes: Int, unplanned: Int, important: Int, workEnd: String) {
        self.freeMinutes = freeMinutes
        self.unplannedMinutes = unplannedMinutes
        self.unplanned = unplanned
        self.important = important
        self.workEnd = workEnd
    }

    init(_ today: TodayResponse, workEnd: String) {
        let open = (today.items + today.overdue).filter { !$0.isDone && !$0.evening && $0.block == nil && $0.suggestion == nil }
        self.init(freeMinutes: today.freeMinutes, unplannedMinutes: today.unplannedMinutes, unplanned: open.count,
                  important: open.filter(\.important).count, workEnd: workEnd)
    }

    /// The sentence in parts: the free time stands out, "排不下" is a warning.
    var parts: [Part] {
        var out: [Part] = []
        if freeMinutes <= 0 {
            out.append(.plain(L("today.full", workEnd)))
        } else {
            out.append(.plain(L("today.freeBefore", workEnd)))
            out.append(.strong(Format.dur(freeMinutes)))
            out.append(.plain(L("today.freeAfter")))
        }
        guard unplanned > 0 else {
            out.append(.plain(L("today.allPlanned")))
            return out
        }
        guard unplannedMinutes > freeMinutes else {
            out.append(.plain(L("today.unplanned", unplanned, Format.dur(unplannedMinutes))))
            return out
        }
        let rest = unplanned - important
        out.append(.plain(L("today.needs", unplanned, Format.dur(unplannedMinutes))))
        out.append(.warning(L("today.noFit")))
        out.append(.plain(important > 0 && rest > 0 ? L("today.triage", important, rest) : L("today.overflow")))
        return out
    }

    var text: String {
        parts.map { part in
            switch part {
            case .plain(let s), .strong(let s), .warning(let s): return s
            }
        }.joined()
    }
}
