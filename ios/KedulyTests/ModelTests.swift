import XCTest
@testable import Keduly

/// When pages reload: on a new revision, and on the two things that change without one.
@MainActor
final class ModelTests: XCTestCase {
    /// What the stub server answers, changed between polls.
    private final class Server: @unchecked Sendable {
        private let lock = NSLock()
        private var _focus = ModelTests.focus(state: "work", now: "2026-10-13T02:06:18Z")
        private var _revision = 41
        var focus: String {
            get { lock.withLock { _focus } }
            set { lock.withLock { _focus = newValue } }
        }
        var revision: Int {
            get { lock.withLock { _revision } }
            set { lock.withLock { _revision = newValue } }
        }
    }

    nonisolated private static func focus(state: String, now: String) -> String {
        let session = state == "idle" ? "null" : Fixtures.sessionBody
        return """
        { "focus": { "state": "\(state)", "session": \(session), "tomatoes_today": 3, "minutes_today": 80,
          "round_size": 4, "round_done": 3, "rest_minutes": 5, "now": "\(now)" } }
        """
    }

    private func signedIn(_ server: Server) async -> AppModel {
        StubProtocol.reset()
        StubProtocol.respond { request in
            switch request.url!.path {
            case "/api/v1/bootstrap": return (200, [:], Fixtures.bootstrap.replacingOccurrences(of: "\"revision\": 41", with: "\"revision\": \(server.revision)"))
            case "/api/v1/counts": return (200, [:], Fixtures.counts.replacingOccurrences(of: "\"revision\": 42", with: "\"revision\": \(server.revision)"))
            case "/api/v1/focus": return (200, [:], server.focus)
            default: return (404, [:], "")
            }
        }
        let store = MemoryStore(SessionCredentials(server: URL(string: "https://keduly.example.com")!, cookie: "secret-1"))
        let model = AppModel(api: APIClient(store: store, protocolClasses: [StubProtocol.self]))
        await model.start()
        XCTAssertEqual(model.phase, .signedIn)
        return model
    }

    func testNothingNewDoesNotReloadThePages() async {
        let server = Server()
        let model = await signedIn(server)
        let version = model.dataVersion
        await model.poll()
        XCTAssertEqual(model.dataVersion, version)
    }

    func testANewRevisionReloadsThePages() async {
        let server = Server()
        let model = await signedIn(server)
        let version = model.dataVersion
        server.revision = 42
        await model.poll()
        XCTAssertEqual(model.dataVersion, version + 1)
        await model.poll()
        XCTAssertEqual(model.dataVersion, version + 1)
    }

    func testATomatoEndingByTheClockReloadsThePagesThoughTheRevisionIsTheSame() async {
        let server = Server()
        let model = await signedIn(server)
        let version = model.dataVersion
        // The session ran out: the server says "over" and nothing was written.
        server.focus = Self.focus(state: "over", now: "2026-10-13T02:25:01Z")
        await model.poll()
        XCTAssertEqual(model.focus?.state, .over)
        XCTAssertEqual(model.dataVersion, version + 1)
        // And once more when it is answered elsewhere.
        server.focus = Self.focus(state: "idle", now: "2026-10-13T02:26:00Z")
        await model.poll()
        XCTAssertEqual(model.dataVersion, version + 2)
    }

    func testMidnightInTheAccountZoneReloadsThePagesThoughNothingWasWritten() async {
        let server = Server()
        server.focus = Self.focus(state: "idle", now: "2026-10-13T15:59:30Z") // 23:59:30 on the 13th in Shanghai
        let model = await signedIn(server)
        XCTAssertEqual(model.today, Day("2026-10-13"))
        let version = model.dataVersion
        server.focus = Self.focus(state: "idle", now: "2026-10-13T15:59:50Z")
        await model.poll()
        XCTAssertEqual(model.dataVersion, version)
        // 00:00:10 on the 14th there; still the 13th in UTC and on a device in America.
        server.focus = Self.focus(state: "idle", now: "2026-10-13T16:00:10Z")
        await model.poll()
        XCTAssertEqual(model.today, Day("2026-10-14"))
        XCTAssertEqual(model.dataVersion, version + 1)
        await model.poll()
        XCTAssertEqual(model.dataVersion, version + 1)
    }
}
