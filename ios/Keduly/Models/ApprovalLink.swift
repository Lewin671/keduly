import Foundation

/// A login request as the approving device sees it: `device` is the server's name for the asker,
/// empty when unknown.
struct PendingLogin: Decodable, Sendable, Equatable {
    let id: String
    let device: String
    let expiresAt: Date
}

/// What a pasted approval link names: a request on this server, one on another, or nothing.
enum ApprovalLink: Equatable {
    case request(String)
    case otherServer(String)
    case notALink

    /// Reads the approval link the asking device shared, `{origin}/#approve={id}`, from pasted
    /// text. Only the request id is taken, and only when the link is for `server`, the origin this
    /// phone is signed in to; a bare id is taken as this server's.
    static func parse(_ text: String, server: URL?) -> ApprovalLink {
        let trimmed = text.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.wholeMatch(of: /[a-z0-9]{16}/) != nil { return .request(trimmed) }
        guard let match = trimmed.firstMatch(of: /https?:\/\/\S*#approve=([a-z0-9]{16})\b/),
              let link = URLComponents(string: String(match.output.0)),
              let server, let ours = URLComponents(url: server, resolvingAgainstBaseURL: false)
        else { return .notALink }
        return origin(link) == origin(ours) ? .request(String(match.output.1)) : .otherServer(link.host ?? "")
    }

    private static func origin(_ url: URLComponents) -> String {
        let scheme = url.scheme?.lowercased() ?? ""
        let port = url.port ?? (scheme == "https" ? 443 : 80)
        return "\(scheme)://\(url.host?.lowercased() ?? ""):\(port)"
    }
}
