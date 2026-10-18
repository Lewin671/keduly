package io.github.lewin671.keduly.ui

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.content.ContextCompat
import io.github.lewin671.keduly.MainActivity
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.HabitOccurrence
import java.time.Instant

/** Device-local schedules are refreshed after successful sync, not claimed as delivered. */
object HabitNotifications {
 private const val prefs="habit_reminders"
 fun select(rows:List<HabitOccurrence>,now:Instant)=rows.filter{it.remindAt!=null&&Instant.parse(it.remindAt)>now.minusSeconds(60)&&!Instant.parse(it.remindAt).isAfter(now.plusSeconds(14L*24*60*60))}.sortedBy{it.remindAt}.take(60)
 fun permitted(c:Context):Boolean = c.getSystemService(NotificationManager::class.java).areNotificationsEnabled() && (Build.VERSION.SDK_INT<33 || ContextCompat.checkSelfPermission(c,Manifest.permission.POST_NOTIFICATIONS)==PackageManager.PERMISSION_GRANTED)
 fun reconcile(c:Context):Boolean {if(!permitted(c)){setEnabled(c,false);return false};return enabled(c)}
 fun requested(c:Context)=c.getSharedPreferences(prefs,0).getBoolean("requested",false)
 fun markRequested(c:Context){c.getSharedPreferences(prefs,0).edit().putBoolean("requested",true).apply()}
 fun denied(c:Context)=!permitted(c)&&(Build.VERSION.SDK_INT<33||requested(c))
 fun enabled(c:Context)=c.getSharedPreferences(prefs,0).getBoolean("enabled",false)
 fun setEnabled(c:Context,value:Boolean){c.getSharedPreferences(prefs,0).edit().putBoolean("enabled",value).apply();if(!value)clear(c)}
 fun clear(c:Context){val p=c.getSharedPreferences(prefs,0);val alarms=c.getSystemService(AlarmManager::class.java);p.getStringSet("ids",emptySet())!!.forEach{id->PendingIntent.getBroadcast(c,0,Intent(c,HabitAlarmReceiver::class.java).setAction(id),PendingIntent.FLAG_NO_CREATE or PendingIntent.FLAG_IMMUTABLE)?.let{alarms.cancel(it);it.cancel()}};p.edit().remove("ids").apply();c.getSystemService(NotificationManager::class.java).cancelAll()}
 fun sync(c:Context,rows:List<HabitOccurrence>){
  if(!reconcile(c))return
  val manager=c.getSystemService(NotificationManager::class.java);manager.createNotificationChannel(NotificationChannel("habits",c.getString(R.string.habit_heading),NotificationManager.IMPORTANCE_DEFAULT))
  val alarms=c.getSystemService(AlarmManager::class.java);val ids=mutableSetOf<String>();val now=Instant.now()
  val p=c.getSharedPreferences(prefs,0)
  val selected=select(rows,now)
  val keep=selected.map{"habit.${it.habitId}.${it.date}"}.toSet()
  p.getStringSet("ids",emptySet())!!.filter{it !in keep}.forEach{id->PendingIntent.getBroadcast(c,0,Intent(c,HabitAlarmReceiver::class.java).setAction(id),PendingIntent.FLAG_NO_CREATE or PendingIntent.FLAG_IMMUTABLE)?.let{alarms.cancel(it);it.cancel()}}
  ids.addAll(p.getStringSet("ids",emptySet())!!.intersect(keep))
  selected.filter{it.remindAt!=null&&Instant.parse(it.remindAt)>now}.forEach{o->
   val id="habit.${o.habitId}.${o.date}";val intent=Intent(c,HabitAlarmReceiver::class.java).setAction(id).putExtra("title",o.title)
   val pending=PendingIntent.getBroadcast(c,0,intent,PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
   val at=Instant.parse(o.remindAt).toEpochMilli()
   try {if(Build.VERSION.SDK_INT<31||alarms.canScheduleExactAlarms())alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP,at,pending)else alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP,at,pending)}catch(_:SecurityException){alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP,at,pending)}
   ids.add(id)
  }
  c.getSharedPreferences(prefs,0).edit().putStringSet("ids",ids).apply()
 }
}
class HabitAlarmReceiver:BroadcastReceiver(){
 override fun onReceive(context:Context,intent:Intent){
  if(!HabitNotifications.enabled(context))return
  if(Build.VERSION.SDK_INT>=33&&ContextCompat.checkSelfPermission(context,Manifest.permission.POST_NOTIFICATIONS)!=PackageManager.PERMISSION_GRANTED)return
  val open=PendingIntent.getActivity(context,0,Intent(context,MainActivity::class.java),PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
  val notice=NotificationCompat.Builder(context,"habits").setSmallIcon(R.drawable.ic_stat_habit).setContentTitle(context.getString(R.string.habit_heading)).setContentText(intent.getStringExtra("title")).setContentIntent(open).setAutoCancel(true).build()
  context.getSystemService(NotificationManager::class.java).notify(intent.action.hashCode(),notice)
 }
}
