// Objects of the HTTP API, as documented in docs/api.md. Field names follow the JSON through the
// snake-case naming strategy of [ApiJson].
package io.github.lewin671.keduly.api

import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonNamingStrategy
import kotlinx.serialization.json.JsonObject

@OptIn(ExperimentalSerializationApi::class)
val ApiJson: Json = Json {
    ignoreUnknownKeys = true
    namingStrategy = JsonNamingStrategy.SnakeCase
}

@Serializable
data class Actor(val kind: String, val name: String)

@Serializable
data class User(
    val id: String,
    val email: String,
    val name: String,
    val timezone: String,
    val timezoneAuto: Boolean,
    val workStart: String,
    val workEnd: String,
    val focusMinutes: Int,
    val restMinutes: Int,
    val longRestMinutes: Int,
    val roundSize: Int,
    val createdAt: String,
)

@Serializable
data class Area(val id: String, val name: String, val position: Int)

@Serializable
data class Project(
    val id: String,
    val areaId: String?,
    val name: String,
    val color: String,
    val notes: String,
    val position: Int,
    val archived: Boolean,
    val openCount: Int,
    val doneCount: Int,
    val createdAt: String,
    val updatedAt: String,
)

@Serializable
data class Heading(val id: String, val projectId: String, val name: String, val position: Int)

@Serializable
data class ItemBlock(val eventId: String, val start: String, val end: String)

@Serializable
data class ItemSuggestion(val id: String, val start: String, val end: String, val reason: String, val actor: Actor)

/** Time spent on an item with the focus timer. A session still running is not counted. */
@Serializable
data class ItemFocus(val tomatoes: Int, val minutes: Int)

@Serializable
data class Item(
    val id: String,
    val projectId: String?,
    val headingId: String?,
    val title: String,
    val notes: String,
    val estimateMinutes: Int?,
    val plannedDate: String?,
    val evening: Boolean,
    val dueDate: String?,
    val dueTime: String?,
    val important: Boolean,
    val status: String,
    val completedAt: String?,
    val position: Int,
    val block: ItemBlock?,
    val suggestion: ItemSuggestion?,
    val focus: ItemFocus,
    val createdBy: Actor,
    val createdAt: String,
    val updatedAt: String,
) {
    val done: Boolean get() = status == "done"
}

/** Something on the calendar. Instances of a recurring series share one `id`; see [key]. */
@Serializable
data class Event(
    val id: String,
    val projectId: String?,
    val itemId: String?,
    val title: String,
    val notes: String,
    val location: String,
    val allDay: Boolean,
    val start: String?,
    val end: String?,
    val startDate: String?,
    val endDate: String?,
    val rrule: String?,
    val recurring: Boolean,
    val instance: String?,
    val status: String,
    val suggestionId: String?,
    val itemDone: Boolean?,
    val readonly: Boolean,
    val createdBy: Actor,
    val updatedAt: String,
) {
    /** What identifies one entry on the calendar: the id alone is shared by a whole series. */
    val key: String get() = "$id|${instance.orEmpty()}|$status"
    val tentative: Boolean get() = status == "tentative"
    val leaving: Boolean get() = status == "leaving"
}

@Serializable
data class FocusSession(
    val id: String,
    val kind: String,
    val itemId: String?,
    val projectId: String?,
    val title: String,
    val start: String,
    val end: String,
    val plannedMinutes: Int,
    val completed: Boolean,
    val createdBy: Actor,
)

/** The state of the user's one timer. */
@Serializable
data class Focus(
    val state: String,
    val session: FocusSession?,
    val tomatoesToday: Int,
    val minutesToday: Int,
    val roundSize: Int,
    val roundDone: Int,
    val restMinutes: Int,
    val now: String,
)

@Serializable
data class Suggestion(
    val id: String,
    val status: String,
    val kind: String,
    val actor: Actor,
    val reason: String,
    val title: String,
    val itemId: String?,
    val eventId: String?,
    val start: String?,
    val end: String?,
    val item: JsonObject?,
    val event: JsonObject?,
    val createdAt: String,
    val decidedAt: String?,
)

@Serializable
data class Activity(
    val id: String,
    val actor: Actor,
    val action: String,
    val summary: String,
    val reason: String?,
    val undoable: Boolean,
    val undone: Boolean,
    val createdAt: String,
)

@Serializable
data class Token(
    val id: String,
    val name: String,
    val kind: String,
    val scope: String,
    val confirmDelete: Boolean,
    val lastUsedAt: String?,
    val createdAt: String,
)

@Serializable
data class Counts(val inbox: Int, val today: Int, val pending: Int)

@Serializable
data class CountsReply(val counts: Counts, val revision: Long)

@Serializable
data class Config(val registration: String, val version: String)

@Serializable
data class Bootstrap(
    val user: User,
    val areas: List<Area>,
    val projects: List<Project>,
    val headings: List<Heading>,
    val counts: Counts,
    val revision: Long,
)

@Serializable
data class ItemPage(val items: List<Item>, val nextCursor: String?, val total: Int)

@Serializable
data class ProjectDetail(
    val project: Project,
    val headings: List<Heading>,
    val upcomingEvents: List<Event>,
    val unplannedCount: Int,
    val focusWeekMinutes: Int,
)

@Serializable
data class TodayView(
    val date: String,
    val items: List<Item>,
    val overdue: List<Item>,
    val events: List<Event>,
    val freeMinutes: Int,
    val unplannedMinutes: Int,
)

@Serializable
data class UpcomingDay(val date: String, val events: List<Event>, val items: List<Item>)

@Serializable
data class OverviewProject(val projectId: String, val total: Int, val items: List<Item>)

@Serializable
data class Quadrant(val total: Int, val unplanned: Int, val items: List<Item>)

@Serializable
data class Quadrants(
    @SerialName("do") val now: Quadrant,
    val plan: Quadrant,
    val quick: Quadrant,
    val later: Quadrant,
)

@Serializable
data class FreeSlot(val start: String, val end: String)

@Serializable
data class FocusAmount(val projectId: String?, val tomatoes: Int, val minutes: Int)

@Serializable
data class FocusDay(val date: String, val tomatoes: Int, val minutes: Int, val projects: List<FocusAmount>)

/** The last 7 days, today included. */
@Serializable
data class FocusStats(
    val days: List<FocusDay>,
    val projects: List<FocusAmount>,
    val tomatoes: Int,
    val minutes: Int,
    val streak: Int,
)

@Serializable
data class ActivityPage(val activities: List<Activity>, val nextCursor: String?)
