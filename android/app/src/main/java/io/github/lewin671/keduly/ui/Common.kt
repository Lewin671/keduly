// The pieces every screen is built from: the page frame, the item row, section headings,
// grouped lists, the progress ring, the tomato mark and the buttons.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.layout
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.ApiClient
import io.github.lewin671.keduly.api.Event
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.Project
import io.github.lewin671.keduly.core.FocusMath
import io.github.lewin671.keduly.core.Times
import kotlinx.coroutines.delay

val LocalApp = staticCompositionLocalOf<AppModel> { error("no app model") }
val LocalFmt = staticCompositionLocalOf<Fmt> { error("no formatter") }
val LocalNav = staticCompositionLocalOf<Nav> { error("no navigation") }

/** Where a screen can go. */
class Nav(val go: (String) -> Unit, val back: () -> Unit, val activity: () -> Unit, val calendarDay: () -> Unit)

/** The distance from the screen edge to the content, and to the text of an item row. */
val PagePad = 20.dp
val RowTextStart = 55.dp

/** Room at the end of a scrolling page so the add button never covers the last row. */
val FabRoom = 96.dp

class Page<T>(val data: T?, val loading: Boolean, val error: String?, val retry: () -> Unit)

/** A screen's data: loaded on appear, when [deps] change, and whenever the app says it may be stale. */
@Composable
fun <T> usePage(key: String, vararg deps: Any?, fetch: suspend (ApiClient) -> T): Page<T> {
    val app = LocalApp.current
    val model: PageModel = viewModel(key = key)
    val tick by app.tick.collectAsStateWithLifecycle()
    val latest by rememberUpdatedState(fetch)
    LaunchedEffect(tick, *deps) { model.load(app) { latest(it) } }
    val state by model.state.collectAsStateWithLifecycle()
    @Suppress("UNCHECKED_CAST")
    return Page(state.data as T?, state.loading, state.error) { model.load(app) { latest(it) } }
}

@Composable
fun projectOf(id: String?): Project? {
    val boot by LocalApp.current.boot.collectAsStateWithLifecycle()
    return boot?.projects?.firstOrNull { it.id == id }
}

@Composable
fun projectColor(id: String?): Color = K.project(projectOf(id)?.color)

/** The server's clock, moving while the caller is on screen. */
@Composable
fun serverNow(): Long {
    val app = LocalApp.current
    val now by produceState(app.serverNow()) {
        while (true) {
            value = app.serverNow()
            delay(250)
        }
    }
    return now
}

/* ---------- the page frame ---------- */

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun Screen(
    title: String,
    small: String? = null,
    back: Boolean = true,
    grouped: Boolean = false,
    bell: Boolean = false,
    background: Color? = null,
    actions: @Composable RowScope.() -> Unit = {},
    content: @Composable ColumnScope.() -> Unit,
) {
    val nav = LocalNav.current
    val bg = background ?: if (grouped) K.grouped else K.bg
    Column(Modifier.fillMaxSize().background(bg)) {
        TopAppBar(
            title = {
                Row(verticalAlignment = Alignment.Bottom) {
                    Text(title, fontSize = 22.sp, fontWeight = FontWeight.Bold, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.alignByBaseline())
                    if (small != null) Text(small, fontSize = 15.sp, color = K.label2, maxLines = 1, modifier = Modifier.padding(start = 8.dp).alignByBaseline())
                }
            },
            navigationIcon = {
                if (back) IconButton(onClick = nav.back) { Icon(KIcons.Back, stringResource(R.string.back), tint = K.tint) }
            },
            actions = {
                actions()
                if (bell) BellButton()
            },
            colors = TopAppBarDefaults.topAppBarColors(containerColor = bg, titleContentColor = K.label, actionIconContentColor = K.tint),
        )
        content()
    }
}

@Composable
fun BellButton() {
    val app = LocalApp.current
    val nav = LocalNav.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val pending = boot?.counts?.pending ?: 0
    IconButton(onClick = nav.activity) {
        BadgedBox(badge = { if (pending > 0) Badge(containerColor = K.red, contentColor = Color.White) { Text(LocalFmt.current.count(pending)) } }) {
            Icon(KIcons.Bell, stringResource(R.string.activity_title), tint = K.tint)
        }
    }
}

/** What a page shows around its data: the line that says loading failed, and a spinner before the first load. */
@Composable
fun <T> ColumnScope.PageBody(page: Page<T>, content: @Composable ColumnScope.(T) -> Unit) {
    if (page.error != null) ErrorLine(page.error, page.retry)
    val data = page.data
    if (data != null) {
        content(data)
    } else if (page.loading) {
        Box(Modifier.fillMaxWidth().padding(top = 120.dp), contentAlignment = Alignment.Center) {
            CircularProgressIndicator(Modifier.size(28.dp), color = K.label3, strokeWidth = 2.5.dp)
        }
    }
}

@Composable
fun ErrorLine(message: String, retry: () -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(horizontal = PagePad, vertical = 4.dp).clip(RoundedCornerShape(10.dp)).background(K.fill).padding(start = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(message, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.weight(1f))
        TextButton(onClick = retry) { Text(stringResource(R.string.retry), color = K.tint) }
    }
}

/** A page that scrolls as one column, with the mockup's side padding. */
@Composable
fun Scroll(modifier: Modifier = Modifier, pad: Dp = PagePad, content: @Composable ColumnScope.() -> Unit) {
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(start = pad, end = pad, bottom = FabRoom), content = content)
}

@Composable
fun Blank(title: String, text: String, top: Dp = 130.dp) {
    Column(Modifier.fillMaxWidth().padding(start = 46.dp, end = 46.dp, top = top), horizontalAlignment = Alignment.CenterHorizontally) {
        Text(title, fontSize = 20.sp, lineHeight = 25.sp, fontWeight = FontWeight.SemiBold, color = K.label, textAlign = TextAlign.Center)
        Text(text, fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 6.dp))
    }
}

fun Modifier.hairline(color: Color, start: Dp = 0.dp): Modifier = drawBehind {
    val y = size.height - 0.5.dp.toPx() / 2
    drawLine(color, Offset(start.toPx(), y), Offset(size.width, y), 0.5.dp.toPx())
}

fun Modifier.dashed(color: Color, radius: Dp, width: Dp = 1.dp): Modifier = drawBehind {
    val w = width.toPx()
    drawRoundRect(
        color, Offset(w / 2, w / 2), Size(size.width - w, size.height - w), CornerRadius(radius.toPx()),
        style = Stroke(w, pathEffect = PathEffect.dashPathEffect(floatArrayOf(3.dp.toPx(), 2.5.dp.toPx()))),
    )
}

/* ---------- small marks ---------- */

@Composable
fun Dot(color: Color, size: Dp = 8.dp, modifier: Modifier = Modifier) {
    Box(modifier.size(size).background(color, CircleShape))
}

/** A project's progress: an outline with a pie inside. */
@Composable
fun Ring(color: Color, fraction: Float, size: Dp = 21.dp, modifier: Modifier = Modifier) {
    Canvas(modifier.size(size)) {
        val unit = this.size.width / 20f
        drawCircle(color, radius = 8.6f * unit, style = Stroke(1.6f * unit))
        val r = 6f * unit
        if (fraction > 0f) {
            drawArc(color, -90f, 360f * fraction.coerceIn(0f, 1f), true, Offset(center.x - r, center.y - r), Size(2 * r, 2 * r))
        }
    }
}

/** The small draining ring of a running timer. */
@Composable
fun RingMini(color: Color, fraction: Float, size: Dp = 20.dp, stroke: Dp = 2.6.dp, modifier: Modifier = Modifier) {
    val track = K.fill2
    Canvas(modifier.size(size)) {
        val w = stroke.toPx()
        val inset = w / 2
        val box = Size(this.size.width - w, this.size.height - w)
        drawArc(track, 0f, 360f, false, Offset(inset, inset), box, style = Stroke(w))
        drawArc(color, -90f, 360f * fraction.coerceIn(0f, 1f), false, Offset(inset, inset), box, style = Stroke(w, cap = StrokeCap.Round))
    }
}

/** A tomato mark: filled when earned, an outline when not. Tomato red is used for nothing else. */
@Composable
fun Tomato(on: Boolean = true, size: Dp = 13.dp, modifier: Modifier = Modifier) {
    val body = if (on) K.tomato else K.label3
    val leaf = if (on) Color(0xFF34C759) else K.label3
    Canvas(modifier.size(size)) {
        val u = this.size.width / 24f
        if (on) drawCircle(body, 7.6f * u, Offset(12 * u, 14 * u)) else drawCircle(body, 7f * u, Offset(12 * u, 14 * u), style = Stroke(1.5f * u))
        val w = (if (on) 1.8f else 1.5f) * u
        val top = if (on) 7.8f else 7.2f
        drawLine(leaf, Offset(12 * u, top * u), Offset(12 * u, 4.2f * u), w, StrokeCap.Round)
        drawLine(leaf, Offset(12 * u, (top - 0.2f) * u), Offset(7.7f * u, 6.3f * u), w, StrokeCap.Round)
        drawLine(leaf, Offset(12 * u, (top - 0.2f) * u), Offset(16.3f * u, 6.3f * u), w, StrokeCap.Round)
    }
}

/** The rounded square that fills and ticks when done. */
@Composable
fun CheckMark(done: Boolean, color: Color = K.tint, size: Dp = 22.dp, modifier: Modifier = Modifier) {
    val border = K.label3
    Canvas(modifier.size(size)) {
        val r = CornerRadius(this.size.width * 7f / 22f)
        if (done) {
            drawRoundRect(color, cornerRadius = r)
            val u = this.size.width / 22f
            val w = 2f * u
            drawLine(Color.White, Offset(6f * u, 11.3f * u), Offset(9.6f * u, 14.8f * u), w, StrokeCap.Round)
            drawLine(Color.White, Offset(9.6f * u, 14.8f * u), Offset(16f * u, 7.6f * u), w, StrokeCap.Round)
        } else {
            val w = 1.6.dp.toPx()
            drawRoundRect(border, Offset(w / 2, w / 2), Size(this.size.width - w, this.size.height - w), r, style = Stroke(w))
        }
    }
}

@Composable
fun DashedTag(text: String, fontSize: Int = 12) {
    Text(text, fontSize = fontSize.sp, lineHeight = 17.sp, color = K.label2, maxLines = 1, modifier = Modifier.dashed(K.label3, 6.dp).padding(horizontal = 6.dp, vertical = 1.dp))
}

/* ---------- headings and lines ---------- */

@Composable
fun Sec(title: String, right: String = "", sub: String = "", red: Boolean = false, icon: ImageVector? = null, first: Boolean = false, onClick: (() -> Unit)? = null) {
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .hairline(K.sep).padding(top = if (first) 10.dp else 22.dp, bottom = 7.dp),
        verticalAlignment = Alignment.Bottom,
    ) {
        Row(Modifier.weight(1f), verticalAlignment = Alignment.Bottom) {
            if (icon != null) Icon(icon, null, tint = K.indigo, modifier = Modifier.padding(end = 5.dp, bottom = 4.dp).size(17.dp))
            Text(
                title, fontSize = 20.sp, lineHeight = 25.sp, fontWeight = FontWeight.Bold, color = if (red) K.red else K.label,
                maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false).alignByBaseline(),
            )
            if (sub.isNotEmpty()) Text(sub, fontSize = 13.sp, color = K.label2, maxLines = 1, modifier = Modifier.padding(start = 8.dp).alignByBaseline())
        }
        if (right.isNotEmpty()) Text(right, fontSize = 13.sp, color = K.label2, maxLines = 1, modifier = Modifier.padding(start = 8.dp).alignByBaseline())
    }
}

@Composable
fun MoreLink(text: String, inset: Boolean = true, onClick: () -> Unit) {
    Box(Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(role = Role.Button, onClick = onClick).padding(start = if (inset) RowTextStart - PagePad else 0.dp), contentAlignment = Alignment.CenterStart) {
        Text(text, fontSize = 15.sp, color = K.tint)
    }
}

@Composable
fun NoneLine(text: String) {
    Text(text, fontSize = 15.sp, color = K.label3, modifier = Modifier.padding(start = RowTextStart - PagePad, top = 12.dp, bottom = 4.dp))
}

/** An event as one quiet line: the time, the project's colour, the title. */
@Composable
fun EventLine(event: Event, onClick: (() -> Unit)? = null) {
    val f = LocalFmt.current
    val color = projectColor(event.projectId)
    val dash = K.bg
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(vertical = 2.5.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(if (event.allDay) stringResource(R.string.cal_all_day) else event.start?.let(f::hhmm).orEmpty(), fontSize = 13.sp, color = K.label2, maxLines = 1, modifier = Modifier.width(43.dp))
        Spacer(Modifier.width(9.dp))
        Box(Modifier.size(3.dp, 16.dp).clip(RoundedCornerShape(2.dp)).background(color).drawBehind {
            if (event.tentative) for (k in 1..2) drawRect(dash, Offset(0f, size.height * k / 3f - 1.dp.toPx()), Size(size.width, 2.dp.toPx()))
        })
        Spacer(Modifier.width(9.dp))
        Text(event.title, fontSize = 15.sp, lineHeight = 20.sp, color = K.label2, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
        if (event.tentative) {
            Spacer(Modifier.width(9.dp))
            DashedTag(stringResource(R.string.cal_pending))
        }
    }
}

/* ---------- the item row ---------- */

/** Tomatoes earned against the estimate, "2/4"; just the count without an estimate. */
@Composable
fun TomatoCount(item: Item) {
    val need = FocusMath.tomatoesNeeded(item.estimateMinutes, LocalFmt.current.focusMinutes)
    Row(verticalAlignment = Alignment.CenterVertically) {
        Tomato(size = 13.dp)
        Text(" ${item.focus.tomatoes}" + if (need > 0 && !item.done) "/$need" else "", fontSize = 13.sp, color = K.label2, maxLines = 1)
    }
}

/**
 * One title line, one line of facts, and on the right only "when". The whole row opens the item;
 * the checkbox has its own 48 dp target.
 */
@Composable
fun ItemRow(given: Item, showProject: Boolean = true, showPlan: Boolean = true, last: Boolean = false) {
    val app = LocalApp.current
    val f = LocalFmt.current
    // An item ticked on this page shows as done in place until the page is left.
    val item = app.sticky[given.id]?.takeIf { it.updatedAt >= given.updatedAt } ?: given
    val busy by app.busy.collectAsStateWithLifecycle()
    val focus by app.focus.collectAsStateWithLifecycle()
    val project = projectOf(item.projectId)
    val running = focus?.takeIf { it.state == "work" && it.session?.itemId == item.id }?.session
    val checkLabel = stringResource(R.string.item_complete, item.title)
    Row(Modifier.fillMaxWidth().clickable { app.sheet.value = Sheet.ItemDetail(item.id) }.padding(start = 7.dp, end = PagePad)) {
        Box(
            Modifier.size(48.dp).clip(CircleShape).clickable(enabled = !busy, role = Role.Checkbox) { app.toggleDone(item) }.semantics { contentDescription = checkLabel },
            contentAlignment = Alignment.Center,
        ) { CheckMark(item.done, if (project != null) K.project(project.color) else K.tint) }
        Column(Modifier.weight(1f).then(if (last) Modifier else Modifier.hairline(K.sep)).padding(top = 10.dp, bottom = 10.dp)) {
            Row {
                Text(
                    buildAnnotatedString {
                        if (item.important) withStyle(SpanStyle(color = K.orange, fontWeight = FontWeight.ExtraBold)) { append("! ") }
                        append(item.title)
                    },
                    fontSize = 17.sp, lineHeight = 22.sp, maxLines = 2, overflow = TextOverflow.Ellipsis,
                    color = if (item.done) K.label2 else K.label, textDecoration = if (item.done) TextDecoration.LineThrough else null,
                    modifier = Modifier.weight(1f),
                )
                Box(Modifier.padding(start = 10.dp, top = 2.dp)) {
                    when {
                        item.done -> Unit
                        running != null -> {
                            val now = serverNow()
                            val end = Times.instant(running.end).toEpochMilli()
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                RingMini(K.tint, FocusMath.fractionLeft(end, running.plannedMinutes, now), size = 16.dp, stroke = 2.2.dp)
                                Text(" " + FocusMath.mmss(FocusMath.secondsLeft(end, now)), fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = K.tint)
                            }
                        }
                        item.suggestion != null -> DashedTag(stringResource(R.string.item_pending, f.whenLabel(item.suggestion.start)))
                        item.block != null -> Text(f.whenLabel(item.block.start), fontSize = 13.sp, color = K.label2, maxLines = 1)
                        showPlan && item.plannedDate != null && item.plannedDate != f.today.toString() ->
                            Text(f.dayLabel(Times.date(item.plannedDate)), fontSize = 13.sp, color = K.label2, maxLines = 1)
                    }
                }
            }
            val due = f.dueText(item)
            val facts = ArrayList<@Composable RowScope.() -> Unit>()
            if (showProject) facts += {
                Row(Modifier.weight(1f, fill = false), verticalAlignment = Alignment.CenterVertically) {
                    if (project != null) {
                        Dot(K.project(project.color))
                        Spacer(Modifier.width(5.dp))
                    }
                    Text(project?.name ?: stringResource(R.string.nav_inbox), fontSize = 13.sp, color = K.label2, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            if (item.done && item.focus.minutes > 0) facts += { Text(stringResource(R.string.focus_took, f.span(item.focus.minutes)), fontSize = 13.sp, color = K.label2, maxLines = 1) }
            else if (item.estimateMinutes != null) facts += { Text(f.dur(item.estimateMinutes), fontSize = 13.sp, color = K.label2, maxLines = 1) }
            if (item.focus.tomatoes > 0) facts += { TomatoCount(item) }
            else if (item.focus.minutes > 0 && !item.done) facts += { Text(stringResource(R.string.focus_used, f.span(item.focus.minutes)), fontSize = 13.sp, color = K.label2, maxLines = 1) }
            if (due != null && !item.done) facts += { Text(due, fontSize = 13.sp, color = if (f.urgent(item)) K.red else K.label2, maxLines = 1) }
            if (facts.isNotEmpty()) {
                Row(Modifier.padding(top = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                    facts.forEachIndexed { k, fact ->
                        if (k > 0) Text(" · ", fontSize = 13.sp, color = K.label3)
                        fact()
                    }
                }
            }
        }
    }
}

/**
 * The rows of one list. Items ticked here keep their place although the server no longer lists
 * them, so the tick can be undone; they are folded away on the next visit.
 */
@Composable
fun ItemRows(items: List<Item>, showProject: Boolean = true, showPlan: Boolean = true) {
    val app = LocalApp.current
    val shown = remember { arrayOf(emptyList<Item>()) }
    val list = ArrayList(items)
    shown[0].forEachIndexed { index, old ->
        val kept = app.sticky[old.id]
        if (kept != null && kept.done && list.none { it.id == old.id }) list.add(minOf(index, list.size), kept)
    }
    shown[0] = list
    // The rows bleed to the screen edge so the whole width is one touch target.
    Column(Modifier.layoutBleed(PagePad)) {
        list.forEachIndexed { k, item -> key(item.id) { ItemRow(item, showProject, showPlan, last = k == list.lastIndex) } }
    }
}

/** Lets a child of a padded column span the full width again. */
fun Modifier.layoutBleed(pad: Dp): Modifier = layout { measurable, constraints ->
    val add = (pad * 2).roundToPx()
    val width = constraints.maxWidth + add
    val placeable = measurable.measure(constraints.copy(minWidth = width, maxWidth = width))
    layout(constraints.maxWidth, placeable.height) { placeable.place(-add / 2, 0) }
}

/* ---------- grouped lists ---------- */

@Composable
fun Group(modifier: Modifier = Modifier, cell: Color = K.cell, content: @Composable ColumnScope.() -> Unit) {
    Column(modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(cell), content = content)
}

@Composable
fun GroupHeader(text: String) {
    Text(text, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(start = 16.dp, top = 22.dp, bottom = 7.dp))
}

@Composable
fun GroupNote(text: String) {
    Text(text, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 7.dp))
}

/** A row of a grouped list: a leading mark, a label, a value on the right and a chevron when it leads somewhere. */
@Composable
fun GroupRow(
    label: String,
    value: String = "",
    last: Boolean = false,
    leading: (@Composable () -> Unit)? = null,
    valueColor: Color = K.label2,
    labelColor: Color = K.label,
    valueDot: Color? = null,
    chevron: Boolean = true,
    enabled: Boolean = true,
    trailing: (@Composable () -> Unit)? = null,
    onClick: (() -> Unit)? = null,
) {
    Row(
        Modifier.fillMaxWidth().heightIn(min = 48.dp).then(if (onClick != null) Modifier.clickable(enabled = enabled, onClick = onClick) else Modifier).padding(start = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (leading != null) {
            Box(Modifier.width(23.dp), contentAlignment = Alignment.Center) { leading() }
            Spacer(Modifier.width(13.dp))
        }
        Row(
            Modifier.weight(1f).heightIn(min = 48.dp).then(if (last) Modifier else Modifier.hairline(K.sep)).padding(end = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(label, fontSize = 17.sp, color = labelColor, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f).padding(end = 8.dp))
            if (valueDot != null && value.isNotEmpty()) {
                Dot(valueDot)
                Spacer(Modifier.width(5.dp))
            }
            if (value.isNotEmpty()) Text(value, fontSize = 17.sp, color = valueColor, maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.End, modifier = Modifier.widthIn(max = 190.dp))
            if (trailing != null) trailing()
            else if (chevron && onClick != null) Icon(KIcons.Right, null, tint = K.label3, modifier = Modifier.padding(start = 8.dp).size(15.dp))
        }
    }
}

@Composable
fun RowIcon(icon: ImageVector, tint: Color = K.tint, size: Dp = 23.dp) {
    Icon(icon, null, tint = tint, modifier = Modifier.size(size))
}

/* ---------- buttons ---------- */

/** The capsule button of the mockup: grey, or the tint for the main action. */
@Composable
fun PillButton(text: String, modifier: Modifier = Modifier, primary: Boolean = false, enabled: Boolean = true, height: Dp = 50.dp, icon: ImageVector? = null, onClick: () -> Unit) {
    val bg = if (primary) K.tint else K.fill
    val fg = if (primary) K.onTint else K.label
    Row(
        modifier.heightIn(min = maxOf(height, 48.dp)).clip(RoundedCornerShape(50)).background(if (enabled) bg else bg.copy(alpha = bg.alpha * 0.45f))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick).padding(horizontal = 14.dp),
        horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) Icon(icon, null, tint = fg, modifier = Modifier.padding(end = 7.dp).size(13.dp))
        Text(text, fontSize = if (height < 50.dp) 15.sp else 17.sp, fontWeight = FontWeight.SemiBold, color = if (enabled) fg else fg.copy(alpha = 0.55f), maxLines = 1)
    }
}

/** A choice in a row of choices, as in the new-item sheet. */
@Composable
fun Chip(text: String, icon: ImageVector? = null, set: Boolean = false, cell: Color = K.sheetCell, dot: Color? = null, onClick: () -> Unit) {
    Box(Modifier.heightIn(min = 48.dp).clickable(role = Role.Button, onClick = onClick), contentAlignment = Alignment.Center) {
        Row(
            Modifier.height(34.dp).clip(RoundedCornerShape(17.dp)).background(cell).padding(horizontal = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val color = if (set) K.label else K.label2
            if (dot != null) {
                Dot(dot)
                Spacer(Modifier.width(6.dp))
            } else if (icon != null) {
                Icon(icon, null, tint = if (set && icon == KIcons.Bang) K.orange else color, modifier = Modifier.padding(end = 6.dp).size(16.dp))
            }
            Text(text, fontSize = 15.sp, color = color, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.widthIn(max = 180.dp))
        }
    }
}

/** The segmented control of the calendar: a 36 dp track inside 48 dp touch targets. */
@Composable
fun Segments(options: List<String>, selected: Int, modifier: Modifier = Modifier, onSelect: (Int) -> Unit) {
    val on = if (K.dark) Color(0xFF636366) else Color.White
    Box(modifier.fillMaxWidth().height(48.dp), contentAlignment = Alignment.Center) {
        Box(Modifier.fillMaxWidth().height(36.dp).clip(RoundedCornerShape(9.dp)).background(K.fill))
        Row(Modifier.fillMaxWidth().padding(horizontal = 2.dp)) {
            options.forEachIndexed { k, text ->
                Box(Modifier.weight(1f).height(48.dp).clickable(role = Role.Tab) { onSelect(k) }, contentAlignment = Alignment.Center) {
                    Box(
                        Modifier.fillMaxWidth().height(32.dp).clip(RoundedCornerShape(7.dp)).background(if (k == selected) on else Color.Transparent),
                        contentAlignment = Alignment.Center,
                    ) { Text(text, fontSize = 13.sp, fontWeight = if (k == selected) FontWeight.SemiBold else FontWeight.Medium, color = K.label) }
                }
            }
        }
    }
}
