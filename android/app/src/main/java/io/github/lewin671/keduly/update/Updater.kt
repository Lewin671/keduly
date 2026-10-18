package io.github.lewin671.keduly.update

import android.content.ClipData
import android.content.Context
import android.content.Intent
import android.content.pm.ApplicationInfo
import android.net.Uri
import android.provider.Settings
import androidx.core.content.FileProvider
import androidx.core.content.pm.PackageInfoCompat
import java.io.File
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import java.util.concurrent.TimeUnit

/** Where a downloaded APK waits for the installer (res/xml/file_paths.xml); the cache could be cleared under it. */
const val UPDATES_DIR = "updates"

sealed interface UpdateState {
    /** Not looked up yet, or the last look failed. */
    data object Unknown : UpdateState
    /** A look is under way and nothing newer is known yet. */
    data object Checking : UpdateState
    data object Latest : UpdateState
    data class Available(val release: Release) : UpdateState
    data class Downloading(val release: Release, val done: Long) : UpdateState
    /** Downloaded and verified: only the system's confirmation is left. */
    data class Ready(val release: Release, val file: File) : UpdateState
    data class Failed(val release: Release) : UpdateState
}

val UpdateState.release: Release?
    get() = when (this) {
        is UpdateState.Available -> release
        is UpdateState.Downloading -> release
        is UpdateState.Ready -> release
        is UpdateState.Failed -> release
        else -> null
    }

/**
 * The app updating itself from the GitHub releases. It looks for a newer release when it comes
 * to the foreground, at most every few hours, and when the user asks. Downloading starts only
 * when the user asks, and installing is the system's dialog: Android replaces the app only with a
 * package that carries its signature. Debug builds never update themselves.
 */
object Updater {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private val releases = Releases(
        OkHttpClient.Builder().connectTimeout(10, TimeUnit.SECONDS).readTimeout(30, TimeUnit.SECONDS).build(),
    )
    private val _state = MutableStateFlow<UpdateState>(UpdateState.Unknown)
    val state: StateFlow<UpdateState> = _state.asStateFlow()

    /** Set when a look the user asked for failed; cleared by the next look. */
    private val _checkFailed = MutableStateFlow(false)
    val checkFailed: StateFlow<Boolean> = _checkFailed.asStateFlow()

    private var checkedAt = 0L
    private var job: Job? = null

    fun enabled(context: Context): Boolean = context.applicationInfo.flags and ApplicationInfo.FLAG_DEBUGGABLE == 0

    fun installedCode(context: Context): Long =
        PackageInfoCompat.getLongVersionCode(context.packageManager.getPackageInfo(context.packageName, 0))

    /** Looks for a newer release. A release already found stays offered unless a newer one takes its place. */
    fun check(context: Context, manual: Boolean = false) {
        val app = context.applicationContext
        if (!enabled(app) || job?.isActive == true || !checkDue(manual, System.currentTimeMillis(), checkedAt)) return
        _checkFailed.value = false
        if (_state.value.release == null) _state.value = UpdateState.Checking
        job = scope.launch {
            try {
                val found = releases.latest()
                checkedAt = System.currentTimeMillis()
                val before = _state.value
                when {
                    !isNewer(found, installedCode(app)) -> {
                        _state.value = UpdateState.Latest
                        withContext(Dispatchers.IO) { File(app.filesDir, UPDATES_DIR).deleteRecursively() }
                    }
                    found == before.release -> Unit
                    else -> _state.value = UpdateState.Available(found)
                }
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                if (_state.value == UpdateState.Checking) _state.value = UpdateState.Unknown
                _checkFailed.value = manual
            }
        }
    }

    /** Downloads the release found and verifies it; the screen in front then opens the installer. */
    fun download(context: Context) {
        val app = context.applicationContext
        val release = _state.value.release ?: return
        if (job?.isActive == true) return
        _state.value = UpdateState.Downloading(release, 0)
        job = scope.launch {
            try {
                val file = releases.download(release, File(app.filesDir, UPDATES_DIR)) { done -> _state.value = UpdateState.Downloading(release, done) }
                val refused = withContext(Dispatchers.IO) {
                    val theirs = app.packageManager.getPackageArchiveInfo(file.path, 0)
                    if (theirs == null) "not a package" else packageMismatch(app.packageName, theirs.packageName, PackageInfoCompat.getLongVersionCode(theirs), release)
                }
                if (refused != null) {
                    file.delete()
                    throw java.io.IOException(refused)
                }
                _state.value = UpdateState.Ready(release, file)
            } catch (e: Exception) {
                if (e is CancellationException) throw e
                _state.value = UpdateState.Failed(release)
            }
        }
    }

    /** Whether the user has allowed this app to install packages (安装未知应用). */
    fun canInstall(context: Context): Boolean = context.packageManager.canRequestPackageInstalls()

    fun permissionIntent(context: Context): Intent =
        Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${context.packageName}"))

    /** The system's installer on a downloaded APK, which only the installer may read. */
    @Suppress("DEPRECATION") // ACTION_INSTALL_PACKAGE is what vendor installers accept most reliably.
    fun installIntent(context: Context, file: File): Intent {
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", file)
        return Intent(Intent.ACTION_INSTALL_PACKAGE).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            clipData = ClipData.newRawUri("keduly", uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
    }
}
