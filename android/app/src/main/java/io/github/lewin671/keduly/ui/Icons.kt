// The line icons of the mockup, built from its SVG paths (24-unit grid, 1.8 stroke, round ends).
package io.github.lewin671.keduly.ui

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.vector.addPathNodes
import androidx.compose.ui.unit.dp

private class Part(val path: String, val filled: Boolean = false, val width: Float = 1.8f)

private fun circle(cx: Float, cy: Float, r: Float) = "M${cx - r} ${cy}a$r $r 0 1 0 ${2 * r} 0a$r $r 0 1 0 ${-2 * r} 0z"

private fun rrect(x: Float, y: Float, w: Float, h: Float, r: Float) =
    "M${x + r} ${y}h${w - 2 * r}a$r $r 0 0 1 $r ${r}v${h - 2 * r}a$r $r 0 0 1 ${-r} ${r}h${-(w - 2 * r)}a$r $r 0 0 1 ${-r} ${-r}v${-(h - 2 * r)}a$r $r 0 0 1 $r ${-r}z"

private fun icon(name: String, vararg parts: Part): ImageVector {
    val builder = ImageVector.Builder(name, 24.dp, 24.dp, 24f, 24f)
    for (part in parts) {
        if (part.filled) {
            builder.addPath(addPathNodes(part.path), fill = SolidColor(Color.Black))
        } else {
            builder.addPath(
                addPathNodes(part.path), stroke = SolidColor(Color.Black), strokeLineWidth = part.width,
                strokeLineCap = StrokeCap.Round, strokeLineJoin = StrokeJoin.Round,
            )
        }
    }
    return builder.build()
}

private fun stroke(name: String, vararg paths: String, width: Float = 1.8f) = icon(name, *paths.map { Part(it, width = width) }.toTypedArray())

object KIcons {
    val Calendar = stroke("calendar", rrect(3.5f, 5f, 17f, 15.5f, 3.5f), "M3.5 10h17M8 3v4M16 3v4")
    val Tray = stroke("tray", "M4 13.5l2.4 -7.2A2 2 0 0 1 8.3 5h7.4a2 2 0 0 1 1.9 1.3l2.4 7.2v4a2 2 0 0 1 -2 2H6a2 2 0 0 1 -2 -2zM4 13.5h4.5a3.5 3.5 0 0 0 7 0H20")
    val Checklist = stroke("checklist", circle(12f, 12f, 8.5f), "M8.3 12.3l2.6 2.6 4.8 -5.2")
    val Bell = stroke("bell", "M6 16.5V11a6 6 0 0 1 12 0v5.5l1.5 2h-15zM10 20.5a2.2 2.2 0 0 0 4 0")
    val Gear = stroke("gear", circle(12f, 12f, 3f), "M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M18.4 5.6l-1.8 1.8M7.4 16.6l-1.8 1.8")
    val Search = stroke("search", circle(10.5f, 10.5f, 6.5f), "M15.5 15.5L20 20", width = 2f)
    val Plus = stroke("plus", "M12 5v14M5 12h14", width = 2.2f)
    val Back = stroke("back", "M11 5.5L4.5 12l6.5 6.5M5 12h14.5", width = 2f)
    val Left = stroke("left", "M14.5 5.5L8 12l6.5 6.5", width = 2.2f)
    val Right = stroke("right", "M9.5 5.5L16 12l-6.5 6.5", width = 2.2f)
    val Down = stroke("down", "M5.5 9.5L12 16l6.5 -6.5", width = 2.4f)
    val Close = stroke("close", "M6 6l12 12M18 6L6 18", width = 2f)
    val Star = icon("star", Part("M12 3.2l2.7 5.5 6 .9 -4.4 4.2 1.1 6L12 17l-5.4 2.8 1.1 -6 -4.4 -4.2 6 -.9z", filled = true))
    val Layers = stroke("layers", "M12 4l8.5 4.5L12 13 3.5 8.5zM3.5 13L12 17.5 20.5 13")
    val Book = stroke("book", rrect(4f, 4f, 16f, 16f, 4.5f), "M8.3 12.3l2.6 2.6 4.8 -5.2")
    val Flag = stroke("flag", "M6 21V4.5M6 5h11.5l-2.4 3.6L17.5 12H6")
    val Clock = stroke("clock", circle(12f, 12f, 8.5f), "M12 7.5V12l3 2")
    val Moon = icon("moon", Part("M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z", filled = true))
    val Grid = stroke("grid", rrect(4f, 4f, 7f, 7f, 2f), rrect(13f, 4f, 7f, 7f, 2f), rrect(4f, 13f, 7f, 7f, 2f), rrect(13f, 13f, 7f, 7f, 2f))
    val Chart = stroke("chart", "M6 19.5v-7M12 19.5v-14M18 19.5v-10", width = 2.6f)
    val Timer = stroke("timer", circle(12f, 13.5f, 7.5f), "M12 9.5v4M9.5 3h5M12 3v3")
    val Play = icon("play", Part("M6 3.5v17l14 -8.5z", filled = true))
    val More = icon("more", Part(circle(5.5f, 12f, 1.6f), filled = true), Part(circle(12f, 12f, 1.6f), filled = true), Part(circle(18.5f, 12f, 1.6f), filled = true))
    val Bang = icon("bang", Part("M12 5v9", width = 2.6f), Part(circle(12f, 18.6f, 1.5f), filled = true))
    val Folder = icon("folder", Part(circle(12f, 12f, 8.5f)), Part(circle(12f, 12f, 3.2f), filled = true))
    val Pin = stroke("pin", "M12 21s6.5 -5.6 6.5 -11a6.5 6.5 0 0 0 -13 0c0 5.4 6.5 11 6.5 11z", circle(12f, 10f, 2.3f))
    val Note = stroke("note", "M5 7h14M5 12h14M5 17h9")
    val Person = stroke("person", circle(12f, 8.5f, 3.7f), "M4.5 20a7.5 7.5 0 0 1 15 0")
    val Check = stroke("check", "M5 12.5l4.5 4.5L19 7", width = 2.2f)
}
