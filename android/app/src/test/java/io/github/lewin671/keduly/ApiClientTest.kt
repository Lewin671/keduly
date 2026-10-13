package io.github.lewin671.keduly

import io.github.lewin671.keduly.api.ApiClient
import io.github.lewin671.keduly.api.ApiException
import io.github.lewin671.keduly.api.EventPatch
import io.github.lewin671.keduly.api.ItemPatch
import io.github.lewin671.keduly.api.MemorySessionStore
import io.github.lewin671.keduly.api.MePatch
import io.github.lewin671.keduly.api.Opt
import io.github.lewin671.keduly.api.SessionPatch
import io.github.lewin671.keduly.api.normalizeServer
import io.github.lewin671.keduly.api.renewedCookie
import java.net.SocketTimeoutException
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import okhttp3.Interceptor
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Protocol
import okhttp3.Request
import okhttp3.Response
import okhttp3.ResponseBody.Companion.toResponseBody
import okio.Buffer
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

/** Answers every request itself, so no socket is opened, and remembers what was sent. */
private class FakeServer(private val answer: (Request) -> Reply) : Interceptor {
    class Reply(val status: Int, val body: String, val cookies: List<String> = emptyList())

    val requests = ArrayList<Request>()
    val bodies = ArrayList<String>()

    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        requests += request
        bodies += request.body?.let { body -> Buffer().also { body.writeTo(it) }.readUtf8() }.orEmpty()
        val reply = answer(request)
        val builder = Response.Builder().request(request).protocol(Protocol.HTTP_1_1).code(reply.status).message("")
            .body(reply.body.toResponseBody("application/json".toMediaType()))
        reply.cookies.forEach { builder.addHeader("Set-Cookie", it) }
        return builder.build()
    }
}

class ApiClientTest {
    private val store = MemorySessionStore("http://127.0.0.1:8080", "secret-session")
    private var signedOut = 0

    private fun client(server: FakeServer) = ApiClient(OkHttpClient.Builder().addInterceptor(server).build(), store) { signedOut++ }
    private fun ok(fixture: String) = FakeServer { FakeServer.Reply(200, Fixtures.text(fixture)) }

    @Test fun deviceAuthorizationClaimsWithTheSecretAndPersistsTheSession() = runTest {
        store.cookie = null
        var approved = false
        val server = FakeServer { request ->
            when {
                request.url.encodedPath.endsWith("/auth/requests") -> FakeServer.Reply(201,
                    """{"request":{"id":"abcdefghijklmnop","expires_at":"2026-10-13T17:00:00Z"},"pin":"4826","secret":"claim-secret"}""")
                !approved -> FakeServer.Reply(200, """{"status":"pending"}""")
                else -> FakeServer.Reply(200, Fixtures.text("user").trimEnd().dropLast(1) + """, "status":"approved"}""",
                    listOf("keduly_session=authorized; Path=/; HttpOnly"))
            }
        }
        val api = client(server)
        val login = api.startLoginRequest()
        assertEquals("4826", login.pin)
        assertEquals("https://keduly.example.com/#approve=abcdefghijklmnop", login.approvalUrl("https://keduly.example.com"))
        assertFalse(login.approvalUrl("https://keduly.example.com").contains(login.secret))
        assertFalse(api.claimLoginRequest(login))
        assertNull(store.cookie)
        assertEquals("1", server.requests.last().header("X-Keduly-Request"))
        assertEquals("""{"secret":"claim-secret"}""", server.bodies.last())
        approved = true
        assertTrue(api.claimLoginRequest(login))
        assertEquals("authorized", store.cookie)
    }

    @Test fun deviceAuthorizationRejectsAnApprovalWithoutACookie() = runTest {
        store.cookie = null
        val api = client(FakeServer { FakeServer.Reply(200, Fixtures.text("user").trimEnd().dropLast(1) + """, "status":"approved"}""") })
        val login = io.github.lewin671.keduly.api.DeviceLoginRequest(
            io.github.lewin671.keduly.api.DeviceLoginRequest.Request("abcdefghijklmnop", "2026-10-13T17:00:00Z"), "4826", "secret")
        try {
            api.claimLoginRequest(login)
            fail("approval without a cookie must not sign in")
        } catch (e: ApiException) { assertEquals("bad_response", e.code) }
    }

    @Test fun everyWriteCarriesTheHeaderAndTheCookie() = runTest {
        val server = FakeServer { request ->
            when {
                request.method == "DELETE" && !request.url.encodedPath.endsWith("/schedule") -> FakeServer.Reply(204, "")
                "/focus/" in request.url.encodedPath -> FakeServer.Reply(200, Fixtures.text("focus"))
                "/events" in request.url.encodedPath -> FakeServer.Reply(200, """{"event":${Fixtures.json("events").getValue("events").let { (it as kotlinx.serialization.json.JsonArray)[0] }}}""")
                "/me" in request.url.encodedPath || "/auth/" in request.url.encodedPath -> FakeServer.Reply(200, Fixtures.text("user"))
                "/suggestions/" in request.url.encodedPath || "/activity/" in request.url.encodedPath -> FakeServer.Reply(200, "{}")
                else -> FakeServer.Reply(200, Fixtures.text("item"))
            }
        }
        val api = client(server)
        api.login("me@example.com", "password")
        api.createItem("写周报", ItemPatch())
        api.updateItem("eeeeeeeeeeeeeeee", ItemPatch(status = Opt.Put("done")))
        api.scheduleItem("eeeeeeeeeeeeeeee", "2026-10-13T06:00:00Z", "2026-10-13T07:00:00Z")
        api.unscheduleItem("eeeeeeeeeeeeeeee")
        api.deleteItem("eeeeeeeeeeeeeeee")
        api.createEvent(EventPatch(title = Opt.Put("设计评审")))
        api.deleteEvent("ffffffffffffffff")
        api.startFocus(null)
        api.stopFocus()
        api.restFocus()
        api.acceptSuggestion("gggggggggggggggg")
        api.rejectSuggestion("gggggggggggggggg")
        api.undoActivity("nnnnnnnnnnnnnnnn")
        api.updateMe(MePatch(focusMinutes = Opt.Put(1)))
        api.logout()
        assertEquals(16, server.requests.size)
        for (request in server.requests) {
            assertTrue(request.method != "GET")
            assertEquals("${request.method} ${request.url}", "1", request.header("X-Keduly-Request"))
            assertEquals("${request.method} ${request.url}", "keduly_session=secret-session", request.header("Cookie"))
        }
        assertEquals("http://127.0.0.1:8080/api/v1/auth/login", server.requests.first().url.toString())
    }

    @Test fun readsCarryTheCookieButNotTheHeader() = runTest {
        val server = ok("today")
        client(server).today()
        val request = server.requests.single()
        assertEquals("GET", request.method)
        assertEquals("keduly_session=secret-session", request.header("Cookie"))
        assertNull(request.header("X-Keduly-Request"))
        assertEquals("/api/v1/today", request.url.encodedPath)
    }

    @Test fun queryParametersAreEncodedAndAbsentOnesLeftOut() = runTest {
        val server = ok("items_page")
        client(server).items(projectId = "none", q = "a b&c", limit = 20)
        val url = server.requests.single().url
        assertEquals("none", url.queryParameter("project_id"))
        assertEquals("a b&c", url.queryParameter("q"))
        assertEquals("20", url.queryParameter("limit"))
        assertEquals(setOf("project_id", "q", "limit"), url.queryParameterNames)
    }

    @Test fun aRenewedCookieReplacesTheStoredOne() = runTest {
        val server = FakeServer { FakeServer.Reply(200, Fixtures.text("counts"), listOf("other=1; Path=/", "keduly_session=renewed; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax")) }
        client(server).counts()
        assertEquals("renewed", store.cookie)
    }

    @Test fun loginStoresTheCookie() = runTest {
        store.cookie = null
        val server = FakeServer { FakeServer.Reply(200, Fixtures.text("user"), listOf("keduly_session=fresh; Path=/; HttpOnly")) }
        val user = client(server).login("me@example.com", "password")
        assertEquals("me@example.com", user.email)
        assertEquals("fresh", store.cookie)
        assertNull(server.requests.single().header("Cookie"))
        assertEquals("1", server.requests.single().header("X-Keduly-Request"))
    }

    @Test fun cookieHeadersAreRead() {
        assertEquals("abc", renewedCookie("keduly_session=abc; Path=/; HttpOnly"))
        assertEquals("", renewedCookie("keduly_session=; Path=/; Max-Age=0"))
        assertNull(renewedCookie("theme=dark; Path=/"))
    }

    @Test fun unauthorizedEndsTheSession() = runTest {
        val api = client(FakeServer { FakeServer.Reply(401, """{"error":{"code":"unauthenticated","message":"sign in"}}""") })
        try {
            api.today()
            fail("expected a failure")
        } catch (e: ApiException) {
            assertEquals(401 to "unauthenticated", e.status to e.code)
        }
        assertEquals(1, signedOut)
    }

    @Test fun aWrongPasswordIsNotAnExpiredSession() = runTest {
        val api = client(FakeServer { FakeServer.Reply(401, """{"error":{"code":"unauthenticated","message":"wrong"}}""") })
        try {
            api.login("me@example.com", "nope")
            fail("expected a failure")
        } catch (e: ApiException) {
            assertEquals(401, e.status)
        }
        assertEquals(0, signedOut)
    }

    @Test fun serverErrorsKeepTheirCodeAndMessage() = runTest {
        val api = client(FakeServer { FakeServer.Reply(400, Fixtures.text("error")) })
        try {
            api.createItem("", ItemPatch())
            fail("expected a failure")
        } catch (e: ApiException) {
            assertEquals(400, e.status)
            assertEquals("invalid_request", e.code)
            assertEquals("title is required", e.message)
        }
    }

    @Test fun aTimeoutIsToldApartFromBeingOffline() = runTest {
        val slow = ApiClient(OkHttpClient.Builder().addInterceptor(Interceptor { throw SocketTimeoutException("timeout") }).build(), store)
        try {
            slow.stopFocus()
            fail("expected a failure")
        } catch (e: ApiException) {
            assertTrue(e.timedOut && e.offline)
        }
        val offline = ApiClient(OkHttpClient.Builder().addInterceptor(Interceptor { throw java.net.ConnectException("refused") }).build(), store)
        try {
            offline.today()
            fail("expected a failure")
        } catch (e: ApiException) {
            assertTrue(e.offline)
            assertFalse(e.timedOut)
        }
    }

    @Test fun aFailedWriteIsSentOnlyOnce() = runTest {
        var calls = 0
        val api = ApiClient(OkHttpClient.Builder().addInterceptor(Interceptor { calls++; throw java.io.IOException("reset") }).build(), store)
        try {
            api.createItem("写周报", ItemPatch())
        } catch (e: ApiException) {
            // expected
        }
        assertEquals(1, calls)
    }

    @Test fun patchLeavesOutWhatIsUnchangedAndSendsNullToClear() {
        assertEquals(JsonObject(emptyMap()), ItemPatch().toJson())
        val patch = ItemPatch(title = Opt.Put("写周报"), dueDate = Opt.Put(null), estimateMinutes = Opt.Put(45), important = Opt.Put(false)).toJson()
        assertEquals(setOf("title", "due_date", "estimate_minutes", "important"), patch.keys)
        assertEquals(JsonNull, patch["due_date"])
        assertEquals(JsonPrimitive(45), patch["estimate_minutes"])
        assertEquals(JsonPrimitive(false), patch["important"])
        assertEquals("""{"title":"写周报","estimate_minutes":45,"due_date":null,"important":false}""", patch.toString())
    }

    @Test fun clearingAndOmittingDifferOnEveryPatch() {
        assertEquals("""{"item_id":null}""", SessionPatch(itemId = Opt.Put(null)).toJson().toString())
        assertEquals("""{"title":"读论文","project_id":null}""", SessionPatch(title = Opt.Put("读论文"), projectId = Opt.Put(null)).toJson().toString())
        assertEquals("""{"project_id":null,"all_day":true,"start_date":"2026-10-11","end_date":"2026-10-12"}""",
            EventPatch(projectId = Opt.Put(null), allDay = Opt.Put(true), startDate = Opt.Put("2026-10-11"), endDate = Opt.Put("2026-10-12")).toJson().toString())
        assertEquals("""{"focus_minutes":1}""", MePatch(focusMinutes = Opt.Put(1)).toJson().toString())
    }

    @Test fun thePatchBodyIsWhatGoesOverTheWire() = runTest {
        val server = ok("item")
        client(server).updateItem("eeeeeeeeeeeeeeee", ItemPatch(plannedDate = Opt.Put(null), evening = Opt.Put(true)))
        assertEquals("PATCH", server.requests.single().method)
        assertEquals("""{"planned_date":null,"evening":true}""", server.bodies.single())
    }

    @Test fun serverAddressesAreNormalised() {
        assertEquals("https://keduly.example.com", normalizeServer(" keduly.example.com/ "))
        assertEquals("http://127.0.0.1:18082", normalizeServer("http://127.0.0.1:18082"))
        assertEquals("https://example.com/keduly", normalizeServer("https://example.com/keduly/"))
        assertNull(normalizeServer("  "))
    }

    @Test fun untouchedEventTimesAreNotSentBack() {
        // 01:30 happens twice in Los Angeles on 1 November 2026; this event is in the second one.
        val la = java.time.ZoneId.of("America/Los_Angeles")
        val stored = "2026-11-01T09:30:00Z"
        val start = java.time.Instant.parse(stored).atZone(la).toLocalDateTime()
        val end = java.time.Instant.parse("2026-11-01T10:30:00Z").atZone(la).toLocalDateTime()
        assertEquals(java.time.LocalDateTime.of(2026, 11, 1, 1, 30), start)
        // The wall-clock time alone reads as the first 01:30, an hour before the stored instant.
        val edited = io.github.lewin671.keduly.api.eventTimes(true, false, start, end, la).toJson()
        assertEquals(JsonPrimitive("2026-11-01T08:30:00Z"), edited["start"])
        // So a save that did not touch the times leaves them out and the instants survive.
        val untouched = io.github.lewin671.keduly.api.eventTimes(false, false, start, end, la).copy(title = Opt.Put("改了标题")).toJson()
        assertEquals("""{"title":"改了标题"}""", untouched.toString())
    }

    @Test fun editedEventTimesAreSentWholeForTimedAndAllDayEvents() {
        val zone = java.time.ZoneId.of("Asia/Shanghai")
        val start = java.time.LocalDateTime.of(2026, 10, 14, 23, 30)
        val end = java.time.LocalDateTime.of(2026, 10, 15, 11, 10)
        assertEquals("""{"all_day":false,"start":"2026-10-14T15:30:00Z","end":"2026-10-15T03:10:00Z"}""",
            io.github.lewin671.keduly.api.eventTimes(true, false, start, end, zone).toJson().toString())
        assertEquals("""{"all_day":true,"start_date":"2026-10-14","end_date":"2026-10-15"}""",
            io.github.lewin671.keduly.api.eventTimes(true, true, start, end, zone).toJson().toString())
    }

    @Test fun activityFollowsCursorsPastOnePage() = runTest {
        fun entries(from: Int, count: Int) = (from until from + count).joinToString(",") {
            """{"id":"a$it","actor":{"kind":"user","name":"Me"},"action":"item.create","summary":"s$it","reason":null,"undoable":true,"undone":false,"created_at":"2026-10-13T00:12:00Z"}"""
        }
        val server = FakeServer { request ->
            when (request.url.queryParameter("cursor")) {
                null -> FakeServer.Reply(200, """{"activities":[${entries(0, 200)}],"next_cursor":"c1"}""")
                "c1" -> FakeServer.Reply(200, """{"activities":[${entries(200, 40)}],"next_cursor":"c2"}""")
                else -> FakeServer.Reply(200, """{"activities":[],"next_cursor":null}""")
            }
        }
        val page = client(server).activityUpTo(240)
        assertEquals(240, page.activities.size)
        assertEquals("a239", page.activities.last().id)
        // More remain on the server, so the page still offers older entries.
        assertEquals("c2", page.nextCursor)
        assertEquals(listOf("200", "40"), server.requests.map { it.url.queryParameter("limit") })
        assertEquals(listOf(null, "c1"), server.requests.map { it.url.queryParameter("cursor") })
    }

    @Test fun aLate401DoesNotEndTheSessionThatReplacedIt() = runTest {
        // The answer to a request of the old session arrives after the user signed in again.
        val server = FakeServer {
            store.cookie = "new-session"
            FakeServer.Reply(401, """{"error":{"code":"unauthenticated","message":"sign in"}}""")
        }
        try {
            client(server).today()
            fail("expected a failure")
        } catch (e: ApiException) {
            assertEquals(401, e.status)
        }
        assertEquals("keduly_session=secret-session", server.requests.single().header("Cookie"))
        assertEquals(0, signedOut)
        assertEquals("new-session", store.cookie)
    }

    @Test fun anExpiredSessionsCookieIsDroppedWithTheAnswer() = runTest {
        var seen: String? = "unset"
        val server = FakeServer { FakeServer.Reply(401, """{"error":{"code":"unauthenticated","message":"sign in"}}""") }
        val api = ApiClient(OkHttpClient.Builder().addInterceptor(server).build(), store) { seen = store.cookie }
        try { api.today(); fail("expected a failure") } catch (e: ApiException) { assertEquals(401, e.status) }
        // Gone before anyone is told, so no other late answer of that session still counts as current.
        assertEquals(null, seen)
    }

    @Test fun aLateSetCookieDoesNotReplaceTheNewSessionsCookie() = runTest {
        val server = FakeServer {
            store.cookie = "new-session"
            FakeServer.Reply(200, Fixtures.text("counts"), listOf("keduly_session=renewed-old; Path=/; HttpOnly"))
        }
        client(server).counts()
        assertEquals("new-session", store.cookie)
        // Nor does a late answer that deletes the cookie sign the new session out.
        store.cookie = "secret-session"
        val deleting = FakeServer {
            store.cookie = "newer-session"
            FakeServer.Reply(200, Fixtures.text("counts"), listOf("keduly_session=; Path=/; Max-Age=0"))
        }
        client(deleting).counts()
        assertEquals("newer-session", store.cookie)
    }

    @Test fun anAnswerFromAnotherServerAddressIsIgnoredToo() = runTest {
        val server = FakeServer {
            store.server = "http://127.0.0.1:9090"
            FakeServer.Reply(401, """{"error":{"code":"unauthenticated","message":"sign in"}}""", listOf("keduly_session=other; Path=/"))
        }
        try {
            client(server).today()
        } catch (e: ApiException) {
            // expected
        }
        assertEquals(0, signedOut)
        assertEquals("secret-session", store.cookie)
    }

    @Test fun aSignedOutSessionStaysSignedOutWhenALateAnswerRenewsItsCookie() = runTest {
        val server = FakeServer {
            store.cookie = null
            FakeServer.Reply(200, Fixtures.text("counts"), listOf("keduly_session=renewed-old; Path=/; HttpOnly"))
        }
        client(server).counts()
        assertNull(store.cookie)
    }
}
