// Request bodies for PATCH. An omitted field means "unchanged" and JSON null clears it, so every
// field says explicitly which of the two it is. The server rejects unknown fields, so only the
// documented ones exist here.
package io.github.lewin671.keduly.api

import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive

sealed interface Opt<out T> {
    /** Leave the field out of the body. */
    data object Keep : Opt<Nothing>

    /** Send the value; `null` clears the field. */
    data class Put<T>(val value: T?) : Opt<T>
}

private fun body(vararg fields: Pair<String, Opt<*>>): JsonObject {
    val out = LinkedHashMap<String, JsonElement>()
    for ((name, field) in fields) {
        if (field !is Opt.Put<*>) continue
        out[name] = when (val value = field.value) {
            null -> JsonNull
            is String -> JsonPrimitive(value)
            is Int -> JsonPrimitive(value)
            is Boolean -> JsonPrimitive(value)
            else -> error("unsupported value for $name")
        }
    }
    return JsonObject(out)
}

data class ItemPatch(
    val title: Opt<String> = Opt.Keep,
    val projectId: Opt<String> = Opt.Keep,
    val headingId: Opt<String> = Opt.Keep,
    val notes: Opt<String> = Opt.Keep,
    val estimateMinutes: Opt<Int> = Opt.Keep,
    val plannedDate: Opt<String> = Opt.Keep,
    val evening: Opt<Boolean> = Opt.Keep,
    val dueDate: Opt<String> = Opt.Keep,
    val dueTime: Opt<String> = Opt.Keep,
    val important: Opt<Boolean> = Opt.Keep,
    val status: Opt<String> = Opt.Keep,
) {
    fun toJson(): JsonObject = body(
        "title" to title, "project_id" to projectId, "heading_id" to headingId, "notes" to notes,
        "estimate_minutes" to estimateMinutes, "planned_date" to plannedDate, "evening" to evening,
        "due_date" to dueDate, "due_time" to dueTime, "important" to important, "status" to status,
    )
}

/** Also the body of `POST /events`, where every field that is set is simply sent. */
data class EventPatch(
    val title: Opt<String> = Opt.Keep,
    val projectId: Opt<String> = Opt.Keep,
    val notes: Opt<String> = Opt.Keep,
    val location: Opt<String> = Opt.Keep,
    val allDay: Opt<Boolean> = Opt.Keep,
    val start: Opt<String> = Opt.Keep,
    val end: Opt<String> = Opt.Keep,
    val startDate: Opt<String> = Opt.Keep,
    val endDate: Opt<String> = Opt.Keep,
) {
    fun toJson(): JsonObject = body(
        "title" to title, "project_id" to projectId, "notes" to notes, "location" to location,
        "all_day" to allDay, "start" to start, "end" to end, "start_date" to startDate, "end_date" to endDate,
    )
}

data class ProjectPatch(
    val name: Opt<String> = Opt.Keep,
    val color: Opt<String> = Opt.Keep,
    val areaId: Opt<String> = Opt.Keep,
    val notes: Opt<String> = Opt.Keep,
) {
    fun toJson(): JsonObject = body("name" to name, "color" to color, "area_id" to areaId, "notes" to notes)
}

data class MePatch(
    val name: Opt<String> = Opt.Keep,
    val workStart: Opt<String> = Opt.Keep,
    val workEnd: Opt<String> = Opt.Keep,
    val focusMinutes: Opt<Int> = Opt.Keep,
    val restMinutes: Opt<Int> = Opt.Keep,
    val longRestMinutes: Opt<Int> = Opt.Keep,
    val roundSize: Opt<Int> = Opt.Keep,
) {
    fun toJson(): JsonObject = body(
        "name" to name, "work_start" to workStart, "work_end" to workEnd, "focus_minutes" to focusMinutes,
        "rest_minutes" to restMinutes, "long_rest_minutes" to longRestMinutes, "round_size" to roundSize,
    )
}

/** `itemId` puts the session on an item; `title` and `projectId` are for free focus only. */
data class SessionPatch(
    val itemId: Opt<String> = Opt.Keep,
    val title: Opt<String> = Opt.Keep,
    val projectId: Opt<String> = Opt.Keep,
) {
    fun toJson(): JsonObject = body("item_id" to itemId, "title" to title, "project_id" to projectId)
}
