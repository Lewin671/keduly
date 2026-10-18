// The pages of the items tab: the list of lists, the matrix, Today, Upcoming, All, Inbox, Done,
// search and the project page.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.repeatOnLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.ItemPage
import io.github.lewin671.keduly.api.OverviewProject
import io.github.lewin671.keduly.api.ProjectDetail
import io.github.lewin671.keduly.api.Quadrant
import io.github.lewin671.keduly.api.Quadrants
import io.github.lewin671.keduly.api.TodayView
import io.github.lewin671.keduly.api.UpcomingDay
import io.github.lewin671.keduly.core.Times
import java.time.LocalDate
import java.time.YearMonth
import kotlinx.coroutines.delay

/* ---------- the list of lists ---------- */

@Composable
fun ListsScreen() {
    val app = LocalApp.current
    val nav = LocalNav.current
    val f = LocalFmt.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val b = boot ?: return
    Screen(
        stringResource(R.string.mode_items), back = false, grouped = true, bell = true,
        actions = { IconButton(onClick = { nav.go("settings") }) { Icon(KIcons.Gear, stringResource(R.string.settings_title), tint = K.tint) } },
    ) {
        Scroll {
            Row(
                Modifier.fillMaxWidth().padding(top = 2.dp, bottom = 14.dp).heightIn(min = 48.dp).clickable { nav.go("search") }.padding(vertical = 6.dp)
                    .clip(RoundedCornerShape(10.dp)).background(K.fill).padding(horizontal = 9.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(KIcons.Search, null, tint = K.label2, modifier = Modifier.size(18.dp))
                Text(stringResource(R.string.search), fontSize = 17.sp, color = K.label2, modifier = Modifier.padding(start = 6.dp, top = 7.dp, bottom = 7.dp))
            }
            Group {
                GroupRow(stringResource(R.string.nav_matrix), leading = { RowIcon(KIcons.Grid, K.orange) }) { nav.go("matrix") }
                GroupRow(stringResource(R.string.nav_today), if (b.counts.today > 0) f.count(b.counts.today) else "", leading = { RowIcon(KIcons.Star, K.yellow) }) { nav.go("today") }
                GroupRow(stringResource(R.string.nav_upcoming), leading = { RowIcon(KIcons.Calendar, K.red) }) { nav.go("upcoming") }
                GroupRow(stringResource(R.string.nav_all), leading = { RowIcon(KIcons.Layers, K.teal) }) { nav.go("all") }
                GroupRow(stringResource(R.string.nav_inbox), if (b.counts.inbox > 0) f.count(b.counts.inbox) else "", leading = { RowIcon(KIcons.Tray, K.blue) }) { nav.go("inbox") }
                GroupRow(stringResource(R.string.habit_heading), leading = { RowIcon(KIcons.Calendar, K.teal) }) { nav.go("habits") }
                GroupRow(stringResource(R.string.nav_done), last = true, leading = { RowIcon(KIcons.Book, K.green) }) { nav.go("done") }
            }
            val projects = b.projects.filter { !it.archived }.sortedBy { it.position }
            val areas = b.areas.sortedBy { it.position }
            val groups = areas.map { area -> area.name to projects.filter { it.areaId == area.id } } +
                (stringResource(R.string.side_projects) to projects.filter { p -> areas.none { it.id == p.areaId } })
            for ((name, list) in groups) {
                if (list.isEmpty()) continue
                GroupHeader(name)
                Group {
                    list.forEachIndexed { k, project ->
                        val all = project.openCount + project.doneCount
                        GroupRow(
                            project.name, if (project.openCount > 0) f.count(project.openCount) else "", last = k == list.lastIndex,
                            leading = { Ring(K.project(project.color), if (all == 0) 0f else project.doneCount.toFloat() / all) },
                        ) { nav.go("project/${project.id}") }
                    }
                }
            }
            Group(Modifier.padding(top = 22.dp)) {
                GroupRow(stringResource(R.string.project_new), last = true, labelColor = K.tint, chevron = false, leading = { RowIcon(KIcons.Plus) }) {
                    app.sheet.value = Sheet.ProjectForm(null)
                }
            }
        }
    }
}

/* ---------- matrix ---------- */

@Composable
fun MatrixScreen() {
    var expanded by rememberSaveable { mutableStateOf("") }
    val page = usePage("matrix", expanded) { api ->
        val q = api.matrix()
        suspend fun full(key: String, quadrant: Quadrant): Quadrant =
            if (key in expanded.split(',') && quadrant.total > quadrant.items.size) quadrant.copy(items = api.itemsUpTo(quadrant.total, quadrant = key).items) else quadrant
        Quadrants(full("do", q.now), full("plan", q.plan), full("quick", q.quick), full("later", q.later))
    }
    Screen(stringResource(R.string.nav_matrix), bell = true) {
        PageBody(page) { q ->
            val boxes = listOf(
                Triple("do", q.now, R.string.matrix_do to R.string.matrix_do_label),
                Triple("plan", q.plan, R.string.matrix_plan to R.string.matrix_plan_label),
                Triple("quick", q.quick, R.string.matrix_quick to R.string.matrix_quick_label),
                Triple("later", q.later, R.string.matrix_later to R.string.matrix_later_label),
            )
            if (boxes.all { it.second.total == 0 }) {
                Blank(stringResource(R.string.all_empty), stringResource(R.string.matrix_empty_tip))
                return@PageBody
            }
            Scroll {
                boxes.forEachIndexed { k, (key, quadrant, names) ->
                    val right = if (key == "plan" && quadrant.unplanned > 0) stringResource(R.string.matrix_unplanned, quadrant.unplanned) else quadrant.total.toString()
                    Sec(stringResource(names.first), right, stringResource(names.second), first = k == 0)
                    if (quadrant.items.isEmpty()) NoneLine(stringResource(R.string.matrix_none))
                    ItemRows(quadrant.items)
                    val rest = quadrant.total - quadrant.items.size
                    if (rest > 0) MoreLink(stringResource(R.string.list_rest, rest)) { expanded = "$expanded,$key" }
                }
            }
        }
    }
}

/* ---------- today ---------- */

private const val TODAY_RELOAD_MILLIS = 60_000L

@Composable
private fun TodaySummary(view: TodayView) {
    val f = LocalFmt.current
    val boot by LocalApp.current.boot.collectAsStateWithLifecycle()
    val end = boot?.user?.workEnd ?: return
    // Overdue items are part of the day's work too, as in the server's unplanned_minutes.
    val gaps = (view.overdue + view.items).filter { !it.done && !it.evening && it.block == null && it.suggestion == null }
    val important = gaps.count { it.important }
    val freeDur = f.dur(view.freeMinutes)
    val overflow = stringResource(R.string.today_overflow_word)
    val text = buildString {
        append(if (view.freeMinutes <= 0) stringResource(R.string.today_full, end) else stringResource(R.string.today_free, end, freeDur))
        when {
            gaps.isEmpty() -> append(stringResource(R.string.today_all_planned))
            view.unplannedMinutes <= view.freeMinutes -> append(stringResource(R.string.today_unplanned, gaps.size, f.dur(view.unplannedMinutes)))
            else -> {
                append(stringResource(R.string.today_too_much, gaps.size, f.dur(view.unplannedMinutes), overflow))
                append(
                    if (important > 0 && important < gaps.size) stringResource(R.string.today_triage, important, gaps.size - important)
                    else stringResource(R.string.today_choose),
                )
            }
        }
    }
    val warn = SpanStyle(color = K.red, fontWeight = FontWeight.SemiBold)
    val strong = SpanStyle(color = K.label, fontWeight = FontWeight.SemiBold)
    Text(
        buildAnnotatedString {
            append(text)
            val at = text.indexOf(overflow)
            if (at >= 0 && view.unplannedMinutes > view.freeMinutes && gaps.isNotEmpty()) addStyle(warn, at, at + overflow.length)
            val free = text.indexOf(freeDur)
            if (free >= 0 && view.freeMinutes > 0) addStyle(strong, free, free + freeDur.length)
        },
        fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp, bottom = 12.dp),
    )
}

@Composable
fun TodayScreen() {
    val f = LocalFmt.current
    val nav = LocalNav.current
    val page = usePage("today") { it.today() }
    // The budget counts from now, so the page asks again every minute while it is in front.
    val lifecycle = LocalLifecycleOwner.current
    val reload by rememberUpdatedState(page.retry)
    LaunchedEffect(lifecycle) {
        lifecycle.repeatOnLifecycle(Lifecycle.State.STARTED) {
            while (true) {
                delay(TODAY_RELOAD_MILLIS)
                reload()
            }
        }
    }
    Screen(stringResource(R.string.nav_today), small = "${f.md(f.today)} ${f.weekdayLong(f.today)}", bell = true) {
        PageBody(page) { view ->
            if (view.habits.isEmpty() && view.items.isEmpty() && view.overdue.isEmpty() && view.events.isEmpty()) {
                Blank(stringResource(R.string.today_empty), stringResource(R.string.today_empty_tip))
                return@PageBody
            }
            Scroll {
                TodaySummary(view)
                HabitOccurrenceRows(view.habits)
                view.events.take(5).forEach { event -> EventLine(event) { nav.calendarDay(null) } }
                if (view.events.size > 5) MoreLink(stringResource(R.string.list_more_events, view.events.size - 5), onClick = { nav.calendarDay(null) })
                val day = view.items.filter { !it.evening }
                val evening = view.items.filter { it.evening }
                if (view.overdue.isNotEmpty()) {
                    Sec(stringResource(R.string.today_overdue), view.overdue.size.toString(), red = true)
                    ItemRows(view.overdue, showPlan = false)
                }
                if (day.isNotEmpty() || (view.overdue.isEmpty() && evening.isEmpty())) {
                    Sec(stringResource(R.string.mode_items), day.size.toString())
                    if (day.isEmpty()) NoneLine(stringResource(R.string.matrix_none))
                    ItemRows(day, showPlan = false)
                }
                if (evening.isNotEmpty()) {
                    Sec(stringResource(R.string.when_tonight), evening.size.toString(), icon = KIcons.Moon)
                    ItemRows(evening, showPlan = false)
                }
            }
        }
    }
}

/* ---------- upcoming ---------- */

@Composable
fun UpcomingScreen() {
    val f = LocalFmt.current
    val nav = LocalNav.current
    var months by rememberSaveable { mutableIntStateOf(1) }
    val first = f.today.plusDays(1)
    val page = usePage("upcoming", months, first) { api ->
        val days = ArrayList<UpcomingDay>()
        for (k in 0 until months) {
            val month = YearMonth.from(first).plusMonths(k.toLong())
            val from = if (k == 0) first else month.atDay(1)
            days += api.upcoming(from.toString(), month.atEndOfMonth().toString())
        }
        days
    }
    Screen(stringResource(R.string.nav_upcoming), bell = true) {
        PageBody(page) { days ->
            Scroll {
                if (days.isEmpty()) Blank(stringResource(R.string.upcoming_empty), stringResource(R.string.upcoming_empty_tip), top = 90.dp)
                days.forEachIndexed { k, day ->
                    val date = Times.date(day.date)
                    Sec(f.dayLabel(date), sub = f.weekdayLong(date), first = k == 0)
                    if (day.events.isNotEmpty()) {
                        Spacer(Modifier.height(8.dp))
                        day.events.forEach { event -> EventLine(event) { nav.calendarDay(date) } }
                    }
                    ItemRows(day.items, showPlan = false)
                    HabitOccurrenceRows(day.habits)
                }
                val next = YearMonth.from(first).plusMonths(months.toLong())
                if (months < 12) MoreLink(stringResource(R.string.upcoming_load, stringResource(R.string.date_m, next.monthValue)), inset = false) { months++ }
            }
        }
    }
}

/* ---------- all ---------- */

@Composable
fun AllScreen() {
    val nav = LocalNav.current
    val app = LocalApp.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val more = remember { mutableStateMapOf<String, Int>() }
    val page = usePage("all", more.toMap()) { api ->
        api.overview().map { group ->
            val want = more[group.projectId] ?: 0
            if (want > group.items.size) group.copy(items = api.itemsUpTo(want, projectId = group.projectId).items) else group
        }
    }
    Screen(stringResource(R.string.nav_all), bell = true) {
        PageBody(page) { groups: List<OverviewProject> ->
            if (groups.isEmpty()) {
                Blank(stringResource(R.string.all_empty), stringResource(R.string.all_empty_tip))
                return@PageBody
            }
            Scroll {
                groups.forEachIndexed { k, group ->
                    val project = boot?.projects?.firstOrNull { it.id == group.projectId }
                    Sec(project?.name.orEmpty(), group.total.toString(), first = k == 0) { nav.go("project/${group.projectId}") }
                    ItemRows(group.items, showProject = false)
                    val rest = group.total - group.items.size
                    if (rest > 0) MoreLink(stringResource(R.string.list_more, minOf(15, rest), rest)) { more[group.projectId] = group.items.size + 15 }
                }
            }
        }
    }
}

/* ---------- inbox ---------- */

@Composable
fun InboxScreen() {
    var count by rememberSaveable { mutableIntStateOf(20) }
    val page = usePage("inbox", count) { it.itemsUpTo(count, projectId = "none") }
    Screen(stringResource(R.string.nav_inbox), bell = true) {
        PageBody(page) { list: ItemPage ->
            if (list.items.isEmpty()) {
                Blank(stringResource(R.string.inbox_empty), stringResource(R.string.inbox_empty_tip))
                return@PageBody
            }
            Scroll {
                Text(stringResource(R.string.inbox_summary), fontSize = 15.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp, bottom = 6.dp))
                ItemRows(list.items, showProject = false)
                val rest = list.total - list.items.size
                if (rest > 0) MoreLink(stringResource(R.string.list_more, minOf(60, rest), rest)) { count += 60 }
            }
        }
    }
}

/* ---------- done ---------- */

@Composable
fun DoneScreen() {
    val f = LocalFmt.current
    var count by rememberSaveable { mutableIntStateOf(50) }
    val page = usePage("done", count) { it.itemsUpTo(count, status = "done") }
    val scroll = rememberScrollState()
    val list = page.data
    // The log only grows, so it loads the next batch when the end comes into view.
    LaunchedEffect(scroll.value, scroll.maxValue, list?.items?.size) {
        if (list != null && !page.loading && list.items.size < list.total && scroll.maxValue > 0 && scroll.value >= scroll.maxValue - 200) count = list.items.size + 50
    }
    Screen(stringResource(R.string.nav_done), bell = true) {
        PageBody(page) { done: ItemPage ->
            if (done.items.isEmpty()) {
                Blank(stringResource(R.string.done_empty), stringResource(R.string.done_empty_tip))
                return@PageBody
            }
            Column(Modifier.fillMaxSize().verticalScroll(scroll).padding(start = PagePad, end = PagePad, bottom = FabRoom)) {
                Text(stringResource(R.string.done_summary, done.total), fontSize = 15.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
                // Newest first, one section per month of completion in the account's zone.
                val groups = done.items.groupBy { item -> item.completedAt?.let { YearMonth.from(f.day(it)) } }
                for ((month, items) in groups) {
                    val title = when {
                        month == null -> ""
                        month == YearMonth.from(f.today) -> stringResource(R.string.done_this_month)
                        else -> stringResource(R.string.date_ym, month.year, month.monthValue)
                    }
                    Sec(title, items.size.toString())
                    ItemRows(items)
                }
                Text(
                    stringResource(if (done.items.size < done.total) R.string.loading else R.string.done_end), fontSize = 13.sp, color = K.label3,
                    modifier = Modifier.fillMaxWidth().padding(top = 18.dp), textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                )
            }
        }
    }
}

/* ---------- search ---------- */

@Composable
fun SearchScreen() {
    var query by rememberSaveable { mutableStateOf("") }
    var settled by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(query) {
        delay(300)
        settled = query.trim()
    }
    // The search runs on the server over everything, never over what happens to be loaded.
    val page = usePage("search", settled) { api -> if (settled.isEmpty()) ItemPage(emptyList(), null, 0) else api.items(status = "any", q = settled, limit = 50) }
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { focus.requestFocus() }
    Screen(stringResource(R.string.search)) {
        OutlinedTextField(
            query, { query = it }, Modifier.fillMaxWidth().padding(horizontal = PagePad).focusRequester(focus), singleLine = true, colors = fieldColors(),
            placeholder = { Text(stringResource(R.string.search_hint), color = K.label3) },
            leadingIcon = { Icon(KIcons.Search, null, tint = K.label2, modifier = Modifier.size(20.dp)) },
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        )
        PageBody(page) { found: ItemPage ->
            Scroll {
                Spacer(Modifier.height(8.dp))
                if (settled.isNotEmpty() && found.items.isEmpty() && !page.loading) Blank(stringResource(R.string.search_none), stringResource(R.string.search_none_tip, settled), top = 60.dp)
                ItemRows(found.items)
                if (found.total > found.items.size) Text(stringResource(R.string.search_more, found.total - found.items.size), fontSize = 13.sp, color = K.label3, modifier = Modifier.padding(top = 12.dp))
            }
        }
    }
}

/* ---------- project ---------- */

private class ProjectPage(val detail: ProjectDetail, val open: List<Item>, val openTotal: Int, val done: List<Item>)

@Composable
fun ProjectScreen(id: String) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val nav = LocalNav.current
    var doneCount by rememberSaveable { mutableIntStateOf(0) }
    var openLimit by rememberSaveable { mutableIntStateOf(200) }
    val page = usePage("project/$id", doneCount, openLimit) { api ->
        val detail = api.project(id)
        val open = api.itemsUpTo(openLimit, projectId = id)
        ProjectPage(detail, open.items, open.total, if (doneCount > 0) api.itemsUpTo(doneCount, status = "done", projectId = id).items else emptyList())
    }
    var menu by remember { mutableStateOf(false) }
    Screen(
        "",
        actions = {
            val project = page.data?.detail?.project
            if (project != null) {
                IconButton(onClick = { menu = true }) { Icon(KIcons.More, stringResource(R.string.project_menu), tint = K.tint) }
                DropdownMenu(menu, { menu = false }) {
                    DropdownMenuItem(text = { Text(stringResource(R.string.project_edit)) }, onClick = { menu = false; app.sheet.value = Sheet.ProjectForm(project) })
                }
            }
        },
    ) {
        PageBody(page) { data ->
            val project = data.detail.project
            val color = K.project(project.color)
            val all = project.openCount + project.doneCount
            Scroll {
                Row(Modifier.padding(bottom = 6.dp)) {
                    Ring(color, if (all == 0) 0f else project.doneCount.toFloat() / all, size = 26.dp, modifier = Modifier.padding(top = 6.dp))
                    Text(project.name, fontSize = 28.sp, lineHeight = 35.sp, fontWeight = FontWeight.Bold, maxLines = 3, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(start = 10.dp))
                }
                if (project.notes.isNotBlank()) Text(project.notes, fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, maxLines = 3, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(bottom = 10.dp))
                if (all > 0) {
                    Text(
                        stringResource(R.string.project_open, project.openCount) +
                            (if (project.openCount > 0) stringResource(R.string.project_unplanned, data.detail.unplannedCount) else "") +
                            (if (data.detail.focusWeekMinutes > 0) stringResource(R.string.project_focus_week, f.span(data.detail.focusWeekMinutes)) else ""),
                        fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, modifier = Modifier.padding(bottom = 10.dp),
                    )
                }
                // The next events show whether or not the project has items yet.
                HabitOccurrenceRows(data.detail.habits)
                data.detail.upcomingEvents.take(2).forEach { event ->
                    EventLine(event) { nav.calendarDay(event.startDate?.let(Times::date) ?: event.start?.let(f::day)) }
                }
                if (all == 0) {
                    Blank(stringResource(R.string.project_empty), stringResource(R.string.project_empty_tip), top = if (data.detail.upcomingEvents.isEmpty()) 100.dp else 56.dp)
                    return@Scroll
                }
                val headings = data.detail.headings.sortedBy { it.position }
                val loose = data.open.filter { item -> headings.none { it.id == item.headingId } }
                var first = true
                if (loose.isNotEmpty()) {
                    Sec(stringResource(if (headings.isEmpty()) R.string.mode_items else R.string.heading_none), loose.size.toString(), first = false)
                    ItemRows(loose, showProject = false)
                    first = false
                }
                for (heading in headings) {
                    val items = data.open.filter { it.headingId == heading.id }
                    if (items.isEmpty()) continue
                    Sec(heading.name, items.size.toString(), first = false)
                    ItemRows(items, showProject = false)
                    first = false
                }
                // A long project loads in batches; the rest is one tap away.
                val more = data.openTotal - data.open.size
                if (more > 0) MoreLink(stringResource(R.string.list_more, minOf(200, more), more)) { openLimit += 200 }
                if (first && project.openCount == 0) NoneLine(stringResource(R.string.project_all_done))
                if (data.done.isNotEmpty()) {
                    Sec(stringResource(R.string.nav_done), project.doneCount.toString())
                    ItemRows(data.done, showProject = false)
                }
                val rest = project.doneCount - data.done.size
                if (rest > 0) {
                    MoreLink(stringResource(if (data.done.isEmpty()) R.string.project_show_done else R.string.list_rest, rest)) { doneCount = if (doneCount == 0) 10 else doneCount + 30 }
                }
            }
        }
    }
}
