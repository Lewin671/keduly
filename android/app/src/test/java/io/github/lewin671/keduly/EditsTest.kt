package io.github.lewin671.keduly

import io.github.lewin671.keduly.core.Draft
import io.github.lewin671.keduly.core.WriteQueue
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class EditsTest {
    @Test fun anUntouchedFieldFollowsTheServerAndIsNotWritten() {
        // Another device renamed the item while the sheet was open.
        assertEquals("新标题", Draft.shown(null, "新标题"))
        assertNull(Draft.toWrite(null, "新标题", allowEmpty = false))
        assertNull(Draft.toWrite(null, "别人写的备注", allowEmpty = true))
    }

    @Test fun anEditedFieldKeepsWhatWasTypedAndIsWritten() {
        assertEquals("我的标题", Draft.shown("我的标题", "新标题"))
        assertEquals("我的标题", Draft.toWrite(" 我的标题 ", "新标题", allowEmpty = false))
    }

    @Test fun typingTheSameTextAgainWritesNothing() {
        assertNull(Draft.toWrite("写周报 ", "写周报", allowEmpty = false))
    }

    @Test fun anEmptiedTitleIsNotWrittenButEmptiedNotesAre() {
        assertNull(Draft.toWrite("   ", "写周报", allowEmpty = false))
        assertEquals("", Draft.toWrite("", "旧备注", allowEmpty = true))
        // Notes that were already empty stay untouched.
        assertNull(Draft.toWrite("", "", allowEmpty = true))
    }

    @Test fun aWriteAskedForWhileAnotherIsInFlightIsSentAfterIt() = runTest {
        val queue = WriteQueue()
        val sent = ArrayList<String>()
        val firstMayFinish = CompletableDeferred<Unit>()
        val first = async {
            queue.run {
                sent += "set estimate"
                firstMayFinish.await()
                "first"
            }
        }
        runCurrent()
        assertTrue(queue.busy.value)
        // The sheet closes now: the text is saved while the first request has not come back.
        val second = async { queue.run { sent += "save title"; "second" } }
        runCurrent()
        assertEquals(listOf("set estimate"), sent)
        assertTrue(queue.busy.value)
        firstMayFinish.complete(Unit)
        assertEquals("first", first.await())
        assertEquals("second", second.await())
        assertEquals(listOf("set estimate", "save title"), sent)
        assertFalse(queue.busy.value)
    }

    @Test fun aFailedWriteDoesNotBlockTheNextAndIsNotRepeated() = runTest {
        val queue = WriteQueue()
        var attempts = 0
        val failed = runCatching { queue.run<Unit> { attempts++; error("no answer") } }
        assertTrue(failed.isFailure)
        assertFalse(queue.busy.value)
        var ran = false
        launch { queue.run { ran = true } }
        runCurrent()
        assertTrue(ran)
        assertEquals(1, attempts)
    }
}
