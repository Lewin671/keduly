// Rules for edits that are made in place and written later, and for the order writes are sent in.
package io.github.lewin671.keduly.core

import java.util.concurrent.atomic.AtomicInteger
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

/**
 * A text field edited in place. [draft] is `null` until the user types in it: an untouched field
 * shows whatever the server has now, however that changes meanwhile, and is never written.
 */
object Draft {
    fun shown(draft: String?, current: String): String = draft ?: current

    /** What to write when the editor goes away, or `null` when there is nothing to write. */
    fun toWrite(draft: String?, current: String, allowEmpty: Boolean): String? {
        val text = draft?.trim() ?: return null
        if (text == current || (text.isEmpty() && !allowEmpty)) return null
        return text
    }
}

/**
 * Sends writes one at a time, in the order they were asked for. A write asked for while another
 * is in flight waits for it; it is neither dropped nor sent twice. [busy] is true until all are done.
 */
class WriteQueue {
    val busy = MutableStateFlow(false)
    private val mutex = Mutex()
    private val pending = AtomicInteger(0)

    suspend fun <T> run(block: suspend () -> T): T {
        pending.incrementAndGet()
        busy.value = true
        try {
            return mutex.withLock { block() }
        } finally {
            if (pending.decrementAndGet() == 0) busy.value = false
        }
    }
}
