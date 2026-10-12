// The two pushed pages reached from the bell and the gear: activity and settings.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextRange
import androidx.compose.ui.text.input.TextFieldValue
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextDecoration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Activity
import io.github.lewin671.keduly.api.MePatch
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.api.Suggestion
import io.github.lewin671.keduly.core.Times
import java.time.LocalTime
import kotlinx.coroutines.launch

/* ---------- activity ---------- */

private class ActivityData(val pending: List<Suggestion>, val recent: List<Activity>, val more: Boolean)

@Composable
private fun suggestionText(s: Suggestion): String {
    val f = LocalFmt.current
    val slot = s.start?.let(f::slotLabel)
    return when (s.kind) {
        "schedule_item" -> stringResource(R.string.pending_schedule, s.title, slot.orEmpty())
        "create_item" -> if (slot != null) stringResource(R.string.pending_create_item_at, s.title, slot) else stringResource(R.string.pending_create_item, s.title)
        "create_event" -> stringResource(R.string.pending_create_event, s.title, s.event?.get("start")?.let { runCatching { f.slotLabel(it.toString().trim('"')) }.getOrNull() }.orEmpty())
        "move_event" -> stringResource(R.string.pending_move, s.title, slot.orEmpty())
        else -> stringResource(R.string.activity_wants_delete, s.title)
    }
}

@Composable
fun ActivityScreen() {
    val app = LocalApp.current
    val f = LocalFmt.current
    val busy by app.busy.collectAsStateWithLifecycle()
    var limit by rememberSaveable { mutableIntStateOf(20) }
    val page = usePage("activity", limit) { api ->
        val pending = api.suggestions()
        val recent = api.activity(limit)
        ActivityData(pending, recent.activities, recent.nextCursor != null)
    }
    Screen(stringResource(R.string.activity_title)) {
        PageBody(page) { data ->
            if (data.pending.isEmpty() && data.recent.isEmpty()) {
                Blank(stringResource(R.string.activity_empty), stringResource(R.string.activity_empty_tip))
                return@PageBody
            }
            Scroll {
                if (data.pending.isNotEmpty()) {
                    Sec(stringResource(R.string.activity_todo), data.pending.size.toString(), first = true)
                    data.pending.forEachIndexed { k, suggestion ->
                        val delete = suggestion.kind.startsWith("delete")
                        Column(Modifier.fillMaxWidth().then(if (k == data.pending.lastIndex) Modifier else Modifier.hairline(K.sep)).padding(vertical = 10.dp)) {
                            Text(suggestionText(suggestion), fontSize = 17.sp, lineHeight = 22.sp)
                            if (suggestion.reason.isNotBlank()) Text(suggestion.reason, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
                            Text("${suggestion.actor.name} · ${f.stamp(suggestion.createdAt)}", fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
                            Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                PillButton(stringResource(R.string.suggest_reject), Modifier.weight(1f), enabled = !busy, height = 40.dp) { app.act { it.rejectSuggestion(suggestion.id) } }
                                PillButton(stringResource(if (delete) R.string.activity_allow else R.string.suggest_accept), Modifier.weight(1f), primary = true, enabled = !busy, height = 40.dp) {
                                    app.act { it.acceptSuggestion(suggestion.id) }
                                }
                            }
                        }
                    }
                    if (data.pending.size > 1) MoreLink(stringResource(R.string.activity_accept_all), inset = false) { if (!busy) app.act { it.acceptAllSuggestions() } }
                }
                if (data.recent.isNotEmpty()) Sec(stringResource(R.string.activity_recent), first = data.pending.isEmpty())
                data.recent.forEachIndexed { k, entry ->
                    Row(Modifier.fillMaxWidth().then(if (k == data.recent.lastIndex) Modifier else Modifier.hairline(K.sep)).padding(vertical = 8.dp), verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(entry.summary, fontSize = 17.sp, lineHeight = 22.sp, color = if (entry.undone) K.label2 else K.label, textDecoration = if (entry.undone) TextDecoration.LineThrough else null)
                            if (!entry.reason.isNullOrBlank()) Text(entry.reason, fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp))
                            Text(
                                "${if (entry.actor.kind == "user") stringResource(R.string.activity_you) else entry.actor.name} · ${f.stamp(entry.createdAt)}",
                                fontSize = 13.sp, lineHeight = 18.sp, color = K.label2, modifier = Modifier.padding(top = 2.dp),
                            )
                        }
                        // Each change can be undone, and an undone one applied again.
                        if (entry.undone) TextButton(enabled = !busy, onClick = { app.act { it.redoActivity(entry.id) } }) { Text(stringResource(R.string.activity_redo), fontSize = 15.sp, color = K.tint) }
                        else if (entry.undoable) TextButton(enabled = !busy, onClick = { app.act { it.undoActivity(entry.id) } }) { Text(stringResource(R.string.activity_undo), fontSize = 15.sp, color = K.tint) }
                    }
                }
                if (data.more && limit < 200) MoreLink(stringResource(R.string.activity_older), inset = false) { limit = minOf(200, limit + 40) }
            }
        }
    }
}

/* ---------- settings ---------- */

/** Edits one value of the account. The server's refusal shows under the field. */
@Composable
private fun ValueDialog(title: String, initial: String, number: Boolean, suffix: String = "", onDismiss: () -> Unit, patch: (String) -> MePatch?) {
    val app = LocalApp.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    // The value starts selected, so typing replaces it.
    var field by remember { mutableStateOf(TextFieldValue(initial, TextRange(0, initial.length))) }
    var error by remember { mutableStateOf<String?>(null) }
    val body = patch(field.text.trim())
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { focus.requestFocus() }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                OutlinedTextField(
                    field, { value -> field = if (number) value.copy(text = value.text.filter(Char::isDigit).take(3)) else value; error = null },
                    Modifier.fillMaxWidth().focusRequester(focus), singleLine = true, colors = fieldColors(),
                    suffix = if (suffix.isEmpty()) null else ({ Text(suffix) }),
                    keyboardOptions = KeyboardOptions(keyboardType = if (number) KeyboardType.Number else KeyboardType.Text),
                )
                FormError(error)
            }
        },
        confirmButton = {
            TextButton(enabled = body != null && !busy, onClick = {
                scope.launch {
                    when (val outcome = app.attempt { it.updateMe(body!!) }) {
                        is Outcome.Done -> onDismiss()
                        is Outcome.Refused -> error = outcome.message
                        Outcome.Failed -> Unit
                    }
                }
            }) { Text(stringResource(R.string.save)) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
fun SettingsScreen() {
    val app = LocalApp.current
    val boot by app.boot.collectAsStateWithLifecycle()
    val busy by app.busy.collectAsStateWithLifecycle()
    val user = boot?.user ?: return
    var dialog by remember { mutableStateOf("") }
    val minutes = stringResource(R.string.focus_settings_minutes)
    Screen(stringResource(R.string.settings_title), grouped = true) {
        Scroll {
            GroupHeader(stringResource(R.string.account_title))
            Group {
                GroupRow(stringResource(R.string.account_name), user.name, leading = { RowIcon(KIcons.Person, K.blue) }) { dialog = "name" }
                GroupRow(stringResource(R.string.account_email), user.email, leading = { RowIcon(KIcons.Tray, K.blue) })
                GroupRow(stringResource(R.string.account_timezone), user.timezone, last = true, leading = { RowIcon(KIcons.Clock, K.blue) })
            }
            GroupNote(stringResource(R.string.account_timezone_note))
            GroupHeader(stringResource(R.string.account_hours))
            Group {
                GroupRow(stringResource(R.string.account_work_start), user.workStart, leading = { RowIcon(KIcons.Calendar, K.orange) }) { dialog = "workStart" }
                GroupRow(stringResource(R.string.account_work_end), user.workEnd, last = true, leading = { RowIcon(KIcons.Calendar, K.orange) }) { dialog = "workEnd" }
            }
            GroupHeader(stringResource(R.string.mode_focus))
            Group {
                GroupRow(stringResource(R.string.focus_settings_tomato), stringResource(R.string.dur_minutes, user.focusMinutes), leading = { Tomato(size = 21.dp) }) { dialog = "focus" }
                GroupRow(stringResource(R.string.focus_settings_rest), stringResource(R.string.dur_minutes, user.restMinutes), leading = { RowIcon(KIcons.Timer, K.green) }) { dialog = "rest" }
                GroupRow(stringResource(R.string.focus_settings_long_rest), stringResource(R.string.dur_minutes, user.longRestMinutes), leading = { RowIcon(KIcons.Timer, K.green) }) { dialog = "longRest" }
                GroupRow(stringResource(R.string.focus_settings_round), stringResource(R.string.stats_count, user.roundSize), last = true, leading = { RowIcon(KIcons.Grid, K.green) }) { dialog = "round" }
            }
            GroupNote(stringResource(R.string.focus_settings_footer))
            GroupHeader(stringResource(R.string.auth_server))
            Group {
                GroupRow(app.store.server.orEmpty().substringAfter("://"), last = true, leading = { RowIcon(KIcons.Layers, K.teal) })
            }
            Group(Modifier.padding(top = 22.dp)) {
                GroupRow(stringResource(R.string.account_sign_out), last = true, labelColor = K.red, chevron = false, enabled = !busy) { dialog = "signOut" }
            }
        }
    }
    val close = { dialog = "" }
    fun range(text: String, range: IntRange): Int? = text.toIntOrNull()?.takeIf { it in range }
    when (dialog) {
        "name" -> ValueDialog(stringResource(R.string.account_name), user.name, number = false, onDismiss = close) { if (it.isEmpty()) null else MePatch(name = Opt.Put(it)) }
        "focus" -> ValueDialog(stringResource(R.string.focus_settings_tomato), user.focusMinutes.toString(), true, minutes, close) { text -> range(text, 1..180)?.let { MePatch(focusMinutes = Opt.Put(it)) } }
        "rest" -> ValueDialog(stringResource(R.string.focus_settings_rest), user.restMinutes.toString(), true, minutes, close) { text -> range(text, 1..60)?.let { MePatch(restMinutes = Opt.Put(it)) } }
        "longRest" -> ValueDialog(stringResource(R.string.focus_settings_long_rest), user.longRestMinutes.toString(), true, minutes, close) { text -> range(text, 1..120)?.let { MePatch(longRestMinutes = Opt.Put(it)) } }
        "round" -> ValueDialog(stringResource(R.string.focus_settings_round), user.roundSize.toString(), true, onDismiss = close) { text -> range(text, 2..12)?.let { MePatch(roundSize = Opt.Put(it)) } }
        "workStart" -> TimeDialog(runCatching { Times.time(user.workStart) }.getOrDefault(LocalTime.of(9, 0)), close) { close(); app.act { api -> api.updateMe(MePatch(workStart = Opt.Put(Times.hhmm(it)))) } }
        "workEnd" -> TimeDialog(runCatching { Times.time(user.workEnd) }.getOrDefault(LocalTime.of(18, 0)), close) { close(); app.act { api -> api.updateMe(MePatch(workEnd = Opt.Put(Times.hhmm(it)))) } }
        "signOut" -> AlertDialog(
            onDismissRequest = close,
            title = { Text(stringResource(R.string.account_sign_out_ask)) },
            text = { Text(user.email) },
            confirmButton = { TextButton(onClick = { close(); app.signOut() }) { Text(stringResource(R.string.account_sign_out), color = K.red, fontWeight = FontWeight.SemiBold) } },
            dismissButton = { TextButton(onClick = close) { Text(stringResource(R.string.cancel)) } },
        )
    }
}
