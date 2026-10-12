// How overlapping entries share the day view's column (DESIGN.md, "Overlaps"): a cluster of
// overlapping entries is laid out greedily in lanes; at most three lanes are drawn, and when a
// cluster needs more, the third lane becomes one "+N" block holding everything from that lane on.
// A short entry is drawn with a minimum height, and that height counts for collisions.
package io.github.lewin671.keduly.core

const val MAX_LANES = 3

/** One block of the day view: an entry in its lane, or the "+N" block when [extra] is not empty. */
data class Placed<T>(
    val entry: T?,
    /** Minutes from midnight where the block is drawn; [end] includes the minimum height. */
    val start: Int,
    val end: Int,
    val lane: Int,
    /** How many lanes the block's cluster is divided into, 1 to [MAX_LANES]. */
    val lanes: Int,
    val extra: List<T> = emptyList(),
)

fun <T> layoutDay(entries: List<T>, minMinutes: Int, start: (T) -> Int, end: (T) -> Int): List<Placed<T>> {
    class Box(val entry: T, val s: Int, val e: Int, var lane: Int = 0)

    val boxes = entries
        .map { Box(it, start(it), maxOf(minOf(end(it), 1440), start(it) + minMinutes)) }
        .sortedWith(compareBy<Box> { it.s }.thenByDescending { it.e })
    val out = ArrayList<Placed<T>>()
    var cluster = ArrayList<Box>()
    var until = -1

    fun flush() {
        if (cluster.isEmpty()) return
        val laneEnds = ArrayList<Int>()
        for (box in cluster) {
            var lane = laneEnds.indexOfFirst { it <= box.s }
            if (lane < 0) {
                lane = laneEnds.size
                laneEnds += box.e
            } else {
                laneEnds[lane] = box.e
            }
            box.lane = lane
        }
        val lanes = minOf(laneEnds.size, MAX_LANES)
        val folded = cluster.filter { it.lane >= MAX_LANES || (laneEnds.size > MAX_LANES && it.lane == MAX_LANES - 1) }
        for (box in cluster) if (box !in folded) out += Placed(box.entry, box.s, box.e, box.lane, lanes)
        if (folded.isNotEmpty()) {
            out += Placed(null, folded.minOf { it.s }, folded.maxOf { it.e }, MAX_LANES - 1, lanes, folded.map { it.entry })
        }
        cluster = ArrayList()
        until = -1
    }

    for (box in boxes) {
        if (box.s >= until) flush()
        cluster += box
        until = maxOf(until, box.e)
    }
    flush()
    return out
}
