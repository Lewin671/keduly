// Every request the app makes goes through this class, one function per endpoint of docs/api.md.
package io.github.lewin671.keduly.api

import java.io.IOException
import java.io.InterruptedIOException
import java.util.concurrent.TimeUnit
import kotlin.coroutines.coroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody

const val SESSION_COOKIE = "keduly_session"

/**
 * A failed request. [status] is the HTTP status, or 0 when no answer arrived: then [code] is
 * `timeout` (the request may or may not have reached the server) or `network`.
 */
class ApiException(val status: Int, val code: String, message: String) : Exception(message) {
    val timedOut: Boolean get() = code == "timeout"
    val offline: Boolean get() = status == 0
}

fun defaultHttpClient(): OkHttpClient = OkHttpClient.Builder()
    .connectTimeout(10, TimeUnit.SECONDS)
    .readTimeout(20, TimeUnit.SECONDS)
    .writeTimeout(20, TimeUnit.SECONDS)
    // A failed write is never repeated behind the user's back.
    .retryOnConnectionFailure(false)
    .followRedirects(false)
    .build()

/** Turns what the user typed into the server's origin, or `null` when it is not an address. */
fun normalizeServer(input: String): String? {
    val text = input.trim().trimEnd('/')
    if (text.isEmpty()) return null
    val url = (if ("://" in text) text else "https://$text").toHttpUrlOrNull() ?: return null
    return url.toString().trimEnd('/')
}

class ApiClient(
    private val http: OkHttpClient,
    private val store: SessionStore,
    /** Called when the server no longer accepts the session. */
    private val onUnauthorized: () -> Unit = {},
) {
    private val jsonType = "application/json; charset=utf-8".toMediaType()

    private fun url(path: String, query: List<Pair<String, String?>>): HttpUrl {
        val base = store.server?.toHttpUrlOrNull() ?: throw ApiException(0, "network", "no server address")
        val builder = base.newBuilder().addPathSegments("api/v1/" + path.trimStart('/'))
        for ((name, value) in query) if (value != null) builder.addQueryParameter(name, value)
        return builder.build()
    }

    /** Sends one request and returns the parsed body, or `null` for an empty response. */
    suspend fun request(
        method: String,
        path: String,
        query: List<Pair<String, String?>> = emptyList(),
        body: JsonObject? = null,
        /** False for the endpoints that are called without a session: a 401 there is an answer, not an expiry. */
        session: Boolean = true,
    ): JsonObject? = withContext(Dispatchers.IO) {
        val builder = Request.Builder().url(url(path, query)).header("Accept", "application/json").header("User-Agent", "Keduly/Android")
        if (method != "GET") builder.header("X-Keduly-Request", "1")
        val server = store.server
        val sent = store.cookie
        if (sent != null) builder.header("Cookie", "$SESSION_COOKIE=$sent")
        builder.method(method, if (method == "GET") null else (body ?: JsonObject(emptyMap())).toString().toRequestBody(jsonType))
        val (status, text, cookies) = try {
            http.newCall(builder.build()).execute().use { Triple(it.code, it.body.string(), it.headers("Set-Cookie")) }
        } catch (e: InterruptedIOException) {
            throw ApiException(0, "timeout", "timed out")
        } catch (e: IOException) {
            throw ApiException(0, "network", e.message ?: "network error")
        }
        coroutineContext.ensureActive()
        // What a response says about the session applies only to the session the request was sent
        // with. An answer that arrives after the user signed in again must neither end the new
        // session nor replace its cookie.
        val expired = synchronized(store) {
            val current = store.cookie == sent && store.server == server
            // The server sends the cookie again whenever it pushes the expiry out, on any response.
            if (current) for (header in cookies) renewedCookie(header)?.let { store.cookie = it.ifEmpty { null } }
            val ended = current && status == 401 && session
            // Dropped here, under the lock: a second late 401 of the same session then finds it is
            // no longer current and cannot end a session the user starts afterwards.
            if (ended) store.cookie = null
            ended
        }
        if (status !in 200..299) {
            val error = failure(status, text)
            if (expired) onUnauthorized()
            throw error
        }
        if (text.isBlank()) return@withContext null
        try {
            ApiJson.parseToJsonElement(text).jsonObject
        } catch (e: Exception) {
            throw ApiException(status, "bad_response", "response is not JSON")
        }
    }

    private fun failure(status: Int, text: String): ApiException {
        try {
            val error = ApiJson.parseToJsonElement(text).jsonObject["error"]?.jsonObject
            val code = error?.get("code")?.jsonPrimitive?.content
            if (code != null) return ApiException(status, code, error["message"]?.jsonPrimitive?.content.orEmpty())
        } catch (e: Exception) {
            // Not JSON: a proxy answered instead of the server.
        }
        return ApiException(status, "http", "HTTP $status")
    }

    private suspend inline fun <reified T> get(path: String, key: String?, vararg query: Pair<String, String?>): T =
        decode(request("GET", path, query.toList()), key)

    private suspend inline fun <reified T> send(method: String, path: String, key: String?, body: JsonObject? = null): T =
        decode(request(method, path, body = body), key)

    private inline fun <reified T> decode(reply: JsonObject?, key: String?): T {
        val element: JsonElement = (if (key == null) reply else reply?.get(key)) ?: throw ApiException(200, "bad_response", "missing ${key ?: "body"}")
        try {
            return ApiJson.decodeFromJsonElement(element)
        } catch (e: Exception) {
            throw ApiException(200, "bad_response", e.message ?: "unexpected response")
        }
    }

    /* ---------- service and account ---------- */

    suspend fun config(): Config = decode(request("GET", "config", session = false), null)

    suspend fun login(email: String, password: String): User =
        decode(request("POST", "auth/login", body = buildJsonObject { put("email", email); put("password", password) }, session = false), "user")

    suspend fun register(email: String, password: String, name: String, timezone: String): User = decode(
        request("POST", "auth/register", body = buildJsonObject { put("email", email); put("password", password); put("name", name); put("timezone", timezone) }, session = false),
        "user",
    )

    suspend fun startLoginRequest(): DeviceLoginRequest =
        decode(request("POST", "auth/requests", session = false), null)

    suspend fun claimLoginRequest(login: DeviceLoginRequest): Boolean {
        val claim: DeviceLoginClaim = decode(request("POST", "auth/requests/${login.request.id}/claim",
            body = buildJsonObject { put("secret", login.secret) }, session = false), null)
        return when (claim.status) {
            "pending" -> false
            "approved" -> {
                if (claim.user == null || store.cookie == null) throw ApiException(200, "bad_response", "missing authorized session")
                true
            }
            else -> throw ApiException(200, "bad_response", "unexpected authorization status")
        }
    }

    suspend fun logout() { request("POST", "auth/logout", session = false) }
    suspend fun updateMe(patch: MePatch): User = send("PATCH", "me", "user", patch.toJson())

    /* ---------- bootstrap and change detection ---------- */

    suspend fun bootstrap(): Bootstrap = get("bootstrap", null)
    suspend fun counts(): CountsReply = get("counts", null)

    /* ---------- projects ---------- */

    suspend fun createProject(name: String, color: String?, areaId: String?, notes: String): Project = send(
        "POST", "projects", "project",
        buildJsonObject { put("name", name); if (color != null) put("color", color); if (areaId != null) put("area_id", areaId); if (notes.isNotEmpty()) put("notes", notes) },
    )

    suspend fun project(id: String): ProjectDetail = get("projects/$id", null)
    suspend fun updateProject(id: String, patch: ProjectPatch): Project = send("PATCH", "projects/$id", "project", patch.toJson())

    /* ---------- items ---------- */

    suspend fun items(
        status: String? = null, projectId: String? = null, quadrant: String? = null, q: String? = null,
        limit: Int? = null, cursor: String? = null,
    ): ItemPage = get(
        "items", null, "status" to status, "project_id" to projectId, "quadrant" to quadrant, "q" to q,
        "limit" to limit?.toString(), "cursor" to cursor,
    )

    /** The first [count] items of a list, following cursors when that is more than one page. */
    suspend fun itemsUpTo(count: Int, status: String? = null, projectId: String? = null, quadrant: String? = null, q: String? = null): ItemPage {
        val all = ArrayList<Item>()
        var cursor: String? = null
        var total: Int
        do {
            val page = items(status, projectId, quadrant, q, minOf(count - all.size, 200), cursor)
            all += page.items
            total = page.total
            cursor = page.nextCursor
        } while (cursor != null && all.size < count)
        return ItemPage(all, cursor, total)
    }

    suspend fun createItem(title: String, fields: ItemPatch): Item =
        send("POST", "items", "item", JsonObject(mapOf<String, JsonElement>("title" to JsonPrimitive(title)) + fields.toJson()))

    suspend fun item(id: String): Item = get("items/$id", "item")
    suspend fun updateItem(id: String, patch: ItemPatch): Item = send("PATCH", "items/$id", "item", patch.toJson())
    suspend fun deleteItem(id: String) { request("DELETE", "items/$id") }
    suspend fun scheduleItem(id: String, start: String, end: String): Item =
        send("POST", "items/$id/schedule", "item", buildJsonObject { put("start", start); put("end", end) })
    suspend fun unscheduleItem(id: String): Item = send("DELETE", "items/$id/schedule", "item")

    /* ---------- views ---------- */

    suspend fun today(): TodayView = get("today", null)
    suspend fun upcoming(from: String, to: String): List<UpcomingDay> = get("upcoming", "days", "from" to from, "to" to to)
    suspend fun overview(): List<OverviewProject> = get("overview", "projects")
    suspend fun matrix(): Quadrants = get("matrix", "quadrants")

    /* ---------- events ---------- */

    suspend fun events(from: String, to: String): List<Event> = get("events", "events", "from" to from, "to" to to)
    suspend fun createEvent(fields: EventPatch): Event = send("POST", "events", "event", fields.toJson())
    suspend fun updateEvent(id: String, patch: EventPatch): Event = send("PATCH", "events/$id", "event", patch.toJson())
    suspend fun deleteEvent(id: String) { request("DELETE", "events/$id") }
    suspend fun free(date: String, duration: Int): List<FreeSlot> = get("free", "slots", "date" to date, "duration" to duration.toString())

    /* ---------- focus ---------- */

    suspend fun focus(): Focus = get("focus", "focus")

    /** Starts a tomato on the item, or free focus without one, which may have a title and a project of its own. */
    suspend fun startFocus(itemId: String?, title: String? = null, projectId: String? = null): Focus = send(
        "POST", "focus/start", "focus",
        buildJsonObject {
            if (itemId != null) put("item_id", itemId)
            else {
                if (!title.isNullOrEmpty()) put("title", title)
                if (projectId != null) put("project_id", projectId)
            }
        },
    )

    suspend fun stopFocus(): Focus = send("POST", "focus/stop", "focus")
    suspend fun restFocus(): Focus = send("POST", "focus/rest", "focus")
    suspend fun focusSessions(from: String, to: String): List<FocusSession> = get("focus/sessions", "sessions", "from" to from, "to" to to)
    suspend fun updateSession(id: String, patch: SessionPatch): FocusSession = send("PATCH", "focus/sessions/$id", "session", patch.toJson())
    suspend fun focusStats(): FocusStats = get("focus/stats", null)

    /* ---------- suggestions ---------- */

    suspend fun suggestions(): List<Suggestion> = get("suggestions", "suggestions", "status" to "pending")
    suspend fun acceptSuggestion(id: String) { request("POST", "suggestions/$id/accept") }
    suspend fun rejectSuggestion(id: String) { request("POST", "suggestions/$id/reject") }
    suspend fun acceptAllSuggestions() { request("POST", "suggestions/accept-all") }

    /* ---------- activity ---------- */

    suspend fun activity(limit: Int, cursor: String? = null): ActivityPage = get("activity", null, "limit" to limit.toString(), "cursor" to cursor)

    /** The newest [count] entries, following cursors past the size of one page. */
    suspend fun activityUpTo(count: Int): ActivityPage {
        val all = ArrayList<Activity>()
        var cursor: String? = null
        do {
            val page = activity(minOf(count - all.size, 200), cursor)
            all += page.activities
            cursor = page.nextCursor
        } while (cursor != null && all.size < count)
        return ActivityPage(all, cursor)
    }
    suspend fun undoActivity(id: String) { request("POST", "activity/$id/undo") }
    suspend fun redoActivity(id: String) { request("POST", "activity/$id/redo") }
}

/**
 * The new value of the session cookie carried by one `Set-Cookie` header: `null` when the header
 * is about another cookie, an empty string when the server deletes the session.
 */
fun renewedCookie(header: String): String? {
    val parts = header.split(';').map { it.trim() }
    val pair = parts.firstOrNull() ?: return null
    if (!pair.startsWith("$SESSION_COOKIE=")) return null
    val value = pair.substringAfter('=')
    val expired = parts.drop(1).any { it.equals("Max-Age=0", ignoreCase = true) || it.startsWith("Max-Age=-", ignoreCase = true) }
    return if (expired) "" else value
}
