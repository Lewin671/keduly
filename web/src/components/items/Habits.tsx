import type { JSX } from 'preact';
import { useEffect,useState } from 'preact/hooks';
import { signal } from '@preact/signals';
import { getHabits,getOccurrences,saveHabit,recordHabit,type Habit,type Occurrence } from '../../api/habits';
import { t } from '../../i18n';
import { addDays,wall,hhmm,ymd,toUtc } from '../../lib/dates';
import { today,user,version,write,activeProjects,reportError } from '../../state/store';
import { navigate } from '../../state/route';
import { habitReminderEnabled,setHabitReminderEnabled } from '../../state/habit-reminders';
export const newHabit=signal(false);
const statusText=(s:Occurrence['status'])=>t(`habit.${s}`);
const stamp=(v:string|null)=>v?`${ymd(wall(v))} ${hhmm(wall(v))}`:'—';
const localValue=(v:string|null)=>v?`${ymd(wall(v))}T${hhmm(wall(v))}`:'';
export function HabitRows({occurrences,expanded=false}:{occurrences:Occurrence[];expanded?:boolean}):JSX.Element {
 const [selected,setSelected]=useState<string|null>(expanded&&occurrences[0]?occurrences[0].habit_id+occurrences[0].date:null);
 const [busy,setBusy]=useState(false);
 const act=async(o:Occurrence,body:Record<string,unknown>)=>{if(busy)return;setBusy(true);try{await write(recordHabit(o.habit_id,o.date,body));}finally{setBusy(false)}};
 return <>{occurrences.map(o=>{const key=o.habit_id+o.date;return <div key={key} class="habit-item">
 <div class="habit-row"><button class={`habit-check ${o.status==='done'?'checked':''}`} disabled={busy||o.status==='future'} aria-label={t(o.status==='done'?'habit.undo':'habit.done')} onClick={()=>void act(o,{status:o.status==='done'?'open':'done'})}>{o.status==='done'?'✓':''}</button><button class="habit-title" onClick={()=>setSelected(selected===key?null:key)}><b>{o.title}</b><small>↻ {statusText(o.status)} · {o.date}{o.completed_at?` · ${stamp(o.completed_at)}`:''}</small></button><time>{o.scheduled_at?stamp(o.scheduled_at):t('habit.untimed')}</time></div>
 {selected===key&&<div class="habit-detail"><p>{o.notes}</p><p>{t('habit.planned')} {stamp(o.scheduled_at)}　{t('habit.actual')} {stamp(o.completed_at)}</p><label>{t('habit.move')}<input type="datetime-local" aria-label={t('habit.move')} defaultValue={localValue(o.scheduled_at)} disabled={busy} onChange={e=>{if(e.currentTarget.value)void act(o,{scheduled_at:toUtc(new Date(e.currentTarget.value))})}} /></label><label>{t('habit.actual')}<input type="datetime-local" aria-label={t('habit.actual')} defaultValue={localValue(o.completed_at)} disabled={busy||o.status==='future'} onChange={e=>{if(e.currentTarget.value)void act(o,{status:'done',completed_at:toUtc(new Date(e.currentTarget.value))})}} /></label><div class="habit-actions"><button disabled={busy||o.status==='future'} onClick={()=>void act(o,{status:'skipped'})}>{t('habit.skip')}</button><button disabled={busy} onClick={()=>void act(o,{status:'open',skip_reason:''})}>{t('habit.undo')}</button><button onClick={()=>navigate({mode:'items',list:'habits'})}>{t('habit.rules')}</button></div></div>}
 </div>})}</>;
}
function HabitForm({habit,onClose}:{habit?:Habit;onClose:()=>void}):JSX.Element{
 const [h,setH]=useState<Partial<Habit>>(habit??{title:'',notes:'',project_id:null,weekdays:[1,2,3,4,5,6,7],time:'',duration_minutes:0,day_offset:0,reminder_minutes:null,paused:false});
 const [busy,setBusy]=useState(false),[error,setError]=useState('');const set=(v:Partial<Habit>)=>setH({...h,...v});
 return <form class="habit-detail" onSubmit={async e=>{e.preventDefault();if(busy)return;setBusy(true);setError('');try{const result=await write(saveHabit(h));if(result)onClose();else setError(t('habit.saveFailed'));}finally{setBusy(false)}}}>
 <label>{t('habit.title')}<input required maxLength={500} value={h.title} onInput={e=>set({title:e.currentTarget.value})}/></label>
 <label>{t('field.project')}<select value={h.project_id??''} onChange={e=>set({project_id:e.currentTarget.value||null})}><option value="">{t('project.none')}</option>{activeProjects.value.map(p=><option value={p.id}>{p.name}</option>)}</select></label>
 <fieldset><legend>{t('habit.repeat')}</legend><div class="habit-days">{t('habit.weekdays').split(',').map((d,i)=><label><input type="checkbox" checked={h.weekdays?.includes(i+1)} onChange={e=>set({weekdays:e.currentTarget.checked?[...(h.weekdays??[]),i+1]:(h.weekdays??[]).filter(x=>x!==i+1)})}/>{d}</label>)}</div></fieldset>
 <label>{t('habit.time')}<input type="time" value={h.time} onChange={e=>set(e.currentTarget.value?{time:e.currentTarget.value}:{time:'',duration_minutes:0,day_offset:0,reminder_minutes:null})}/></label>
 {h.time&&<><label>{t('habit.nextDay')}<input type="checkbox" checked={h.day_offset===1} onChange={e=>set({day_offset:e.currentTarget.checked?1:0})}/></label><label>{t('habit.duration')}<input type="number" min="0" max="720" value={h.duration_minutes} onInput={e=>set({duration_minutes:Number(e.currentTarget.value)})}/></label><label>{t('habit.reminder')}<select value={h.reminder_minutes??'off'} onChange={e=>set({reminder_minutes:e.currentTarget.value==='off'?null:Number(e.currentTarget.value)})}><option value="off">{t('habit.noReminder')}</option>{[0,5,15,30].map(n=><option value={n}>{t('habit.minutesBefore',{n})}</option>)}</select></label></>}
 <label>{t('habit.notes')}<textarea value={h.notes} maxLength={20000} onInput={e=>set({notes:e.currentTarget.value})}/></label>
 <p class="hint">{t(habit?'habit.tomorrow':'habit.startsToday')}</p>{error&&<p role="alert">{error}</p>}<div class="habit-actions"><button type="submit" class="btn primary" disabled={busy}>{t('common.save')}</button><button type="button" onClick={onClose}>{t('common.cancel')}</button></div></form>;
}
export function Habits():JSX.Element{
 const [list,setList]=useState<Habit[]>([]),[records,setRecords]=useState<Occurrence[]>([]),[edit,setEdit]=useState<Habit|null>(null),[failed,setFailed]=useState(false),[loading,setLoading]=useState(true),[from,setFrom]=useState(addDays(today.value,-6));
 const v=version.value,day=today.value,uid=user.value?.id,creating=newHabit.value;
 useEffect(()=>{if(creating)setEdit(null)},[creating]);
 useEffect(()=>{let live=true;setLoading(true);Promise.all([getHabits(),getOccurrences(from,addDays(from,6),true)]).then(([h,o])=>{if(live){setList(h);setRecords(o);setFailed(false)}}).catch(e=>{if(live){setFailed(true);reportError(e)}}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[v,from,day,uid]);
 return <><div class="lh"><h2>{t('habit.heading')}</h2></div><p class="summary">{t('habit.summary')}</p><label class="habit-remind"><input type="checkbox" checked={habitReminderEnabled.value} onChange={e=>void setHabitReminderEnabled(e.currentTarget.checked)}/>{t('habit.webReminder')}</label><p class="hint">{t('habit.webLimit')}</p>
 {(newHabit.value||edit)&&<HabitForm key={edit?.id??'new'} habit={edit??undefined} onClose={()=>{newHabit.value=false;setEdit(null)}}/>}
 {failed?<p role="alert">{t('habit.loadFailed')}</p>:loading?<p>{t('habit.loading')}</p>:list.length===0?<p class="hint">{t('habit.empty')}</p>:list.map(h=><div class="habit-row" key={h.id}><button class="habit-title" onClick={()=>{setEdit(h);newHabit.value=false}}><b>{h.title}</b><small>{h.paused?t('habit.paused'):t('habit.active')} · {h.time||t('habit.untimed')} · {t('habit.effective')} {h.effective_date}</small></button><button class="blue" onClick={()=>void write(saveHabit({...h,paused:!h.paused}))}>{t(h.paused?'habit.resume':'habit.pause')}</button></div>)}
 <div class="sec">{t('habit.history')}</div><label>{t('habit.weekFrom')} <input type="date" value={from} onChange={e=>{if(e.currentTarget.value)setFrom(e.currentTarget.value)}}/></label><p class="hint">{t('habit.rate',{done:records.filter(o=>o.status==='done').length,total:records.filter(o=>o.status!=='future').length,skipped:records.filter(o=>o.status==='skipped').length})}</p><HabitRows occurrences={records}/></>;
}
