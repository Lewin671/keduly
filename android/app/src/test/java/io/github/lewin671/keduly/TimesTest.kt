package io.github.lewin671.keduly

import io.github.lewin671.keduly.core.DaySpan
import io.github.lewin671.keduly.core.Times
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.YearMonth
import java.time.ZoneId
import java.util.TimeZone
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class TimesTest {
    private val shanghai = ZoneId.of("Asia/Shanghai")
    private val la = ZoneId.of("America/Los_Angeles")
    private lateinit var deviceZone: TimeZone

    // The device sits in a zone that is neither of the account zones under test.
    @Before fun setDeviceZone() {
        deviceZone = TimeZone.getDefault()
        TimeZone.setDefault(TimeZone.getTimeZone("Europe/Berlin"))
    }

    @After fun restoreDeviceZone() = TimeZone.setDefault(deviceZone)

    @Test fun showsWallClockTimeInTheAccountZone() {
        assertEquals("15:30", Times.hhmm("2026-10-13T07:30:00Z", shanghai))
        assertEquals("00:30", Times.hhmm("2026-10-13T07:30:00Z", la))
        assertEquals(LocalDate.of(2026, 10, 14), Times.wall("2026-10-13T16:00:00Z", shanghai).toLocalDate())
        assertEquals(LocalDate.of(2026, 10, 12), Times.wall("2026-10-13T06:59:00Z", la).toLocalDate())
    }

    @Test fun writesWallClockTimeBackAsUtc() {
        assertEquals("2026-10-13T07:30:00Z", Times.utc(LocalDate.of(2026, 10, 13), LocalTime.of(15, 30), shanghai))
        // Pacific time is UTC-7 in summer and UTC-8 in winter.
        assertEquals("2026-07-01T16:00:00Z", Times.utc(LocalDate.of(2026, 7, 1), LocalTime.of(9, 0), la))
        assertEquals("2026-12-01T17:00:00Z", Times.utc(LocalDate.of(2026, 12, 1), LocalTime.of(9, 0), la))
        assertEquals("2026-10-13T07:30:00Z", Times.utc(Instant.parse("2026-10-13T07:30:00.987Z")))
    }

    @Test fun dayBoundsFollowDaylightSaving() {
        // 8 March 2026 has 23 hours in Los Angeles and 1 November has 25.
        assertEquals("2026-03-08T08:00:00Z" to "2026-03-09T07:00:00Z", Times.utcRange(LocalDate.of(2026, 3, 8), LocalDate.of(2026, 3, 9), la))
        assertEquals("2026-11-01T07:00:00Z" to "2026-11-02T08:00:00Z", Times.utcRange(LocalDate.of(2026, 11, 1), LocalDate.of(2026, 11, 2), la))
        assertEquals("2026-10-12T16:00:00Z" to "2026-10-13T16:00:00Z", Times.utcRange(LocalDate.of(2026, 10, 13), LocalDate.of(2026, 10, 14), shanghai))
    }

    @Test fun placesAnEventOnTheDaysItTouches() {
        // 22:30 to 00:30 in Shanghai.
        val start = Instant.parse("2026-10-13T14:30:00Z")
        val end = Instant.parse("2026-10-13T16:30:00Z")
        assertEquals(DaySpan(1350, 1440, startsEarlier = false, continues = true), Times.spanOnDay(start, end, LocalDate.of(2026, 10, 13), shanghai))
        assertEquals(DaySpan(0, 30, startsEarlier = true, continues = false), Times.spanOnDay(start, end, LocalDate.of(2026, 10, 14), shanghai))
        assertNull(Times.spanOnDay(start, end, LocalDate.of(2026, 10, 15), shanghai))
        // The same instants are one afternoon in Los Angeles.
        assertEquals(DaySpan(450, 570), Times.spanOnDay(start, end, LocalDate.of(2026, 10, 13), la))
    }

    @Test fun anEventEndingAtMidnightStaysOnItsDay() {
        val start = Instant.parse("2026-10-13T14:00:00Z")
        val end = Instant.parse("2026-10-13T16:00:00Z")
        assertEquals(DaySpan(1320, 1440), Times.spanOnDay(start, end, LocalDate.of(2026, 10, 13), shanghai))
        assertNull(Times.spanOnDay(start, end, LocalDate.of(2026, 10, 14), shanghai))
    }

    @Test fun readsTheClockOnTheNightDaylightSavingEnds() {
        // 01:30 happens twice on 1 November 2026 in Los Angeles; both read 01:30.
        assertEquals("01:30", Times.hhmm("2026-11-01T08:30:00Z", la))
        assertEquals("01:30", Times.hhmm("2026-11-01T09:30:00Z", la))
        assertEquals(90, Times.minutesOfDay(Instant.parse("2026-11-01T09:30:00Z"), la))
    }

    @Test fun allDayEventsIncludeTheirEndDate() {
        val day = { d: Int -> LocalDate.of(2026, 10, d) }
        assertFalse(Times.allDayCovers("2026-10-11", "2026-10-13", day(10)))
        assertTrue(Times.allDayCovers("2026-10-11", "2026-10-13", day(11)))
        assertTrue(Times.allDayCovers("2026-10-11", "2026-10-13", day(13)))
        assertFalse(Times.allDayCovers("2026-10-11", "2026-10-13", day(14)))
        assertEquals(3, Times.allDayLength("2026-10-11", "2026-10-13"))
        assertEquals(1, Times.allDayLength("2026-10-11", "2026-10-11"))
        // Across the autumn change of clocks the length is still counted in calendar days.
        assertEquals(3, Times.allDayLength("2026-10-31", "2026-11-02"))
    }

    @Test fun calendarArithmeticDoesNotDependOnTheLengthOfADay() {
        assertEquals(LocalDate.of(2026, 3, 9), LocalDate.of(2026, 3, 8).plusDays(1))
        assertEquals(2, Times.daysBetween(LocalDate.of(2026, 10, 31), LocalDate.of(2026, 11, 2)))
        assertEquals(-12, Times.daysBetween(LocalDate.of(2026, 10, 13), LocalDate.of(2026, 10, 1)))
    }

    @Test fun weeksStartOnMonday() {
        assertEquals(LocalDate.of(2026, 10, 5), Times.weekStart(LocalDate.of(2026, 10, 11)))
        assertEquals(LocalDate.of(2026, 10, 12), Times.weekStart(LocalDate.of(2026, 10, 12)))
        assertEquals(LocalDate.of(2026, 10, 11), Times.weekDays(LocalDate.of(2026, 10, 7)).last())
    }

    @Test fun monthGridCoversWholeWeeks() {
        val october = Times.monthGrid(YearMonth.of(2026, 10))
        assertEquals(35, october.size)
        assertEquals(LocalDate.of(2026, 9, 28), october.first())
        assertEquals(LocalDate.of(2026, 11, 1), october.last())
        // February 2027 starts on a Monday and fills exactly four weeks.
        assertEquals(28, Times.monthGrid(YearMonth.of(2027, 2)).size)
        assertEquals(42, Times.monthGrid(YearMonth.of(2026, 8)).size)
    }
}
