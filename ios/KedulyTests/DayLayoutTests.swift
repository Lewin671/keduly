import XCTest
@testable import Keduly

/// The rule of DESIGN.md, "Overlaps": clusters, greedy lanes, at most three, "+N" from the fourth on.
final class DayLayoutTests: XCTestCase {
    private typealias Span = DayLayout.Span
    private typealias Block = DayLayout.Block

    private func layout(_ spans: [(Int, Int)], min: Int = 25) -> [Block] {
        DayLayout.layout(spans.map { Span(start: $0.0, end: $0.1) }, minMinutes: min)
    }

    private func entry(_ blocks: [Block], _ index: Int) -> Block? {
        blocks.first { $0.content == .entry(index) }
    }

    func testEventsThatDoNotOverlapEachTakeTheWholeColumn() {
        let blocks = layout([(480, 510), (600, 660), (780, 840)])
        XCTAssertEqual(blocks.count, 3)
        XCTAssertTrue(blocks.allSatisfy { $0.lane == 0 && $0.lanes == 1 })
    }

    func testTouchingEventsDoNotCollide() {
        let blocks = layout([(600, 660), (660, 720)])
        XCTAssertTrue(blocks.allSatisfy { $0.lane == 0 && $0.lanes == 1 })
    }

    func testTwoOverlappingEventsShareTheColumn() {
        let blocks = layout([(600, 660), (630, 690)])
        XCTAssertEqual(entry(blocks, 0)?.lane, 0)
        XCTAssertEqual(entry(blocks, 1)?.lane, 1)
        XCTAssertTrue(blocks.allSatisfy { $0.lanes == 2 })
    }

    func testAClusterIsEverythingLinkedByOverlaps() {
        // A overlaps B, B overlaps C, A does not overlap C: one cluster of two lanes, C back in lane 0.
        let blocks = layout([(600, 660), (640, 720), (670, 730), (900, 960)])
        XCTAssertEqual(entry(blocks, 0)?.lane, 0)
        XCTAssertEqual(entry(blocks, 1)?.lane, 1)
        XCTAssertEqual(entry(blocks, 2)?.lane, 0)
        XCTAssertEqual(entry(blocks, 0)?.lanes, 2)
        XCTAssertEqual(entry(blocks, 2)?.lanes, 2)
        // The event in the afternoon is a cluster of its own.
        XCTAssertEqual(entry(blocks, 3)?.lanes, 1)
    }

    func testThreeLanesAreDrawnInFull() {
        let blocks = layout([(600, 690), (615, 660), (630, 700)])
        XCTAssertEqual(blocks.count, 3)
        XCTAssertEqual(Set(blocks.map(\.lane)), [0, 1, 2])
        XCTAssertTrue(blocks.allSatisfy { $0.lanes == 3 })
    }

    func testFromTheFourthLaneOnTheThirdBecomesOneBlock() {
        // The crowded morning of the mockup: four events between 10:00 and 11:40.
        let blocks = layout([(600, 690), (615, 660), (630, 645), (640, 700)])
        XCTAssertEqual(blocks.count, 3)
        XCTAssertEqual(entry(blocks, 0)?.lane, 0)
        XCTAssertEqual(entry(blocks, 1)?.lane, 1)
        XCTAssertNil(entry(blocks, 2))
        XCTAssertNil(entry(blocks, 3))
        let more = blocks.first { if case .more = $0.content { return true } else { return false } }
        XCTAssertEqual(more?.content, .more([2, 3]))
        XCTAssertEqual(more?.lane, 2)
        XCTAssertEqual(more?.start, 630)
        XCTAssertEqual(more?.end, 700)
        XCTAssertTrue(blocks.allSatisfy { $0.lanes == 3 })
    }

    func testFiveLanesFoldThreeEvents() {
        let blocks = layout([(600, 700), (601, 700), (602, 700), (603, 700), (604, 700)])
        XCTAssertEqual(blocks.count, 3)
        XCTAssertEqual(blocks.last?.content, .more([2, 3, 4]))
    }

    func testALaneIsReusedOnceItIsFree() {
        // The third event starts after the first has ended and takes its lane back.
        let blocks = layout([(600, 630), (610, 700), (640, 700)])
        XCTAssertEqual(entry(blocks, 2)?.lane, 0)
        XCTAssertTrue(blocks.allSatisfy { $0.lanes == 2 })
    }

    func testAShortEventKeepsItsMinimumHeightAndCollidesWithIt() {
        // 13:00–13:05 is drawn 25 minutes tall, so an event at 13:10 does not fit under it.
        let blocks = layout([(780, 785), (790, 850)])
        XCTAssertEqual(entry(blocks, 0)?.end, 805)
        XCTAssertEqual(entry(blocks, 0)?.lane, 0)
        XCTAssertEqual(entry(blocks, 1)?.lane, 1)
        XCTAssertTrue(blocks.allSatisfy { $0.lanes == 2 })
        // Without the minimum height the two would not touch.
        XCTAssertTrue(layout([(780, 785), (790, 850)], min: 0).allSatisfy { $0.lanes == 1 })
    }

    func testAnEventPastMidnightEndsWithTheDay() {
        let blocks = layout([(1350, 1560)])
        XCTAssertEqual(blocks.first?.end, 1440)
    }

    func testEqualStartsPutTheLongerEventFirst() {
        let blocks = layout([(600, 630), (600, 720)])
        XCTAssertEqual(entry(blocks, 1)?.lane, 0)
        XCTAssertEqual(entry(blocks, 0)?.lane, 1)
    }

    func testNothingToLayOut() {
        XCTAssertTrue(layout([]).isEmpty)
    }
}
