import Foundation

/// The server's clock, estimated between two answers of `GET /focus`.
///
/// The timer lives on the server. A countdown is the session's `end` minus the server's time, and
/// the server's time is its last `now` plus what a monotonic clock says has passed since. The
/// device's wall clock, which may be wrong or be changed, never enters into it.
struct ServerClock: Equatable, Sendable {
    let serverNow: Date
    let anchor: ContinuousClock.Instant

    init(serverNow: Date, anchor: ContinuousClock.Instant = .now) {
        self.serverNow = serverNow
        self.anchor = anchor
    }

    func now(at instant: ContinuousClock.Instant = .now) -> Date {
        let elapsed = anchor.duration(to: instant)
        let seconds = Double(elapsed.components.seconds) + Double(elapsed.components.attoseconds) / 1e18
        return serverNow.addingTimeInterval(seconds)
    }
}

enum FocusMath {
    /// Seconds until the session ends by the server's clock. Never negative.
    static func secondsLeft(end: Date, now: Date) -> TimeInterval {
        max(0, end.timeIntervalSince(now))
    }

    /// The share of the session still to run: 1 at the start, 0 at the end.
    static func fractionLeft(end: Date, plannedMinutes: Int, now: Date) -> Double {
        guard plannedMinutes > 0 else { return 0 }
        return min(1, secondsLeft(end: end, now: now) / Double(plannedMinutes * 60))
    }

    /// A countdown as `MM:SS`. A second that has begun still shows.
    static func mmss(_ seconds: TimeInterval) -> String {
        let s = max(0, Int(seconds.rounded(.up)))
        return String(format: "%02d:%02d", s / 60, s % 60)
    }

    /// An estimate expressed in tomatoes; 0 without an estimate.
    static func tomatoesNeeded(estimateMinutes: Int?, tomatoMinutes: Int) -> Int {
        guard let estimateMinutes, estimateMinutes > 0, tomatoMinutes > 0 else { return 0 }
        return (estimateMinutes + tomatoMinutes - 1) / tomatoMinutes
    }

    /// The top of the chart's scale: an even number, at least 4, that holds the busiest day.
    static func chartScale(_ counts: [Int]) -> Int {
        max(4, ((counts.max() ?? 0) + 1) / 2 * 2)
    }
}
