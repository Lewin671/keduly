import XCTest
@testable import Keduly

/// Dates and wall-clock times are the account's, whatever zone the device is in.
final class DayTests: XCTestCase {
    private let shanghai = Zone(identifier: "Asia/Shanghai")
    private let losAngeles = Zone(identifier: "America/Los_Angeles")

    private func instant(_ string: String) -> Date {
        try! Date(string, strategy: .iso8601)
    }

    func testParsesAndPrintsDates() {
        XCTAssertEqual(Day("2026-10-13"), Day(year: 2026, month: 10, day: 13))
        XCTAssertEqual(Day(year: 2026, month: 3, day: 8).description, "2026-03-08")
        XCTAssertNil(Day("2026-13-01"))
        XCTAssertNil(Day("2026-1-1"))
        XCTAssertNil(Day("tomorrow"))
    }

    func testTheDayOfAnInstantDependsOnTheAccountZone() {
        // 17:30 UTC is already the next day in Shanghai and still the morning in Los Angeles.
        let moment = instant("2026-10-12T17:30:00Z")
        XCTAssertEqual(shanghai.day(of: moment), Day("2026-10-13"))
        XCTAssertEqual(losAngeles.day(of: moment), Day("2026-10-12"))
        XCTAssertEqual(shanghai.hhmm(moment), "01:30")
        XCTAssertEqual(losAngeles.hhmm(moment), "10:30")
    }

    func testTheResultDoesNotDependOnTheDeviceZone() {
        let moment = instant("2026-10-12T17:30:00Z")
        let saved = NSTimeZone.default
        defer { NSTimeZone.default = saved }
        for identifier in ["Pacific/Kiritimati", "Pacific/Pago_Pago", "UTC"] {
            NSTimeZone.default = TimeZone(identifier: identifier)!
            XCTAssertEqual(shanghai.day(of: moment), Day("2026-10-13"), identifier)
            XCTAssertEqual(shanghai.hhmm(moment), "01:30", identifier)
            XCTAssertEqual(shanghai.date(on: Day("2026-10-13")!, minutes: 90), moment, identifier)
        }
    }

    func testWallClockTimeBecomesTheRightInstant() {
        XCTAssertEqual(shanghai.date(on: Day("2026-10-13")!, minutes: 15 * 60 + 30), instant("2026-10-13T07:30:00Z"))
        // Los Angeles is UTC-7 in summer and UTC-8 in winter.
        XCTAssertEqual(losAngeles.date(on: Day("2026-07-01")!, minutes: 9 * 60), instant("2026-07-01T16:00:00Z"))
        XCTAssertEqual(losAngeles.date(on: Day("2026-12-01")!, minutes: 9 * 60), instant("2026-12-01T17:00:00Z"))
    }

    func testDaysAroundAClockChangeAreNot24HoursLong() {
        // Clocks go forward on 8 March 2026 and back on 1 November 2026 in Los Angeles.
        let spring = Day("2026-03-08")!, autumn = Day("2026-11-01")!
        XCTAssertEqual(losAngeles.start(of: spring.adding(days: 1)).timeIntervalSince(losAngeles.start(of: spring)), 23 * 3600)
        XCTAssertEqual(losAngeles.start(of: autumn.adding(days: 1)).timeIntervalSince(losAngeles.start(of: autumn)), 25 * 3600)
        XCTAssertEqual(losAngeles.start(of: spring), instant("2026-03-08T08:00:00Z"))
        XCTAssertEqual(losAngeles.start(of: spring.adding(days: 1)), instant("2026-03-09T07:00:00Z"))
        // 09:00 on the day after is 09:00 on the wall, not 24 hours after 09:00.
        XCTAssertEqual(losAngeles.hhmm(losAngeles.date(on: spring.adding(days: 1), minutes: 540)), "09:00")
        XCTAssertEqual(losAngeles.day(of: instant("2026-03-08T07:59:59Z")), Day("2026-03-07"))
        XCTAssertEqual(losAngeles.minutes(of: instant("2026-03-08T10:30:00Z")), 3 * 60 + 30)
    }

    func testCalendarArithmetic() {
        XCTAssertEqual(Day("2026-10-31")!.adding(days: 1), Day("2026-11-01"))
        XCTAssertEqual(Day("2026-03-01")!.adding(days: -1), Day("2026-02-28"))
        XCTAssertEqual(Day("2028-02-28")!.adding(days: 1), Day("2028-02-29"))
        XCTAssertEqual(Day("2026-01-31")!.adding(months: 1), Day("2026-02-28"))
        XCTAssertEqual(Day("2026-12-15")!.adding(months: 1), Day("2027-01-15"))
        XCTAssertEqual(Day("2026-10-11")!.days(until: Day("2026-10-13")!), 2)
        XCTAssertEqual(Day("2026-10-11")!.days(until: Day("2026-09-29")!), -12)
        // Across the clock change the count is still in days.
        XCTAssertEqual(Day("2026-03-07")!.days(until: Day("2026-03-09")!), 2)
        XCTAssertLessThan(Day("2026-09-30")!, Day("2026-10-01")!)
    }

    func testWeeksStartOnMonday() {
        let sunday = Day("2026-10-11")!
        XCTAssertEqual(sunday.weekdayIndex, 6)
        XCTAssertEqual(sunday.startOfWeek, Day("2026-10-05"))
        XCTAssertEqual(Day("2026-10-12")!.weekdayIndex, 0)
        XCTAssertEqual(Day("2026-10-12")!.startOfWeek, Day("2026-10-12"))
        XCTAssertEqual(sunday.startOfMonth, Day("2026-10-01"))
        XCTAssertEqual(sunday.endOfMonth, Day("2026-10-31"))
        XCTAssertEqual(Day("2026-02-10")!.daysInMonth, 28)
    }

    func testATimedEventCoversTheDaysItTouches() {
        // 22:30 to 00:30 in Shanghai: two days. Ending exactly at midnight: one.
        XCTAssertEqual(shanghai.days(from: instant("2026-10-11T14:30:00Z"), to: instant("2026-10-11T16:30:00Z")), Day("2026-10-11")!...Day("2026-10-12")!)
        XCTAssertEqual(shanghai.days(from: instant("2026-10-11T14:30:00Z"), to: instant("2026-10-11T16:00:00Z")), Day("2026-10-11")!...Day("2026-10-11")!)
    }

    func testTheSpanOfAnEventIsCutToTheDay() throws {
        let start = instant("2026-10-11T14:30:00Z"), end = instant("2026-10-11T16:30:00Z")
        let first = try XCTUnwrap(shanghai.span(start: start, end: end, on: Day("2026-10-11")!))
        XCTAssertEqual(first.start, 22 * 60 + 30)
        XCTAssertEqual(first.end, 1440)
        let second = try XCTUnwrap(shanghai.span(start: start, end: end, on: Day("2026-10-12")!))
        XCTAssertEqual(second.start, 0)
        XCTAssertEqual(second.end, 30)
        XCTAssertNil(shanghai.span(start: start, end: end, on: Day("2026-10-13")!))
    }

    func testAnInclusiveMultiDayAllDayEventIsOnEachOfItsDays() throws {
        let json = Fixtures.allDayEvent.data(using: .utf8)!
        let event: Event = try APIClient.decodeBody(json, key: "event")
        // The index runs in a zone west of UTC, where a naive conversion would lose a day.
        let index = CalendarIndex(events: [event], zone: losAngeles)
        XCTAssertTrue(index.entries(on: Day("2026-10-10")!).isEmpty)
        XCTAssertEqual(index.entries(on: Day("2026-10-11")!).map(\.id), [event.id])
        XCTAssertEqual(index.entries(on: Day("2026-10-12")!).map(\.id), [event.id])
        XCTAssertTrue(index.entries(on: Day("2026-10-13")!).isEmpty)
    }

    func testLabels() {
        let today = Day("2026-10-11")!
        XCTAssertEqual(Format.dayLabel(today, today: today), "今天")
        XCTAssertEqual(Format.dayLabel(Day("2026-10-12")!, today: today), "明天")
        XCTAssertEqual(Format.dayLabel(Day("2026-10-10")!, today: today), "昨天")
        XCTAssertEqual(Format.dayLabel(Day("2026-10-07")!, today: today), "周三")
        XCTAssertEqual(Format.dayLabel(Day("2026-10-21")!, today: today), "10月21日")
        XCTAssertEqual(Format.dayLabel(Day("2027-01-02")!, today: today), "2027年1月2日")
        XCTAssertEqual(Format.slotLabel(instant("2026-10-11T06:00:00Z"), zone: shanghai, today: today), "今天 14:00")
        XCTAssertEqual(Format.rangeLabel(start: instant("2026-10-11T06:00:00Z"), end: instant("2026-10-11T07:30:00Z"), zone: shanghai, today: today), "今天 14:00–15:30")
        XCTAssertEqual(Format.dur(45), "45 分钟")
        XCTAssertEqual(Format.dur(120), "2 小时")
        XCTAssertEqual(Format.dur(90), "1.5 小时")
        XCTAssertEqual(Format.span(125), "2 小时 5 分钟")
        XCTAssertEqual(Format.cap(120), "99+")
    }

    func testTodaySummaryFollowsTheWebApp() {
        func text(_ free: Int, _ minutes: Int, _ count: Int, _ important: Int) -> String {
            TodayBudget(freeMinutes: free, unplannedMinutes: minutes, unplanned: count, important: important, workEnd: "18:00").text
        }
        XCTAssertEqual(text(120, 0, 0, 0), "18:00 前还有 2 小时空闲，事项都已安排")
        XCTAssertEqual(text(120, 90, 2, 1), "18:00 前还有 2 小时空闲，2 件事项未安排（1.5 小时）")
        XCTAssertEqual(text(60, 90, 3, 1), "18:00 前还有 1 小时空闲，3 件事项未安排（1.5 小时），排不下：先保住 1 件重要的，其余 2 件可以往后放")
        XCTAssertEqual(text(60, 90, 2, 2), "18:00 前还有 1 小时空闲，2 件事项未安排（1.5 小时），排不下，需要取舍")
        XCTAssertEqual(text(0, 30, 1, 0), "18:00 前已经排满，1 件事项未安排（30 分钟），排不下，需要取舍")
        let budget = TodayBudget(freeMinutes: 60, unplannedMinutes: 90, unplanned: 3, important: 1, workEnd: "18:00")
        XCTAssertTrue(budget.parts.contains(.warning("排不下")))
    }
}
