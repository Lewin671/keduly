package io.github.lewin671.keduly

import android.content.Intent

/**
 * Debug builds only: lets a developer start the app signed in and on a given screen, e.g.
 *
 *     adb shell am start -n io.github.lewin671.keduly/.MainActivity \
 *         --es server http://127.0.0.1:8080 --es email demo@example.com --es password demo-password --es route today
 *
 * The release build has an empty counterpart, so none of this ships.
 */
object DebugHooks {
    fun read(intent: Intent?): Launch? {
        intent ?: return null
        val launch = Launch(intent.getStringExtra("server"), intent.getStringExtra("email"), intent.getStringExtra("password"), intent.getStringExtra("route"))
        return launch.takeIf { it.server != null || it.route != null }
    }
}
