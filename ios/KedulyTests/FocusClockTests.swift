import XCTest
@testable import Keduly

/// The countdown comes from the server's clock and a monotonic one, never from the device's wall clock.
final class FocusClockTests: XCTestCase {
    private func instant(_ string: String) -> Date {
        try! Date(string, strategy: .iso8601)
    }

    func testCountsDownFromTheServersNowWhateverTheDeviceClockSays() {
        // The device may be hours off: nothing here reads Date().
        let serverNow = instant("2026-10-13T02:00:00Z")
        let end = instant("2026-10-13T02:25:00Z")
        let anchor = ContinuousClock.now
        let clock = ServerClock(serverNow: serverNow, anchor: anchor)

        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: clock.now(at: anchor)), 1500, accuracy: 0.001)
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: clock.now(at: anchor + .seconds(60))), 1440, accuracy: 0.001)
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: clock.now(at: anchor + .milliseconds(1_499_500))), 0.5, accuracy: 0.001)
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: clock.now(at: anchor + .seconds(1500))), 0, accuracy: 0.001)
    }

    func testNeverGoesBelowZero() {
        let clock = ServerClock(serverNow: instant("2026-10-13T02:00:00Z"), anchor: .now)
        let end = instant("2026-10-13T02:01:00Z")
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: clock.now(at: clock.anchor + .seconds(3600))), 0)
        XCTAssertEqual(FocusMath.fractionLeft(end: end, plannedMinutes: 1, now: clock.now(at: clock.anchor + .seconds(3600))), 0)
    }

    func testANewAnswerFromTheServerReplacesTheEstimate() {
        // The first answer was read late; the next one corrects the countdown instead of adding to a counter.
        let end = instant("2026-10-13T02:25:00Z")
        let anchor = ContinuousClock.now
        let first = ServerClock(serverNow: instant("2026-10-13T02:00:00Z"), anchor: anchor)
        let second = ServerClock(serverNow: instant("2026-10-13T02:00:42Z"), anchor: anchor + .seconds(40))
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: first.now(at: anchor + .seconds(50))), 1450, accuracy: 0.001)
        XCTAssertEqual(FocusMath.secondsLeft(end: end, now: second.now(at: anchor + .seconds(50))), 1448, accuracy: 0.001)
    }

    func testFractionLeft() {
        let now = instant("2026-10-13T02:00:00Z")
        XCTAssertEqual(FocusMath.fractionLeft(end: now.addingTimeInterval(1500), plannedMinutes: 25, now: now), 1, accuracy: 0.0001)
        XCTAssertEqual(FocusMath.fractionLeft(end: now.addingTimeInterval(1125), plannedMinutes: 25, now: now), 0.75, accuracy: 0.0001)
        // A server whose clock runs ahead of the session's start never shows more than a full ring.
        XCTAssertEqual(FocusMath.fractionLeft(end: now.addingTimeInterval(1600), plannedMinutes: 25, now: now), 1)
    }

    func testCountdownText() {
        XCTAssertEqual(FocusMath.mmss(1500), "25:00")
        XCTAssertEqual(FocusMath.mmss(1121.2), "18:42")
        XCTAssertEqual(FocusMath.mmss(0.2), "00:01")
        XCTAssertEqual(FocusMath.mmss(0), "00:00")
        XCTAssertEqual(FocusMath.mmss(-5), "00:00")
        XCTAssertEqual(FocusMath.mmss(180 * 60), "180:00")
    }

    func testTomatoesNeededUsesTheUsersLength() {
        XCTAssertEqual(FocusMath.tomatoesNeeded(estimateMinutes: 90, tomatoMinutes: 25), 4)
        XCTAssertEqual(FocusMath.tomatoesNeeded(estimateMinutes: 50, tomatoMinutes: 25), 2)
        XCTAssertEqual(FocusMath.tomatoesNeeded(estimateMinutes: 90, tomatoMinutes: 45), 2)
        XCTAssertEqual(FocusMath.tomatoesNeeded(estimateMinutes: nil, tomatoMinutes: 25), 0)
    }

    func testChartScale() {
        XCTAssertEqual(FocusMath.chartScale([0, 0, 0]), 4)
        XCTAssertEqual(FocusMath.chartScale([3, 1, 4]), 4)
        XCTAssertEqual(FocusMath.chartScale([5]), 6)
        XCTAssertEqual(FocusMath.chartScale([8]), 8)
    }
}
