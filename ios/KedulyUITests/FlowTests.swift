import XCTest

/// Drives the app through every screen and action of the first version against a real server.
///
/// Start a development server with demo data (see scripts/seed-demo.py), then:
///
///     TEST_RUNNER_KEDULY_UITEST_SERVER=http://127.0.0.1:8080 xcodebuild test -collect-test-diagnostics never \
///       -project Keduly.xcodeproj -scheme KedulyUITests -destination 'platform=iOS Simulator,name=…'
///
/// `TEST_RUNNER_KEDULY_UITEST_SHOTS=<directory>` also saves a screenshot of each step. The tests run in
/// order and change the demo account (they set a tomato to one minute), so use a fresh data directory.
/// Without the server variable every test is skipped.
final class FlowTests: XCTestCase {
    private var app: XCUIApplication!
    private var server = ""

    private static let email = "demo@example.com"
    private static let password = "demo-password"

    override func setUpWithError() throws {
        continueAfterFailure = false
        server = try XCTUnwrap(ProcessInfo.processInfo.environment["KEDULY_UITEST_SERVER"].flatMap { $0.isEmpty ? nil : $0 }.orSkip())
        app = XCUIApplication()
    }

    // MARK: - Helpers

    /// Starts the app already signed in, on a given screen, through the debug launch options.
    private func launch(_ screen: String, email: String = FlowTests.email) {
        app.launchEnvironment = ["KEDULY_SERVER": server, "KEDULY_EMAIL": email, "KEDULY_PASSWORD": Self.password, "KEDULY_SCREEN": screen]
        app.launch()
    }

    private func element(_ id: String) -> XCUIElement {
        app.descendants(matching: .any).matching(identifier: id).firstMatch
    }

    @discardableResult
    private func wait(_ element: XCUIElement, _ seconds: TimeInterval = 8, file: StaticString = #filePath, line: UInt = #line) -> XCUIElement {
        XCTAssertTrue(element.waitForExistence(timeout: seconds), "missing: \(element)", file: file, line: line)
        return element
    }

    private func tap(_ id: String, file: StaticString = #filePath, line: UInt = #line) {
        wait(element(id), file: file, line: line).tap()
    }

    private func text(_ string: String) -> XCUIElement {
        app.descendants(matching: .any).matching(NSPredicate(format: "label CONTAINS %@", string)).firstMatch
    }

    private func gone(_ element: XCUIElement, _ seconds: TimeInterval = 8, file: StaticString = #filePath, line: UInt = #line) {
        let predicate = NSPredicate(format: "exists == false")
        let expectation = XCTNSPredicateExpectation(predicate: predicate, object: element)
        XCTAssertEqual(XCTWaiter.wait(for: [expectation], timeout: seconds), .completed, "still there: \(element)", file: file, line: line)
    }

    private func shot(_ name: String) {
        guard let directory = ProcessInfo.processInfo.environment["KEDULY_UITEST_SHOTS"], !directory.isEmpty else { return }
        // Let animations settle.
        Thread.sleep(forTimeInterval: 0.8)
        let url = URL(fileURLWithPath: directory).appendingPathComponent("\(name).png")
        try? XCUIScreen.main.screenshot().pngRepresentation.write(to: url)
    }

    /// Calls the API as the demo user, for what the app itself cannot do: only an agent's token can make a suggestion.
    private func api(_ method: String, _ path: String, _ body: [String: Any] = [:], cookie: String? = nil, bearer: String? = nil) throws -> (json: [String: Any], cookie: String?) {
        var request = URLRequest(url: URL(string: server + "/api/v1" + path)!)
        request.httpMethod = method
        request.setValue("1", forHTTPHeaderField: "X-Keduly-Request")
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        if let cookie { request.setValue(cookie, forHTTPHeaderField: "Cookie") }
        if let bearer { request.setValue("Bearer \(bearer)", forHTTPHeaderField: "Authorization") }
        if method != "GET" { request.httpBody = try JSONSerialization.data(withJSONObject: body) }
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpShouldSetCookies = false
        let done = expectation(description: path)
        nonisolated(unsafe) var result: (Data?, URLResponse?) = (nil, nil)
        URLSession(configuration: configuration).dataTask(with: request) { data, response, _ in
            result = (data, response)
            done.fulfill()
        }.resume()
        wait(for: [done], timeout: 10)
        let http = try XCTUnwrap(result.1 as? HTTPURLResponse)
        XCTAssertLessThan(http.statusCode, 300, "\(method) \(path)")
        let json = (try? JSONSerialization.jsonObject(with: result.0 ?? Data())) as? [String: Any] ?? [:]
        let set = http.value(forHTTPHeaderField: "Set-Cookie")?.split(separator: ";").first.map(String.init)
        return (json, set)
    }

    // MARK: - Flows

    func test01SignIn() throws {
        app.launchEnvironment = ["KEDULY_SERVER": server, "KEDULY_EMAIL": Self.email, "KEDULY_PASSWORD": "wrong-password"]
        app.launch()
        // The debug sign-in failed on purpose, which leaves the sign-in screen.
        let serverField = wait(element("signin-server"))
        shot("00-signin")
        serverField.tap()
        serverField.typeText(server)
        element("signin-email").tap()
        element("signin-email").typeText(Self.email)
        element("signin-password").tap()
        element("signin-password").typeText("not-the-password")
        tap("signin-submit")
        wait(element("signin-error"))
        shot("00-signin-wrong")
        let password = element("signin-password")
        password.tap()
        password.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: 20) + Self.password + "\n")
        // The app opens on Matrix, pushed on the list of lists.
        wait(text("四象限"))
        wait(text("马上做"))
        shot("02-matrix")
    }

    func test02Lists() throws {
        launch("lists")
        wait(element("list-四象限"))
        shot("01-lists")
        for (list, heading, name) in [("今天", "逾期", "03-today"), ("计划", "明天", "13-upcoming"), ("全部", "团队事务", "14-all"), ("收件箱", "给自行车打气", "15-inbox"), ("已完成", "画出三个栏的草图", "16-done")] {
            tap("list-\(list)")
            wait(text(heading))
            shot(name)
            app.navigationBars.buttons.firstMatch.tap()
            wait(element("list-四象限"))
        }
        // Matrix: the first quadrant holds more than six.
        tap("list-四象限")
        let more = wait(app.buttons.containing(NSPredicate(format: "label BEGINSWITH %@", "显示其余")).firstMatch)
        more.tap()
        gone(more)
        app.navigationBars.buttons.firstMatch.tap()
        // A project, and search.
        tap("project-团队事务")
        wait(text("核对上个月的报销"))
        app.navigationBars.buttons.firstMatch.tap()
        let search = wait(app.textFields["search-field"])
        search.tap()
        search.typeText("自行车")
        wait(element("item-给自行车打气"))
        shot("17-search")
    }

    func test03ItemLifecycle() throws {
        let title = "界面测试事项"
        launch("today")
        wait(text("逾期"))
        // Create: started from Today, it is planned for today.
        tap("add")
        wait(element("new-item-title")).typeText(title)
        shot("05-new-item")
        tap("new-item-add")
        let row = wait(element("item-\(title)"))
        // Edit: estimate, deadline, important, project.
        row.tap()
        tap("row-estimate")
        tap("estimate-30")
        tap("row-due")
        tap("due-明天")
        wait(element("item-important")).coordinate(withNormalizedOffset: CGVector(dx: 0.8, dy: 0.5)).tap()
        tap("row-project")
        tap("pick-project-生活")
        wait(text("生活"))
        wait(text("30 分钟"))
        shot("05-item-edited")
        // Schedule: a time block on the calendar.
        tap("row-when")
        tap("when-schedule")
        let when = wait(element("row-when"))
        XCTAssertTrue(wait(when.staticTexts.containing(NSPredicate(format: "label CONTAINS %@", ":")).firstMatch).exists)
        shot("05-item-scheduled")
        tap("item-done")
        wait(text("明天截止"))
        // Complete, and take it back: the row stays in place.
        let check = app.buttons["完成 \(title)"]
        wait(check).tap()
        XCTAssertTrue(wait(element("item-\(title)")).exists)
        shot("03-today-item-done")
        check.tap()
        // Delete.
        element("item-\(title)").tap()
        tap("item-more")
        wait(app.buttons["删除"]).tap()
        wait(app.buttons["删除"]).tap()
        gone(element("item-\(title)"))
    }

    func test04EventLifecycle() throws {
        launch("month")
        wait(element("calendar-mode"))
        shot("06-month")
        tap("add")
        wait(element("event-title")).tap()
        element("event-title").typeText("界面测试日程")
        shot("06-new-event")
        tap("event-save")
        let row = wait(element("agenda-界面测试日程"))
        row.tap()
        let field = wait(element("event-title"))
        field.tap()
        field.typeText("改")
        tap("event-save")
        let renamed = wait(app.buttons.containing(NSPredicate(format: "identifier BEGINSWITH %@ AND identifier CONTAINS %@", "agenda-", "改")).firstMatch)
        renamed.tap()
        tap("event-delete")
        wait(app.buttons["删除"]).tap()
        gone(renamed)
        // The day view, with its overlaps folded into "+N".
        app.segmentedControls.buttons["日"].tap()
        wait(element("block-more"))
        shot("07-day")
        tap("block-more")
        wait(text("供应商电话"))
        shot("07-day-more")
    }

    func test05Suggestions() throws {
        // A second suggestion, so that one can be accepted and one rejected.
        let login = try api("POST", "/auth/login", ["email": Self.email, "password": Self.password])
        let token = try api("POST", "/tokens", ["name": "UI test", "kind": "agent"], cookie: login.cookie)
        let secret = try XCTUnwrap((token.json["token"] as? [String: Any])?["token"] as? String)
        let tomorrow = Date().addingTimeInterval(86_400)
        let format = ISO8601DateFormatter()
        _ = try api("POST", "/suggestions", ["kind": "create_event", "reason": "界面测试的建议", "event": ["title": "界面测试建议", "start": format.string(from: tomorrow), "end": format.string(from: tomorrow.addingTimeInterval(3600))]], bearer: secret)

        launch("item")
        // The item sheet shows the suggestion with its reason; accept it there.
        wait(element("suggestion-accept"))
        shot("05-item")
        tap("suggestion-accept")
        gone(element("suggestion-accept"))
        tap("item-done")
        // The other one waits in activity; reject it.
        app.navigationBars.buttons.firstMatch.tap()
        tap("bell")
        wait(text("界面测试建议"))
        shot("11-activity")
        tap("suggestion-reject")
        gone(text("界面测试的建议"))
        // Undo the newest change, which is accepting the first suggestion.
        let undo = wait(app.buttons.containing(NSPredicate(format: "identifier BEGINSWITH %@", "undo-")).firstMatch)
        undo.tap()
        wait(text("已撤销"))
        shot("11-activity-undone")
    }

    func test06SettingsAndFocus() throws {
        launch("settings")
        // A tomato and a rest of one minute each, so that a whole cycle fits in the test.
        for (id, start) in [("focus-minutes", 25), ("rest-minutes", 5)] {
            let stepper = wait(element(id))
            var value = start
            while value > 1 {
                stepper.buttons.element(boundBy: 0).tap()
                value -= 1
                wait(text("\(value) 分钟"), 5)
            }
        }
        let name = wait(element("settings-name"))
        name.tap()
        name.typeText("X\n")
        shot("12-settings")

        // Idle: pick free focus and start.
        tap("tab-专注")
        wait(element("focus-start"))
        shot("08-focus")
        tap("focus-pick")
        shot("08-focus-picker")
        tap("pick-free")
        wait(text("自由专注"))
        tap("focus-start")
        wait(element("timer-giveup"))
        shot("09-timer")
        // Away from the timer a strip above the tab bar leads back to it.
        tap("timer-collapse")
        wait(element("timer-strip"))
        tap("tab-日历")
        wait(element("timer-strip"))
        shot("07-day-strip")
        tap("timer-strip")
        // The server decides when the tomato is earned; the page then asks what next.
        wait(element("timer-rest"), 90)
        wait(text("完成第"))
        shot("09-timer-over")
        // File the session: say what it was.
        tap("timer-file")
        let what = wait(element("filing-title"))
        what.tap()
        what.typeText("界面测试专注\n")
        shot("09-filing")
        tap("filing-done")
        wait(text("界面测试专注"))
        // Rest, then skip it.
        tap("timer-rest")
        wait(element("timer-skip"))
        shot("09-timer-rest")
        tap("timer-skip")
        // 收起 led back to the calendar, where the timer was opened from the strip.
        tap("tab-专注")
        wait(element("focus-start"))
        wait(text("个番茄 ·"))
        // A tomato on an item, given up.
        tap("focus-pick")
        wait(app.buttons.containing(NSPredicate(format: "identifier BEGINSWITH %@ AND NOT identifier == %@", "pick-", "pick-free")).firstMatch).tap()
        tap("focus-start")
        tap("timer-giveup")
        wait(element("focus-start"))
        // Statistics, and filing from the records.
        tap("stats")
        wait(text("时间花在哪"))
        shot("10-stats")
        tap("record-0")
        wait(element("filing-project")).tap()
        wait(app.buttons["生活"]).tap()
        tap("filing-done")
        wait(text("生活"))
        shot("10-stats-filed")
    }

    func test07SignOutAndEmptyAccount() throws {
        launch("settings")
        app.swipeUp()
        tap("sign-out")
        // The row and the confirmation carry the same words; the confirmation is the later one.
        let confirm = app.buttons.matching(NSPredicate(format: "label == %@", "退出登录"))
        wait(confirm.element(boundBy: 1)).tap()
        wait(element("signin-server"))

        launch("matrix", email: "empty@example.com")
        wait(text("还没有事项"))
        shot("20-empty-matrix")
        app.navigationBars.buttons.firstMatch.tap()
        wait(element("new-project"))
        shot("20-empty-lists")
        tap("list-今天")
        wait(text("今天没有安排"))
        shot("20-empty-today")
        tap("tab-日历")
        wait(text("没有日程"))
        shot("20-empty-month")
        tap("tab-专注")
        wait(text("自由专注"))
        shot("20-empty-focus")
        tap("stats")
        wait(text("还没有专注记录"))
        shot("20-empty-stats")
        // A first project.
        tap("tab-事项")
        tap("tab-事项")
        tap("new-project")
        wait(element("project-name")).tap()
        element("project-name").typeText("界面测试项目")
        tap("project-save")
        tap("project-界面测试项目")
        wait(text("这个项目还没有事项"))
        shot("20-empty-project")
        tap("project-menu")
        wait(app.buttons["编辑项目"]).tap()
        let field = wait(element("project-name"))
        field.tap()
        field.typeText("二")
        tap("project-save")
        wait(text("界面测试项目二"))
    }
}

private extension Optional {
    /// Unwraps, or skips the test.
    func orSkip() throws -> Wrapped {
        guard let self else { throw XCTSkip("set TEST_RUNNER_KEDULY_UITEST_SERVER to run the flows") }
        return self
    }
}
