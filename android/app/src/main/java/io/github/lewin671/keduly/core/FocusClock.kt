// The arithmetic of the focus timer. The server owns the timer: the app only estimates the
// server's clock and counts down to the session's end. Nothing here stores a counter.
package io.github.lewin671.keduly.core

import kotlin.math.ceil

/**
 * An estimate of the server's clock. It is anchored to `focus.now` of the last reply and advanced
 * with a monotonic clock, so neither a wrong device clock nor a change of it moves the countdown.
 */
class ServerClock(private val serverMillis: Long, private val monotonicMillis: Long) {
    fun now(monotonicNow: Long): Long = serverMillis + (monotonicNow - monotonicMillis)
}

object FocusMath {
    /** Milliseconds until the session ends. Never negative. */
    fun millisLeft(endMillis: Long, nowMillis: Long): Long = maxOf(0L, endMillis - nowMillis)

    /** Whole seconds to show: a second that has begun still shows. */
    fun secondsLeft(endMillis: Long, nowMillis: Long): Long = ceil(millisLeft(endMillis, nowMillis) / 1000.0).toLong()

    /** A countdown as `MM:SS`. */
    fun mmss(seconds: Long): String = "%02d:%02d".format(seconds / 60, seconds % 60)

    /** The share of the session still to run, 1 at the start and 0 at the end. */
    fun fractionLeft(endMillis: Long, plannedMinutes: Int, nowMillis: Long): Float =
        if (plannedMinutes <= 0) 0f else minOf(1f, millisLeft(endMillis, nowMillis) / (plannedMinutes * 60_000f))

    /** An estimate expressed in tomatoes; 0 without an estimate. */
    fun tomatoesNeeded(estimateMinutes: Int?, tomatoMinutes: Int): Int =
        if (estimateMinutes == null || estimateMinutes <= 0 || tomatoMinutes <= 0) 0 else (estimateMinutes + tomatoMinutes - 1) / tomatoMinutes

    /** The top of the chart's scale: an even number, at least 4, that holds the busiest day. */
    fun chartScale(tomatoes: List<Int>): Int = maxOf(4, ((tomatoes.maxOrNull() ?: 0) + 1) / 2 * 2)
}
