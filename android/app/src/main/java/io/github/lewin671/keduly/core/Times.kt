// Date and time arithmetic. Every date and wall-clock time the app shows or edits is in the
// account's time zone, never the device's, so every function here takes the zone explicitly.
// Days are moved with calendar arithmetic, never by adding 24 hours.
package io.github.lewin671.keduly.core

import java.time.Instant
import java.time.LocalDate
import java.time.LocalDateTime
import java.time.LocalTime
import java.time.YearMonth
import java.time.ZoneId
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.time.temporal.ChronoUnit

object Times {
    private val hhmm: DateTimeFormatter = DateTimeFormatter.ofPattern("HH:mm")

    fun instant(timestamp: String): Instant = Instant.parse(timestamp)

    /** The wall-clock reading of an API timestamp in the account's zone. */
    fun wall(timestamp: String, zone: ZoneId): ZonedDateTime = instant(timestamp).atZone(zone)

    fun date(day: String): LocalDate = LocalDate.parse(day)

    fun time(clock: String): LocalTime = LocalTime.parse(clock, hhmm)

    fun hhmm(time: LocalTime): String = time.format(hhmm)

    fun hhmm(timestamp: String, zone: ZoneId): String = wall(timestamp, zone).toLocalTime().format(hhmm)

    /** RFC 3339 in UTC without fractional seconds, the format the API uses. */
    fun utc(instant: Instant): String = DateTimeFormatter.ISO_INSTANT.format(instant.truncatedTo(ChronoUnit.SECONDS))

    /** The API timestamp of a wall-clock time on a day in the account's zone. */
    fun utc(day: LocalDate, time: LocalTime, zone: ZoneId): String = utc(LocalDateTime.of(day, time).atZone(zone).toInstant())

    /** UTC bounds of the days [from] (inclusive) to [to] (exclusive), for `GET /events`. */
    fun utcRange(from: LocalDate, to: LocalDate, zone: ZoneId): Pair<String, String> =
        utc(from.atStartOfDay(zone).toInstant()) to utc(to.atStartOfDay(zone).toInstant())

    /** Whether an all-day event, whose end date is inclusive, covers a day. */
    fun allDayCovers(startDate: String, endDate: String, day: LocalDate): Boolean = !day.isBefore(date(startDate)) && !day.isAfter(date(endDate))

    /** How many days an all-day event lasts, both ends included. */
    fun allDayLength(startDate: String, endDate: String): Int = ChronoUnit.DAYS.between(date(startDate), date(endDate)).toInt() + 1

    /** Whole days from [a] to [b]; negative when [b] is earlier. */
    fun daysBetween(a: LocalDate, b: LocalDate): Int = ChronoUnit.DAYS.between(a, b).toInt()

    /** Monday of the week a day is in. */
    fun weekStart(day: LocalDate): LocalDate = day.minusDays((day.dayOfWeek.value - 1).toLong())

    fun weekDays(day: LocalDate): List<LocalDate> = weekStart(day).let { first -> List(7) { first.plusDays(it.toLong()) } }

    /** The weeks that cover a month, Monday first, padded with days of the neighbouring months. */
    fun monthGrid(month: YearMonth): List<LocalDate> {
        val first = weekStart(month.atDay(1))
        val cells = (Times.daysBetween(first, month.atEndOfMonth()) / 7 + 1) * 7
        return List(cells) { first.plusDays(it.toLong()) }
    }

    /**
     * The part of an interval that falls on a day, in wall-clock minutes from midnight (0–1440).
     * `null` when the interval does not touch that day. [continues] says that it runs past midnight.
     */
    fun spanOnDay(start: Instant, end: Instant, day: LocalDate, zone: ZoneId): DaySpan? {
        val dayStart = day.atStartOfDay(zone).toInstant()
        val dayEnd = day.plusDays(1).atStartOfDay(zone).toInstant()
        if (!end.isAfter(dayStart) || !start.isBefore(dayEnd)) {
            // A zero-length entry still belongs to the day it starts on.
            if (!(start == end && !start.isBefore(dayStart) && start.isBefore(dayEnd))) return null
        }
        val from = if (start.isBefore(dayStart)) 0 else minutesOfDay(start, zone)
        val to = if (!end.isBefore(dayEnd)) 1440 else minutesOfDay(end, zone)
        return DaySpan(from, maxOf(to, from), startsEarlier = start.isBefore(dayStart), continues = end.isAfter(dayEnd))
    }

    fun minutesOfDay(instant: Instant, zone: ZoneId): Int = instant.atZone(zone).toLocalTime().let { it.hour * 60 + it.minute }
}

data class DaySpan(val start: Int, val end: Int, val startsEarlier: Boolean = false, val continues: Boolean = false)

/** The start and end of a timed event while its form is being edited, in wall-clock terms. */
data class EventSpan(val start: LocalDateTime, val end: LocalDateTime) {
    /** Moving the start keeps the length; an event without a positive length gets one hour. */
    fun withStart(time: LocalTime): EventSpan {
        val length = java.time.Duration.between(start, end)
        val next = start.toLocalDate().atTime(time)
        return EventSpan(next, next.plus(if (length.isNegative || length.isZero) java.time.Duration.ofHours(1) else length))
    }

    /** An end at or before the start's time of day means the next day. */
    fun withEnd(time: LocalTime): EventSpan {
        val day = start.toLocalDate()
        return copy(end = (if (time.isAfter(start.toLocalTime())) day else day.plusDays(1)).atTime(time))
    }

    /** Moving the day keeps the times and the number of days the event spans. */
    fun withDay(day: LocalDate): EventSpan {
        val days = ChronoUnit.DAYS.between(start.toLocalDate(), end.toLocalDate()).coerceAtLeast(0)
        return EventSpan(day.atTime(start.toLocalTime()), day.plusDays(days).atTime(end.toLocalTime()))
    }
}
