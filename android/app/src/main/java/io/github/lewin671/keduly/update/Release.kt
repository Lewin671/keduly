// What the latest published release says, and fetching its APK. Each GitHub release carries
// latest.properties, written by scripts/release-manifest.py, next to the APK it describes.
package io.github.lewin671.keduly.update

import java.io.File
import java.io.IOException
import java.io.StringReader
import java.security.MessageDigest
import java.util.Properties
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request

const val RELEASES_URL = "https://github.com/Lewin671/keduly/releases/"
const val RELEASE_MANIFEST = "latest.properties"

data class Release(
    val version: String,
    val versionCode: Long,
    /** The APK's file name next to the manifest. */
    val apk: String,
    val sha256: String,
    val size: Long,
    /** What changed, as written for the release; may be empty. */
    val notes: String,
)

private val VERSION = Regex("""\d+\.\d+\.\d+""")
private val SHA256 = Regex("[0-9a-f]{64}")
private const val MAX_APK_BYTES = 200L * 1024 * 1024

/** Reads a manifest. Anything missing or out of shape is refused, so a broken one offers nothing. */
fun parseRelease(text: String): Release {
    val p = Properties().apply { load(StringReader(text)) }
    fun field(name: String) = p.getProperty(name)?.trim().takeUnless { it.isNullOrEmpty() } ?: throw IOException("release manifest lacks $name")
    val release = Release(
        version = field("version"),
        versionCode = field("versionCode").toLongOrNull() ?: throw IOException("versionCode is not a number"),
        apk = field("apk"),
        sha256 = field("sha256").lowercase(),
        size = field("size").toLongOrNull() ?: throw IOException("size is not a number"),
        notes = p.getProperty("notes").orEmpty().trim(),
    )
    // The version and the file name become part of the address the APK is fetched from: one shape only.
    if (!VERSION.matches(release.version) || release.apk != "keduly-android-v${release.version}.apk" ||
        !SHA256.matches(release.sha256) || release.versionCode <= 0 || release.size !in 1..MAX_APK_BYTES
    ) throw IOException("release manifest is malformed")
    return release
}

private const val CHECK_EVERY_MS = 6 * 3600_000L

/** Whether to look for a release now: the user asked, or the last look was a while ago. */
fun checkDue(manual: Boolean, now: Long, lastAtMs: Long): Boolean = manual || now - lastAtMs >= CHECK_EVERY_MS

/** Only the version code decides, as it does for the system's installer. */
fun isNewer(release: Release, installedCode: Long): Boolean = release.versionCode > installedCode

/** Why a downloaded package must not go to the installer, or null when it is this release of this app. */
fun packageMismatch(installedName: String, name: String?, versionCode: Long, release: Release): String? = when {
    name != installedName -> "not this app"
    versionCode != release.versionCode -> "version differs from the release"
    else -> null
}

class Releases(http: OkHttpClient, private val base: String = RELEASES_URL) {
    // Release files are served from another host by redirect; never down to plain HTTP.
    private val http = http.newBuilder().followRedirects(true).followSslRedirects(false).build()

    suspend fun latest(): Release = withContext(Dispatchers.IO) {
        http.newCall(Request.Builder().url(base + "latest/download/" + RELEASE_MANIFEST).header("Cache-Control", "no-cache").build()).execute().use { res ->
            if (!res.isSuccessful) throw IOException("HTTP ${res.code}")
            parseRelease(res.body.string())
        }
    }

    /**
     * Downloads [release]'s APK into [into] and returns it only if it is the file the manifest
     * describes, by size and SHA-256; otherwise nothing is left behind.
     */
    suspend fun download(release: Release, into: File, progress: (done: Long) -> Unit = {}): File = withContext(Dispatchers.IO) {
        val file = File(into, release.apk)
        if (file.length() == release.size && sha256(file) == release.sha256) return@withContext file
        into.mkdirs()
        val part = File(into, release.apk + ".part")
        try {
            // Under its own tag, not "latest": the file the manifest described stays there when a newer release comes.
            http.newCall(Request.Builder().url(base + "download/v${release.version}/" + release.apk).build()).execute().use { res ->
                if (!res.isSuccessful) throw IOException("HTTP ${res.code}")
                part.outputStream().use { out ->
                    val src = res.body.byteStream()
                    val buf = ByteArray(64 * 1024)
                    var done = 0L
                    while (true) {
                        ensureActive()
                        val n = src.read(buf)
                        if (n < 0) break
                        done += n
                        if (done > release.size) throw IOException("download is larger than the release says")
                        out.write(buf, 0, n)
                        progress(done)
                    }
                }
            }
            if (part.length() != release.size || sha256(part) != release.sha256) throw IOException("download does not match the release")
            if (!part.renameTo(file)) throw IOException("cannot keep the download")
            file
        } finally {
            part.delete()
        }
    }
}

internal fun sha256(file: File): String {
    val md = MessageDigest.getInstance("SHA-256")
    file.inputStream().use { src ->
        val buf = ByteArray(64 * 1024)
        while (true) {
            val n = src.read(buf)
            if (n < 0) break
            md.update(buf, 0, n)
        }
    }
    return md.digest().joinToString("") { "%02x".format(it) }
}
