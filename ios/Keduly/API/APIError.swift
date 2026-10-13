import Foundation

enum APIError: Error, Equatable, Sendable {
    /// `401`: the session is gone.
    case unauthenticated
    /// Any other answer that is an error, with the server's `code` and `message`.
    case server(status: Int, code: String, message: String)
    /// The request got no answer in time. A write may or may not have happened.
    case timeout
    /// The server could not be reached.
    case network
    /// The answer was not what docs/api.md describes.
    case badResponse

    var status: Int? {
        if case .server(let status, _, _) = self { return status }
        return nil
    }

    /// What to tell the user. A 400 carries the server's own words, which name the field.
    var message: String {
        switch self {
        case .unauthenticated: return L("error.unauthenticated")
        case .timeout, .network: return L("error.network")
        case .badResponse: return L("error.bad_response")
        case .server(let status, let code, let message):
            switch status {
            case 400: return message.isEmpty ? L("error.invalid_request") : message
            case 403: return code == "registration_closed" ? L("error.registration_closed") : L("error.forbidden")
            case 404: return L("error.not_found")
            case 409: return L("error.conflict")
            case 429: return L("error.rate_limited")
            default: return L("error.internal")
            }
        }
    }
}
