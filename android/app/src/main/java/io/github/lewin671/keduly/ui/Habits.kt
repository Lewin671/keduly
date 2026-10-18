package io.github.lewin671.keduly.ui

import android.Manifest
import android.content.Intent
import android.net.Uri
import android.provider.Settings
import java.time.format.DateTimeFormatter
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.Habit
import io.github.lewin671.keduly.api.HabitOccurrence
import kotlinx.coroutines.launch
import kotlinx.serialization.json.*
import java.time.LocalDate
import java.time.Instant
import java.time.LocalDateTime

@Composable
fun HabitsScreen() {
 val app=LocalApp.current
 val tick by app.tick.collectAsStateWithLifecycle()
 val scope=rememberCoroutineScope()
 var habits by remember{mutableStateOf<List<Habit>>(emptyList())}
 var rows by remember{mutableStateOf<List<HabitOccurrence>>(emptyList())}
 var from by remember{mutableStateOf(LocalDate.now(app.zone()).minusDays(6).toString())}
 var edit by remember{mutableStateOf<Habit?>(null)}
 var creating by remember{mutableStateOf(false)}
 var error by remember{mutableStateOf(false)}
 val context=LocalContext.current
 var remindersEnabled by remember{mutableStateOf(HabitNotifications.enabled(context))}
 var permissionDenied by remember{mutableStateOf(false)}
 val request=rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()){granted->HabitNotifications.setEnabled(context,granted);remindersEnabled=granted;permissionDenied=!granted;app.syncHabitNotifications()}
 LaunchedEffect(tick,from){remindersEnabled=HabitNotifications.reconcile(context);permissionDenied=HabitNotifications.denied(context);try{val day=LocalDate.parse(from);habits=app.api.habits();rows=app.api.habitOccurrences(from,day.plusDays(6).toString(),plan=true);error=false}catch(_:Exception){error=true}}
 Screen(stringResource(R.string.habit_heading)) {
  Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(16.dp)) {

   TextButton(onClick={if(HabitNotifications.enabled(context)){HabitNotifications.setEnabled(context,false);remindersEnabled=false}else if(Build.VERSION.SDK_INT>=33){HabitNotifications.markRequested(context);request.launch(Manifest.permission.POST_NOTIFICATIONS)}else{HabitNotifications.setEnabled(context,true);remindersEnabled=true;app.syncHabitNotifications()}}){Text(stringResource(if(remindersEnabled)R.string.habit_disableReminders else R.string.habit_enableReminders))}
   if(Build.VERSION.SDK_INT>=31)TextButton(onClick={context.startActivity(Intent(Settings.ACTION_REQUEST_SCHEDULE_EXACT_ALARM,Uri.parse("package:"+context.packageName)))}){Text(stringResource(R.string.habit_exactSettings))}
   Text(stringResource(R.string.habit_androidLimit),style=MaterialTheme.typography.bodySmall)
   if(permissionDenied)Text(stringResource(R.string.habit_permissionDenied),style=MaterialTheme.typography.bodySmall)
   else if(!HabitNotifications.permitted(context))Text(stringResource(R.string.habit_permissionInvite),style=MaterialTheme.typography.bodySmall)
   if(error)Text(stringResource(R.string.habit_loadFailed),color=MaterialTheme.colorScheme.error)
   habits.forEach { h->
    Row(Modifier.fillMaxWidth().padding(vertical=6.dp)){
     TextButton(onClick={edit=h},modifier=Modifier.weight(1f)){Text(h.title+" · "+h.time)}
     TextButton(onClick={scope.launch{app.attempt{api->api.saveHabit(h.id,buildJsonObject{put("version",h.version);put("paused",!h.paused)})}}}){Text(stringResource(if(h.paused)R.string.habit_resume else R.string.habit_pause))}
    }
   }
   Text(stringResource(R.string.habit_history),style=MaterialTheme.typography.titleMedium)
   OutlinedTextField(from,{from=it},label={Text(stringResource(R.string.habit_weekFrom))})
   Text("${rows.count{it.status=="done"}}/${rows.count{it.status!="future"}}",style=MaterialTheme.typography.bodySmall)
   HabitOccurrenceRows(rows)
  }
 }
 if(creating||edit!=null)HabitEditor(edit){creating=false;edit=null}
}
private fun habitBody(h:Habit?,title:String,days:List<Int>,time:String,duration:String,offset:Boolean,reminder:String,notes:String,project:String):JsonObject=buildJsonObject{
 put("title",title);put("notes",notes);put("project_id",if(project.isEmpty())JsonNull else JsonPrimitive(project))
 put("weekdays",JsonArray(days.map{JsonPrimitive(it)}));put("time",time)
 put("duration_minutes",if(time.isBlank())0 else duration.toInt());put("day_offset",if(offset && time.isNotBlank())1 else 0)
 put("reminder_minutes",if(reminder.isBlank() || time.isBlank())JsonNull else JsonPrimitive(reminder.toInt()));h?.let{put("version",it.version)}
}
@Composable
fun HabitEditor(h:Habit?,close:()->Unit){
 val app=LocalApp.current;val scope=rememberCoroutineScope();val busy by app.busy.collectAsStateWithLifecycle()
 var title by remember{mutableStateOf(h?.title?:"")};var days by remember{mutableStateOf(h?.weekdays?:listOf(1,2,3,4,5,6,7))}
 var time by remember{mutableStateOf(h?.time?:"")};var duration by remember{mutableStateOf((h?.durationMinutes?:0).toString())};var offset by remember{mutableStateOf(h?.dayOffset==1)}
 var reminder by remember{mutableStateOf(h?.reminderMinutes?.toString()?:"")};var notes by remember{mutableStateOf(h?.notes?:"")};var project by remember{mutableStateOf(h?.projectId?:"")};var error by remember{mutableStateOf(false)}
 AlertDialog(onDismissRequest=close,title={Text(stringResource(if(h==null)R.string.habit_new else R.string.habit_rules))},text={Column(Modifier.verticalScroll(rememberScrollState())){
 OutlinedTextField(title,{title=it},label={Text(stringResource(R.string.habit_title))})
 Text(stringResource(R.string.habit_repeat))
 listOf(R.string.habit_day1,R.string.habit_day2,R.string.habit_day3,R.string.habit_day4,R.string.habit_day5,R.string.habit_day6,R.string.habit_day7).forEachIndexed{index,label->Row{Checkbox(index+1 in days,{checked->days=if(checked)days+(index+1)else days.filter{it!=index+1}});Text(stringResource(label))}}
 OutlinedTextField(time,{time=it},label={Text(stringResource(R.string.habit_time))})
 Row{Checkbox(offset,{offset=it});Text(stringResource(R.string.habit_nextDay))}
 OutlinedTextField(duration,{duration=it},label={Text(stringResource(R.string.habit_duration))})
 OutlinedTextField(reminder,{reminder=it},label={Text(stringResource(R.string.habit_reminder)+" (min)")})
 OutlinedTextField(notes,{notes=it},label={Text(stringResource(R.string.habit_notes))})
 val boot by app.boot.collectAsStateWithLifecycle()
 boot?.projects?.forEach{p->Row{RadioButton(project==p.id,{project=if(project==p.id)"" else p.id});Text(p.name)}}
 Text(stringResource(if(h==null)R.string.habit_startsToday else R.string.habit_tomorrow),style=MaterialTheme.typography.bodySmall)
 if(error)Text(stringResource(R.string.habit_saveFailed),color=MaterialTheme.colorScheme.error)
 }},confirmButton={TextButton(enabled=!busy,onClick={scope.launch{try{val body=habitBody(h,title,days,time,duration,offset,reminder,notes,project);when(app.attempt{it.saveHabit(h?.id,body)}){is Outcome.Done->close();else->error=true}}catch(_:Exception){error=true}}}){Text(stringResource(R.string.save))}},dismissButton={TextButton(onClick=close){Text(stringResource(R.string.cancel))}})
}
@Composable
fun HabitOccurrenceRows(rows:List<HabitOccurrence>){
 val app=LocalApp.current;val scope=rememberCoroutineScope();val busy by app.busy.collectAsStateWithLifecycle();var selected by remember{mutableStateOf<HabitOccurrence?>(null)}
 rows.forEach{o->Row(Modifier.fillMaxWidth().padding(vertical=5.dp)){
 Checkbox(o.status=="done",{scope.launch{app.attempt{it.recordHabit(o.habitId,o.date,buildJsonObject{put("status",if(o.status=="done")"open" else "done")})}}},enabled=!busy&&o.status!="future")
 TextButton(onClick={selected=o},modifier=Modifier.weight(1f)){Column{Text(o.title);o.scheduledAt?.let{Text(Instant.parse(it).atZone(app.zone()).format(DateTimeFormatter.ofPattern("MM-dd HH:mm")),style=MaterialTheme.typography.bodySmall)};Text(o.date+" · "+stringResource(when(o.status){"done"->R.string.habit_done;"skipped"->R.string.habit_skipped;"missed"->R.string.habit_missed;"future"->R.string.habit_future;else->R.string.habit_open}),style=MaterialTheme.typography.bodySmall)}}
 }}
 selected?.let{o->HabitOccurrenceEditor(o){selected=null}}
}
@Composable
private fun HabitOccurrenceEditor(o:HabitOccurrence,close:()->Unit){
 val app=LocalApp.current;val scope=rememberCoroutineScope();var move by remember{mutableStateOf(o.scheduledAt?.let{Instant.parse(it).atZone(app.zone()).toLocalDateTime().toString()}?:"")};var actual by remember{mutableStateOf(o.completedAt?.let{Instant.parse(it).atZone(app.zone()).toLocalDateTime().toString()}?:LocalDateTime.now(app.zone()).withSecond(0).withNano(0).toString())};var error by remember{mutableStateOf(false)}
 fun act(kind:String){scope.launch{try{val body=buildJsonObject{if(kind=="move")put("scheduled_at",LocalDateTime.parse(move).atZone(app.zone()).toInstant().toString())else{put("status",kind);if(kind=="done")put("completed_at",LocalDateTime.parse(actual).atZone(app.zone()).toInstant().toString())}};when(app.attempt{it.recordHabit(o.habitId,o.date,body)}){is Outcome.Done->close();else->error=true}}catch(_:Exception){error=true}}}
 AlertDialog(onDismissRequest=close,title={Text(o.title)},text={Column(Modifier.verticalScroll(rememberScrollState())){
 OutlinedTextField(move,{move=it},label={Text(stringResource(R.string.habit_move)+" YYYY-MM-DDTHH:MM")});TextButton(onClick={act("move")}){Text(stringResource(R.string.habit_move))}
 OutlinedTextField(actual,{actual=it},label={Text(stringResource(R.string.habit_actual))})
 TextButton(enabled=o.status!="future",onClick={act("done")}){Text(stringResource(R.string.habit_done))};TextButton(enabled=o.status!="future",onClick={act("skipped")}){Text(stringResource(R.string.habit_skip))};TextButton(onClick={act("open")}){Text(stringResource(R.string.habit_undo))};if(error)Text(stringResource(R.string.habit_saveFailed))
 }},confirmButton={TextButton(onClick=close){Text(stringResource(R.string.cancel))}})
}
