package io.github.lewin671.keduly

import io.github.lewin671.keduly.update.Release
import io.github.lewin671.keduly.update.checkDue
import io.github.lewin671.keduly.update.isNewer
import io.github.lewin671.keduly.update.packageMismatch
import io.github.lewin671.keduly.update.parseRelease
import java.io.IOException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class UpdateTest {
    private val sha = "a".repeat(64)
    private val manifest = """
        version=0.3.0
        versionCode=4
        apk=keduly-android-v0.3.0.apk
        sha256=$sha
        size=1234
        notes=Habits\nand updates
    """.trimIndent()

    @Test fun readsManifest() {
        val r = parseRelease(manifest)
        assertEquals(Release("0.3.0", 4, "keduly-android-v0.3.0.apk", sha, 1234, "Habits\nand updates"), r)
    }

    @Test fun refusesMalformedManifests() {
        val bad = listOf(
            manifest.replace("version=0.3.0", "version=0.3"),
            manifest.replace("apk=keduly-android-v0.3.0.apk", "apk=../other.apk"),
            manifest.replace("sha256=$sha", "sha256=xyz"),
            manifest.replace("size=1234", "size=0"),
            manifest.replace("versionCode=4", "versionCode=four"),
            manifest.lines().filterNot { it.startsWith("sha256") }.joinToString("\n"),
        )
        for (text in bad) {
            try {
                parseRelease(text)
                throw AssertionError("accepted: $text")
            } catch (_: IOException) {
            }
        }
    }

    @Test fun onlyTheVersionCodeDecides() {
        val r = parseRelease(manifest)
        assertTrue(isNewer(r, 3))
        assertFalse(isNewer(r, 4))
        assertFalse(isNewer(r, 5))
    }

    @Test fun refusesAnotherPackageOrVersion() {
        val r = parseRelease(manifest)
        assertNull(packageMismatch("io.github.lewin671.keduly", "io.github.lewin671.keduly", 4, r))
        assertTrue(packageMismatch("io.github.lewin671.keduly", "com.example.other", 4, r) != null)
        assertTrue(packageMismatch("io.github.lewin671.keduly", "io.github.lewin671.keduly", 5, r) != null)
    }

    @Test fun looksAtMostEverySixHoursUnlessAsked() {
        val hour = 3600_000L
        assertFalse(checkDue(false, 10 * hour, 5 * hour))
        assertTrue(checkDue(false, 11 * hour, 5 * hour))
        assertTrue(checkDue(true, 5 * hour, 5 * hour))
    }
}
