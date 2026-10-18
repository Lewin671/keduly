import XCTest
import UserNotifications
@testable import Keduly

/// Every object of docs/api.md decodes from JSON as the server sends it.
final class DecodingTests: XCTestCase {
    private func decode<T: Decodable>(_ json: String, key: String? = nil, as type: T.Type = T.self) throws -> T {
        try APIClient.decodeBody(Data(json.utf8), key: key)
    }

    private func instant(_ string: String) -> Date {
        try! Date(string, strategy: .iso8601)
    }

    func testUser() throws {
        let user: User = try decode(Fixtures.user, key: "user")
        XCTAssertEqual(user.email, "me@example.com")
        XCTAssertEqual(user.timezone, "Asia/Shanghai")
        XCTAssertFalse(user.timezoneAuto)
        XCTAssertEqual(user.workStart, "09:00")
        XCTAssertEqual(user.workEnd, "18:00")
        XCTAssertEqual([user.focusMinutes, user.restMinutes, user.longRestMinutes, user.roundSize], [25, 5, 15, 4])
        XCTAssertEqual(user.createdAt, instant("2026-10-01T00:00:00Z"))
    }

    func testAreaProjectHeading() throws {
        let area: Area = try decode(Fixtures.area, key: "area")
        XCTAssertEqual(area.name, "工作")
        let project: Project = try decode(Fixtures.project, key: "project")
        XCTAssertNil(project.areaId)
        XCTAssertEqual(project.color, .blue)
        XCTAssertEqual(project.openCount, 7)
        XCTAssertEqual(project.doneCount, 3)
        XCTAssertFalse(project.archived)
        let heading: Heading = try decode(Fixtures.heading, key: "heading")
        XCTAssertEqual(heading.projectId, "cccccccccccccccc")
    }

    func testEveryProjectColour() throws {
        for color in ["blue", "indigo", "orange", "teal", "green", "pink", "purple", "brown"] {
            let json = Fixtures.project.replacingOccurrences(of: "\"blue\"", with: "\"\(color)\"")
            let project: Project = try decode(json, key: "project")
            XCTAssertEqual(project.color.rawValue, color)
        }
    }

    func testItem() throws {
        let item: Item = try decode(Fixtures.item, key: "item")
        XCTAssertEqual(item.title, "实现 CalDAV 同步")
        XCTAssertEqual(item.projectId, "cccccccccccccccc")
        XCTAssertNil(item.headingId)
        XCTAssertEqual(item.estimateMinutes, 90)
        XCTAssertEqual(item.plannedDate, Day("2026-10-13"))
        XCTAssertEqual(item.dueDate, Day("2026-10-13"))
        XCTAssertEqual(item.dueTime, "18:00")
        XCTAssertTrue(item.important)
        XCTAssertEqual(item.status, .open)
        XCTAssertNil(item.completedAt)
        XCTAssertEqual(item.block?.eventId, "ffffffffffffffff")
        XCTAssertEqual(item.block?.start, instant("2026-10-13T02:00:00Z"))
        XCTAssertEqual(item.suggestion?.actor, ChangeActor(kind: .agent, name: "Claude Code · MacBook"))
        XCTAssertEqual(item.suggestion?.end, instant("2026-10-13T07:30:00Z"))
        XCTAssertEqual(item.focus, ItemFocus(tomatoes: 2, minutes: 50))
        XCTAssertEqual(item.createdBy.kind, .user)
        // A timestamp with a fraction of a second is still a timestamp.
        XCTAssertEqual(item.updatedAt.timeIntervalSince(instant("2026-10-11T08:00:00Z")), 0.123, accuracy: 0.001)
    }

    func testItemWithNothingOptional() throws {
        let item: Item = try decode(Fixtures.bareItem, key: "item")
        XCTAssertNil(item.projectId)
        XCTAssertNil(item.estimateMinutes)
        XCTAssertNil(item.plannedDate)
        XCTAssertNil(item.dueDate)
        XCTAssertNil(item.dueTime)
        XCTAssertNil(item.block)
        XCTAssertNil(item.suggestion)
        XCTAssertTrue(item.isDone)
        XCTAssertEqual(item.completedAt, instant("2026-10-11T08:00:00Z"))
    }

    func testEvent() throws {
        let event: Event = try decode(Fixtures.event, key: "event")
        XCTAssertEqual(event.title, "设计评审")
        XCTAssertFalse(event.allDay)
        XCTAssertEqual(event.start, instant("2026-10-13T02:00:00Z"))
        XCTAssertEqual(event.end, instant("2026-10-13T03:30:00Z"))
        XCTAssertNil(event.startDate)
        XCTAssertNil(event.rrule)
        XCTAssertEqual(event.status, .confirmed)
        XCTAssertNil(event.itemDone)
        XCTAssertFalse(event.readonly)
        XCTAssertEqual(event.createdBy.kind, .caldav)
    }

    func testAllDayEvent() throws {
        let event: Event = try decode(Fixtures.allDayEvent, key: "event")
        XCTAssertTrue(event.allDay)
        XCTAssertNil(event.start)
        XCTAssertNil(event.end)
        XCTAssertEqual(event.startDate, Day("2026-10-11"))
        XCTAssertEqual(event.endDate, Day("2026-10-12"))
    }

    func testRecurringInstancesAndPendingEntriesAreToldApart() throws {
        let events: [Event] = try decode(Fixtures.events, key: "events")
        XCTAssertEqual(events.count, 4)
        // The two instances share the series id; the instance tells them apart.
        XCTAssertEqual(events[0].id, events[1].id)
        XCTAssertTrue(events[0].recurring && events[0].readonly)
        XCTAssertEqual(events[0].rrule, "FREQ=WEEKLY;BYDAY=MO")
        XCTAssertEqual(events[1].instance, instant("2026-10-19T02:00:00Z"))
        XCTAssertNotEqual(events[0].entryKey, events[1].entryKey)
        // A pending move shows the event twice: leaving its old place, tentative at the new one.
        XCTAssertEqual(events[2].status, .leaving)
        XCTAssertEqual(events[3].status, .tentative)
        XCTAssertEqual(events[3].suggestionId, "gggggggggggggggg")
        XCTAssertEqual(events[3].itemDone, false)
        XCTAssertEqual(Set(events.map(\.entryKey)).count, 4)
    }

    func testFocusSessionAndFocus() throws {
        let session: FocusSession = try decode(Fixtures.session, key: "session")
        XCTAssertEqual(session.kind, .work)
        XCTAssertEqual(session.plannedMinutes, 25)
        XCTAssertTrue(session.completed)
        XCTAssertEqual(session.end, instant("2026-10-13T02:25:00Z"))

        let focus: Focus = try decode(Fixtures.focus, key: "focus")
        XCTAssertEqual(focus.state, .work)
        XCTAssertEqual(focus.session, session)
        XCTAssertEqual(focus.tomatoesToday, 3)
        XCTAssertEqual(focus.minutesToday, 80)
        XCTAssertEqual(focus.roundSize, 4)
        XCTAssertEqual(focus.roundDone, 3)
        XCTAssertEqual(focus.restMinutes, 5)
        XCTAssertEqual(focus.now, instant("2026-10-13T02:06:18Z"))

        let idle: Focus = try decode(Fixtures.idleFocus, key: "focus")
        XCTAssertEqual(idle.state, .idle)
        XCTAssertNil(idle.session)
        XCTAssertEqual(idle.roundSize, 6)
        XCTAssertEqual(idle.restMinutes, 10)
    }

    func testEveryFocusState() throws {
        for state in ["idle", "work", "over", "rest"] {
            let focus: Focus = try decode(Fixtures.focus.replacingOccurrences(of: "\"state\": \"work\"", with: "\"state\": \"\(state)\""), key: "focus")
            XCTAssertEqual(focus.state.rawValue, state)
        }
    }

    func testSuggestionsOfEveryKind() throws {
        let suggestions: [Suggestion] = try decode(Fixtures.suggestions, key: "suggestions")
        XCTAssertEqual(suggestions.map(\.kind), [.scheduleItem, .createItem, .createEvent, .moveEvent, .deleteEvent, .deleteItem])
        XCTAssertEqual(suggestions.map(\.status), [.pending, .accepted, .rejected, .pending, .pending, .pending])
        XCTAssertEqual(suggestions[0].title, "写周报")
        XCTAssertEqual(suggestions[0].start, instant("2026-10-13T06:00:00Z"))
        XCTAssertNil(suggestions[0].decidedAt)
        XCTAssertEqual(suggestions[1].item?.title, "订机票")
        XCTAssertEqual(suggestions[1].item?.dueDate, Day("2026-10-20"))
        XCTAssertEqual(suggestions[1].item?.important, true)
        XCTAssertEqual(suggestions[1].decidedAt, instant("2026-10-13T00:20:00Z"))
        XCTAssertEqual(suggestions[2].event?.allDay, true)
        XCTAssertEqual(suggestions[2].event?.startDate, Day("2026-10-20"))
        XCTAssertEqual(suggestions[3].eventId, "ffffffffffffffff")
    }

    func testActivity() throws {
        let page: ActivityPage = try decode(Fixtures.activity)
        XCTAssertEqual(page.nextCursor, "abc")
        XCTAssertEqual(page.activities[0].action, "item.create")
        XCTAssertEqual(page.activities[0].reason, "为下周的评审做准备")
        XCTAssertTrue(page.activities[0].undoable)
        XCTAssertNil(page.activities[1].reason)
        XCTAssertTrue(page.activities[1].undone)
        XCTAssertEqual(page.activities[1].actor.kind, .system)
    }

    func testTokens() throws {
        let tokens: [Token] = try decode(Fixtures.tokens, key: "tokens")
        XCTAssertEqual(tokens.map(\.kind), [.agent, .caldav])
        XCTAssertEqual(tokens.map(\.scope), [.write, .read])
        XCTAssertTrue(tokens[0].confirmDelete)
        XCTAssertNil(tokens[0].lastUsedAt)
        XCTAssertNotNil(tokens[1].lastUsedAt)
    }

    func testConfigBootstrapAndCounts() throws {
        let config: ServerConfig = try decode(Fixtures.config)
        XCTAssertEqual(config.registration, .open)
        let boot: Bootstrap = try decode(Fixtures.bootstrap)
        XCTAssertEqual(boot.user.focusMinutes, 30)
        XCTAssertTrue(boot.user.timezoneAuto)
        XCTAssertEqual(boot.areas.count, 1)
        XCTAssertEqual(boot.counts, Counts(inbox: 2, today: 6, pending: 4))
        XCTAssertEqual(boot.revision, 41)
        let counts: CountsResponse = try decode(Fixtures.counts)
        XCTAssertEqual(counts.revision, 42)
    }

    func testViews() throws {
        let page: ItemPage = try decode(Fixtures.itemPage)
        XCTAssertEqual(page.total, 1)
        XCTAssertNil(page.nextCursor)

        let detail: ProjectDetail = try decode(Fixtures.projectDetail)
        XCTAssertEqual(detail.project.color, .brown)
        XCTAssertEqual(detail.headings.first?.name, "同步")
        XCTAssertEqual(detail.upcomingEvents.count, 1)
        XCTAssertEqual(detail.unplannedCount, 4)
        XCTAssertEqual(detail.focusWeekMinutes, 300)

        let today: TodayResponse = try decode(Fixtures.today)
        XCTAssertEqual(today.date, Day("2026-10-13"))
        XCTAssertEqual(today.freeMinutes, 255)
        XCTAssertEqual(today.unplannedMinutes, 40)
        XCTAssertEqual(today.items.count, 1)
        XCTAssertEqual(today.events.count, 1)

        let days: [UpcomingDay] = try decode(Fixtures.upcoming, key: "days")
        XCTAssertEqual(days.first?.date, Day("2026-10-14"))

        let overview: [OverviewProject] = try decode(Fixtures.overview, key: "projects")
        XCTAssertEqual(overview.first?.total, 9)

        let matrix: Quadrants = try decode(Fixtures.matrix, key: "quadrants")
        XCTAssertEqual(matrix[.do].total, 8)
        XCTAssertEqual(matrix[.do].unplanned, 7)
        XCTAssertEqual(matrix[.do].items.count, 1)
        XCTAssertEqual(matrix[.plan].total, 3)
        XCTAssertEqual(matrix[.quick].total, 1)
        XCTAssertEqual(matrix[.later].total, 0)

        let slots: [FreeSlot] = try decode(Fixtures.slots, key: "slots")
        XCTAssertEqual(slots.first?.end, instant("2026-10-13T10:00:00Z"))
    }

    func testFocusStats() throws {
        let stats: FocusStats = try decode(Fixtures.stats)
        XCTAssertEqual(stats.days.map(\.date), [Day("2026-10-07")!, Day("2026-10-08")!])
        XCTAssertEqual(stats.days[0].projects, [FocusAmount(projectId: "cccccccccccccccc", tomatoes: 2, minutes: 50)])
        XCTAssertNil(stats.projects[1].projectId)
        XCTAssertEqual(stats.tomatoes, 22)
        XCTAssertEqual(stats.minutes, 562)
        XCTAssertEqual(stats.streak, 7)
    }

    func testSomethingElseIsABadResponse() {
        XCTAssertThrowsError(try decode(Fixtures.failure, key: "item", as: Item.self)) { error in
            XCTAssertEqual(error as? APIError, .badResponse)
        }
    }
}

extension DecodingTests {
    func testHabitOccurrenceKeepsMidnightDateAndActualInstant() throws {
        let json = #"{"habit_id":"h","plan_date":"2026-10-13","date":"2026-10-13","title":"Bed","notes":"","project_id":null,"timezone":"Asia/Shanghai","time":"00:00","day_offset":1,"duration_minutes":0,"scheduled_at":"2026-10-13T16:00:00Z","end_at":"2026-10-13T16:00:00Z","remind_at":null,"status":"done","completed_at":"2026-10-13T16:12:00Z","skip_reason":""}"#
        let value: HabitOccurrence = try APIClient.decodeBody(Data(json.utf8))
        XCTAssertEqual(value.date,"2026-10-13")
        XCTAssertEqual(value.dayOffset,1)
        XCTAssertEqual(value.completedAt?.timeIntervalSince(value.scheduledAt!),720)
    }
}

extension DecodingTests {
    @MainActor func testHabitNotificationBoundIncludesRecentlyDueRecords() throws {
        let json = #"{"habit_id":"h","plan_date":"2026-10-13","date":"2026-10-13","title":"Walk","notes":"","project_id":null,"timezone":"UTC","time":"00:00","day_offset":0,"duration_minutes":0,"scheduled_at":"2026-10-13T16:00:00Z","end_at":"2026-10-13T16:00:00Z","remind_at":"2026-10-13T16:00:00Z","status":"open","completed_at":null,"skip_reason":""}"#
        let rows: [HabitOccurrence] = try (0..<100).map { n in try APIClient.decodeBody(Data(json.replacingOccurrences(of:"\"h\"",with:"\"h\(n)\"").utf8)) }
        let selected=HabitNotifications.select(rows,now:ISO8601DateFormatter().date(from:"2026-10-13T16:00:30Z")!)
        XCTAssertEqual(selected.count,60)
        XCTAssertTrue(HabitNotifications.select(rows,now:ISO8601DateFormatter().date(from:"2026-09-28T16:00:00Z")!).isEmpty)
        XCTAssertTrue(HabitNotifications.select(rows,now:ISO8601DateFormatter().date(from:"2026-10-13T16:02:00Z")!).isEmpty)
    }
}

extension DecodingTests {
    @MainActor func testRevokedAndUnrequestedPermissionDisableLocalOptIn() {
        let before=UserDefaults.standard.bool(forKey:"habitReminders")
        defer { UserDefaults.standard.set(before,forKey:"habitReminders") }
        UserDefaults.standard.set(true,forKey:"habitReminders")
        XCTAssertEqual(HabitNotifications.applyPermission(.denied),L("habit.permissionDenied"))
        XCTAssertFalse(HabitNotifications.enabled)
        UserDefaults.standard.set(true,forKey:"habitReminders")
        XCTAssertEqual(HabitNotifications.applyPermission(.notDetermined),L("habit.permissionInvite"))
        XCTAssertFalse(HabitNotifications.enabled)
    }
}
