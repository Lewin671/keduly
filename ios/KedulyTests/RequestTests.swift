import XCTest
@testable import Keduly

/// What goes out on the wire: the header on every request that is not a GET, the cookie on all of
/// them, a renewed cookie taken over, and a 401 ending the session.
final class RequestTests: XCTestCase {
    private let server = URL(string: "https://keduly.example.com")!

    override func setUp() {
        StubProtocol.reset()
    }

    private func client(_ store: MemoryStore = MemoryStore(SessionCredentials(server: URL(string: "https://keduly.example.com")!, cookie: "secret-1"))) async -> APIClient {
        let api = APIClient(store: store, protocolClasses: [StubProtocol.self])
        if await api.restore() == nil { await api.use(server: server) }
        return api
    }

    func testEveryNonGETRequestCarriesTheHeaderAndTheCookie() async throws {
        let api = await client()
        StubProtocol.respond { request in
            switch (request.httpMethod, request.url!.path) {
            case ("POST", "/api/v1/items"), ("PATCH", _), ("POST", "/api/v1/items/x/schedule"): return (200, [:], Fixtures.item)
            case ("POST", _): return (200, [:], Fixtures.focus)
            default: return (204, [:], "")
            }
        }
        _ = try await api.createItem(NewItem(title: "买牛奶"))
        _ = try await api.updateItem("x", ItemPatch(status: .set(.done)))
        _ = try await api.scheduleItem("x", start: Date(), end: Date())
        _ = try await api.startFocus(itemId: nil)
        _ = try await api.stopFocus()
        try await api.deleteItem("x")
        try await api.deleteEvent("y")
        try await api.acceptSuggestion("z")
        try await api.logout()

        let requests = StubProtocol.requests
        XCTAssertEqual(requests.count, 9)
        for request in requests {
            XCTAssertNotEqual(request.httpMethod, "GET")
            XCTAssertEqual(request.value(forHTTPHeaderField: "X-Keduly-Request"), "1", "\(request.httpMethod!) \(request.url!.path)")
            XCTAssertEqual(request.value(forHTTPHeaderField: "Cookie"), "keduly_session=secret-1", "\(request.httpMethod!) \(request.url!.path)")
        }
        XCTAssertEqual(requests.map { $0.httpMethod! }, ["POST", "PATCH", "POST", "POST", "POST", "DELETE", "DELETE", "POST", "POST"])
        XCTAssertEqual(requests[0].url?.absoluteString, "https://keduly.example.com/api/v1/items")
    }

    func testLoginCarriesTheHeaderAndKeepsTheCookieItIsGiven() async throws {
        let store = MemoryStore(nil)
        let api = await client(store)
        StubProtocol.respond { _ in
            (200, ["Set-Cookie": "keduly_session=fresh; Path=/; Max-Age=2592000; HttpOnly; Secure; SameSite=Lax"], Fixtures.user)
        }
        let user = try await api.login(email: "me@example.com", password: "pass-word")
        XCTAssertEqual(user.email, "me@example.com")
        let login = try XCTUnwrap(StubProtocol.requests.first)
        XCTAssertEqual(login.httpMethod, "POST")
        XCTAssertEqual(login.value(forHTTPHeaderField: "X-Keduly-Request"), "1")
        XCTAssertNil(login.value(forHTTPHeaderField: "Cookie"))
        XCTAssertEqual(StubProtocol.bodies.first.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: String] }, ["email": "me@example.com", "password": "pass-word"])
        XCTAssertEqual(store.load(), SessionCredentials(server: server, cookie: "fresh"))

        StubProtocol.respond { _ in (200, [:], Fixtures.counts) }
        _ = try await api.counts()
        XCTAssertEqual(StubProtocol.requests.last?.value(forHTTPHeaderField: "Cookie"), "keduly_session=fresh")
    }

    func testAGETCarriesTheCookieButNotTheHeader() async throws {
        let api = await client()
        StubProtocol.respond { _ in (200, [:], Fixtures.today) }
        _ = try await api.today()
        let request = try XCTUnwrap(StubProtocol.requests.first)
        XCTAssertEqual(request.httpMethod, "GET")
        XCTAssertNil(request.value(forHTTPHeaderField: "X-Keduly-Request"))
        XCTAssertEqual(request.value(forHTTPHeaderField: "Cookie"), "keduly_session=secret-1")
        XCTAssertNil(request.httpBody)
    }

    func testDeviceAuthorizationClaimsOnlyWithItsSecretAndKeepsTheSession() async throws {
        let store = MemoryStore(nil)
        let api = await client(store)
        StubProtocol.respond { _ in
            (201, [:], #"{"request":{"id":"abcdefghijklmnop","expires_at":"2026-10-13T17:00:00Z"},"pin":"4826","secret":"claim-secret"}"#)
        }
        let request = try await api.startLoginRequest()
        XCTAssertEqual(request.pin, "4826")
        XCTAssertEqual(request.approvalURL(server: server)?.absoluteString, "https://keduly.example.com/#approve=abcdefghijklmnop")
        XCTAssertFalse(request.approvalURL(server: server)!.absoluteString.contains(request.secret))
        StubProtocol.respond { _ in (200, [:], #"{"status":"pending"}"#) }
        let pending = try await api.claimLoginRequest(request)
        XCTAssertFalse(pending)
        XCTAssertNil(store.load())
        XCTAssertEqual(StubProtocol.requests.last?.url?.path, "/api/v1/auth/requests/abcdefghijklmnop/claim")
        XCTAssertEqual(StubProtocol.requests.last?.value(forHTTPHeaderField: "X-Keduly-Request"), "1")
        XCTAssertEqual(StubProtocol.bodies.last.flatMap { try? JSONSerialization.jsonObject(with: $0) as? [String: String] }, ["secret": "claim-secret"])
        let user = String(Fixtures.user.dropLast()) + #", "status":"approved"}"#
        StubProtocol.respond { _ in (200, ["Set-Cookie": "keduly_session=authorized; Path=/; HttpOnly; Secure"], user) }
        let approved = try await api.claimLoginRequest(request)
        XCTAssertTrue(approved)
        XCTAssertEqual(store.load(), SessionCredentials(server: server, cookie: "authorized"))
    }

    func testDeviceAuthorizationRejectsApprovalWithoutASession() async throws {
        let api = await client(MemoryStore(nil))
        let request = DeviceLoginRequest(request: .init(id: "abcdefghijklmnop", expiresAt: Date()), pin: "4826", secret: "secret")
        let user = String(Fixtures.user.dropLast()) + #", "status":"approved"}"#
        StubProtocol.respond { _ in (200, [:], user) }
        do {
            _ = try await api.claimLoginRequest(request)
            XCTFail("approval without a cookie must not sign in")
        } catch { XCTAssertEqual(error as? APIError, .badResponse) }
        StubProtocol.respond { _ in (404, [:], #"{"error":{"code":"not_found","message":"expired"}}"#) }
        do {
            _ = try await api.claimLoginRequest(request)
            XCTFail("expired request must not sign in")
        } catch let error as APIError { XCTAssertEqual(error.status, 404) }
    }

    func testARenewedCookieOnAnyAnswerReplacesTheOldOne() async throws {
        let store = MemoryStore(SessionCredentials(server: server, cookie: "secret-1"))
        let api = await client(store)
        StubProtocol.respond { _ in (200, ["Set-Cookie": "keduly_session=secret-2; Path=/; Max-Age=2592000; HttpOnly; SameSite=Lax"], Fixtures.counts) }
        _ = try await api.counts()
        XCTAssertEqual(store.load()?.cookie, "secret-2")
        StubProtocol.respond { _ in (200, [:], Fixtures.counts) }
        _ = try await api.counts()
        XCTAssertEqual(StubProtocol.requests.last?.value(forHTTPHeaderField: "Cookie"), "keduly_session=secret-2")
    }

    func testA401ClearsTheSessionAndTellsTheApp() async throws {
        let store = MemoryStore(SessionCredentials(server: server, cookie: "secret-1"))
        let api = await client(store)
        let told = expectation(description: "the app is told")
        await api.setUnauthenticatedHandler { told.fulfill() }
        StubProtocol.respond { _ in (401, [:], #"{ "error": { "code": "unauthenticated", "message": "sign in" } }"#) }
        do {
            _ = try await api.today()
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual(error as? APIError, .unauthenticated)
        }
        await fulfillment(of: [told], timeout: 2)
        XCTAssertNil(store.load())
        let hasSession = await api.hasSession
        XCTAssertFalse(hasSession)
    }

    /// Waits until the stub has seen a request to the path.
    private func sent(_ path: String) async {
        while !StubProtocol.requests.contains(where: { $0.url?.path == path }) { await Task.yield() }
    }

    func testALate401DoesNotEndTheSessionOfANewSignIn() async throws {
        let store = MemoryStore(SessionCredentials(server: server, cookie: "secret-1"))
        let api = await client(store)
        let ended = Counter()
        await api.setUnauthenticatedHandler { ended.increment() }
        // The answer to /today is held back; everything else is answered at once.
        let gate = DispatchSemaphore(value: 0)
        StubProtocol.respond { request in
            switch request.url!.path {
            case "/api/v1/today":
                gate.wait()
                return (401, [:], #"{ "error": { "code": "unauthenticated", "message": "expired" } }"#)
            case "/api/v1/auth/login":
                return (200, ["Set-Cookie": "keduly_session=fresh; Path=/; HttpOnly"], Fixtures.user)
            default:
                return (401, [:], #"{ "error": { "code": "unauthenticated", "message": "expired" } }"#)
            }
        }
        let slow = Task { try await api.today() }
        await sent("/api/v1/today")
        // The first 401 ends the old session, and the user signs in again.
        do { _ = try await api.counts(); XCTFail("expected a failure") } catch { XCTAssertEqual(error as? APIError, .unauthenticated) }
        XCTAssertEqual(ended.value, 1)
        XCTAssertNil(store.load())
        _ = try await api.login(email: "me@example.com", password: "pass-word")
        XCTAssertEqual(store.load()?.cookie, "fresh")
        // Now the 401 of the old session arrives. It fails its own request and nothing else.
        gate.signal()
        do { _ = try await slow.value; XCTFail("expected a failure") } catch { XCTAssertEqual(error as? APIError, .unauthenticated) }
        XCTAssertEqual(store.load(), SessionCredentials(server: server, cookie: "fresh"))
        XCTAssertEqual(ended.value, 1)
        let hasSession = await api.hasSession
        XCTAssertTrue(hasSession)
    }

    func testALateCookieRenewalDoesNotReplaceTheCookieOfANewSignIn() async throws {
        let store = MemoryStore(SessionCredentials(server: server, cookie: "secret-1"))
        let api = await client(store)
        let gate = DispatchSemaphore(value: 0)
        StubProtocol.respond { request in
            switch request.url!.path {
            case "/api/v1/today":
                gate.wait()
                return (200, ["Set-Cookie": "keduly_session=secret-1-renewed; Path=/; HttpOnly"], Fixtures.today)
            case "/api/v1/auth/login":
                return (200, ["Set-Cookie": "keduly_session=fresh; Path=/; HttpOnly"], Fixtures.user)
            default:
                return (200, [:], Fixtures.counts)
            }
        }
        let slow = Task { try await api.today() }
        await sent("/api/v1/today")
        await api.forget()
        _ = try await api.login(email: "me@example.com", password: "pass-word")
        gate.signal()
        _ = try await slow.value
        XCTAssertEqual(store.load()?.cookie, "fresh")
        _ = try await api.counts()
        XCTAssertEqual(StubProtocol.requests.last?.value(forHTTPHeaderField: "Cookie"), "keduly_session=fresh")
    }

    func testAWrongPasswordIsNotALostSession() async throws {
        let api = await client()
        await api.setUnauthenticatedHandler { XCTFail("signing in failed; no session ended") }
        StubProtocol.respond { _ in (401, [:], #"{ "error": { "code": "unauthenticated", "message": "wrong" } }"#) }
        do {
            _ = try await api.login(email: "me@example.com", password: "nope")
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual(error as? APIError, .unauthenticated)
        }
    }

    func testErrorsCarryTheServersCodeAndMessage() async throws {
        let api = await client()
        StubProtocol.respond { _ in (400, [:], Fixtures.failure) }
        do {
            _ = try await api.createItem(NewItem(title: ""))
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual(error as? APIError, .server(status: 400, code: "invalid_request", message: "title is required"))
            XCTAssertEqual((error as? APIError)?.message, "title is required")
        }
        StubProtocol.respond { _ in (409, [:], #"{ "error": { "code": "conflict", "message": "changed" } }"#) }
        do {
            try await api.acceptSuggestion("z")
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual((error as? APIError)?.status, 409)
            XCTAssertEqual((error as? APIError)?.message, "内容已经变了，这个操作无法完成")
        }
        // A write is sent once, whatever the answer.
        XCTAssertEqual(StubProtocol.requests.count, 2)
    }

    func testAFailedWriteIsNotRetried() async throws {
        let api = await client()
        StubProtocol.fail(with: URLError(.timedOut))
        do {
            _ = try await api.createItem(NewItem(title: "买牛奶"))
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual(error as? APIError, .timeout)
        }
        XCTAssertEqual(StubProtocol.requests.count, 1)
        StubProtocol.fail(with: URLError(.notConnectedToInternet))
        do {
            _ = try await api.today()
            XCTFail("expected a failure")
        } catch {
            XCTAssertEqual(error as? APIError, .network)
        }
    }

    func testQueriesAndBodiesUseTheDocumentedNames() async throws {
        let api = await client()
        StubProtocol.respond { _ in (200, [:], Fixtures.itemPage) }
        _ = try await api.items(.init(status: "done", projectId: "none", quadrant: .plan, search: "周报", limit: 20, cursor: "abc"))
        let url = try XCTUnwrap(StubProtocol.requests.first?.url)
        let query = Dictionary(uniqueKeysWithValues: URLComponents(url: url, resolvingAgainstBaseURL: false)!.queryItems!.map { ($0.name, $0.value!) })
        XCTAssertEqual(query, ["status": "done", "project_id": "none", "quadrant": "plan", "q": "周报", "limit": "20", "cursor": "abc"])

        StubProtocol.respond { _ in (200, [:], Fixtures.focus) }
        _ = try await api.startFocus(itemId: nil, title: "回邮件", projectId: "cccccccccccccccc")
        let free = try XCTUnwrap(StubProtocol.bodies.last.flatMap { try JSONSerialization.jsonObject(with: $0) as? [String: String] })
        XCTAssertEqual(free, ["title": "回邮件", "project_id": "cccccccccccccccc"])
        // With an item the server takes nothing else.
        _ = try await api.startFocus(itemId: "eeeeeeeeeeeeeeee", title: "实现 CalDAV 同步", projectId: "cccccccccccccccc")
        let onItem = try XCTUnwrap(StubProtocol.bodies.last.flatMap { try JSONSerialization.jsonObject(with: $0) as? [String: String] })
        XCTAssertEqual(onItem, ["item_id": "eeeeeeeeeeeeeeee"])
    }

    /// A page of `count` numbered items starting at `offset`, out of `total`.
    private static func page(offset: Int, count: Int, total: Int) -> String {
        let items = (offset..<min(offset + count, total)).map { Fixtures.itemBody.replacingOccurrences(of: "eeeeeeeeeeeeeeee", with: "item\($0)") }
        let next = offset + count < total ? "\"c\(offset + count)\"" : "null"
        return "{ \"items\": [\(items.joined(separator: ","))], \"next_cursor\": \(next), \"total\": \(total) }"
    }

    private func servePages(total: Int) {
        StubProtocol.respond { request in
            let query = Dictionary(uniqueKeysWithValues: URLComponents(url: request.url!, resolvingAgainstBaseURL: false)!.queryItems!.map { ($0.name, $0.value!) })
            let offset = query["cursor"].map { Int($0.dropFirst())! } ?? 0
            return (200, [:], Self.page(offset: offset, count: Int(query["limit"]!)!, total: total))
        }
    }

    private func queries() -> [[String: String]] {
        StubProtocol.requests.map { Dictionary(uniqueKeysWithValues: URLComponents(url: $0.url!, resolvingAgainstBaseURL: false)!.queryItems!.map { ($0.name, $0.value!) }) }
    }

    func testLoadingAWholeListFollowsTheCursorPastTheServersLimit() async throws {
        let api = await client()
        servePages(total: 450)
        let page = try await api.items(.init(quadrant: .do), upTo: .max)
        XCTAssertEqual(page.items.count, 450)
        XCTAssertEqual(page.items.map(\.id), (0..<450).map { "item\($0)" })
        XCTAssertEqual(page.total, 450)
        XCTAssertNil(page.nextCursor)
        // Three requests, each for at most 200, each continuing where the last one ended.
        XCTAssertEqual(queries().map { $0["limit"] }, ["200", "200", "200"])
        XCTAssertEqual(queries().map { $0["cursor"] }, [nil, "c200", "c400"])
        XCTAssertTrue(queries().allSatisfy { $0["quadrant"] == "do" })
    }

    func testLoadingPartOfAListStopsAtTheCountAndKeepsTheCursor() async throws {
        let api = await client()
        servePages(total: 450)
        let page = try await api.items(.init(projectId: "p"), upTo: 230)
        XCTAssertEqual(page.items.count, 230)
        XCTAssertEqual(page.items.last?.id, "item229")
        XCTAssertEqual(page.nextCursor, "c230")
        XCTAssertEqual(queries().map { $0["limit"] }, ["200", "30"])
    }

    func testLoadingAShortListTakesOneRequest() async throws {
        let api = await client()
        servePages(total: 3)
        let page = try await api.items(.init(), upTo: 20)
        XCTAssertEqual(page.items.count, 3)
        XCTAssertNil(page.nextCursor)
        XCTAssertEqual(queries().map { $0["limit"] }, ["20"])
    }

    func testServerAddress() throws {
        XCTAssertEqual(try ServerAddress.parse(" keduly.example.com/ ", allowsHTTP: false).absoluteString, "https://keduly.example.com")
        XCTAssertEqual(try ServerAddress.parse("https://keduly.example.com:8443", allowsHTTP: false).absoluteString, "https://keduly.example.com:8443")
        XCTAssertEqual(try ServerAddress.parse("http://127.0.0.1:8080", allowsHTTP: true).absoluteString, "http://127.0.0.1:8080")
        XCTAssertThrowsError(try ServerAddress.parse("http://keduly.example.com", allowsHTTP: false)) { XCTAssertEqual($0 as? ServerAddress.Problem, .insecure) }
        XCTAssertThrowsError(try ServerAddress.parse("ftp://keduly.example.com", allowsHTTP: true)) { XCTAssertEqual($0 as? ServerAddress.Problem, .invalid) }
        XCTAssertThrowsError(try ServerAddress.parse("   ", allowsHTTP: true))
    }
}

/// Counts calls from any thread.
final class Counter: @unchecked Sendable {
    private let lock = NSLock()
    private var count = 0
    var value: Int { lock.withLock { count } }
    func increment() { lock.withLock { count += 1 } }
}

/// A session store that lives in memory.
final class MemoryStore: SessionStore, @unchecked Sendable {
    private let lock = NSLock()
    private var credentials: SessionCredentials?

    init(_ credentials: SessionCredentials?) {
        self.credentials = credentials
    }

    func load() -> SessionCredentials? { lock.withLock { credentials } }
    func save(_ credentials: SessionCredentials) { lock.withLock { self.credentials = credentials } }
    func clear() { lock.withLock { credentials = nil } }
}

/// Answers requests from a closure and records what was sent.
final class StubProtocol: URLProtocol, @unchecked Sendable {
    typealias Answer = (status: Int, headers: [String: String], body: String)

    private static let lock = NSLock()
    nonisolated(unsafe) private static var handler: (@Sendable (URLRequest) -> Answer)?
    nonisolated(unsafe) private static var failure: URLError?
    nonisolated(unsafe) private static var recorded: [(URLRequest, Data?)] = []

    static var requests: [URLRequest] { lock.withLock { recorded.map(\.0) } }
    static var bodies: [Data] { lock.withLock { recorded.compactMap(\.1) } }

    static func reset() {
        lock.withLock {
            handler = nil
            failure = nil
            recorded = []
        }
    }

    static func respond(_ handler: @escaping @Sendable (URLRequest) -> Answer) {
        lock.withLock {
            self.handler = handler
            failure = nil
        }
    }

    static func fail(with error: URLError) {
        lock.withLock { failure = error }
    }

    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }

    override func startLoading() {
        // URLSession hands the body over as a stream.
        var body = request.httpBody
        if body == nil, let stream = request.httpBodyStream {
            stream.open()
            var data = Data()
            var buffer = [UInt8](repeating: 0, count: 4096)
            while stream.hasBytesAvailable {
                let read = stream.read(&buffer, maxLength: buffer.count)
                if read <= 0 { break }
                data.append(buffer, count: read)
            }
            stream.close()
            body = data
        }
        let (handler, failure) = Self.lock.withLock {
            Self.recorded.append((request, body))
            return (Self.handler, Self.failure)
        }
        if let failure {
            client?.urlProtocol(self, didFailWithError: failure)
            return
        }
        // Off the loading thread, so that a handler may hold one answer back while others are served.
        let request = request
        nonisolated(unsafe) let this = self
        DispatchQueue.global().async {
            let answer: Answer = handler?(request) ?? (500, [:], "")
            let response = HTTPURLResponse(url: request.url!, statusCode: answer.status, httpVersion: "HTTP/1.1", headerFields: answer.headers)!
            this.client?.urlProtocol(this, didReceive: response, cacheStoragePolicy: .notAllowed)
            this.client?.urlProtocol(this, didLoad: Data(answer.body.utf8))
            this.client?.urlProtocolDidFinishLoading(this)
        }
    }

    override func stopLoading() {}
}
