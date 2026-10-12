package io.github.lewin671.keduly.api

/** Where the server address and the session cookie are kept between launches. */
interface SessionStore {
    /** The server's origin without a trailing slash, e.g. `https://keduly.example.com`. */
    var server: String?

    /** The value of the `keduly_session` cookie. */
    var cookie: String?

    fun clear() {
        cookie = null
    }
}

class MemorySessionStore(override var server: String? = null, override var cookie: String? = null) : SessionStore
