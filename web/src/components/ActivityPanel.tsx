// The bell: what is waiting for a decision, and the recent changes, each of which can be undone.
import type { JSX } from 'preact';
import { useEffect, useRef, useState } from 'preact/hooks';
import * as api from '../api/client';
import type { Activity, Actor } from '../api/types';
import { t } from '../i18n';
import { useEscape } from '../lib/keys';
import { stamp } from '../lib/format';
import { track } from '../state/resource';
import { attempt, refresh, reportError, today, version } from '../state/store';
import { acceptAllPlans, decide, pendingDeletions, pendingPlans } from '../state/suggestions';
import { Icon, type IconName } from './Icons';
import { Skeleton } from './items/parts';

const ACTOR_ICON: Record<Actor['kind'], { icon: IconName; color: string }> = {
  user: { icon: 'person', color: 'var(--blue)' },
  agent: { icon: 'term', color: 'var(--indigo)' },
  caldav: { icon: 'phone', color: 'var(--gray)' },
  system: { icon: 'gear', color: 'var(--gray)' },
};

function ActorIcon({ actor }: { actor: Actor }): JSX.Element {
  const { icon, color } = ACTOR_ICON[actor.kind] ?? ACTOR_ICON.system;
  return <span class="ico" style={{ '--c': color }}><Icon name={icon} /></span>;
}

const actorName = (actor: Actor) => (actor.kind === 'user' ? t('activity.you') : actor.name);

interface Log {
  rows: Activity[];
  cursor: string | null;
}

export function ActivityPanel({ onClose }: { onClose: () => void }): JSX.Element {
  const [log, setLog] = useState<Log | undefined>(undefined);
  const [loadingMore, setLoadingMore] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const v = version.value;
  useEscape(onClose);

  useEffect(() => {
    let live = true;
    track(api.listActivity()).then(
      // Older rows already on screen stay; the newest page replaces the top of the list.
      res => {
        if (!live) return;
        setLog(old => {
          if (!old || old.rows.length <= res.activities.length) return { rows: res.activities, cursor: res.next_cursor };
          const ids = new Set(res.activities.map(a => a.id));
          return { rows: [...res.activities, ...old.rows.filter(a => !ids.has(a.id))], cursor: old.cursor };
        });
      },
      err => { if (live) { reportError(err); setLog(old => old ?? { rows: [], cursor: null }); } },
    );
    return () => { live = false; };
  }, [v]);

  useEffect(() => {
    const down = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && !root.current?.contains(target) && !target.closest('[data-panel-toggle]')) onClose();
    };
    document.addEventListener('pointerdown', down, true);
    return () => document.removeEventListener('pointerdown', down, true);
  }, []);

  const older = async () => {
    if (!log?.cursor) return;
    setLoadingMore(true);
    const res = await attempt(api.listActivity(log.cursor));
    setLoadingMore(false);
    if (res) setLog({ rows: [...log.rows, ...res.activities], cursor: res.next_cursor });
  };

  const flip = async (row: Activity) => {
    const changed = await attempt(row.undone ? api.redoActivity(row.id) : api.undoActivity(row.id));
    if (changed) setLog(old => old && { ...old, rows: old.rows.map(a => (a.id === changed.id ? changed : a)) });
    void refresh();
  };

  const deletions = pendingDeletions();
  const plans = pendingPlans();
  const waiting = deletions.length + (plans.length ? 1 : 0);
  const rows = log?.rows ?? [];

  return (
    <div class="panel" ref={root} role="dialog" aria-label={t('activity.title')}>
      {waiting > 0 && <h3>{t('activity.todo')}</h3>}
      {deletions.map((s, k) => (
        <div key={s.id} class={`p-row ${k === waiting - 1 ? 'last' : ''}`}>
          <ActorIcon actor={s.actor} />
          <div class="gb">
            <div class="gm">
              <div>{t('activity.wantsDelete', { title: s.title })}</div>
              <div class="sub">{[s.actor.name, s.reason].filter(Boolean).join(' · ')}</div>
            </div>
            <button class="sb" onClick={() => { void decide(s.id, s.title, false); }}>{t('suggest.reject')}</button>
            <button class="sb go" onClick={() => { void decide(s.id, s.title, true); }}>{t('activity.allow')}</button>
          </div>
        </div>
      ))}
      {plans.length > 0 && (
        <div class="p-row last">
          <span class="ico" style={{ '--c': 'var(--indigo)' }}><Icon name="term" /></span>
          <div class="gb">
            <div class="gm">
              <div>{t('activity.plans', { n: plans.length })}</div>
              <div class="sub">{t('activity.plansTip')}</div>
            </div>
            {plans.length > 1 && <button class="tb" onClick={() => { void acceptAllPlans(); }}>{t('activity.acceptAll')}</button>}
          </div>
        </div>
      )}
      {!log && <Skeleton rows={2} pad />}
      {rows.length > 0 && <h3>{t('activity.recent')}</h3>}
      {rows.map(row => (
        <div key={row.id} class={`p-row ${row.undone ? 'undone' : ''}`}>
          <ActorIcon actor={row.actor} />
          <div class="gb">
            <div class="gm">
              <div>{row.summary}</div>
              <div class="sub">{actorName(row.actor)} · {stamp(row.created_at, today.value)}{row.reason ? ` · ${row.reason}` : ''}</div>
            </div>
            {row.undoable && <button class="tb" onClick={() => { void flip(row); }}>{row.undone ? t('activity.redo') : t('activity.undo')}</button>}
          </div>
        </div>
      ))}
      {loadingMore ? <Skeleton rows={2} pad /> : log?.cursor ? <button class="morebtn pad" onClick={() => { void older(); }}>{t('activity.older')}</button> : null}
      {log && !rows.length && !waiting && (
        <div class="blank" style="padding:36px 20px"><b>{t('activity.empty')}</b><span>{t('activity.emptyTip')}</span></div>
      )}
    </div>
  );
}
