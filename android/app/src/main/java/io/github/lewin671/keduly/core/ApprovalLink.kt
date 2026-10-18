package io.github.lewin671.keduly.core

import java.net.URI

/** What a pasted approval link names: a request on this server, one on another, or nothing. */
sealed interface ApprovalLink {
    data class Request(val id: String) : ApprovalLink
    data class OtherServer(val host: String) : ApprovalLink
    data object NotALink : ApprovalLink
}

private val ID = Regex("[a-z0-9]{16}")
private val LINK = Regex("""https?://\S*#approve=([a-z0-9]{16})\b""")

/**
 * Reads the approval link the asking device shared, `{origin}/#approve={id}`, from pasted text.
 * Only the request id is taken, and only when the link is for [server], the origin this phone is
 * signed in to; a bare id is taken as this server's.
 */
fun parseApprovalLink(text: String, server: String): ApprovalLink {
    val trimmed = text.trim()
    if (ID.matches(trimmed)) return ApprovalLink.Request(trimmed)
    val match = LINK.find(trimmed) ?: return ApprovalLink.NotALink
    val link = runCatching { URI(match.value) }.getOrNull() ?: return ApprovalLink.NotALink
    val ours = runCatching { URI(server) }.getOrNull() ?: return ApprovalLink.NotALink
    return if (origin(link) == origin(ours)) ApprovalLink.Request(match.groupValues[1]) else ApprovalLink.OtherServer(link.host.orEmpty())
}

private fun origin(uri: URI): String {
    val scheme = uri.scheme.orEmpty().lowercase()
    val port = when {
        uri.port != -1 -> uri.port
        scheme == "https" -> 443
        else -> 80
    }
    return "$scheme://${uri.host.orEmpty().lowercase()}:$port"
}
