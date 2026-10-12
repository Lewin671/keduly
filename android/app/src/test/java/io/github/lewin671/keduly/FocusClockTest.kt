package io.github.lewin671.keduly

import io.github.lewin671.keduly.core.FocusMath
import io.github.lewin671.keduly.core.ServerClock
import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Test

class FocusClockTest {
    private val serverNow = Instant.parse("2026-10-13T01:06:18Z").toEpochMilli()
    private val end = Instant.parse("2026-10-13T01:25:00Z").toEpochMilli()

    @Test fun countsDownFromTheServersClock() {
        val clock = ServerClock(serverNow, monotonicMillis = 5_000)
        assertEquals("18:42", FocusMath.mmss(FocusMath.secondsLeft(end, clock.now(5_000))))
        assertEquals("18:41", FocusMath.mmss(FocusMath.secondsLeft(end, clock.now(6_000))))
        assertEquals("00:00", FocusMath.mmss(FocusMath.secondsLeft(end, clock.now(5_000 + 19 * 60_000))))
    }

    @Test fun aWrongDeviceClockDoesNotMoveTheCountdown() {
        // The device believes it is three hours later than the server; only elapsed time counts.
        val deviceWallClock = serverNow + 3 * 3_600_000
        val clock = ServerClock(serverNow, monotonicMillis = 1_000)
        assertEquals(1_122L, FocusMath.secondsLeft(end, clock.now(1_000)))
        assertEquals(0L, FocusMath.secondsLeft(end, deviceWallClock))
        // Changing the device's clock mid-session changes nothing either: it is never read.
        assertEquals(1_062L, FocusMath.secondsLeft(end, clock.now(61_000)))
    }

    @Test fun aNewReplyReanchorsTheClock() {
        // Sixty seconds passed on the device but the server says only 58 did.
        val later = ServerClock(serverNow + 58_000, monotonicMillis = 61_000)
        assertEquals(1_064L, FocusMath.secondsLeft(end, later.now(61_000)))
    }

    @Test fun aSecondThatHasBegunStillShows() {
        assertEquals(1L, FocusMath.secondsLeft(end, end - 1))
        assertEquals(60L, FocusMath.secondsLeft(end, end - 59_001))
        assertEquals(0L, FocusMath.secondsLeft(end, end))
        assertEquals(0L, FocusMath.secondsLeft(end, end + 5_000))
    }

    @Test fun theDialDrainsFromOneToZero() {
        val start = end - 25 * 60_000
        assertEquals(1f, FocusMath.fractionLeft(end, 25, start), 0.0001f)
        assertEquals(0.75f, FocusMath.fractionLeft(end, 25, start + 375_000), 0.0001f)
        assertEquals(0f, FocusMath.fractionLeft(end, 25, end + 1), 0.0001f)
        // A clock that is behind the session's start never shows more than a full dial.
        assertEquals(1f, FocusMath.fractionLeft(end, 25, start - 60_000), 0.0001f)
    }

    @Test fun estimatesAreCountedInTomatoesOfTheUsersLength() {
        assertEquals(4, FocusMath.tomatoesNeeded(90, 25))
        assertEquals(2, FocusMath.tomatoesNeeded(50, 25))
        assertEquals(90, FocusMath.tomatoesNeeded(90, 1))
        assertEquals(0, FocusMath.tomatoesNeeded(null, 25))
    }

    @Test fun theChartScaleIsEvenAndAtLeastFour() {
        assertEquals(4, FocusMath.chartScale(listOf(0, 1, 3)))
        assertEquals(6, FocusMath.chartScale(listOf(5)))
        assertEquals(8, FocusMath.chartScale(listOf(8, 2)))
        assertEquals(4, FocusMath.chartScale(emptyList()))
    }
}
