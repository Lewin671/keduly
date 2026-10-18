// The calendar tab: the month grid over the selected day's agenda, and the day view (a strip of
// the week's seven days over one day's time axis, which is also the week view), plus the sheets
// for an event.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Event
import io.github.lewin671.keduly.api.EventPatch
import io.github.lewin671.keduly.api.eventTimes
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.core.DaySpan
import io.github.lewin671.keduly.core.EventSpan
import io.github.lewin671.keduly.core.Placed
import io.github.lewin671.keduly.core.Times
import io.github.lewin671.keduly.core.layoutDay
import java.time.LocalDate
import java.time.LocalTime
import java.time.YearMonth
import java.time.ZoneId
import kotlinx.coroutines.launch

/** The height of one hour on the time axis, and the least a block is drawn. */
private val HourHeight = 58.dp
private val MinBlock = 24.dp
private const val MIN_MINUTES = 25 // MinBlock at HourHeight, rounded up

/** An event on one day: [span] is its part of that day, `null` for an all-day event. */
class DayEntry(val event: Event, val span: DaySpan?)

fun entriesOn(events: List<Event>, day: LocalDate, zone: ZoneId): List<DayEntry> = events.mapNotNull { event ->
    if (event.allDay) {
        val from = event.startDate ?: return@mapNotNull null
        if (Times.allDayCovers(from, event.endDate ?: from, day)) DayEntry(event, null) else null
    } else {
        val start = event.start ?: return@mapNotNull null
        val end = event.end ?: start
        Times.spanOnDay(Times.instant(start), Times.instant(end), day, zone)?.let { DayEntry(event, it) }
    }
}.sortedWith(compareBy({ it.span != null }, { it.span?.start ?: 0 }))

private fun open(app: AppModel, event: Event, day: LocalDate) {
    app.sheet.value = when {
        event.tentative || event.leaving -> Sheet.EventInfo(event)
        event.itemId != null -> Sheet.ItemDetail(event.itemId)
        event.readonly -> Sheet.EventInfo(event)
        else -> Sheet.EventForm(event, day)
    }
}

@Composable
fun CalendarScreen() {
    val app = LocalApp.current
    val f = LocalFmt.current
    val mode by app.calendarMode.collectAsStateWithLifecycle()
    val chosen by app.calendarDate.collectAsStateWithLifecycle()
    val selected = chosen ?: f.today
    val grid = Times.monthGrid(YearMonth.from(selected))
    val week = Times.weekDays(selected)
    // Only the visible range is fetched: the month's grid and the selected day's week.
    val from = minOf(grid.first(), week.first())
    val to = maxOf(grid.last(), week.last()).plusDays(1)
    val page = usePage("calendar", from, to) { api ->
        val (a, b) = Times.utcRange(from, to, f.zone)
        api.events(a, b)
    }
    val move = { step: Int -> app.calendarDate.value = if (mode == 1) selected.plusMonths(step.toLong()) else selected.plusWeeks(step.toLong()) }
    Screen(
        stringResource(R.string.date_m, selected.monthValue), small = stringResource(R.string.date_y, selected.year), back = false, bell = true,
        actions = {
            IconButton(onClick = { move(-1) }) { Icon(KIcons.Left, stringResource(if (mode == 1) R.string.cal_prev_month else R.string.cal_prev_week), tint = K.tint) }
            IconButton(onClick = { move(1) }) { Icon(KIcons.Right, stringResource(if (mode == 1) R.string.cal_next_month else R.string.cal_next_week), tint = K.tint) }
            TextButton(onClick = { app.calendarDate.value = null }) { Text(stringResource(R.string.day_today), fontSize = 17.sp, color = K.red) }
        },
    ) {
        Segments(listOf(stringResource(R.string.view_day), stringResource(R.string.view_month)), mode, Modifier.padding(horizontal = PagePad)) { app.calendarMode.value = it }
        if (page.error != null) ErrorLine(page.error, page.retry)
        val events = page.data.orEmpty()
        val swipe = Modifier.pointerInput(mode, selected) {
            var dragged = 0f
            detectHorizontalDragGestures(
                onDragStart = { dragged = 0f },
                onDragEnd = { if (dragged > 60.dp.toPx()) move(-1) else if (dragged < -60.dp.toPx()) move(1) },
            ) { _, amount -> dragged += amount }
        }
        if (mode == 1) MonthView(selected, grid, events, swipe) else DayView(selected, week, events, swipe)
    }
}

/* ---------- month ---------- */

@Composable
private fun DayNumber(day: LocalDate, selected: Boolean, dim: Boolean) {
    val f = LocalFmt.current
    val today = day == f.today
    val bg = when {
        selected && today -> K.red
        selected -> K.label
        else -> Color.Transparent
    }
    val fg = when {
        selected && today -> Color.White
        selected -> K.bg
        today -> K.red
        dim -> K.label3
        day.dayOfWeek.value >= 6 -> K.label2
        else -> K.label
    }
    Box(Modifier.size(34.dp).background(bg, CircleShape), contentAlignment = Alignment.Center) {
        Text(day.dayOfMonth.toString(), fontSize = 19.sp, color = fg, fontWeight = if (selected || today) FontWeight.SemiBold else FontWeight.Normal)
    }
}

@Composable
private fun MonthView(selected: LocalDate, grid: List<LocalDate>, events: List<Event>, swipe: Modifier) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val colors = boot?.projects.orEmpty().associate { it.id to K.project(it.color) }
    val none = K.label3
    Scroll {
        Column(swipe.layoutBleed(8.dp)) {
            Row {
                for (day in grid.take(7)) {
                    Text(f.weekdayLetter(day), fontSize = 11.sp, fontWeight = FontWeight.SemiBold, color = K.label3, textAlign = TextAlign.Center, modifier = Modifier.weight(1f).padding(bottom = 4.dp))
                }
            }
            for (row in grid.chunked(7)) {
                Row {
                    for (day in row) {
                        val entries = entriesOn(events, day, f.zone)
                        // Up to three dots per day in project colours, hollow when tentative.
                        val dots = entries.sortedBy { !it.event.allDay }.take(3)
                        Column(
                            Modifier.weight(1f).height(52.dp).clickable(role = Role.Button) { app.calendarDate.value = day }.padding(top = 3.dp),
                            horizontalAlignment = Alignment.CenterHorizontally,
                        ) {
                            DayNumber(day, day == selected, dim = day.month != selected.month)
                            Row(Modifier.padding(top = 3.dp), horizontalArrangement = Arrangement.spacedBy(3.dp)) {
                                for (entry in dots) {
                                    val color = colors[entry.event.projectId] ?: none
                                    Box(Modifier.size(6.dp).then(if (entry.event.tentative) Modifier.border(1.3.dp, color, CircleShape) else Modifier.background(color, CircleShape)))
                                }
                            }
                        }
                    }
                }
            }
        }
        val entries = entriesOn(events, selected, f.zone)
        Row(Modifier.layoutBleed(PagePad).drawBehind { drawLine(none, Offset.Zero, Offset(size.width, 0f), 0.5.dp.toPx()) }.padding(start = PagePad, end = PagePad, top = 11.dp, bottom = 9.dp), verticalAlignment = Alignment.Bottom) {
            Text(f.md(selected), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.alignByBaseline())
            Text(f.weekdayLong(selected), fontSize = 15.sp, color = K.label2, modifier = Modifier.padding(start = 8.dp).alignByBaseline())
            Spacer(Modifier.weight(1f))
            if (entries.isNotEmpty()) Text(stringResource(R.string.cal_count, entries.size), fontSize = 13.sp, color = K.label2, modifier = Modifier.alignByBaseline())
        }
        if (entries.isEmpty()) Text(stringResource(R.string.cal_none), fontSize = 15.sp, color = K.label2, modifier = Modifier.padding(vertical = 14.dp))
        entries.forEachIndexed { k, entry -> AgendaRow(entry, selected, last = k == entries.lastIndex) }
    }
}

/** One entry of a day's agenda: start and end, the project's colour, the title and where it belongs. */
@Composable
fun AgendaRow(entry: DayEntry, day: LocalDate, last: Boolean) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val event = entry.event
    val project = projectOf(event.projectId)
    val color = K.project(project?.color)
    val gap = K.bg
    val span = entry.span
    Row(Modifier.fillMaxWidth().heightIn(min = 50.dp).height(IntrinsicSize.Min).clickable { open(app, event, day) }.alpha(if (event.leaving) 0.45f else 1f)) {
        Column(Modifier.width(43.dp).padding(top = 7.dp)) {
            if (span == null) {
                Text(stringResource(R.string.cal_all_day), fontSize = 13.sp, lineHeight = 18.sp, color = K.label)
            } else {
                Text(if (span.startsEarlier) "00:00" else f.hhmm(event.start!!), fontSize = 13.sp, lineHeight = 18.sp, color = K.label)
                Text(if (span.continues) "24:00" else f.hhmm(event.end ?: event.start!!), fontSize = 13.sp, lineHeight = 18.sp, color = K.label2)
            }
        }
        Spacer(Modifier.width(11.dp))
        Box(Modifier.padding(vertical = 7.dp).width(4.dp).fillMaxHeight().clip(RoundedCornerShape(2.dp)).background(color).drawBehind {
            if (event.tentative) {
                var y = 4.dp.toPx()
                while (y < size.height) {
                    drawRect(gap, Offset(0f, y), Size(size.width, 3.dp.toPx()))
                    y += 7.dp.toPx()
                }
            }
        })
        Spacer(Modifier.width(11.dp))
        Column(Modifier.weight(1f).fillMaxHeight().then(if (last) Modifier else Modifier.hairline(K.sep)).padding(top = 6.dp, bottom = 7.dp)) {
            Text(event.title, fontSize = 17.sp, lineHeight = 22.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, textDecoration = if (event.leaving) TextDecoration.LineThrough else null)
            val facts = listOfNotNull(
                project?.name,
                if (event.allDay && event.startDate != null && event.endDate != null && event.endDate != event.startDate) "${f.md(Times.date(event.startDate))}–${f.md(Times.date(event.endDate))}" else null,
                if (event.tentative) stringResource(R.string.cal_pending) else null,
                if (span?.continues == true && event.end != null) stringResource(R.string.cal_until_next_day, f.hhmm(event.end)) else null,
                event.location.takeIf { it.isNotBlank() },
            )
            if (facts.isNotEmpty()) Text(facts.joinToString(" · "), fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

/* ---------- day ---------- */

@Composable
private fun DayView(selected: LocalDate, week: List<LocalDate>, events: List<Event>, swipe: Modifier) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val entries = entriesOn(events, selected, f.zone)
    Row(swipe.padding(horizontal = PagePad - 8.dp)) {
        for (day in week) {
            Column(Modifier.weight(1f).heightIn(min = 56.dp).clickable(role = Role.Button) { app.calendarDate.value = day }.padding(bottom = 6.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                Text(f.weekdayLetter(day), fontSize = 11.sp, lineHeight = 13.sp, fontWeight = FontWeight.SemiBold, color = K.label3, modifier = Modifier.padding(bottom = 3.dp))
                DayNumber(day, day == selected, dim = false)
            }
        }
    }
    // All-day events sit in their own rows above the axis.
    for (entry in entries.filter { it.span == null }) {
        val color = projectColor(entry.event.projectId)
        Row(Modifier.fillMaxWidth().hairlineTop(K.sep).heightIn(min = 48.dp).clickable { open(app, entry.event, selected) }.padding(start = 12.dp, end = PagePad), verticalAlignment = Alignment.CenterVertically) {
            Text(stringResource(R.string.cal_all_day), fontSize = 11.sp, color = K.label2, textAlign = TextAlign.End, modifier = Modifier.width(44.dp))
            Spacer(Modifier.width(6.dp))
            Text(
                entry.event.title, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = K.ink(color), maxLines = 1, overflow = TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f).clip(RoundedCornerShape(6.dp)).background(K.wash(color, 0.2f)).padding(horizontal = 9.dp, vertical = 4.dp),
            )
        }
    }
    val density = LocalDensity.current
    // The axis covers all 24 hours and opens at about 07:30.
    val scroll = rememberScrollState(with(density) { (HourHeight * 7.3f).roundToPx() })
    val timed = entries.filter { it.span != null }
    val blocks = remember(timed.map { it.event.key to it.span }) { layoutDay(timed, MIN_MINUTES, { it.span!!.start }, { it.span!!.end }) }
    val line = K.sep
    BoxWithConstraints(Modifier.fillMaxSize().hairlineTop(K.sep).verticalScroll(scroll)) {
        val column = maxWidth - 60.dp - 12.dp
        Box(Modifier.fillMaxWidth().padding(top = 10.dp, bottom = FabRoom).height(HourHeight * 24).drawBehind {
            for (h in 0..24) drawLine(line, Offset(58.dp.toPx(), h * HourHeight.toPx()), Offset(size.width - 12.dp.toPx(), h * HourHeight.toPx()), 0.5.dp.toPx())
        }) {
            for (h in 1..23) {
                Text("%02d:00".format(h), fontSize = 11.sp, lineHeight = 13.sp, fontWeight = FontWeight.Medium, color = K.label2, textAlign = TextAlign.End, modifier = Modifier.offset(y = HourHeight * h - 7.dp).width(50.dp))
            }
            for (placed in blocks) {
                val top = HourHeight * (placed.start / 60f) + 1.dp
                val height = HourHeight * ((placed.end - placed.start) / 60f) - 2.dp
                val width = column / placed.lanes
                Box(Modifier.offset(x = 60.dp + width * placed.lane + 1.dp, y = top).size(width - 2.dp, height)) { Block(placed, height, selected) }
            }
            if (selected == f.today) NowLine()
        }
    }
}

private fun Modifier.hairlineTop(color: Color): Modifier = drawBehind { drawLine(color, Offset.Zero, Offset(size.width, 0f), 0.5.dp.toPx()) }

/** The current time: a red line across the day with the time in a red capsule. */
@Composable
private fun NowLine() {
    val f = LocalFmt.current
    val now = java.time.Instant.ofEpochMilli(serverNow()).atZone(f.zone).toLocalTime()
    val red = K.red
    val y = HourHeight * ((now.hour * 60 + now.minute) / 60f)
    Box(Modifier.fillMaxWidth().offset(y = y - 9.dp).height(18.dp).drawBehind {
        drawLine(red, Offset(56.dp.toPx(), size.height / 2), Offset(size.width - 12.dp.toPx(), size.height / 2), 1.5.dp.toPx())
        drawCircle(red, 4.5.dp.toPx(), Offset(56.5.dp.toPx(), size.height / 2))
    }) {
        Text(
            Times.hhmm(now), fontSize = 11.sp, lineHeight = 18.sp, fontWeight = FontWeight.SemiBold, color = Color.White, textAlign = TextAlign.Center,
            modifier = Modifier.padding(start = 6.dp).width(44.dp).height(18.dp).background(red, RoundedCornerShape(9.dp)),
        )
    }
}

/** One block of the day view. */
@Composable
private fun Block(placed: Placed<DayEntry>, height: Dp, day: LocalDate) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val shape = RoundedCornerShape(7.dp)
    val entry = placed.entry
    if (entry == null) {
        // From the fourth overlapping event on, the third lane is one block that opens the list.
        Column(Modifier.fillMaxSize().clip(shape).background(K.fill).clickable { app.sheet.value = Sheet.EventList(placed.extra.map { it.event }) }.padding(start = 7.dp, top = 4.dp, end = 4.dp)) {
            Text(stringResource(R.string.cal_plus, placed.extra.size), fontSize = 13.sp, lineHeight = 17.sp, fontWeight = FontWeight.SemiBold, color = K.label2, maxLines = 1)
            if (height >= 36.dp) Text("${hm(placed.start)}–${hm(placed.end)}", fontSize = 11.5.sp, lineHeight = 15.sp, color = K.label2, maxLines = 1, overflow = TextOverflow.Clip)
        }
        return
    }
    val event = entry.event
    val span = entry.span!!
    val color = projectColor(event.projectId)
    val task = event.itemId != null && !event.tentative
    val short = height < 36.dp
    val ink = K.ink(color)
    val edge = K.wash(color, 0.62f)
    val start = if (span.startsEarlier) "00:00" else f.hhmm(event.start!!)
    val end = (if (span.continues) stringResource(R.string.cal_next_day) + " " else "") + f.hhmm(event.end ?: event.start!!)
    val time = (if (placed.lanes > 2 && !short) start else "$start–$end") + if (event.tentative) " · " + stringResource(R.string.cal_pending) else ""
    val title = TextStyle(
        fontSize = 13.sp, lineHeight = 17.sp, color = ink, fontWeight = if (event.tentative) FontWeight.Medium else FontWeight.SemiBold,
        textDecoration = if (event.leaving || event.itemDone == true) TextDecoration.LineThrough else null,
    )
    val background = when {
        event.tentative -> Modifier.background(K.wash(color, 0.05f)).dashed(edge, 7.dp)
        task -> Modifier.background(K.wash(color, 0.11f))
        else -> Modifier.background(K.wash(color, 0.18f)).drawBehind {
            drawRoundRect(color, Offset(3.dp.toPx(), 3.dp.toPx()), Size(3.dp.toPx(), size.height - 6.dp.toPx()), androidx.compose.ui.geometry.CornerRadius(2.dp.toPx()))
        }
    }
    Box(Modifier.fillMaxSize().alpha(if (event.leaving) 0.45f else 1f).clip(shape).then(background).clickable { open(app, event, day) }) {
        if (task) CheckMark(event.itemDone == true, color, 15.dp, Modifier.padding(start = 8.dp, top = if (short) 4.dp else 6.dp))
        val pad = Modifier.padding(start = if (task) 30.dp else 10.dp, end = 6.dp)
        if (short) {
            Row(pad.fillMaxHeight(), verticalAlignment = Alignment.CenterVertically) {
                Text(event.title, style = title, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                Text(time, fontSize = 11.5.sp, lineHeight = 15.sp, color = ink.copy(alpha = 0.8f), maxLines = 1, softWrap = false, modifier = Modifier.padding(start = 6.dp))
            }
        } else {
            Column(pad.padding(top = 4.dp)) {
                Text(event.title, style = title, maxLines = if (height > 56.dp) 2 else 1, overflow = TextOverflow.Ellipsis)
                Text(time, fontSize = 11.5.sp, lineHeight = 15.sp, color = ink.copy(alpha = 0.8f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
    }
}

private fun hm(minutes: Int): String = "%02d:%02d".format(minutes / 60 % 24, minutes % 60)

/* ---------- sheets ---------- */

/** What is wrong with the times of an event form, or `null`. */
private enum class TimeFault { EndBeforeStart }

@Composable
fun EventFormSheet(sheet: Sheet.EventForm, close: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    val event = sheet.event
    val nowTime = java.time.Instant.ofEpochMilli(app.serverNow()).atZone(f.zone).toLocalTime()
    val defaultStart = LocalTime.of(minOf(nowTime.hour + 1, 23), 0)
    val startAt = event?.start?.let(f::wall)
    val endAt = event?.end?.let(f::wall)
    var title by rememberSaveable { mutableStateOf(event?.title.orEmpty()) }
    var allDay by rememberSaveable { mutableStateOf(event?.allDay ?: false) }
    var startDay by remember { mutableStateOf(event?.startDate?.let(Times::date) ?: startAt?.toLocalDate() ?: sheet.date) }
    var endDay by remember { mutableStateOf(event?.endDate?.let(Times::date) ?: endAt?.toLocalDate() ?: sheet.date) }
    var startTime by remember { mutableStateOf(startAt?.toLocalTime() ?: defaultStart) }
    var endTime by remember { mutableStateOf(endAt?.toLocalTime() ?: if (defaultStart.hour == 23) LocalTime.of(23, 59) else defaultStart.plusHours(1)) }
    var projectId by rememberSaveable { mutableStateOf(event?.projectId) }
    var location by rememberSaveable { mutableStateOf(event?.location.orEmpty()) }
    var notes by rememberSaveable { mutableStateOf(event?.notes.orEmpty()) }
    var error by remember { mutableStateOf<String?>(null) }
    var picker by remember { mutableStateOf("") }
    var timesEdited by rememberSaveable { mutableStateOf(false) }
    val project = projectOf(projectId)
    val fault = when {
        allDay && endDay.isBefore(startDay) -> TimeFault.EndBeforeStart
        !allDay && !endDay.atTime(endTime).isAfter(startDay.atTime(startTime)) -> TimeFault.EndBeforeStart
        else -> null
    }
    val faultText = stringResource(R.string.event_end_before_start)

    val submit = {
        if (title.isNotBlank() && !busy) {
            if (fault != null) error = faultText
            else scope.launch {
                val span = EventSpan(startDay.atTime(startTime), endDay.atTime(endTime))
                val times = eventTimes(true, allDay, span.start, span.end, f.zone)
                val outcome = app.attempt { api ->
                    if (event == null) {
                        api.createEvent(
                            times.copy(
                                title = Opt.Put(title.trim()), projectId = projectId?.let { Opt.Put(it) } ?: Opt.Keep,
                                location = if (location.isBlank()) Opt.Keep else Opt.Put(location.trim()), notes = if (notes.isBlank()) Opt.Keep else Opt.Put(notes.trim()),
                            ),
                        )
                    } else {
                        // Only what changed is sent; a project that was taken away is sent as null. The
                        // times go only when the user edited them, so untouched instants stay exact.
                        api.updateEvent(
                            event.id,
                            eventTimes(timesEdited, allDay, span.start, span.end, f.zone).copy(
                                title = if (title.trim() != event.title) Opt.Put(title.trim()) else Opt.Keep,
                                projectId = if (projectId != event.projectId) Opt.Put(projectId) else Opt.Keep,
                                location = if (location.trim() != event.location) Opt.Put(location.trim()) else Opt.Keep,
                                notes = if (notes.trim() != event.notes) Opt.Put(notes.trim()) else Opt.Keep,
                            ),
                        )
                    }
                }
                when (outcome) {
                    is Outcome.Done -> close()
                    is Outcome.Refused -> error = outcome.message
                    Outcome.Failed -> Unit
                }
            }
        }
        Unit
    }

    val span = EventSpan(startDay.atTime(startTime), endDay.atTime(endTime))
    val apply = { next: EventSpan ->
        timesEdited = true
        startDay = next.start.toLocalDate()
        startTime = next.start.toLocalTime()
        endDay = next.end.toLocalDate()
        endTime = next.end.toLocalTime()
    }
    SheetBar(
        stringResource(if (event == null) R.string.event_new else R.string.event_edit), stringResource(if (event == null) R.string.add else R.string.save),
        enabled = title.isNotBlank() && !busy, onCancel = close, onAction = submit,
    )
    PlainField(
        title, { title = it; error = null }, stringResource(R.string.event_title), TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.SemiBold),
        Modifier.fillMaxWidth().padding(start = 4.dp, top = 8.dp, end = 4.dp), singleLine = true,
    )
    FormError(error)
    Group(Modifier.padding(top = 14.dp), cell = K.sheetCell) {
        GroupRow(
            stringResource(R.string.cal_all_day), leading = { RowIcon(KIcons.Clock, K.label2, 21.dp) },
            trailing = { Switch(allDay, { allDay = it; timesEdited = true; error = null }, colors = SwitchDefaults.colors(checkedTrackColor = K.green, checkedThumbColor = Color.White)) },
        )
        if (allDay) {
            GroupRow(stringResource(R.string.field_start_date), "${f.dayLabel(startDay)} ${f.weekdayShort(startDay)}", chevron = false) { picker = "startDay" }
            GroupRow(stringResource(R.string.field_end_date), "${f.dayLabel(endDay)} ${f.weekdayShort(endDay)}", last = true, chevron = false, valueColor = if (fault != null) K.red else K.label2) { picker = "endDay" }
        } else {
            GroupRow(stringResource(R.string.field_date), "${f.dayLabel(startDay)} ${f.weekdayShort(startDay)}", chevron = false) { picker = "startDay" }
            GroupRow(stringResource(R.string.field_start), Times.hhmm(startTime), chevron = false) { picker = "startTime" }
            GroupRow(stringResource(R.string.field_end_date), "${f.dayLabel(endDay)} ${f.weekdayShort(endDay)}", chevron = false, valueColor = if (fault != null) K.red else K.label2) { picker = "endDay" }
            GroupRow(stringResource(R.string.field_end), Times.hhmm(endTime), last = true, chevron = false, valueColor = if (fault != null) K.red else K.label2) { picker = "endTime" }
        }
    }
    Group(Modifier.padding(top = 14.dp), cell = K.sheetCell) {
        GroupRow(
            stringResource(R.string.field_project), project?.name ?: stringResource(R.string.project_none), valueDot = project?.let { K.project(it.color) },
            leading = { RowIcon(KIcons.Folder, K.label2, 21.dp) },
        ) { picker = "project" }
        Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(start = 16.dp, end = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.width(23.dp), contentAlignment = Alignment.Center) { RowIcon(KIcons.Pin, K.label2, 21.dp) }
            PlainField(location, { location = it }, stringResource(R.string.event_location), TextStyle(fontSize = 17.sp), Modifier.weight(1f).padding(start = 13.dp), singleLine = true)
        }
        Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).padding(start = 16.dp, end = 12.dp, top = 12.dp, bottom = 12.dp)) {
            Box(Modifier.width(23.dp), contentAlignment = Alignment.Center) { RowIcon(KIcons.Note, K.label2, 21.dp) }
            PlainField(notes, { notes = it }, stringResource(R.string.item_notes), TextStyle(fontSize = 17.sp, lineHeight = 22.sp), Modifier.weight(1f).padding(start = 13.dp))
        }
    }
    if (event != null) {
        PillButton(stringResource(R.string.event_delete), Modifier.fillMaxWidth().padding(top = 16.dp), enabled = !busy) { picker = "delete" }
    }
    when (picker) {
        "startDay" -> DayPickerDialog(startDay, { picker = "" }) {
            if (allDay) {
                // Moving the start keeps the length in days.
                endDay = it.plusDays(Times.daysBetween(startDay, endDay).toLong().coerceAtLeast(0))
                startDay = it
                timesEdited = true
            } else {
                apply(span.withDay(it))
            }
            picker = ""
        }
        "endDay" -> DayPickerDialog(endDay, { picker = "" }) { apply(span.withEndDay(it)); picker = "" }
        "startTime" -> TimeDialog(startTime, { picker = "" }) { apply(span.withStart(it)); picker = "" }
        "endTime" -> TimeDialog(endTime, { picker = "" }) { apply(span.withEnd(it)); picker = "" }
        "project" -> ProjectDialog(projectId, stringResource(R.string.project_none), { picker = "" }) { projectId = it; picker = "" }
        "delete" -> AlertDialog(
            onDismissRequest = { picker = "" },
            title = { Text(stringResource(R.string.event_delete_ask)) },
            confirmButton = { TextButton(enabled = !busy, onClick = { picker = ""; app.act(then = close) { it.deleteEvent(event!!.id) } }) { Text(stringResource(R.string.delete), color = K.red) } },
            dismissButton = { TextButton(onClick = { picker = "" }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

/** An event that cannot be edited here: an agent's pending suggestion, or an instance of a recurring series. */
@Composable
fun EventInfoSheet(event: Event, close: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val busy by app.busy.collectAsStateWithLifecycle()
    val project = projectOf(event.projectId)
    if(event.habitId!=null && event.habitDate!=null){
        val occurrence=usePage("habit-calendar",event.id){api->api.habitOccurrences(event.habitDate,event.habitDate).firstOrNull{it.habitId==event.habitId}}
        occurrence.data?.let{HabitOccurrenceRows(listOf(it))}
        return
    }
    val pending = event.suggestionId != null
    val suggestions = usePage("suggestions", event.suggestionId) { api -> if (pending) api.suggestions() else emptyList() }
    val suggestion = suggestions.data?.firstOrNull { it.id == event.suggestionId }
    Box(Modifier.fillMaxWidth().height(48.dp)) {
        TextButton(onClick = close, modifier = Modifier.align(Alignment.CenterEnd)) { Text(stringResource(R.string.done), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = K.tint) }
    }
    Text(event.title, fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.Bold, textDecoration = if (event.leaving) TextDecoration.LineThrough else null, modifier = Modifier.padding(horizontal = 4.dp))
    val time = when {
        event.allDay && event.startDate != null -> listOfNotNull(event.startDate, event.endDate?.takeIf { it != event.startDate }).joinToString("–") { f.dayLabel(Times.date(it)) } + " · " + stringResource(R.string.cal_all_day)
        event.start != null -> f.rangeLabel(event.start, event.end ?: event.start)
        else -> ""
    }
    Text(listOfNotNull(time, project?.name, event.location.takeIf { it.isNotBlank() }).joinToString(" · "), fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, modifier = Modifier.padding(start = 4.dp, top = 4.dp, end = 4.dp))
    if (event.notes.isNotBlank()) Text(event.notes, fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, modifier = Modifier.padding(start = 4.dp, top = 8.dp, end = 4.dp))
    if (pending) {
        Column(Modifier.padding(top = 14.dp).fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(K.sheetCell).dashed(K.label3, 14.dp).padding(14.dp)) {
            val what = when {
                suggestion?.kind == "delete_event" -> stringResource(R.string.suggest_delete_event)
                event.leaving && suggestion?.start != null -> stringResource(R.string.suggest_move, f.slotLabel(suggestion.start))
                event.leaving -> stringResource(R.string.suggest_move_away)
                suggestion?.kind == "create_event" -> stringResource(R.string.suggest_create_event, time)
                suggestion?.kind == "move_event" -> stringResource(R.string.suggest_move, time)
                else -> stringResource(R.string.suggest_schedule, time)
            }
            Text(what, fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold)
            if (suggestion != null) {
                if (suggestion.reason.isNotBlank()) Text(suggestion.reason, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
                Text("${suggestion.actor.name} · ${f.stamp(suggestion.createdAt)}", fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
            }
            Row(Modifier.padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                PillButton(stringResource(R.string.suggest_reject), Modifier.weight(1f), enabled = !busy, height = 40.dp) { app.act(then = close) { it.rejectSuggestion(event.suggestionId!!) } }
                PillButton(stringResource(R.string.suggest_accept), Modifier.weight(1f), primary = true, enabled = !busy, height = 40.dp) { app.act(then = close) { it.acceptSuggestion(event.suggestionId!!) } }
            }
        }
    } else if (event.readonly) {
        Text(stringResource(R.string.event_recurring_note), fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(start = 4.dp, top = 14.dp))
    }
}

/** The events folded into a "+N" block. */
@Composable
fun EventListSheet(events: List<Event>) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val day = app.calendarDate.collectAsStateWithLifecycle().value ?: f.today
    val entries = entriesOn(events, day, f.zone)
    Text(stringResource(R.string.cal_count, entries.size), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 4.dp, top = 14.dp, bottom = 6.dp))
    entries.forEachIndexed { k, entry -> AgendaRow(entry, day, last = k == entries.lastIndex) }
}
