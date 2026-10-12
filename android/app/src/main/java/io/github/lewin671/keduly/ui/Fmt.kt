// How dates, times and amounts are worded. The wording follows the mockup and web/src/i18n/zh.ts.
package io.github.lewin671.keduly.ui

import android.content.res.Resources
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.core.Times
import java.time.LocalDate
import java.time.ZoneId
import java.time.ZonedDateTime
import java.util.Locale

/** Everything is worded for one account: its zone and its "today" (the date of `GET /today`). */
class Fmt(private val res: Resources, val zone: ZoneId, val today: LocalDate, val focusMinutes: Int) {
    private fun number(value: Double): String = if (value % 1.0 == 0.0) value.toInt().toString() else String.format(Locale.US, "%.1f", value)

    /** A length of time: "45 分钟", "2 小时", "1.5 小时". */
    fun dur(minutes: Int): String =
        if (minutes < 60) res.getString(R.string.dur_minutes, minutes) else res.getString(R.string.dur_hours, number(minutes / 60.0))

    /** Time spent, to the minute: "50 分钟", "2 小时", "2 小时 15 分钟". */
    fun span(minutes: Int): String {
        val m = maxOf(1, minutes)
        return when {
            m < 60 -> res.getString(R.string.dur_minutes, m)
            m % 60 == 0 -> res.getString(R.string.dur_hours, (m / 60).toString())
            else -> res.getString(R.string.dur_hours_minutes, m / 60, m % 60)
        }
    }

    fun weekdayShort(day: LocalDate): String = res.getStringArray(R.array.weekday_short)[day.dayOfWeek.value - 1]
    fun weekdayLong(day: LocalDate): String = res.getStringArray(R.array.weekday_long)[day.dayOfWeek.value - 1]
    fun weekdayLetter(day: LocalDate): String = res.getStringArray(R.array.weekday_letter)[day.dayOfWeek.value - 1]

    fun monthDay(day: LocalDate): String =
        if (day.year == today.year) res.getString(R.string.date_md, day.monthValue, day.dayOfMonth)
        else res.getString(R.string.date_ymd, day.year, day.monthValue, day.dayOfMonth)

    fun md(day: LocalDate): String = res.getString(R.string.date_md, day.monthValue, day.dayOfMonth)

    /** "今天", "明天", "昨天", otherwise the date. */
    fun dayLabel(day: LocalDate): String = when (day) {
        today -> res.getString(R.string.day_today)
        today.plusDays(1) -> res.getString(R.string.day_tomorrow)
        today.minusDays(1) -> res.getString(R.string.day_yesterday)
        else -> monthDay(day)
    }

    fun wall(timestamp: String): ZonedDateTime = Times.wall(timestamp, zone)
    fun hhmm(timestamp: String): String = Times.hhmm(timestamp, zone)
    fun day(timestamp: String): LocalDate = wall(timestamp).toLocalDate()

    /** Where a time block sits, as short as a row needs: "15:30", "明天 09:00". */
    fun whenLabel(timestamp: String): String = if (day(timestamp) == today) hhmm(timestamp) else slotLabel(timestamp)

    /** A day and a time in full: "今天 14:00", "10月21日 10:00". */
    fun slotLabel(timestamp: String): String = "${dayLabel(day(timestamp))} ${hhmm(timestamp)}"

    /** "今天 14:00–15:30", with the end's day when it is another one. */
    fun rangeLabel(start: String, end: String): String {
        val endDay = day(end)
        val tail = if (endDay == day(start)) hhmm(end) else "${dayLabel(endDay)} ${hhmm(end)}"
        return "${slotLabel(start)}–$tail"
    }

    fun timeRange(start: String, end: String): String = "${hhmm(start)}–${hhmm(end)}"

    /** When something happened: "今天 07:58", "昨天 21:14", "10月11日 20:30". */
    fun stamp(timestamp: String): String = slotLabel(timestamp)

    fun dueDay(item: Item): LocalDate? = item.dueDate?.let(Times::date)

    /** Urgent as the server counts it: overdue, or due today or tomorrow. */
    fun urgent(item: Item): Boolean = dueDay(item)?.let { !it.isAfter(today.plusDays(1)) } ?: false

    /** "逾期 2 天", "今天截止", "明天 18:00 截止", "10月18日截止". */
    fun dueText(item: Item): String? {
        val due = dueDay(item) ?: return null
        if (due.isBefore(today) && !item.done) return res.getString(R.string.item_overdue, Times.daysBetween(due, today))
        return res.getString(R.string.due_label, item.dueTime?.let { "${dayLabel(due)} $it " } ?: dayLabel(due))
    }

    /** The deadline on its own: "今天", "明天 18:00". */
    fun dueLabel(item: Item): String? {
        val due = dueDay(item) ?: return null
        return item.dueTime?.let { "${dayLabel(due)} $it" } ?: dayLabel(due)
    }

    fun count(n: Int): String = if (n > 99) "99+" else n.toString()
}
