import Foundation

/// The one way to the server: `/api/v1` with the session cookie.
///
/// Cookies are handled here, not by `URLSession`, so that the session lives only in the secure
/// store: the cookie is sent on every request, a renewed one on any answer replaces it, and a
/// `401` drops it. Every request that is not a `GET` carries `X-Keduly-Request: 1`.
actor APIClient {
    static let cookieName = "keduly_session"

    private let session: URLSession
    private let store: any SessionStore
    private var server: URL?
    private var cookie: String?
    private var onUnauthenticated: (@Sendable () async -> Void)?

    init(store: any SessionStore, protocolClasses: [AnyClass]? = nil) {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        configuration.httpShouldSetCookies = false
        configuration.httpCookieAcceptPolicy = .never
        configuration.urlCache = nil
        configuration.requestCachePolicy = .reloadIgnoringLocalCacheData
        configuration.timeoutIntervalForRequest = 15
        if let protocolClasses { configuration.protocolClasses = protocolClasses }
        self.session = URLSession(configuration: configuration)
        self.store = store
    }

    /// Picks up the session kept from the last launch. Returns its server, if there is one.
    func restore() -> URL? {
        guard let saved = store.load() else { return nil }
        server = saved.server
        cookie = saved.cookie
        return saved.server
    }

    /// Points the client at a server before signing in. Any session for another server is dropped.
    func use(server: URL) {
        if server != self.server { forget() }
        self.server = server
    }

    func setUnauthenticatedHandler(_ handler: @escaping @Sendable () async -> Void) {
        onUnauthenticated = handler
    }

    /// Drops the session from memory and from the secure store.
    func forget() {
        cookie = nil
        store.clear()
    }

    var currentServer: URL? { server }
    var hasSession: Bool { cookie != nil }

    // MARK: - Requests

    func get<T: Decodable & Sendable>(_ path: String, _ query: [String: String] = [:], key: String? = nil) async throws -> T {
        try decode(await perform("GET", path, query: query), key: key)
    }

    func send<T: Decodable & Sendable>(_ method: String, _ path: String, body: (any Encodable & Sendable)? = nil, key: String? = nil) async throws -> T {
        try decode(await perform(method, path, body: body.map(Self.encode)), key: key)
    }

    func sendDiscarding(_ method: String, _ path: String, body: (any Encodable & Sendable)? = nil) async throws {
        _ = try await perform(method, path, body: body.map(Self.encode))
    }

    /// `expectsSession` is false for signing in, where a `401` means a wrong password, not a lost session.
    func perform(_ method: String, _ path: String, query: [String: String] = [:], body: Data? = nil, expectsSession: Bool = true) async throws -> Data {
        guard let server else { throw APIError.unauthenticated }
        var components = URLComponents(url: server, resolvingAgainstBaseURL: false)!
        components.path = (components.path.hasSuffix("/") ? String(components.path.dropLast()) : components.path) + "/api/v1" + path
        if !query.isEmpty {
            components.queryItems = query.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        guard let url = components.url else { throw APIError.badResponse }
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        request.setValue("Keduly/iPhone", forHTTPHeaderField: "User-Agent")
        if method != "GET" {
            request.setValue("1", forHTTPHeaderField: "X-Keduly-Request")
            if method != "DELETE" || body != nil {
                request.setValue("application/json", forHTTPHeaderField: "Content-Type")
                request.httpBody = body ?? Data("{}".utf8)
            }
        }
        // What the answer may do to the session (renew its cookie, end it) only holds for the session
        // the request was sent with. By the time a slow answer arrives the user may have signed in again.
        let sentWith = cookie
        if let cookie {
            request.setValue("\(Self.cookieName)=\(cookie)", forHTTPHeaderField: "Cookie")
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch let error as URLError where error.code == .cancelled {
            throw CancellationError()
        } catch let error as URLError where error.code == .timedOut {
            throw APIError.timeout
        } catch is CancellationError {
            throw CancellationError()
        } catch {
            throw APIError.network
        }
        try Task.checkCancellation()
        guard let http = response as? HTTPURLResponse else { throw APIError.badResponse }
        let current = cookie == sentWith && self.server == server
        if current { adoptCookie(from: http, server: server) }

        switch http.statusCode {
        case 200..<300:
            return data
        case 401:
            if expectsSession && current {
                forget()
                await onUnauthenticated?()
            }
            throw APIError.unauthenticated
        default:
            let failure = try? JSONDecoder().decode(Failure.self, from: data)
            throw APIError.server(status: http.statusCode, code: failure?.error.code ?? "", message: failure?.error.message ?? "")
        }
    }

    /// The server sends the cookie again whenever it pushes the session's expiry out.
    private func adoptCookie(from response: HTTPURLResponse, server: URL) {
        guard let header = response.value(forHTTPHeaderField: "Set-Cookie"), let url = response.url else { return }
        let cookies = HTTPCookie.cookies(withResponseHeaderFields: ["Set-Cookie": header], for: url)
        guard let value = cookies.first(where: { $0.name == Self.cookieName })?.value, !value.isEmpty, value != cookie else { return }
        cookie = value
        store.save(SessionCredentials(server: server, cookie: value))
    }

    private struct Failure: Decodable {
        struct Body: Decodable {
            let code: String
            let message: String
        }
        let error: Body
    }

    // MARK: - JSON

    static func makeEncoder() -> JSONEncoder {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        encoder.dateEncodingStrategy = .custom { date, encoder in
            var container = encoder.singleValueContainer()
            try container.encode(date.formatted(.iso8601))
        }
        return encoder
    }

    static func makeDecoder() -> JSONDecoder {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let string = try container.decode(String.self)
            if let date = try? Date(string, strategy: .iso8601) { return date }
            if let date = try? Date(string, strategy: Date.ISO8601FormatStyle(includingFractionalSeconds: true)) { return date }
            throw DecodingError.dataCorruptedError(in: container, debugDescription: "not a timestamp: \(string)")
        }
        return decoder
    }

    static func encode(_ body: any Encodable & Sendable) throws -> Data {
        try makeEncoder().encode(body)
    }

    /// Decodes the whole body, or with `key` the object under that name, e.g. `{ "item": { … } }`.
    static func decodeBody<T: Decodable>(_ data: Data, key: String? = nil) throws -> T {
        let decoder = makeDecoder()
        do {
            guard let key else { return try decoder.decode(T.self, from: data) }
            decoder.userInfo[Envelope<T>.keyInfo] = key
            return try decoder.decode(Envelope<T>.self, from: data).value
        } catch {
            throw APIError.badResponse
        }
    }

    private func decode<T: Decodable>(_ data: Data, key: String?) throws -> T {
        try Self.decodeBody(data, key: key)
    }
}

private struct Envelope<T: Decodable>: Decodable {
    static var keyInfo: CodingUserInfoKey { CodingUserInfoKey(rawValue: "envelope")! }

    struct Key: CodingKey {
        let stringValue: String
        var intValue: Int? { nil }
        init(stringValue: String) { self.stringValue = stringValue }
        init?(intValue: Int) { nil }
    }

    let value: T

    init(from decoder: Decoder) throws {
        let key = decoder.userInfo[Envelope.keyInfo] as? String ?? ""
        value = try decoder.container(keyedBy: Key.self).decode(T.self, forKey: Key(stringValue: key))
    }
}
