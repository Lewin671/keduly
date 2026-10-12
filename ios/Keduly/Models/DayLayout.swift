import Foundation

/// Where the blocks of one day sit on the time axis when they overlap.
///
/// A cluster of overlapping blocks shares the column in lanes, each block in the first lane that
/// is free. At most three lanes are drawn: when a cluster needs more, the third lane becomes one
/// "+N" block that stands for everything from the third lane on. A short block is drawn at a
/// readable minimum height, and that height is what counts for collisions.
enum DayLayout {
    struct Span: Equatable, Sendable {
        /// Minutes from midnight.
        let start: Int
        let end: Int
    }

    struct Block: Equatable, Sendable {
        enum Content: Equatable, Sendable {
            /// The index of the span in the input.
            case entry(Int)
            /// The spans folded into one "+N" block, in the order they are drawn.
            case more([Int])
        }
        let content: Content
        /// What is drawn: the start, and the end after the minimum height was applied.
        let start: Int
        let end: Int
        let lane: Int
        let lanes: Int
    }

    static let maxLanes = 3

    /// `minMinutes` is the minimum height expressed in minutes of the axis.
    static func layout(_ spans: [Span], minMinutes: Int) -> [Block] {
        struct Placed {
            let index: Int
            let start: Int
            let end: Int
            var lane = 0
        }
        let sorted = spans.enumerated()
            .map { Placed(index: $0.offset, start: $0.element.start, end: max(min($0.element.end, 1440), $0.element.start + minMinutes)) }
            .sorted { ($0.start, -$0.end, $0.index) < ($1.start, -$1.end, $1.index) }

        var out: [Block] = []
        var cluster: [Placed] = []
        var clusterEnd = -1

        func flush() {
            guard !cluster.isEmpty else { return }
            var laneEnds: [Int] = []
            for i in cluster.indices {
                let lane = laneEnds.firstIndex { $0 <= cluster[i].start } ?? laneEnds.count
                if lane == laneEnds.count { laneEnds.append(cluster[i].end) } else { laneEnds[lane] = cluster[i].end }
                cluster[i].lane = lane
            }
            let lanes = min(laneEnds.count, maxLanes)
            let folded = cluster.filter { laneEnds.count > maxLanes && $0.lane >= maxLanes - 1 }
            for e in cluster where !(laneEnds.count > maxLanes && e.lane >= maxLanes - 1) {
                out.append(Block(content: .entry(e.index), start: e.start, end: e.end, lane: e.lane, lanes: lanes))
            }
            if !folded.isEmpty {
                out.append(Block(content: .more(folded.map(\.index)), start: folded.map(\.start).min()!, end: folded.map(\.end).max()!, lane: maxLanes - 1, lanes: lanes))
            }
            cluster = []
            clusterEnd = -1
        }

        for e in sorted {
            if e.start >= clusterEnd { flush() }
            cluster.append(e)
            clusterEnd = max(clusterEnd, e.end)
        }
        flush()
        return out
    }
}
