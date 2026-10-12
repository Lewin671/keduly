package io.github.lewin671.keduly

import android.content.Intent

/** The release build takes nothing from the launching intent. */
object DebugHooks {
    @Suppress("UNUSED_PARAMETER")
    fun read(intent: Intent?): Launch? = null
}
