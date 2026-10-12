package io.github.lewin671.keduly

import io.github.lewin671.keduly.api.Activity
import io.github.lewin671.keduly.api.ActivityPage
import io.github.lewin671.keduly.api.ApiJson
import io.github.lewin671.keduly.api.Area
import io.github.lewin671.keduly.api.Bootstrap
import io.github.lewin671.keduly.api.Config
import io.github.lewin671.keduly.api.CountsReply
import io.github.lewin671.keduly.api.Event
import io.github.lewin671.keduly.api.Focus
import io.github.lewin671.keduly.api.FocusSession
import io.github.lewin671.keduly.api.FocusStats
import io.github.lewin671.keduly.api.FreeSlot
import io.github.lewin671.keduly.api.Heading
import io.github.lewin671.keduly.api.Item
import io.github.lewin671.keduly.api.ItemPage
import io.github.lewin671.keduly.api.OverviewProject
import io.github.lewin671.keduly.api.Project
import io.github.lewin671.keduly.api.ProjectDetail
import io.github.lewin671.keduly.api.Quadrants
import io.github.lewin671.keduly.api.Suggestion
import io.github.lewin671.keduly.api.TodayView
import io.github.lewin671.keduly.api.Token
import io.github.lewin671.keduly.api.UpcomingDay
import io.github.lewin671.keduly.api.User
import kotlinx.serialization.json.decodeFromJsonElement
import kotlinx.serialization.json.jsonPrimitive
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class ModelsTest {
    private inline fun <reified T> field(fixture: String, key: String): T = ApiJson.decodeFromJsonElement(Fixtures.json(fixture).getValue(key))
    private inline fun <reified T> whole(fixture: String): T = ApiJson.decodeFromJsonElement(Fixtures.json(fixture))

    @Test fun user() {
        val user: User = field("user", "user")
        assertEquals("Asia/Shanghai", user.timezone)
        assertFalse(user.timezoneAuto)
        assertEquals("09:00" to "18:00", user.workStart to user.workEnd)
        assertEquals(listOf(25, 5, 15, 4), listOf(user.focusMinutes, user.restMinutes, user.longRestMinutes, user.roundSize))
    }

    @Test fun areaProjectAndHeading() {
        assertEquals("工作", field<Area>("area", "area").name)
        val project: Project = field("project", "project")
        assertEquals("blue", project.color)
        assertEquals(7 to 3, project.openCount to project.doneCount)
        assertEquals("bbbbbbbbbbbbbbbb", project.areaId)
        assertEquals("cccccccccccccccc", field<Heading>("heading", "heading").projectId)
    }

    @Test fun item() {
        val item: Item = field("item", "item")
        assertEquals(90, item.estimateMinutes)
        assertEquals("2026-10-13" to "18:00", item.dueDate to item.dueTime)
        assertTrue(item.important)
        assertFalse(item.done)
        assertEquals("ffffffffffffffff", item.block?.eventId)
        assertEquals("agent", item.suggestion?.actor?.kind)
        assertEquals(2 to 50, item.focus.tomatoes to item.focus.minutes)
    }

    @Test fun itemWithNothingSet() {
        val item: Item = field("item_bare", "item")
        assertNull(item.projectId)
        assertNull(item.estimateMinutes)
        assertNull(item.plannedDate)
        assertNull(item.block)
        assertNull(item.suggestion)
    }

    @Test fun events() {
        val events: List<Event> = field("events", "events")
        val (timed, allDay, recurring, tentative) = events
        assertEquals("2026-10-13T02:00:00Z", timed.start)
        assertFalse(timed.allDay)
        assertTrue(allDay.allDay)
        assertNull(allDay.start)
        assertEquals("2026-10-11" to "2026-10-12", allDay.startDate to allDay.endDate)
        assertTrue(recurring.recurring && recurring.readonly)
        assertEquals("2026-10-12T02:00:00Z", recurring.instance)
        assertTrue(tentative.tentative)
        assertEquals("gggggggggggggggg", tentative.suggestionId)
        assertEquals(false, tentative.itemDone)
    }

    @Test fun instancesOfASeriesAreToldApartByTheirInstance() {
        val recurring = field<List<Event>>("events", "events")[2]
        val next = recurring.copy(instance = "2026-10-19T02:00:00Z")
        assertEquals(recurring.id, next.id)
        assertNotEquals(recurring.key, next.key)
    }

    @Test fun focusAndSession() {
        val session: FocusSession = field("session", "session")
        assertEquals("work", session.kind)
        assertTrue(session.completed)
        assertEquals(25, session.plannedMinutes)
        val focus: Focus = field("focus", "focus")
        assertEquals("work", focus.state)
        assertEquals("2026-10-13T01:25:00Z", focus.session?.end)
        assertEquals(listOf(3, 80, 4, 3, 5), listOf(focus.tomatoesToday, focus.minutesToday, focus.roundSize, focus.roundDone, focus.restMinutes))
        assertEquals("2026-10-13T01:06:18Z", focus.now)
        assertNull(field<Focus>("focus_idle", "focus").session)
    }

    @Test fun suggestions() {
        val (schedule, create) = field<List<Suggestion>>("suggestions", "suggestions")
        assertEquals("schedule_item", schedule.kind)
        assertEquals("2026-10-13T06:00:00Z", schedule.start)
        assertNull(schedule.decidedAt)
        assertEquals("调研 FullCalendar 授权", create.item?.getValue("title")?.jsonPrimitive?.content)
        assertNull(create.start)
    }

    @Test fun activityAndToken() {
        val page: ActivityPage = whole("activity")
        val (first, second) = page.activities
        assertNull(page.nextCursor)
        assertEquals("item.create", first.action)
        assertTrue(first.undoable && !first.undone)
        assertNull(first.reason)
        assertEquals("挪到下午", second.reason)
        assertTrue(second.undone)
        assertEquals("write", field<List<Token>>("tokens", "tokens").single().scope)
        assertEquals(1, listOf<Activity>(first).size)
    }

    @Test fun bootstrapCountsAndConfig() {
        val boot: Bootstrap = whole("bootstrap")
        assertEquals(42L, boot.revision)
        assertEquals(listOf(2, 6, 4), listOf(boot.counts.inbox, boot.counts.today, boot.counts.pending))
        assertEquals(1, boot.projects.size)
        assertEquals(43L, whole<CountsReply>("counts").revision)
        assertEquals("open", whole<Config>("config").registration)
    }

    @Test fun listsAndViews() {
        val page: ItemPage = whole("items_page")
        assertEquals(2 to 12, page.items.size to page.total)
        assertEquals("b3BhcXVl", page.nextCursor)
        val detail: ProjectDetail = whole("project_detail")
        assertEquals(4 to 300, detail.unplannedCount to detail.focusWeekMinutes)
        assertEquals("设计评审", detail.upcomingEvents.single().title)
        val today: TodayView = whole("today")
        assertEquals("2026-10-13", today.date)
        assertEquals(320 to 95, today.freeMinutes to today.unplannedMinutes)
        assertEquals(1 to 1, today.items.size to today.overdue.size)
        assertEquals("2026-10-14", field<List<UpcomingDay>>("upcoming", "days").single().date)
        assertEquals(7, field<List<OverviewProject>>("overview", "projects").single().total)
        val quadrants: Quadrants = field("matrix", "quadrants")
        assertEquals(1, quadrants.now.total)
        assertEquals(0, quadrants.plan.items.size)
        assertEquals(1, quadrants.later.unplanned)
        assertEquals("2026-10-13T10:00:00Z", field<List<FreeSlot>>("free", "slots").single().end)
    }

    @Test fun focusStats() {
        val stats: FocusStats = whole("focus_stats")
        assertEquals(7, stats.days.size)
        assertEquals(8 to 200, stats.tomatoes to stats.minutes)
        assertEquals(1, stats.streak)
        assertNull(stats.projects[1].projectId)
        assertTrue(stats.days[1].projects.isEmpty())
    }
}
