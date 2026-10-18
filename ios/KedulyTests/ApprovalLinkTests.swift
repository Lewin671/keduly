import XCTest
@testable import Keduly

/// Only the request id of a pasted approval link is taken, and only for this server.
final class ApprovalLinkTests: XCTestCase {
    private let server = URL(string: "https://keduly.example.com")!
    private let id = "cuzdsntqa4txq2ct"

    func testTakesTheIdOfALinkForThisServer() {
        XCTAssertEqual(ApprovalLink.parse("https://keduly.example.com/#approve=\(id)", server: server), .request(id))
        XCTAssertEqual(ApprovalLink.parse("  请打开 https://Keduly.Example.com:443/#approve=\(id) 批准\n", server: server), .request(id))
        XCTAssertEqual(ApprovalLink.parse(id, server: server), .request(id))
    }

    func testRefusesAnotherServer() {
        XCTAssertEqual(ApprovalLink.parse("https://keduly.example.org/#approve=\(id)", server: server), .otherServer("keduly.example.org"))
        XCTAssertEqual(ApprovalLink.parse("http://keduly.example.com/#approve=\(id)", server: server), .otherServer("keduly.example.com"))
        XCTAssertEqual(ApprovalLink.parse("https://keduly.example.com:8443/#approve=\(id)", server: server), .otherServer("keduly.example.com"))
    }

    func testRefusesAnythingElse() {
        for text in ["", "hello", "https://keduly.example.com/", "https://keduly.example.com/#approve=short", "https://keduly.example.com/#approve=UPPERCASEUPPERCAS"] {
            XCTAssertEqual(ApprovalLink.parse(text, server: server), .notALink, text)
        }
    }

    func testDecodesAPendingRequest() throws {
        let json = #"{"request":{"id":"cuzdsntqa4txq2ct","device":"Keduly · Android","expires_at":"2026-10-18T16:24:30Z"}}"#
        let request: PendingLogin = try APIClient.decodeBody(Data(json.utf8), key: "request")
        XCTAssertEqual(request.id, id)
        XCTAssertEqual(request.device, "Keduly · Android")
    }
}
