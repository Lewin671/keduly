import { effect,signal } from '@preact/signals';
import { getOccurrences } from '../api/habits';
import { today,user,session,version } from './store';
import { addDays } from '../lib/dates';
import { showToast } from './ui';
import { t } from '../i18n';
export const habitReminderEnabled=signal(false);
let generation=0;
const key=()=>`keduly.habit.reminders.${user.value?.id??''}`;
export async function setHabitReminderEnabled(enabled:boolean):Promise<void>{
 if(enabled&&typeof Notification!=='undefined'&&Notification.permission==='default')await Notification.requestPermission();
 if(enabled&&(typeof Notification==='undefined'||Notification.permission!=='granted')){enabled=false;showToast(t('habit.permissionDenied'));}
 habitReminderEnabled.value=enabled;try{localStorage.setItem(key(),enabled?'on':'off')}catch{/* Private browsing may deny storage. */}
}
effect(()=>{const id=user.value?.id;void id;try{habitReminderEnabled.value=localStorage.getItem(key())==='on'&&typeof Notification!=='undefined'&&Notification.permission==='granted'}catch{habitReminderEnabled.value=false}});
effect(()=>{
 const on=habitReminderEnabled.value,ready=session.value==='ready',id=user.value?.id,day=today.value;void version.value;const g=++generation;
 if(!on||!ready||!id)return;
 let running=false;
 const tick=async()=>{if(running)return;running=true;try{const rows=await getOccurrences(addDays(day,-9),addDays(day,3));if(g!==generation)return;const now=Date.now();for(const o of rows){if(!o.remind_at)continue;const at=Date.parse(o.remind_at);if(at>now||now-at>60_000)continue;const k=`habit.notice.${id}.${o.habit_id}.${o.date}.${o.remind_at}`;if(localStorage.getItem(k))continue;localStorage.setItem(k,'shown');showToast(o.title);if(typeof Notification!=='undefined'&&Notification.permission==='granted')new Notification(t('habit.heading'),{body:o.title,tag:k});}}catch{/* A failed sync must not notify from stale data. */}finally{running=false}};
 void tick();const timer=setInterval(()=>void tick(),30_000);return()=>{generation++;clearInterval(timer)};
});
