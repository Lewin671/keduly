package io.github.lewin671.keduly
import org.junit.Test
import org.junit.Assert.*
import io.github.lewin671.keduly.api.ApiJson
import io.github.lewin671.keduly.api.HabitOccurrence
import kotlinx.serialization.decodeFromString
import java.time.Instant
class HabitsTest {
 @Test fun midnightKeepsLogicalDayAndActualInstant(){
 val o=ApiJson.decodeFromString<HabitOccurrence>("""{"habit_id":"h","plan_date":"2026-10-13","date":"2026-10-13","title":"Bed","notes":"","project_id":null,"timezone":"Asia/Shanghai","time":"00:00","day_offset":1,"duration_minutes":0,"scheduled_at":"2026-10-13T16:00:00Z","end_at":"2026-10-13T16:00:00Z","remind_at":null,"status":"done","completed_at":"2026-10-13T16:12:00Z","skip_reason":""}""")
 assertEquals("2026-10-13",o.date);assertEquals(1,o.dayOffset);assertEquals(720L,Instant.parse(o.completedAt).epochSecond-Instant.parse(o.scheduledAt).epochSecond)
 }
}

class HabitNotificationSelectionTest {
 @Test fun includesRecentDueWithinTheSameBound(){
 val value=ApiJson.decodeFromString<HabitOccurrence>("""{"habit_id":"h","plan_date":"2026-10-13","date":"2026-10-13","title":"Walk","notes":"","project_id":null,"timezone":"UTC","time":"00:00","day_offset":0,"duration_minutes":0,"scheduled_at":"2026-10-13T16:00:00Z","end_at":"2026-10-13T16:00:00Z","remind_at":"2026-10-13T16:00:00Z","status":"open","completed_at":null,"skip_reason":""}""")
 val rows=(0..99).map{value.copy(habitId="h$it")}
 assertEquals(60,io.github.lewin671.keduly.ui.HabitNotifications.select(rows,Instant.parse("2026-10-13T16:00:30Z")).size)
 assertEquals(0,io.github.lewin671.keduly.ui.HabitNotifications.select(rows,Instant.parse("2026-09-28T16:00:00Z")).size)
 assertEquals(0,io.github.lewin671.keduly.ui.HabitNotifications.select(rows,Instant.parse("2026-10-13T16:02:00Z")).size)
 }
}
