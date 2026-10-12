// Colours and type. The values are the mockup's (docs/design/mobile.html), light and dark; the
// app does not use dynamic colour.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.lerp

class Palette(
    val dark: Boolean,
    val bg: Color,
    val grouped: Color,
    val cell: Color,
    val sheet: Color,
    val sheetCell: Color,
    val label: Color,
    val label2: Color,
    val label3: Color,
    val sep: Color,
    val fill: Color,
    val fill2: Color,
    val tint: Color,
    val onTint: Color,
    val red: Color,
    val orange: Color,
    val green: Color,
    val blue: Color,
    val indigo: Color,
    val purple: Color,
    val teal: Color,
    val pink: Color,
    val brown: Color,
    val yellow: Color,
    val tomato: Color,
) {
    /** What `color-mix` does towards black in light and white in dark. */
    val mix: Color get() = if (dark) Color.White else Color.Black

    /** The colour of a project, by the API's name for it. */
    fun project(name: String?): Color = when (name) {
        "blue" -> blue
        "indigo" -> indigo
        "orange" -> orange
        "teal" -> teal
        "green" -> green
        "pink" -> pink
        "purple" -> purple
        "brown" -> brown
        else -> label3
    }

    /** A tinted background: [share] of the colour over the page. */
    fun wash(color: Color, share: Float, over: Color = bg): Color = lerp(over, color.copy(alpha = 1f), share)

    /** Text on a tinted block: the colour pulled towards black (light) or white (dark). */
    fun ink(color: Color): Color = lerp(mix, color.copy(alpha = 1f), 0.64f)
}

val PROJECT_COLORS = listOf("blue", "indigo", "orange", "teal", "green", "pink", "purple", "brown")

private val Light = Palette(
    dark = false,
    bg = Color(0xFFFFFFFF), grouped = Color(0xFFF2F2F7), cell = Color(0xFFFFFFFF), sheet = Color(0xFFF2F2F7), sheetCell = Color(0xFFFFFFFF),
    label = Color(0xFF1C1C1E), label2 = Color(0x9E3C3C43), label3 = Color(0x4D3C3C43),
    sep = Color(0x293C3C43), fill = Color(0x1F787880), fill2 = Color(0x33787880),
    tint = Color(0xFF2867D6), onTint = Color.White,
    red = Color(0xFFFF3B30), orange = Color(0xFFF08C00), green = Color(0xFF34C759), blue = Color(0xFF007AFF),
    indigo = Color(0xFF5856D6), purple = Color(0xFFAF52DE), teal = Color(0xFF30B0C7), pink = Color(0xFFFF2D55), brown = Color(0xFFA2845E),
    yellow = Color(0xFFF5B400), tomato = Color(0xFFF25C45),
)

private val Dark = Palette(
    dark = true,
    bg = Color(0xFF000000), grouped = Color(0xFF000000), cell = Color(0xFF1C1C1E), sheet = Color(0xFF1C1C1E), sheetCell = Color(0xFF2C2C2E),
    label = Color(0xFFFFFFFF), label2 = Color(0x9EEBEBF5), label3 = Color(0x4DEBEBF5),
    sep = Color(0x99545458), fill = Color(0x3D787880), fill2 = Color(0x5C787880),
    tint = Color(0xFF6C9DFF), onTint = Color(0xFF06122B),
    red = Color(0xFFFF453A), orange = Color(0xFFFF9F0A), green = Color(0xFF30D158), blue = Color(0xFF0A84FF),
    indigo = Color(0xFF7D7AFF), purple = Color(0xFFBF5AF2), teal = Color(0xFF40C8E0), pink = Color(0xFFFF375F), brown = Color(0xFFAC8E68),
    yellow = Color(0xFFF5B400), tomato = Color(0xFFFF6B55),
)

val LocalPalette = staticCompositionLocalOf { Light }

/** The mockup's colours for the current theme. */
val K: Palette
    @Composable get() = LocalPalette.current

@Composable
fun KedulyTheme(content: @Composable () -> Unit) {
    val p = if (isSystemInDarkTheme()) Dark else Light
    val scheme = if (p.dark) {
        darkColorScheme(
            primary = p.tint, onPrimary = p.onTint, secondary = p.tint, onSecondary = p.onTint,
            primaryContainer = p.wash(p.tint, 0.3f), onPrimaryContainer = p.label,
            secondaryContainer = p.wash(p.tint, 0.28f), onSecondaryContainer = p.label,
            background = p.bg, onBackground = p.label, surface = p.bg, onSurface = p.label,
            surfaceVariant = p.cell, onSurfaceVariant = p.label2, surfaceContainer = p.cell, surfaceContainerHigh = p.sheetCell,
            surfaceContainerHighest = p.sheetCell, surfaceContainerLow = p.sheet, surfaceContainerLowest = p.bg,
            error = p.red, onError = Color.White, outline = p.label3, outlineVariant = p.sep,
        )
    } else {
        lightColorScheme(
            primary = p.tint, onPrimary = p.onTint, secondary = p.tint, onSecondary = p.onTint,
            primaryContainer = p.wash(p.tint, 0.14f), onPrimaryContainer = p.label,
            secondaryContainer = p.wash(p.tint, 0.14f), onSecondaryContainer = p.label,
            background = p.bg, onBackground = p.label, surface = p.bg, onSurface = p.label,
            surfaceVariant = p.grouped, onSurfaceVariant = p.label2, surfaceContainer = p.grouped, surfaceContainerHigh = p.grouped,
            surfaceContainerHighest = p.grouped, surfaceContainerLow = p.sheet, surfaceContainerLowest = p.bg,
            error = p.red, onError = Color.White, outline = p.label3, outlineVariant = p.sep,
        )
    }
    CompositionLocalProvider(LocalPalette provides p) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
