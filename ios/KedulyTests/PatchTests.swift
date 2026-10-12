import XCTest
@testable import Keduly

/// In a PATCH body an omitted field is "unchanged" and `null` is "clear it".
final class PatchTests: XCTestCase {
    private func json(_ body: any Encodable & Sendable) throws -> [String: Any] {
        try XCTUnwrap(JSONSerialization.jsonObject(with: APIClient.encode(body)) as? [String: Any])
    }

    func testAnEmptyPatchSendsNothing() throws {
        XCTAssertTrue(try json(ItemPatch()).isEmpty)
        XCTAssertTrue(try json(EventPatch()).isEmpty)
        XCTAssertTrue(try json(SessionPatch()).isEmpty)
        XCTAssertTrue(try json(ProjectPatch()).isEmpty)
        XCTAssertTrue(try json(UserPatch()).isEmpty)
    }

    func testClearingIsNullAndUnchangedIsAbsent() throws {
        let body = try json(ItemPatch(dueDate: .clear, dueTime: .clear))
        XCTAssertEqual(Set(body.keys), ["due_date", "due_time"])
        XCTAssertTrue(body["due_date"] is NSNull)
        XCTAssertTrue(body["due_time"] is NSNull)
    }

    func testSettingSendsTheValueUnderItsDocumentedName() throws {
        let body = try json(ItemPatch(title: .set("写周报"), projectId: .set("cccccccccccccccc"), estimateMinutes: .set(45),
                                      plannedDate: .set(Day("2026-10-13")!), evening: .set(true), important: .set(false), status: .set(.done)))
        XCTAssertEqual(Set(body.keys), ["title", "project_id", "estimate_minutes", "planned_date", "evening", "important", "status"])
        XCTAssertEqual(body["title"] as? String, "写周报")
        XCTAssertEqual(body["project_id"] as? String, "cccccccccccccccc")
        XCTAssertEqual(body["estimate_minutes"] as? Int, 45)
        XCTAssertEqual(body["planned_date"] as? String, "2026-10-13")
        XCTAssertEqual(body["evening"] as? Bool, true)
        XCTAssertEqual(body["important"] as? Bool, false)
        XCTAssertEqual(body["status"] as? String, "done")
    }

    func testAnOptionalBecomesSetOrClear() throws {
        XCTAssertEqual(Patch<String>(nil), .clear)
        XCTAssertEqual(Patch("x"), .set("x"))
        let moved = try json(ItemPatch(projectId: Patch(nil), headingId: .clear))
        XCTAssertTrue(moved["project_id"] is NSNull)
        XCTAssertTrue(moved["heading_id"] is NSNull)
        XCTAssertEqual(moved.count, 2)
    }

    func testEventTimesAreUTCTimestampsAndDatesAreDates() throws {
        let start = try Date("2026-10-13T02:00:00Z", strategy: .iso8601)
        let timed = try json(EventPatch(start: .set(start), end: .set(start.addingTimeInterval(5400))))
        XCTAssertEqual(timed["start"] as? String, "2026-10-13T02:00:00Z")
        XCTAssertEqual(timed["end"] as? String, "2026-10-13T03:30:00Z")
        XCTAssertEqual(timed.count, 2)

        let allDay = try json(EventPatch(projectId: .clear, allDay: .set(true), startDate: .set(Day("2026-10-11")!), endDate: .set(Day("2026-10-12")!)))
        XCTAssertEqual(allDay["all_day"] as? Bool, true)
        XCTAssertEqual(allDay["start_date"] as? String, "2026-10-11")
        XCTAssertEqual(allDay["end_date"] as? String, "2026-10-12")
        XCTAssertTrue(allDay["project_id"] is NSNull)
        XCTAssertNil(allDay["start"])
    }

    func testFilingASession() throws {
        let off = try json(SessionPatch(itemId: .clear))
        XCTAssertTrue(off["item_id"] is NSNull)
        XCTAssertEqual(off.count, 1)
        let free = try json(SessionPatch(title: .set("回邮件"), projectId: .clear))
        XCTAssertEqual(free["title"] as? String, "回邮件")
        XCTAssertTrue(free["project_id"] is NSNull)
        XCTAssertNil(free["item_id"])
    }

    func testSettingsSendOnlyWhatChanged() throws {
        let body = try json(UserPatch(focusMinutes: 1))
        XCTAssertEqual(body.count, 1)
        XCTAssertEqual(body["focus_minutes"] as? Int, 1)
        let hours = try json(UserPatch(workStart: "08:30", workEnd: "17:00"))
        XCTAssertEqual(Set(hours.keys), ["work_start", "work_end"])
    }

    func testANewItemLeavesOutWhatWasNotSet() throws {
        let body = try json(NewItem(title: "买牛奶"))
        XCTAssertEqual(body.count, 1)
        XCTAssertEqual(body["title"] as? String, "买牛奶")
        let full = try json(NewItem(title: "买牛奶", projectId: "cccccccccccccccc", estimateMinutes: 15, plannedDate: Day("2026-10-13"), dueDate: Day("2026-10-14"), important: true))
        XCTAssertEqual(Set(full.keys), ["title", "project_id", "estimate_minutes", "planned_date", "due_date", "important"])
    }
}
