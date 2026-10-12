// The focus tab. The server owns the timer: everything here counts down to the session's end by
// the server's clock and asks the server again when the countdown reaches zero. No tomato is ever
// awarded on the phone.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Focus
import io.github.lewin671.keduly.api.FocusSession
import io.github.lewin671.keduly.api.FocusStats
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.api.SessionPatch
import io.github.lewin671.keduly.core.FocusMath
import io.github.lewin671.keduly.core.Times
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** Asks the server what happened once a running session's countdown reaches zero, and until it answers. */
@Composable
fun FocusWatcher() {
    val app = LocalApp.current
    val focus by app.focus.collectAsStateWithLifecycle()
    val f = focus ?: return
    val session = f.session ?: return
    if (f.state != "work" && f.state != "rest") return
    val ended = FocusMath.secondsLeft(Times.instant(session.end).toEpochMilli(), serverNow()) == 0L
    LaunchedEffect(session.id, f.state, ended) {
        while (ended) {
            app.refreshFocus()
            delay(2_000)
        }
    }
}

/** One ring, large light figures, nothing else inside it. */
@Composable
private fun Dial(color: Color, fraction: Float, size: Dp, content: @Composable ColumnScope.() -> Unit) {
    val track = K.fill
    Box(Modifier.size(size), contentAlignment = Alignment.Center) {
        Canvas(Modifier.fillMaxSize()) {
            val w = 9.dp.toPx()
            val box = Size(this.size.width - w, this.size.height - w)
            drawArc(track, 0f, 360f, false, Offset(w / 2, w / 2), box, style = Stroke(w))
            if (fraction > 0f) drawArc(color, -90f, 360f * fraction.coerceIn(0f, 1f), false, Offset(w / 2, w / 2), box, style = Stroke(w, cap = StrokeCap.Round))
        }
        Column(horizontalAlignment = Alignment.CenterHorizontally, content = content)
    }
}

@Composable
private fun DialTime(text: String, small: String) {
    Text(text, fontSize = 68.sp, lineHeight = 72.sp, fontWeight = FontWeight.ExtraLight, letterSpacing = (-2).sp, color = K.label, maxLines = 1)
    Text(small, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 4.dp))
}

@Composable
private fun dialSize(): Dp = if (LocalConfiguration.current.screenHeightDp >= 740) 272.dp else 224.dp

/** The marks of the current round, and how far the long rest is. */
@Composable
private fun Round(f: Focus) {
    Row(Modifier.padding(top = 18.dp), verticalAlignment = Alignment.CenterVertically) {
        // The round's length is the account's setting; many marks are drawn smaller to fit.
        val size = if (f.roundSize <= 6) 22.dp else 16.dp
        repeat(f.roundSize) { k -> Tomato(k < f.roundDone, size, Modifier.padding(end = if (f.roundSize <= 6) 7.dp else 3.dp)) }
        Text(
            when (f.roundDone) {
                f.roundSize -> stringResource(R.string.focus_round_done)
                0 -> stringResource(R.string.focus_round_size, f.roundSize)
                else -> stringResource(R.string.focus_round_left, f.roundSize - f.roundDone)
            },
            fontSize = 13.sp, color = K.label2, modifier = Modifier.padding(start = 5.dp),
        )
    }
}

/** "● project · 已得 2/4" for an item; what free focus is filed under otherwise. */
@Composable
private fun SubjectFacts(item: Item?, projectId: String?, free: FocusSession? = null) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val project = projectOf(projectId)
    Row(Modifier.padding(top = 3.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.Center) {
        if (project != null) {
            Dot(K.project(project.color))
            Spacer(Modifier.width(5.dp))
        }
        val need = FocusMath.tomatoesNeeded(item?.estimateMinutes, f.focusMinutes)
        Text(
            project?.name ?: stringResource(if (item != null) R.string.nav_inbox else R.string.focus_free_note),
            fontSize = 15.sp, lineHeight = 20.sp, color = K.label2, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false),
        )
        // The project name gives way first; the count always shows in full.
        if (item != null && (item.focus.tomatoes > 0 || need > 0)) {
            Text(
                " · " + stringResource(R.string.focus_earned_count, item.focus.tomatoes.toString() + if (need > 0) "/$need" else ""),
                fontSize = 15.sp, lineHeight = 20.sp, color = K.label2, maxLines = 1, softWrap = false,
            )
        }
        if (free != null) {
            // Free focus can be named and filed while it runs and when it has just been earned.
            Text(
                " · " + stringResource(if (free.title.isNotEmpty() || free.projectId != null) R.string.focus_refile else R.string.focus_file),
                fontSize = 15.sp, color = K.tint, modifier = Modifier.heightIn(min = 48.dp).clickable(role = Role.Button) { app.sheet.value = Sheet.Filing(free) }.wrapContentHeight(),
            )
        }
    }
}

@Composable
fun FocusScreen() {
    val app = LocalApp.current
    val nav = LocalNav.current
    val f = LocalFmt.current
    val focus by app.focus.collectAsStateWithLifecycle()
    val boot by app.boot.collectAsStateWithLifecycle()
    val busy by app.busy.collectAsStateWithLifecycle()
    val pick by app.pick.collectAsStateWithLifecycle()
    val today = usePage("focus/today") { it.today() }
    val state = focus ?: return
    val user = boot?.user ?: return
    // Today's open items, important first; without any, free focus is the only choice.
    val candidates = today.data?.let { view -> (view.items + view.overdue).filter { !it.done }.sortedByDescending { it.important } }.orEmpty()
    val chosen = pick
    val item = if (chosen == null) candidates.firstOrNull() else chosen.item?.let { picked -> candidates.firstOrNull { it.id == picked.id } ?: if (today.data == null) picked else candidates.firstOrNull() }
    val session = state.session?.takeIf { state.state != "idle" }
    Screen(
        stringResource(R.string.mode_focus), back = false, bell = true,
        actions = { IconButton(onClick = { nav.go("stats") }) { Icon(KIcons.Chart, stringResource(R.string.focus_stats), tint = K.tint) } },
    ) {
        if (today.error != null) ErrorLine(today.error, today.retry)
        Column(Modifier.fillMaxSize().padding(horizontal = PagePad), horizontalAlignment = Alignment.CenterHorizontally) {
            Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                if (session == null) {
                    Text(stringResource(R.string.focus_nth, state.tomatoesToday + 1), fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = K.label2)
                    Spacer(Modifier.height(16.dp))
                    Dial(item?.projectId?.let { projectColor(it) } ?: K.tint, 1f, dialSize()) {
                        DialTime(FocusMath.mmss(user.focusMinutes * 60L), stringResource(R.string.dur_minutes, user.focusMinutes))
                    }
                    Column(
                        Modifier.padding(top = 14.dp).heightIn(min = 48.dp).clip(RoundedCornerShape(12.dp)).clickable(role = Role.Button) { app.sheet.value = Sheet.FocusPick }.padding(horizontal = 8.dp, vertical = 4.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                item?.title ?: stringResource(R.string.focus_free), fontSize = 20.sp, lineHeight = 25.sp, fontWeight = FontWeight.SemiBold,
                                maxLines = 2, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center, modifier = Modifier.weight(1f, fill = false),
                            )
                            Icon(KIcons.Down, stringResource(R.string.focus_pick), tint = K.label3, modifier = Modifier.padding(start = 5.dp).size(14.dp))
                        }
                        SubjectFacts(item, item?.projectId)
                    }
                } else {
                    // The timer runs but was collapsed: the same dial, counting, and a way back.
                    RunningDial(state, session)
                }
                Round(state)
            }
            Text(
                if (state.minutesToday > 0) stringResource(R.string.focus_today_total, state.tomatoesToday, f.span(state.minutesToday)) else stringResource(R.string.focus_today_none),
                fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(bottom = 10.dp),
            )
            // The main button shares its row with the add button, which sits at the end.
            Box(Modifier.fillMaxWidth().padding(end = 76.dp, bottom = 19.dp)) {
                if (session == null) {
                    PillButton(stringResource(R.string.focus_start), Modifier.fillMaxWidth(), primary = true, enabled = !busy) { app.startFocus(item) }
                } else {
                    PillButton(stringResource(R.string.focus_back), Modifier.fillMaxWidth(), primary = true) { app.zen.value = true }
                }
            }
        }
    }
}

private fun focusLabel(f: Focus): Pair<Int, Int> = when (f.state) {
    "over" -> R.string.focus_earned to f.tomatoesToday
    "rest" -> (if (f.roundDone == f.roundSize) R.string.focus_long_rest else R.string.focus_rest) to 0
    else -> R.string.focus_nth to f.tomatoesToday + 1
}

@Composable
private fun focusColor(f: Focus, session: FocusSession): Color = when (f.state) {
    "over" -> K.tomato
    "rest" -> K.green
    else -> session.projectId?.let { projectColor(it) } ?: K.tint
}

@Composable
private fun RunningDial(f: Focus, session: FocusSession) {
    val fmt = LocalFmt.current
    val (label, n) = focusLabel(f)
    val color = focusColor(f, session)
    Text(stringResource(label, n), fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = K.label2)
    Spacer(Modifier.height(16.dp))
    if (f.state == "over") {
        // A finished tomato closes the ring in tomato red.
        Dial(color, 1f, dialSize()) { Tomato(size = 92.dp) }
    } else {
        val now = serverNow()
        val end = Times.instant(session.end).toEpochMilli()
        Dial(color, FocusMath.fractionLeft(end, session.plannedMinutes, now), dialSize()) {
            DialTime(
                FocusMath.mmss(FocusMath.secondsLeft(end, now)),
                if (f.state == "work") stringResource(R.string.focus_until, fmt.hhmm(session.end)) else stringResource(R.string.focus_resting),
            )
        }
    }
}

/** The running timer, over the whole screen and tinted with the project's colour. */
@Composable
fun ZenScreen(f: Focus) {
    val app = LocalApp.current
    val session = f.session ?: return
    val busy by app.busy.collectAsStateWithLifecycle()
    val color = focusColor(f, session)
    val working = f.state == "work" || f.state == "over"
    val itemPage = usePage("focus/item", session.itemId) { api -> session.itemId?.let { api.item(it) } }
    val item = itemPage.data?.takeIf { it.id == session.itemId }
    Surface(Modifier.fillMaxSize(), color = K.wash(color, 0.09f), contentColor = K.label) {
        Column(Modifier.fillMaxSize().systemBarsPadding().padding(horizontal = PagePad), horizontalAlignment = Alignment.CenterHorizontally) {
            Row(Modifier.fillMaxWidth().height(56.dp), verticalAlignment = Alignment.CenterVertically) {
                Row(
                    Modifier.heightIn(min = 48.dp).clickable(role = Role.Button) { app.zen.value = false }.padding(vertical = 6.dp).clip(RoundedCornerShape(18.dp)).background(K.fill).padding(start = 9.dp, end = 14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Icon(KIcons.Down, null, tint = K.label, modifier = Modifier.size(17.dp))
                    Text(stringResource(R.string.focus_collapse), fontSize = 15.sp, fontWeight = FontWeight.Medium, modifier = Modifier.padding(start = 3.dp, top = 8.dp, bottom = 8.dp))
                }
            }
            Column(Modifier.weight(1f), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
                RunningDial(f, session)
                if (working) {
                    Text(
                        session.title.ifEmpty { stringResource(R.string.focus_free) }, fontSize = 20.sp, lineHeight = 25.sp, fontWeight = FontWeight.SemiBold,
                        maxLines = 2, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 22.dp, start = 6.dp, end = 6.dp),
                    )
                    SubjectFacts(item, session.projectId, free = session.takeIf { it.itemId == null })
                }
                Round(f)
            }
            // A tomato on an item can end with the item being done; free focus has nothing to tick.
            val canFinish = session.itemId != null && item?.done != true
            if (f.state == "over") {
                // The rest is the main action and has a row of its own, so every label fits.
                PillButton(
                    stringResource(if (f.roundDone == f.roundSize) R.string.focus_take_long_rest else R.string.focus_take_rest, f.restMinutes),
                    Modifier.fillMaxWidth().padding(bottom = 10.dp), primary = true, enabled = !busy,
                ) { app.restFocus() }
            }
            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                when (f.state) {
                    "work" -> {
                        PillButton(stringResource(R.string.focus_give_up), Modifier.weight(1f), enabled = !busy) { app.stopFocus() }
                        if (canFinish) PillButton(stringResource(R.string.focus_finish), Modifier.weight(1f), primary = true, enabled = !busy) { app.finishFocus() }
                    }
                    "over" -> {
                        if (canFinish) PillButton(stringResource(R.string.focus_finish), Modifier.weight(1f), enabled = !busy) { app.finishFocus() }
                        PillButton(stringResource(R.string.focus_again), Modifier.weight(1f), enabled = !busy) { app.focusAgain() }
                    }
                    "rest" -> PillButton(stringResource(R.string.focus_skip_rest), Modifier.weight(1f), enabled = !busy) { app.stopFocus() }
                }
            }
            Text(
                when (f.state) {
                    "work" -> stringResource(R.string.focus_no_pause)
                    "over" -> stringResource(R.string.focus_over_note)
                    else -> ""
                },
                fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 12.dp, bottom = 24.dp),
            )
        }
    }
}

/** Picks what the next tomato is for: one of today's open items, or free focus. */
@Composable
fun FocusPickSheet(close: () -> Unit) {
    val app = LocalApp.current
    val today = usePage("focus/today") { it.today() }
    val pick by app.pick.collectAsStateWithLifecycle()
    val candidates = today.data?.let { view -> (view.items + view.overdue).filter { !it.done }.sortedByDescending { it.important } }.orEmpty()
    val current = if (pick == null) candidates.firstOrNull()?.id else pick?.item?.id
    Text(stringResource(R.string.focus_pick), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.fillMaxWidth().padding(vertical = 14.dp), textAlign = TextAlign.Center)
    Group(cell = K.sheetCell) {
        for (item in candidates) {
            val project = projectOf(item.projectId)
            GroupRow(
                item.title, last = false, leading = { Dot(K.project(project?.color), 10.dp) },
                trailing = {
                    if (item.focus.tomatoes > 0) TomatoCount(item)
                    if (item.id == current) Icon(KIcons.Check, null, tint = K.tint, modifier = Modifier.padding(start = 8.dp).size(20.dp))
                },
            ) { app.pick.value = Pick(item); close() }
        }
        GroupRow(
            stringResource(R.string.focus_free), stringResource(R.string.focus_free_note), last = true, leading = { Tomato(false, 18.dp) },
            trailing = { if (current == null) Icon(KIcons.Check, null, tint = K.tint, modifier = Modifier.padding(start = 8.dp).size(20.dp)) },
        ) { app.pick.value = Pick(null); close() }
    }
}

/* ---------- statistics ---------- */

private class StatsPage(val stats: FocusStats, val sessions: List<FocusSession>)

@Composable
fun StatsScreen() {
    val app = LocalApp.current
    val f = LocalFmt.current
    val boot by app.boot.collectAsStateWithLifecycle()
    var days by remember { mutableIntStateOf(2) }
    val page = usePage("stats", days, f.today) { api -> StatsPage(api.focusStats(), api.focusSessions(f.today.minusDays(days - 1L).toString(), f.today.toString())) }
    Screen(stringResource(R.string.focus_stats)) {
        PageBody(page) { data ->
            val stats = data.stats
            val now = app.serverNow()
            // Sessions that have ended, newest day and newest session first.
            val ended = data.sessions.filter { Times.instant(it.end).toEpochMilli() <= now }.sortedByDescending { it.start }
            if (stats.minutes == 0 && ended.isEmpty()) {
                Blank(stringResource(R.string.stats_empty), stringResource(R.string.stats_empty_tip))
                return@PageBody
            }
            Scroll {
                Row(Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 22.dp), horizontalArrangement = Arrangement.SpaceBetween) {
                    Figure(stats.days.lastOrNull()?.tomatoes?.toString() ?: "0", stringResource(R.string.stats_unit_count), stringResource(R.string.stats_today))
                    Figure(stats.tomatoes.toString(), stringResource(R.string.stats_unit_count), stringResource(R.string.stats_week))
                    Figure(average(stats), stringResource(R.string.stats_unit_count), stringResource(R.string.stats_average))
                    Figure(stats.streak.toString(), stringResource(R.string.stats_unit_days), stringResource(R.string.stats_streak))
                }
                Chart(stats)
                val colors = boot?.projects.orEmpty().associateBy { it.id }
                val top = stats.projects.maxOfOrNull { it.minutes } ?: 0
                if (stats.projects.isNotEmpty()) Sec(stringResource(R.string.stats_where))
                for (amount in stats.projects) {
                    val project = colors[amount.projectId]
                    val color = K.project(project?.color)
                    Row(Modifier.padding(top = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                        Dot(color)
                        Text(project?.name ?: stringResource(R.string.stats_no_project), fontSize = 17.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f).padding(start = 5.dp, end = 12.dp))
                        if (amount.tomatoes > 0) {
                            Tomato(size = 13.dp)
                            Text(" ${amount.tomatoes} · ", fontSize = 15.sp, color = K.label2)
                        }
                        Text(f.span(amount.minutes), fontSize = 15.sp, color = K.label2)
                    }
                    Box(Modifier.padding(top = 6.dp, bottom = 10.dp).fillMaxWidth().height(4.dp).clip(RoundedCornerShape(2.dp)).background(K.fill)) {
                        Box(Modifier.fillMaxWidth(if (top == 0) 0f else amount.minutes.toFloat() / top).fillMaxHeight().background(color, RoundedCornerShape(2.dp)))
                    }
                }
                for ((day, sessions) in ended.groupBy { f.day(it.start) }) {
                    Sec(f.dayLabel(day), stringResource(R.string.stats_tomatoes, sessions.count { it.completed }))
                    sessions.forEachIndexed { k, session ->
                        Row(
                            Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable { app.sheet.value = Sheet.Filing(session) }.then(if (k == sessions.lastIndex) Modifier else Modifier.hairline(K.sep)),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Text(f.timeRange(session.start, session.end), fontSize = 15.sp, color = K.label2, maxLines = 1, modifier = Modifier.width(100.dp))
                            Text(session.title.ifEmpty { stringResource(R.string.focus_free) }, fontSize = 17.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f).padding(end = 10.dp))
                            if (session.completed) Tomato(size = 15.dp)
                            else {
                                val minutes = ((Times.instant(session.end).toEpochMilli() - Times.instant(session.start).toEpochMilli()) / 60_000).toInt()
                                Text(stringResource(R.string.stats_unfinished, f.span(minutes)), fontSize = 13.sp, color = K.label2, maxLines = 1)
                            }
                        }
                    }
                }
                if (days < 62) MoreLink(stringResource(R.string.stats_older), inset = false) { days = if (days < 14) 14 else 62 }
            }
        }
    }
}

private fun average(stats: FocusStats): String {
    val value = stats.tomatoes / 7.0
    return if (value == 0.0) "0" else "%.1f".format(java.util.Locale.US, value)
}

@Composable
private fun Figure(value: String, unit: String, label: String) {
    Column {
        Row(verticalAlignment = Alignment.Bottom) {
            Text(value, fontSize = 28.sp, lineHeight = 34.sp, fontWeight = FontWeight.Bold, modifier = Modifier.alignByBaseline())
            Text(unit, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(start = 1.dp).alignByBaseline())
        }
        Text(label, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2)
    }
}

/** One bar per day for the last 7 days, stacked by project colour, with a dashed line for the average. */
@Composable
private fun Chart(stats: FocusStats) {
    val f = LocalFmt.current
    val boot by LocalApp.current.boot.collectAsStateWithLifecycle()
    val projects = boot?.projects.orEmpty().associateBy { it.id }
    val scale = FocusMath.chartScale(stats.days.map { it.tomatoes })
    val height = 150.dp
    val unit = (height - 22.dp) / scale
    val average = stats.tomatoes / 7f
    val dash = K.label3
    Box(Modifier.fillMaxWidth().height(height).hairline(K.sep)) {
        Row(Modifier.fillMaxSize().padding(end = 34.dp)) {
            for (day in stats.days) {
                Column(Modifier.weight(1f).fillMaxHeight(), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Bottom) {
                    if (day.tomatoes > 0) Text(day.tomatoes.toString(), fontSize = 12.sp, lineHeight = 16.sp, fontWeight = FontWeight.SemiBold, color = K.label2, modifier = Modifier.padding(bottom = 3.dp))
                    Column(Modifier.width(26.dp).clip(RoundedCornerShape(topStart = 6.dp, topEnd = 6.dp))) {
                        for (amount in day.projects.reversed()) {
                            if (amount.tomatoes > 0) Box(Modifier.fillMaxWidth().height(unit * amount.tomatoes).background(K.project(projects[amount.projectId]?.color)))
                        }
                    }
                }
            }
        }
        if (average > 0f) {
            Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().padding(bottom = unit * average).height(18.dp)) {
                Box(Modifier.fillMaxWidth().padding(end = 34.dp).height(18.dp).drawBehind {
                    drawLine(dash, Offset(0f, size.height / 2), Offset(size.width, size.height / 2), 1.dp.toPx(), pathEffect = PathEffect.dashPathEffect(floatArrayOf(4.dp.toPx(), 3.dp.toPx())))
                })
                Text(stringResource(R.string.stats_average), fontSize = 11.sp, lineHeight = 18.sp, color = K.label2, textAlign = TextAlign.End, modifier = Modifier.align(Alignment.CenterEnd).width(30.dp))
            }
        }
    }
    Row(Modifier.fillMaxWidth().padding(top = 6.dp, end = 34.dp, bottom = 2.dp)) {
        for (day in stats.days) {
            val date = Times.date(day.date)
            val today = date == f.today
            Text(f.weekdayLetter(date), fontSize = 12.sp, color = if (today) K.red else K.label2, fontWeight = if (today) FontWeight.SemiBold else FontWeight.Normal, textAlign = TextAlign.Center, modifier = Modifier.weight(1f))
        }
    }
}

/* ---------- filing a session ---------- */

/**
 * Says what a session was for. The three fields apply at once: the project, the item, and for a
 * session on no item what was done. Choosing another project takes the session off its item.
 */
@Composable
fun FilingSheet(start: FocusSession, close: () -> Unit) {
    val app = LocalApp.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    var session by remember { mutableStateOf(start) }
    var title by remember { mutableStateOf(start.title.takeIf { start.itemId == null }.orEmpty()) }
    var dialog by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    val project = projectOf(session.projectId)
    // The project's open items; without a project, today's.
    val items = usePage("filing/items", session.projectId) { api ->
        val projectId = session.projectId
        if (projectId != null) api.itemsUpTo(200, projectId = projectId).items else api.today().let { view -> (view.overdue + view.items).filter { !it.done } }
    }
    fun send(vararg patches: SessionPatch) {
        scope.launch {
            val outcome = app.attempt { api ->
                var last = session
                for (patch in patches) last = api.updateSession(session.id, patch)
                last
            }
            when (outcome) {
                is Outcome.Done -> {
                    session = outcome.value
                    if (outcome.value.itemId == null) title = outcome.value.title
                    error = null
                }
                is Outcome.Refused -> error = outcome.message
                Outcome.Failed -> Unit
            }
        }
    }
    SheetBar(stringResource(R.string.session_pick), stringResource(R.string.done), cancel = null, onCancel = close, onAction = close)
    Group(cell = K.sheetCell) {
        GroupRow(
            stringResource(R.string.field_project), project?.name ?: stringResource(R.string.stats_no_project), valueDot = project?.let { K.project(it.color) },
            enabled = !busy, leading = { RowIcon(KIcons.Folder, K.label2, 21.dp) },
        ) { dialog = "project" }
        GroupRow(
            stringResource(R.string.session_item), if (session.itemId != null) session.title else stringResource(R.string.session_no_item), last = true,
            enabled = !busy, leading = { RowIcon(KIcons.Checklist, K.label2, 21.dp) },
        ) { dialog = "item" }
    }
    if (session.itemId == null) {
        val save = { if (title.trim() != session.title && !busy) send(SessionPatch(title = Opt.Put(title.trim()))) }
        OutlinedTextField(
            title, { title = it.take(500) }, Modifier.fillMaxWidth().padding(top = 14.dp), singleLine = true, colors = fieldColors(),
            label = { Text(stringResource(R.string.session_what)) },
            trailingIcon = { if (title.trim() != session.title) TextButton(enabled = !busy, onClick = save) { Text(stringResource(R.string.save), color = K.tint) } },
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done), keyboardActions = KeyboardActions(onDone = { save() }),
        )
    }
    FormError(error)
    when (dialog) {
        "project" -> ProjectDialog(session.projectId, stringResource(R.string.stats_no_project), { dialog = "" }) { projectId ->
            dialog = ""
            if (projectId != session.projectId) {
                // A project can only be set on free focus, so a session on an item leaves it first.
                if (session.itemId != null) {
                    if (projectId == null) send(SessionPatch(itemId = Opt.Put(null))) else send(SessionPatch(itemId = Opt.Put(null)), SessionPatch(projectId = Opt.Put(projectId)))
                } else {
                    send(SessionPatch(projectId = Opt.Put(projectId)))
                }
            }
        }
        "item" -> ChoiceDialog(stringResource(R.string.session_item), { dialog = "" }) {
            ChoiceRow(stringResource(R.string.session_no_item), session.itemId == null) {
                dialog = ""
                if (session.itemId != null) send(SessionPatch(itemId = Opt.Put(null)))
            }
            val list = items.data.orEmpty()
            // The session's own item stays the choice even when it is done or in another list.
            if (session.itemId != null && list.none { it.id == session.itemId }) ChoiceRow(session.title, true) { dialog = "" }
            for (item in list) {
                ChoiceRow(item.title, item.id == session.itemId) {
                    dialog = ""
                    if (item.id != session.itemId) send(SessionPatch(itemId = Opt.Put(item.id)))
                }
            }
        }
    }
}
