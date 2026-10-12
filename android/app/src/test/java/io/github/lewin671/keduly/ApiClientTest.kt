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
}
