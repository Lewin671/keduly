package io.github.lewin671.keduly

import android.content.Context
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import io.github.lewin671.keduly.ui.App
import io.github.lewin671.keduly.ui.KedulyTheme
import java.util.Locale

/** What a debug build may be told on launch; see `DebugHooks`. */
data class Launch(val server: String?, val email: String?, val password: String?, val route: String?)

class MainActivity : ComponentActivity() {
    // The interface has one language, so the pickers of the platform speak it too.
    override fun attachBaseContext(base: Context) {
        val configuration = Configuration(base.resources.configuration).apply { setLocale(Locale.SIMPLIFIED_CHINESE) }
        super.attachBaseContext(base.createConfigurationContext(configuration))
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val launch = if (savedInstanceState == null) DebugHooks.read(intent) else null
        setContent { KedulyTheme { App(launch) } }
    }
}
