// What the whole app shares: the session, the account's lists and counts, the focus timer, and
// the one way writes are made. Nothing is kept on disk besides the session; screens hold their
// data in memory and reload it when [AppModel.tick] moves.
package io.github.lewin671.keduly.ui

import android.app.Application
import android.os.SystemClock
import androidx.compose.runtime.mutableStateMapOf
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.ApiClient
import io.github.lewin671.keduly.api.ApiException
import io.github.lewin671.keduly.api.Bootstrap
import io.github.lewin671.keduly.api.Event
import io.github.lewin671.keduly.api.Focus
import io.github.lewin671.keduly.api.FocusSession
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.ItemPatch
import io.github.lewin671.keduly.api.KeystoreSessionStore
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.api.Project
import io.github.lewin671.keduly.api.SessionStore
import io.github.lewin671.keduly.api.defaultHttpClient
import io.github.lewin671.keduly.core.ServerClock
import io.github.lewin671.keduly.core.WriteQueue
import io.github.lewin671.keduly.core.Times
import java.time.LocalDate
import java.time.ZoneId
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class Phase { Starting, SignedOut, SignedIn }

/** What is risen from the bottom of the screen. */
sealed interface Sheet {
    data class ItemDetail(val id: String) : Sheet
    data class NewItem(val projectId: String? = null, val today: Boolean = false) : Sheet
    data class EventForm(val event: Event?, val date: LocalDate) : Sheet
    data class EventInfo(val event: Event) : Sheet
    data class EventList(val events: List<Event>) : Sheet
    data class Filing(val session: FocusSession) : Sheet
    data class ProjectForm(val project: Project?) : Sheet
    data object FocusPick : Sheet
}

/** How a write ended. Only a rejected form ([Refused]) is left for the caller to show. */
sealed interface Outcome<out T> {
    data class Done<T>(val value: T) : Outcome<T>

    /** The server refused the content (400); [message] is its own and belongs next to the form. */
    data class Refused(val message: String) : Outcome<Nothing>

    /** Anything else; the user has already been told. */
    data object Failed : Outcome<Nothing>
}

/** What the next tomato is for: an item, or free focus. */
data class Pick(val item: Item?)

class AppModel(application: Application) : AndroidViewModel(application) {
    val store: SessionStore = KeystoreSessionStore(application)
    val api = ApiClient(defaultHttpClient(), store) { endSession() }

    val phase = MutableStateFlow(if (store.server != null && store.cookie != null) Phase.Starting else Phase.SignedOut)
    val boot = MutableStateFlow<Bootstrap?>(null)

    /** The account's current day, as `GET /today` reports it. */
    val today = MutableStateFlow<LocalDate?>(null)
    val focus = MutableStateFlow<Focus?>(null)

    /** Moves whenever what is on screen may be out of date; screens reload when it does. */
    val tick = MutableStateFlow(0)

    /** True while a write is in flight: every button that writes is disabled meanwhile. */
    private val writes = WriteQueue()
    val busy = writes.busy
    val messages = MutableSharedFlow<String>(extraBufferCapacity = 8)

    /** Why the account's data could not be loaded, when it could not. */
    val bootError = MutableStateFlow<String?>(null)
    val sheet = MutableStateFlow<Sheet?>(null)

    /** Whether the running timer covers the whole screen. */
    val zen = MutableStateFlow(false)
    val pick = MutableStateFlow<Pick?>(null)

    /** The calendar's selected day (`null` is today) and its view: 0 is the day, 1 the month. */
    val calendarDate = MutableStateFlow<LocalDate?>(null)
    val calendarMode = MutableStateFlow(1)

    /** Items ticked on the page that is showing: they stay where they are until the page is left. */
    val sticky = mutableStateMapOf<String, Item>()

    /** Counts the sessions of this process, so that nothing of one session's screens carries over into the next. */
    val generation = MutableStateFlow(0)

    private var clock: ServerClock? = null
    private var revision = -1L
    private var poll: Job? = null
    private var foreground = false

    fun text(id: Int, vararg args: Any): String = getApplication<Application>().getString(id, *args)

    fun zone(): ZoneId = boot.value?.user?.timezone?.let { runCatching { ZoneId.of(it) }.getOrNull() } ?: ZoneId.systemDefault()

    /** The server's clock, estimated from the last reply and the time that passed since. */
    fun serverNow(): Long = clock?.now(SystemClock.elapsedRealtime()) ?: System.currentTimeMillis()

    fun describe(e: ApiException): String = when {
        e.timedOut -> text(R.string.error_timeout)
        e.offline -> text(R.string.error_network)
        e.status == 400 -> e.message?.takeIf { it.isNotBlank() } ?: text(R.string.error_invalid_request)
        e.status == 401 -> text(R.string.error_unauthenticated)
        e.status == 403 -> text(if (e.code == "registration_closed") R.string.error_registration_closed else R.string.error_forbidden)
        e.status == 404 -> text(R.string.error_not_found)
        e.status == 409 -> text(R.string.error_conflict)
        e.status == 429 -> text(R.string.error_rate_limited)
        e.code == "bad_response" -> text(R.string.error_bad_response)
        else -> text(R.string.error_http)
    }

    /* ---------- session ---------- */

    /** A 401 anywhere: forget the session and everything that was loaded. */
    private fun endSession() {
        store.clear()
        poll?.cancel()
        boot.value = null
        today.value = null
        focus.value = null
        clock = null
        revision = -1
        sheet.value = null
        zen.value = false
        pick.value = null
        calendarDate.value = null
        calendarMode.value = 1
        sticky.clear()
        bootError.value = null
        generation.update { it + 1 }
        phase.value = Phase.SignedOut
    }

    /** Signs in or registers. Returns the message to show next to the form, or `null` when it worked. */
    suspend fun signIn(server: String, email: String, password: String, name: String?): String? {
        busy.value = true
        try {
            store.server = server
            store.cookie = null
            if (name == null) api.login(email, password) else api.register(email, password, name, ZoneId.systemDefault().id)
            if (store.cookie == null) return text(R.string.error_bad_response)
            load()
            phase.value = Phase.SignedIn
            startPolling()
            return null
        } catch (e: ApiException) {
            store.cookie = null
            return when {
                e.status == 401 -> text(R.string.auth_wrong)
                e.status == 409 -> text(R.string.auth_taken)
                else -> describe(e)
            }
        } finally {
            busy.value = false
        }
    }

    fun signOut() {
        viewModelScope.launch {
            busy.value = true
            try {
                api.logout()
            } catch (e: ApiException) {
                // The session is dropped on this device either way.
            }
            busy.value = false
            endSession()
        }
    }

    /** Loads the account when the app starts with a stored session. */
    fun start() {
        if (phase.value != Phase.Starting) return
        viewModelScope.launch {
            bootError.value = null
            try {
                load()
                phase.value = Phase.SignedIn
                startPolling()
            } catch (e: ApiException) {
                if (e.status != 401) bootError.value = describe(e)
            }
        }
    }

    private suspend fun load() = coroutineScope {
        val bootstrap = async { api.bootstrap() }
        val day = async { api.today().date }
        val timer = async { api.focus() }
        val b = bootstrap.await()
        today.value = Times.date(day.await())
        setFocus(timer.await())
        revision = b.revision
        boot.value = b
    }

    /** Reloads the lists and counts, then tells the screens to reload. Failures leave what is shown. */
    fun refresh() {
        viewModelScope.launch {
            try {
                load()
            } catch (e: ApiException) {
                // The page that is showing reports its own failure and offers to retry.
            }
            tick.update { it + 1 }
        }
    }

    fun setForeground(on: Boolean) {
        foreground = on
        if (!on) {
            poll?.cancel()
        } else if (phase.value == Phase.SignedIn) {
            refresh()
            startPolling()
        }
    }

    private fun startPolling() {
        poll?.cancel()
        if (!foreground) return
        poll = viewModelScope.launch {
            while (true) {
                delay(POLL_MILLIS)
                try {
                    val reply = api.counts()
                    val dayChanged = today.value != LocalDate.now(zone())
                    if (reply.revision != revision || dayChanged) {
                        load()
                        tick.update { it + 1 }
                    } else {
                        setFocus(api.focus())
                    }
                } catch (e: ApiException) {
                    // Offline for now; the next round tries again.
                }
            }
        }
    }

    /* ---------- writes ---------- */

    /**
     * Makes one write. It is never retried: when it timed out, what is shown is reloaded so the
     * user sees whether it arrived before trying again. A write asked for while another is in
     * flight (text saved as a sheet closes, say) waits its turn instead of being dropped.
     */
    suspend fun <T> attempt(block: suspend (ApiClient) -> T): Outcome<T> = writes.run {
        try {
            val value = block(api)
            refresh()
            Outcome.Done(value)
        } catch (e: ApiException) {
            when {
                e.status == 400 -> return@run Outcome.Refused(describe(e))
                e.status == 409 -> {
                    messages.tryEmit(text(R.string.error_conflict))
                    refresh()
                }
                e.timedOut -> {
                    messages.tryEmit(text(R.string.error_timeout))
                    refresh()
                }
                e.status == 401 -> Unit
                else -> messages.tryEmit(describe(e))
            }
            Outcome.Failed
        }
    }

    /** A write made from a button with no form: a refusal is shown like any other failure. */
    fun act(then: () -> Unit = {}, block: suspend (ApiClient) -> Unit) {
        viewModelScope.launch {
            when (val outcome = attempt(block)) {
                is Outcome.Done -> then()
                is Outcome.Refused -> messages.tryEmit(outcome.message)
                Outcome.Failed -> Unit
            }
        }
    }

    fun toggleDone(item: Item) {
        viewModelScope.launch {
            val outcome = attempt { it.updateItem(item.id, ItemPatch(status = Opt.Put(if (item.done) "open" else "done"))) }
            if (outcome is Outcome.Done) sticky[item.id] = outcome.value
            if (outcome is Outcome.Refused) messages.tryEmit(outcome.message)
        }
    }

    /* ---------- focus ---------- */

    private fun setFocus(next: Focus) {
        clock = ServerClock(Times.instant(next.now).toEpochMilli(), SystemClock.elapsedRealtime())
        focus.value = next
        if (next.state == "idle") zen.value = false
    }

    fun refreshFocus() {
        viewModelScope.launch {
            try {
                setFocus(api.focus())
            } catch (e: ApiException) {
                // Keep counting from what is known.
            }
        }
    }

    private fun focusWrite(open: Boolean = false, block: suspend (ApiClient) -> Focus) {
        viewModelScope.launch {
            val outcome = attempt(block)
            if (outcome is Outcome.Done) {
                setFocus(outcome.value)
                if (open && outcome.value.state != "idle") zen.value = true
            }
            if (outcome is Outcome.Refused) messages.tryEmit(outcome.message)
        }
    }

    fun startFocus(item: Item?) = focusWrite(open = true) { it.startFocus(item?.id) }

    /** Another tomato on the same thing; free focus keeps its title and project. */
    fun focusAgain() {
        val session = focus.value?.session ?: return
        focusWrite(open = true) { it.startFocus(session.itemId, session.title, session.projectId) }
    }

    fun stopFocus() = focusWrite { it.stopFocus() }
    fun restFocus() = focusWrite(open = true) { it.restFocus() }

    /** Ticks the item the timer is on, which also ends the timer. */
    fun finishFocus() {
        val itemId = focus.value?.session?.itemId ?: return
        focusWrite {
            it.updateItem(itemId, ItemPatch(status = Opt.Put("done")))
            it.focus()
        }
    }

    private companion object {
        const val POLL_MILLIS = 20_000L
    }
}

data class PageState(val data: Any? = null, val loading: Boolean = true, val error: String? = null)

/** One screen's data: fetched when the screen appears and whenever [AppModel.tick] moves. */
class PageModel : ViewModel() {
    val state = MutableStateFlow(PageState())
    private var job: Job? = null

    fun load(app: AppModel, fetch: suspend (ApiClient) -> Any?) {
        job?.cancel()
        job = viewModelScope.launch {
            state.update { it.copy(loading = true) }
            try {
                state.value = PageState(fetch(app.api), loading = false)
            } catch (e: ApiException) {
                // What was loaded stays on screen, with a line that offers to retry.
                state.update { it.copy(loading = false, error = if (e.offline) app.text(R.string.error_network) else app.describe(e)) }
            }
        }
    }
}
