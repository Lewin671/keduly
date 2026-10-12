// What rises from the bottom: the sheet frame, the pickers the sheets share, the item sheet, the
// new-item sheet and the project form. The system back gesture closes the sheet first.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
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
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.ItemPatch
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.api.Project
import io.github.lewin671.keduly.api.ProjectPatch
import io.github.lewin671.keduly.core.Times
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneOffset
import kotlinx.coroutines.launch

val DURATIONS = listOf(15, 30, 60, 90, 120)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SheetHost() {
    val app = LocalApp.current
    val sheet by app.sheet.collectAsStateWithLifecycle()
    val current = sheet ?: return
    val close = { app.sheet.value = null }
    key(current) {
        ModalBottomSheet(
            onDismissRequest = close,
            sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true),
            containerColor = K.sheet, contentColor = K.label,
            shape = RoundedCornerShape(topStart = 28.dp, topEnd = 28.dp),
            dragHandle = { Box(Modifier.padding(top = 6.dp).size(36.dp, 5.dp).background(K.label3, RoundedCornerShape(3.dp))) },
        ) {
            Column(Modifier.fillMaxWidth().verticalScroll(rememberScrollState()).padding(start = 16.dp, end = 16.dp, bottom = 20.dp)) {
                when (current) {
                    is Sheet.ItemDetail -> ItemSheet(current.id, close)
                    is Sheet.NewItem -> NewItemSheet(current, close)
                    is Sheet.EventForm -> EventFormSheet(current, close)
                    is Sheet.EventInfo -> EventInfoSheet(current.event, close)
                    is Sheet.EventList -> EventListSheet(current.events)
                    is Sheet.Filing -> FilingSheet(current.session, close)
                    is Sheet.ProjectForm -> ProjectFormSheet(current.project, close)
                    Sheet.FocusPick -> FocusPickSheet(close)
                }
            }
        }
    }
}

/** The top of a sheet: cancel on the left, the title, the main action on the right. */
@Composable
fun SheetBar(title: String, action: String, enabled: Boolean = true, cancel: String? = stringResource(R.string.cancel), onCancel: () -> Unit, onAction: () -> Unit) {
    Box(Modifier.fillMaxWidth().height(48.dp)) {
        if (cancel != null) TextButton(onClick = onCancel, modifier = Modifier.align(Alignment.CenterStart)) { Text(cancel, fontSize = 17.sp, color = K.tint) }
        Text(title, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.align(Alignment.Center))
        TextButton(onClick = onAction, enabled = enabled, modifier = Modifier.align(Alignment.CenterEnd)) {
            Text(action, fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = if (enabled) K.tint else K.tint.copy(alpha = 0.4f))
        }
    }
}

@Composable
fun FormError(message: String?) {
    if (message != null) Text(message, fontSize = 14.sp, lineHeight = 19.sp, color = K.red, modifier = Modifier.padding(start = 4.dp, top = 8.dp))
}

/** A text field without a frame, with grey text while it is empty. */
@Composable
fun PlainField(
    value: String, onChange: (String) -> Unit, hint: String, style: TextStyle, modifier: Modifier = Modifier,
    singleLine: Boolean = false, imeAction: ImeAction = ImeAction.Default, onDone: () -> Unit = {},
) {
    BasicTextField(
        value, onChange, modifier, textStyle = style.copy(color = if (style.color == androidx.compose.ui.graphics.Color.Unspecified) K.label else style.color),
        singleLine = singleLine, cursorBrush = SolidColor(K.tint),
        keyboardOptions = KeyboardOptions(imeAction = imeAction), keyboardActions = KeyboardActions(onDone = { onDone() }),
        decorationBox = { inner ->
            Box {
                if (value.isEmpty()) Text(hint, style = style, color = K.label3)
                inner()
            }
        },
    )
}

/* ---------- pickers ---------- */

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DayPickerDialog(initial: LocalDate, onDismiss: () -> Unit, onPick: (LocalDate) -> Unit) {
    // The picker counts in UTC days; the date it returns is a plain calendar date.
    val state = rememberDatePickerState(initialSelectedDateMillis = initial.atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli())
    DatePickerDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            TextButton(onClick = { state.selectedDateMillis?.let { onPick(Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate()) } }) { Text(stringResource(R.string.ok)) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    ) { DatePicker(state, showModeToggle = false) }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun TimeDialog(initial: LocalTime, onDismiss: () -> Unit, onPick: (LocalTime) -> Unit) {
    val state = rememberTimePickerState(initial.hour, initial.minute, is24Hour = true)
    // The clock face needs more width than an alert dialog gives its text, so the dialog is built by hand.
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Surface(shape = RoundedCornerShape(28.dp), color = K.sheet, contentColor = K.label, modifier = Modifier.padding(16.dp)) {
            Column(Modifier.padding(start = 24.dp, top = 24.dp, end = 24.dp, bottom = 12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                TimePicker(
                    state,
                    colors = TimePickerDefaults.colors(
                        clockDialColor = K.wash(K.label, 0.07f, K.sheet), selectorColor = K.tint, containerColor = K.sheet,
                        clockDialSelectedContentColor = K.onTint, clockDialUnselectedContentColor = K.label,
                        timeSelectorSelectedContainerColor = K.wash(K.tint, 0.2f, K.sheet), timeSelectorUnselectedContainerColor = K.wash(K.label, 0.07f, K.sheet),
                        timeSelectorSelectedContentColor = K.label, timeSelectorUnselectedContentColor = K.label,
                    ),
                )
                Row(Modifier.align(Alignment.End)) {
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) }
                    TextButton(onClick = { onPick(LocalTime.of(state.hour, state.minute)) }) { Text(stringResource(R.string.ok)) }
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun EstimateDialog(current: Int?, onDismiss: () -> Unit, onPick: (Int?) -> Unit) {
    val f = LocalFmt.current
    var custom by remember { mutableStateOf(current?.takeIf { it !in DURATIONS }?.toString().orEmpty()) }
    val minutes = custom.toIntOrNull()?.takeIf { it in 1..1440 }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.estimate_title)) },
        text = {
            Column {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    for (m in DURATIONS) Chip(f.dur(m), set = m == current, cell = if (m == current) K.wash(K.tint, 0.2f, K.sheetCell) else K.fill) { onPick(m) }
                }
                OutlinedTextField(
                    custom, { text -> custom = text.filter(Char::isDigit).take(4) }, Modifier.fillMaxWidth().padding(top = 8.dp), singleLine = true, colors = fieldColors(),
                    label = { Text(stringResource(R.string.estimate_custom)) }, keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                )
            }
        },
        confirmButton = { TextButton(enabled = minutes != null, onClick = { onPick(minutes) }) { Text(stringResource(R.string.ok)) } },
        dismissButton = { if (current != null) TextButton(onClick = { onPick(null) }) { Text(stringResource(R.string.clear)) } },
    )
}

/** One of a list of choices, with a tick on the current one. */
@Composable
fun ChoiceRow(label: String, selected: Boolean, dot: androidx.compose.ui.graphics.Color? = null, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(role = Role.RadioButton, onClick = onClick).padding(horizontal = 4.dp), verticalAlignment = Alignment.CenterVertically) {
        if (dot != null) {
            Dot(dot, 10.dp)
            Spacer(Modifier.width(10.dp))
        }
        Text(label, fontSize = 17.sp, color = K.label, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
        if (selected) Icon(KIcons.Check, null, tint = K.tint, modifier = Modifier.size(20.dp))
    }
}

@Composable
fun ChoiceDialog(title: String, onDismiss: () -> Unit, content: @Composable ColumnScope.() -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { Column(Modifier.verticalScroll(rememberScrollState()), content = content) },
        confirmButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

/** Picks a project, or none: [noneLabel] names that choice. */
@Composable
fun ProjectDialog(current: String?, noneLabel: String, onDismiss: () -> Unit, onPick: (String?) -> Unit) {
    val boot by LocalApp.current.boot.collectAsStateWithLifecycle()
    ChoiceDialog(stringResource(R.string.project_pick), onDismiss) {
        ChoiceRow(noneLabel, current == null) { onPick(null) }
        for (project in boot?.projects.orEmpty().filter { !it.archived }.sortedBy { it.position }) {
            ChoiceRow(project.name, project.id == current, K.project(project.color)) { onPick(project.id) }
        }
    }
}

/* ---------- the item sheet ---------- */

@Composable
fun ItemSheet(id: String, close: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val page = usePage("item/$id") { it.item(id) }
    val busy by app.busy.collectAsStateWithLifecycle()
    val focus by app.focus.collectAsStateWithLifecycle()
    if (page.error != null) ErrorLine(page.error, page.retry)
    val item = page.data?.takeIf { it.id == id }
    if (item == null) {
        Box(Modifier.fillMaxWidth().height(200.dp), contentAlignment = Alignment.Center) {
            if (page.loading) CircularProgressIndicator(Modifier.size(28.dp), color = K.label3, strokeWidth = 2.5.dp)
        }
        return
    }
    val project = projectOf(item.projectId)
    var title by remember(id) { mutableStateOf(item.title) }
    var notes by remember(id) { mutableStateOf(item.notes) }
    var dialog by remember { mutableStateOf("") }
    val latest by rememberUpdatedState(item)
    // Title and notes are saved when the sheet goes away; everything else applies at once.
    DisposableEffect(id) {
        onDispose {
            val changedTitle = title.trim().takeIf { it.isNotEmpty() && it != latest.title }
            val changedNotes = notes.trim().takeIf { it != latest.notes }
            if (changedTitle != null || changedNotes != null) {
                app.act { api -> api.updateItem(id, ItemPatch(title = changedTitle?.let { Opt.Put(it) } ?: Opt.Keep, notes = changedNotes?.let { Opt.Put(it) } ?: Opt.Keep)) }
            }
        }
    }
    fun patch(patch: ItemPatch) = app.act { it.updateItem(id, patch) }

    Box(Modifier.fillMaxWidth().height(48.dp)) {
        TextButton(onClick = close, modifier = Modifier.align(Alignment.CenterEnd)) { Text(stringResource(R.string.done), fontSize = 17.sp, fontWeight = FontWeight.SemiBold, color = K.tint) }
    }
    Row {
        Box(Modifier.size(48.dp).clip(CircleShape).clickable(enabled = !busy, role = Role.Checkbox) { app.toggleDone(item) }, contentAlignment = Alignment.Center) {
            CheckMark(item.done, if (project != null) K.project(project.color) else K.tint)
        }
        Column(Modifier.weight(1f).padding(top = 9.dp, end = 4.dp)) {
            PlainField(
                title, { title = it }, stringResource(R.string.item_title),
                TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.Bold, textDecoration = if (item.done) TextDecoration.LineThrough else null),
                Modifier.fillMaxWidth(),
            )
            PlainField(notes, { notes = it }, stringResource(R.string.item_notes), TextStyle(fontSize = 15.sp, lineHeight = 21.sp, color = K.label2), Modifier.fillMaxWidth().padding(top = 4.dp))
        }
    }
    item.suggestion?.let { suggestion ->
        Column(Modifier.padding(top = 14.dp).fillMaxWidth().clip(RoundedCornerShape(14.dp)).background(K.sheetCell).dashed(K.label3, 14.dp).padding(14.dp)) {
            Text(stringResource(R.string.suggest_schedule, f.rangeLabel(suggestion.start, suggestion.end)), fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold)
            if (suggestion.reason.isNotBlank()) Text(suggestion.reason, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
            Text(suggestion.actor.name, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
            Row(Modifier.padding(top = 10.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                PillButton(stringResource(R.string.suggest_reject), Modifier.weight(1f), enabled = !busy, height = 40.dp) { app.act { it.rejectSuggestion(suggestion.id) } }
                PillButton(stringResource(R.string.suggest_accept), Modifier.weight(1f), primary = true, enabled = !busy, height = 40.dp) { app.act { it.acceptSuggestion(suggestion.id) } }
            }
        }
    }
    val spent = if (item.focus.minutes > 0) stringResource(R.string.focus_used, f.span(item.focus.minutes)) else null
    Group(Modifier.padding(top = 14.dp), cell = K.sheetCell) {
        GroupRow(
            stringResource(R.string.field_time),
            when {
                item.suggestion != null -> stringResource(R.string.item_pending, f.whenLabel(item.suggestion.start))
                item.block != null -> f.slotLabel(item.block.start)
                item.plannedDate != null -> f.dayLabel(Times.date(item.plannedDate)) + if (item.evening) " · " + stringResource(R.string.when_tonight) else ""
                else -> stringResource(R.string.when_none)
            },
            leading = { RowIcon(KIcons.Calendar, K.label2, 21.dp) },
        ) { dialog = "when" }
        GroupRow(
            stringResource(R.string.field_due), f.dueLabel(item) ?: stringResource(R.string.none),
            valueColor = if (f.urgent(item) && !item.done) K.red else K.label2, leading = { RowIcon(KIcons.Flag, K.label2, 21.dp) },
        ) { dialog = "due" }
        GroupRow(
            stringResource(R.string.field_estimate),
            listOfNotNull(item.estimateMinutes?.let(f::dur), spent).joinToString(" · ").ifEmpty { stringResource(R.string.none) },
            leading = { RowIcon(KIcons.Clock, K.label2, 21.dp) },
        ) { dialog = "estimate" }
        GroupRow(
            stringResource(R.string.field_project), project?.name ?: stringResource(R.string.nav_inbox), valueDot = project?.let { K.project(it.color) },
            leading = { RowIcon(KIcons.Folder, K.label2, 21.dp) },
        ) { dialog = "project" }
        GroupRow(
            stringResource(R.string.item_important), last = true, leading = { RowIcon(KIcons.Bang, K.label2, 21.dp) },
            trailing = {
                Switch(
                    item.important, { patch(ItemPatch(important = Opt.Put(it))) }, enabled = !busy,
                    colors = SwitchDefaults.colors(checkedTrackColor = K.green, checkedThumbColor = androidx.compose.ui.graphics.Color.White),
                )
            },
        )
    }
    Row(Modifier.padding(top = 16.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        val onIt = focus?.takeIf { it.state == "work" && it.session?.itemId == id } != null
        PillButton(
            stringResource(if (onIt) R.string.focus_back else R.string.focus_start), Modifier.weight(1f), primary = true,
            enabled = !busy && !item.done, icon = if (onIt) null else KIcons.Play,
        ) {
            close()
            if (onIt) app.zen.value = true else app.startFocus(item)
        }
        Box {
            var menu by remember { mutableStateOf(false) }
            Box(Modifier.size(50.dp).clip(CircleShape).background(K.fill).clickable { menu = true }, contentAlignment = Alignment.Center) {
                Icon(KIcons.More, stringResource(R.string.more), tint = K.label, modifier = Modifier.size(24.dp))
            }
            DropdownMenu(menu, { menu = false }) {
                DropdownMenuItem(text = { Text(stringResource(R.string.delete), color = K.red) }, onClick = { menu = false; dialog = "delete" })
            }
        }
    }

    when (dialog) {
        "when" -> WhenDialog(item, { dialog = "" })
        "due" -> DueDialog(item, { dialog = "" })
        "estimate" -> EstimateDialog(item.estimateMinutes, { dialog = "" }) { dialog = ""; patch(ItemPatch(estimateMinutes = Opt.Put(it))) }
        "project" -> ProjectDialog(item.projectId, stringResource(R.string.nav_inbox), { dialog = "" }) {
            dialog = ""
            // Moving to another project leaves the old project's heading behind.
            if (it != item.projectId) patch(ItemPatch(projectId = Opt.Put(it), headingId = Opt.Put(null)))
        }
        "delete" -> AlertDialog(
            onDismissRequest = { dialog = "" },
            title = { Text(stringResource(R.string.item_delete_ask)) },
            confirmButton = {
                TextButton(enabled = !busy, onClick = { dialog = ""; app.act(then = close) { it.deleteItem(id) } }) { Text(stringResource(R.string.delete), color = K.red) }
            },
            dismissButton = { TextButton(onClick = { dialog = "" }) { Text(stringResource(R.string.cancel)) } },
        )
    }
}

/** A row of a dialog that shows a value and opens a picker for it. */
@Composable
fun PickRow(label: String, value: String, onClick: () -> Unit) {
    Row(Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(onClick = onClick), verticalAlignment = Alignment.CenterVertically) {
        Text(label, fontSize = 16.sp, color = K.label, modifier = Modifier.weight(1f))
        Text(value, fontSize = 16.sp, color = K.tint)
    }
}

/** When the item will be done: only a day, this evening, or a time block on the calendar. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun WhenDialog(item: Item, onDismiss: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val busy by app.busy.collectAsStateWithLifecycle()
    var day by remember { mutableStateOf(item.block?.let { f.day(it.start) } ?: item.plannedDate?.let(Times::date) ?: f.today) }
    var evening by remember { mutableStateOf(item.evening) }
    var picker by remember { mutableStateOf("") }
    val minutes = item.block?.let { maxOf(15, ((Times.instant(it.end).toEpochMilli() - Times.instant(it.start).toEpochMilli()) / 60_000).toInt()) } ?: item.estimateMinutes ?: 60
    val slots = usePage("free/$day/$minutes", day, minutes) { it.free(day.toString(), minutes) }
    fun schedule(start: LocalTime) {
        val from = Times.utc(day, start, f.zone)
        val to = Times.utc(Times.instant(from).plusSeconds(minutes * 60L))
        app.act(then = onDismiss) { it.scheduleItem(item.id, from, to) }
    }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.when_title)) },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState())) {
                PickRow(stringResource(R.string.field_date), "${f.dayLabel(day)} ${f.weekdayShort(day)}") { picker = "day" }
                Row(Modifier.fillMaxWidth().heightIn(min = 48.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(stringResource(R.string.when_tonight), fontSize = 16.sp, color = K.label, modifier = Modifier.weight(1f))
                    Switch(evening, { evening = it }, colors = SwitchDefaults.colors(checkedTrackColor = K.green, checkedThumbColor = androidx.compose.ui.graphics.Color.White))
                }
                Text(stringResource(R.string.when_schedule, f.dur(minutes)), fontSize = 13.sp, color = K.label2, modifier = Modifier.padding(top = 10.dp, bottom = 2.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    // The start of each free stretch of the working day that is long enough.
                    for (slot in slots.data.orEmpty().take(6)) {
                        Chip(f.hhmm(slot.start), cell = K.fill, set = true) { if (!busy) schedule(f.wall(slot.start).toLocalTime()) }
                    }
                    Chip(stringResource(R.string.when_custom), icon = KIcons.Clock, cell = K.fill) { picker = "time" }
                }
                if (slots.data?.isEmpty() == true) Text(stringResource(R.string.when_no_free), fontSize = 13.sp, color = K.label3)
            }
        },
        confirmButton = {
            TextButton(enabled = !busy, onClick = {
                app.act(then = onDismiss) { api ->
                    if (item.block != null) api.unscheduleItem(item.id)
                    api.updateItem(item.id, ItemPatch(plannedDate = Opt.Put(day.toString()), evening = Opt.Put(evening)))
                }
            }) { Text(stringResource(R.string.when_date_only)) }
        },
        dismissButton = {
            if (item.block != null || item.plannedDate != null) {
                TextButton(enabled = !busy, onClick = {
                    app.act(then = onDismiss) { api ->
                        if (item.block != null) api.unscheduleItem(item.id)
                        api.updateItem(item.id, ItemPatch(plannedDate = Opt.Put(null), evening = Opt.Put(false)))
                    }
                }) { Text(stringResource(R.string.when_clear)) }
            }
        },
    )
    when (picker) {
        "day" -> DayPickerDialog(day, { picker = "" }) { day = it; picker = "" }
        "time" -> TimeDialog(LocalTime.of(9, 0), { picker = "" }) { picker = ""; schedule(it) }
    }
}

@Composable
private fun DueDialog(item: Item, onDismiss: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val busy by app.busy.collectAsStateWithLifecycle()
    var day by remember { mutableStateOf(item.dueDate?.let(Times::date) ?: f.today) }
    var time by remember { mutableStateOf(item.dueTime) }
    var picker by remember { mutableStateOf("") }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.due_title)) },
        text = {
            Column {
                PickRow(stringResource(R.string.due_date), "${f.dayLabel(day)} ${f.weekdayShort(day)}") { picker = "day" }
                PickRow(stringResource(R.string.due_time), time ?: stringResource(R.string.none)) { picker = "time" }
                if (time != null) TextButton(onClick = { time = null }) { Text(stringResource(R.string.due_no_time), color = K.tint) }
            }
        },
        confirmButton = {
            TextButton(enabled = !busy, onClick = { app.act(then = onDismiss) { it.updateItem(item.id, ItemPatch(dueDate = Opt.Put(day.toString()), dueTime = Opt.Put(time))) } }) {
                Text(stringResource(R.string.save))
            }
        },
        dismissButton = {
            if (item.dueDate != null) {
                TextButton(enabled = !busy, onClick = { app.act(then = onDismiss) { it.updateItem(item.id, ItemPatch(dueDate = Opt.Put(null), dueTime = Opt.Put(null))) } }) {
                    Text(stringResource(R.string.clear))
                }
            }
        },
    )
    when (picker) {
        "day" -> DayPickerDialog(day, { picker = "" }) { day = it; picker = "" }
        "time" -> TimeDialog(time?.let(Times::time) ?: LocalTime.of(18, 0), { picker = "" }) { time = Times.hhmm(it); picker = "" }
    }
}

/* ---------- new item ---------- */

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun NewItemSheet(start: Sheet.NewItem, close: () -> Unit) {
    val app = LocalApp.current
    val f = LocalFmt.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    var title by rememberSaveable { mutableStateOf("") }
    var today by rememberSaveable { mutableStateOf(start.today) }
    var due by remember { mutableStateOf<LocalDate?>(null) }
    var estimate by rememberSaveable { mutableStateOf<Int?>(null) }
    var projectId by rememberSaveable { mutableStateOf(start.projectId) }
    var important by rememberSaveable { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var dialog by remember { mutableStateOf("") }
    val project = projectOf(projectId)
    val inboxed = stringResource(R.string.item_added_to_inbox)
    val submit = {
        if (title.isNotBlank() && !busy) scope.launch {
            // Only what was chosen is sent; nothing is cleared on a new item.
            val fields = ItemPatch(
                projectId = projectId?.let { Opt.Put(it) } ?: Opt.Keep,
                plannedDate = if (today) Opt.Put(f.today.toString()) else Opt.Keep,
                dueDate = due?.let { Opt.Put(it.toString()) } ?: Opt.Keep,
                estimateMinutes = estimate?.let { Opt.Put(it) } ?: Opt.Keep,
                important = if (important) Opt.Put(true) else Opt.Keep,
            )
            when (val outcome = app.attempt { it.createItem(title.trim(), fields) }) {
                is Outcome.Done -> {
                    if (projectId == null && !today) app.messages.tryEmit(inboxed)
                    close()
                }
                is Outcome.Refused -> error = outcome.message
                Outcome.Failed -> Unit
            }
        }
        Unit
    }
    SheetBar(stringResource(R.string.item_new), stringResource(R.string.add), enabled = title.isNotBlank() && !busy, onCancel = close, onAction = submit)
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { focus.requestFocus() }
    PlainField(
        title, { title = it; error = null }, stringResource(R.string.item_what),
        TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.SemiBold), Modifier.fillMaxWidth().padding(start = 4.dp, top = 8.dp, end = 4.dp).focusRequester(focus),
        imeAction = ImeAction.Done, onDone = submit,
    )
    FormError(error)
    FlowRow(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        Chip(stringResource(R.string.nav_today), KIcons.Star, set = today) { today = !today }
        Chip(due?.let { stringResource(R.string.due_label, f.dayLabel(it)) } ?: stringResource(R.string.field_due), KIcons.Flag, set = due != null) { if (due == null) dialog = "due" else due = null }
        Chip(estimate?.let(f::dur) ?: stringResource(R.string.field_estimate), KIcons.Clock, set = estimate != null) { dialog = "estimate" }
        Chip(project?.name ?: stringResource(R.string.nav_inbox), KIcons.Tray, set = project != null, dot = project?.let { K.project(it.color) }) { dialog = "project" }
        Chip(stringResource(R.string.item_important), KIcons.Bang, set = important) { important = !important }
    }
    when (dialog) {
        "due" -> DayPickerDialog(f.today, { dialog = "" }) { due = it; dialog = "" }
        "estimate" -> EstimateDialog(estimate, { dialog = "" }) { estimate = it; dialog = "" }
        "project" -> ProjectDialog(projectId, stringResource(R.string.nav_inbox), { dialog = "" }) { projectId = it; dialog = "" }
    }
}

/* ---------- project form ---------- */

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun ProjectFormSheet(project: Project?, close: () -> Unit) {
    val app = LocalApp.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    val boot by app.boot.collectAsStateWithLifecycle()
    var name by rememberSaveable { mutableStateOf(project?.name.orEmpty()) }
    var notes by rememberSaveable { mutableStateOf(project?.notes.orEmpty()) }
    var color by rememberSaveable { mutableStateOf(project?.color) }
    var areaId by rememberSaveable { mutableStateOf(project?.areaId) }
    var error by remember { mutableStateOf<String?>(null) }
    val submit = {
        if (name.isNotBlank() && !busy) scope.launch {
            val outcome = app.attempt { api ->
                if (project == null) {
                    api.createProject(name.trim(), color, areaId, notes.trim())
                } else {
                    api.updateProject(
                        project.id,
                        ProjectPatch(
                            name = if (name.trim() != project.name) Opt.Put(name.trim()) else Opt.Keep,
                            color = if (color != null && color != project.color) Opt.Put(color) else Opt.Keep,
                            // No area is sent as null: the project leaves the area it was in.
                            areaId = if (areaId != project.areaId) Opt.Put(areaId) else Opt.Keep,
                            notes = if (notes.trim() != project.notes) Opt.Put(notes.trim()) else Opt.Keep,
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
        Unit
    }
    SheetBar(
        stringResource(if (project == null) R.string.project_new else R.string.project_edit), stringResource(if (project == null) R.string.create else R.string.save),
        enabled = name.isNotBlank() && !busy, onCancel = close, onAction = submit,
    )
    OutlinedTextField(name, { name = it; error = null }, Modifier.fillMaxWidth(), singleLine = true, colors = fieldColors(), label = { Text(stringResource(R.string.project_name)) })
    OutlinedTextField(notes, { notes = it }, Modifier.fillMaxWidth().padding(top = 8.dp), minLines = 2, maxLines = 5, colors = fieldColors(), label = { Text(stringResource(R.string.item_notes)) })
    FormError(error)
    Text(stringResource(R.string.project_color), fontSize = 13.sp, color = K.label2, modifier = Modifier.padding(start = 4.dp, top = 14.dp))
    val colorNames = androidx.compose.ui.res.stringArrayResource(R.array.color_names)
    FlowRow {
        PROJECT_COLORS.forEachIndexed { k, key ->
            Box(Modifier.size(48.dp).clip(CircleShape).clickable(role = Role.RadioButton) { color = key }.semantics { contentDescription = colorNames[k] }, contentAlignment = Alignment.Center) {
                Box(Modifier.size(30.dp).background(K.project(key), CircleShape).then(if (color == key) Modifier.border(3.dp, K.sheet, CircleShape) else Modifier))
                if (color == key) Box(Modifier.size(38.dp).border(2.dp, K.project(key), CircleShape))
            }
        }
    }
    val areas = boot?.areas.orEmpty().sortedBy { it.position }
    if (areas.isNotEmpty()) {
        Text(stringResource(R.string.project_area), fontSize = 13.sp, color = K.label2, modifier = Modifier.padding(start = 4.dp, top = 8.dp))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Chip(stringResource(R.string.none), set = areaId == null, cell = if (areaId == null) K.wash(K.tint, 0.2f, K.sheetCell) else K.sheetCell) { areaId = null }
            for (area in areas) Chip(area.name, set = areaId == area.id, cell = if (areaId == area.id) K.wash(K.tint, 0.2f, K.sheetCell) else K.sheetCell) { areaId = area.id }
        }
    }
}
