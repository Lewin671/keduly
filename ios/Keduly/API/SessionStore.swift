import Foundation
import Security

/// What the app remembers between launches: which server, and the session cookie for it.
struct SessionCredentials: Codable, Equatable, Sendable {
    var server: URL
    var cookie: String
}

protocol SessionStore: Sendable {
    func load() -> SessionCredentials?
    func save(_ credentials: SessionCredentials)
    func clear()
}

/// Keeps the session in the Keychain, readable only on this device once it has been unlocked.
struct KeychainSessionStore: SessionStore {
    private let service = "io.github.lewin671.keduly.session"
    private let account = "session"

    private var query: [String: Any] {
        [kSecClass as String: kSecClassGenericPassword, kSecAttrService as String: service, kSecAttrAccount as String: account]
    }

    func load() -> SessionCredentials? {
        var q = query
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let data = out as? Data else { return nil }
        return try? JSONDecoder().decode(SessionCredentials.self, from: data)
    }

    func save(_ credentials: SessionCredentials) {
        guard let data = try? JSONEncoder().encode(credentials) else { return }
        SecItemDelete(query as CFDictionary)
        var q = query
        q[kSecValueData as String] = data
        q[kSecAttrAccessible as String] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        SecItemAdd(q as CFDictionary, nil)
    }

    func clear() {
        SecItemDelete(query as CFDictionary)
    }
}
