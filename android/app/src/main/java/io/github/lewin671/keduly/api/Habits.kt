package io.github.lewin671.keduly.api

import kotlinx.serialization.Serializable
import kotlinx.serialization.SerialName

@Serializable
data class Habit(
 val id:String, val title:String, val notes:String, @SerialName("project_id") val projectId:String?,
 val weekdays:List<Int>, val time:String, @SerialName("duration_minutes") val durationMinutes:Int,
 @SerialName("day_offset") val dayOffset:Int, @SerialName("reminder_minutes") val reminderMinutes:Int?,
 val paused:Boolean, val timezone:String, @SerialName("effective_date") val effectiveDate:String, val version:Int,
)
@Serializable
data class HabitOccurrence(
 @SerialName("plan_date") val planDate:String,
 @SerialName("habit_id") val habitId:String,val date:String,val title:String,val notes:String,
 @SerialName("project_id") val projectId:String?,val timezone:String,val time:String,
 @SerialName("duration_minutes") val durationMinutes:Int,@SerialName("day_offset") val dayOffset:Int,
 @SerialName("scheduled_at") val scheduledAt:String?,@SerialName("end_at") val endAt:String?,
 @SerialName("remind_at") val remindAt:String?,val status:String,
 @SerialName("completed_at") val completedAt:String?,@SerialName("skip_reason") val skipReason:String,
)
