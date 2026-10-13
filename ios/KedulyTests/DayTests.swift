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

    func testSwitchingATimedEventToAllDayKeepsTheDaysItCovers() {
        // 22:30 to 00:30 the next day in Shanghai covers two days; in Los Angeles the same instants are one morning.
        let start = instant("2026-10-11T14:30:00Z"), end = instant("2026-10-11T16:30:00Z")
        XCTAssertEqual(EventDates.allDay(start: start, end: end, zone: shanghai), Day("2026-10-11")!...Day("2026-10-12")!)
        XCTAssertEqual(EventDates.allDay(start: start, end: end, zone: losAngeles), Day("2026-10-11")!...Day("2026-10-11")!)
        // A date changed in the timed pickers is the date the all-day event gets.
        let moved = instant("2026-10-20T02:00:00Z")
        XCTAssertEqual(EventDates.allDay(start: moved, end: moved.addingTimeInterval(3600), zone: shanghai), Day("2026-10-20")!...Day("2026-10-20")!)
        // Ending exactly at midnight does not reach into the next day.
        XCTAssertEqual(EventDates.allDay(start: instant("2026-10-11T14:00:00Z"), end: instant("2026-10-11T16:00:00Z"), zone: shanghai), Day("2026-10-11")!...Day("2026-10-11")!)
    }

    func testSwitchingAnAllDayEventToTimedKeepsItsDaysAndTheTimesOfDay() {
        let previousStart = instant("2026-10-11T01:00:00Z"), previousEnd = instant("2026-10-11T02:00:00Z") // 09:00 and 10:00 in Shanghai
        let span = EventDates.timed(days: Day("2026-10-20")!...Day("2026-10-22")!, previousStart: previousStart, previousEnd: previousEnd, zone: shanghai)
        XCTAssertEqual(span.start, instant("2026-10-20T01:00:00Z"))
        XCTAssertEqual(span.end, instant("2026-10-22T02:00:00Z"))
        // One day: the same times on that day.
        let day = EventDates.timed(days: Day("2026-10-20")!...Day("2026-10-20")!, previousStart: previousStart, previousEnd: previousEnd, zone: shanghai)
        XCTAssertEqual(day.end.timeIntervalSince(day.start), 3600)
        // Times that would end before they start give an hour.
        let odd = EventDates.timed(days: Day("2026-10-20")!...Day("2026-10-20")!, previousStart: previousEnd, previousEnd: previousStart, zone: shanghai)
        XCTAssertEqual(odd.start, instant("2026-10-20T02:00:00Z"))
        XCTAssertEqual(odd.end, instant("2026-10-20T03:00:00Z"))
        // Across the clock change in Los Angeles the wall-clock times hold: 09:00 is 16:00Z before and 17:00Z after.
        let la = EventDates.timed(days: Day("2026-10-31")!...Day("2026-11-02")!, previousStart: instant("2026-07-01T16:00:00Z"), previousEnd: instant("2026-07-01T17:00:00Z"), zone: losAngeles)
        XCTAssertEqual(la.start, instant("2026-10-31T16:00:00Z"))
        XCTAssertEqual(la.end, instant("2026-11-02T18:00:00Z"))
        // Round trip.
        XCTAssertEqual(EventDates.allDay(start: span.start, end: span.end, zone: shanghai), Day("2026-10-20")!...Day("2026-10-22")!)
    }

    func testProjectSummary() {
        XCTAssertEqual(ProjectSummary.text(open: 7, unplanned: 4, focusWeekMinutes: 300), "7 件未完成，4 件没排进日历 · 本周已专注 5 小时")
        XCTAssertEqual(ProjectSummary.text(open: 2, unplanned: 0, focusWeekMinutes: 0), "2 件未完成，都已排进日历")
        XCTAssertEqual(ProjectSummary.text(open: 0, unplanned: 0, focusWeekMinutes: 50), "本周已专注 50 分钟")
        XCTAssertNil(ProjectSummary.text(open: 0, unplanned: 0, focusWeekMinutes: 0))
    }

    func testEveryDesignSizeFollowsDynamicType() {
        for size: CGFloat in [34, 28, 22, 20, 17, 16, 15, 13, 12, 11] { XCTAssertNotNil(AppType.style(for: size), "\(size)") }
        // The dial, the tab bar and the day numbers keep their size.
        for size: CGFloat in [72, 10, 19] { XCTAssertNil(AppType.style(for: size), "\(size)") }
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

    func testTodaySummaryUsesThePhoneWordingAndTheWebAppsCases() {
        func text(_ free: Int, _ minutes: Int, _ count: Int, _ important: Int) -> String {
            TodayBudget(freeMinutes: free, unplannedMinutes: minutes, unplanned: count, important: important, workEnd: "18:00").text
        }
        XCTAssertEqual(text(120, 0, 0, 0), "18:00 前还有 2 小时空闲，事项都已安排。")
        XCTAssertEqual(text(120, 90, 2, 1), "18:00 前还有 2 小时空闲，2 件事项没排进日历，约需 1.5 小时。")
        XCTAssertEqual(text(60, 90, 3, 1), "18:00 前还有 1 小时空闲，3 件事项要 1.5 小时，排不下。先保住 1 件重要的，其余 2 件可以改天。")
        XCTAssertEqual(text(60, 90, 2, 2), "18:00 前还有 1 小时空闲，2 件事项要 1.5 小时，排不下。需要取舍。")
        XCTAssertEqual(text(60, 90, 2, 0), "18:00 前还有 1 小时空闲，2 件事项要 1.5 小时，排不下。需要取舍。")
        XCTAssertEqual(text(0, 30, 1, 0), "18:00 前已经排满，1 件事项要 30 分钟，排不下。需要取舍。")
        XCTAssertEqual(text(0, 0, 0, 0), "18:00 前已经排满，事项都已安排。")
        let budget = TodayBudget(freeMinutes: 60, unplannedMinutes: 90, unplanned: 3, important: 1, workEnd: "18:00")
        XCTAssertTrue(budget.parts.contains(.warning("排不下")))
    }
}
