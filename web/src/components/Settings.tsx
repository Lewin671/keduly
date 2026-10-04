// The settings sheet: account, agent credentials, system calendar access, the CLI, appearance.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../api/client';
import { ApiError } from '../api/client';
import type { NewToken, Token } from '../api/types';
import { keepComposing } from '../lib/keys';
import { t } from '../i18n';
import { addDays } from '../lib/dates';
import { stamp } from '../lib/format';
import { useResource } from '../state/resource';
import { config, reportError, signOut, today, user, write, keepZone } from '../state/store';
import { setTheme, theme, type Theme } from '../state/theme';
import { showHud, showToast } from '../state/ui';
import { Icon } from './Icons';
import { Dialog } from './Popover';
import { deviceZone } from '../lib/dates';

function CopyButton({ text }: { text: string }): JSX.Element {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      showHud(t('settings.copyFailed'));
    }
  };
  return <button type="button" class="tb" onClick={() => { void copy(); }}>{copied ? t('common.copied') : t('common.copy')}</button>;
}


/** Every zone the browser knows, with the current one included even if the list lacks it. */
function timeZones(current: string): string[] {
  const supported = (Intl as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf?.('timeZone') ?? [];
  return supported.includes(current) ? supported : [current, ...supported];
}

function Account(): JSX.Element | null {
  const me = user.value;
  const [changing, setChanging] = useState(false);
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  if (!me) return null;
  const save = (body: Parameters<typeof api.updateMe>[0]) => { void write(api.updateMe(body)); };
  const changePassword = async () => {
    if (next.length < 8) return;
    try {
      await api.changePassword(current, next);
    } catch (err) {
      // A wrong current password must not be mistaken for a lost session.
      if (err instanceof ApiError && err.status >= 400 && err.status < 500 && err.status !== 429) showToast(t('account.passwordRejected'));
      else reportError(err);
      return;
    }
    setChanging(false);
    setCurrent('');
    setNext('');
    showHud(t('account.passwordChanged'));
  };
  return (
    <div>
      <div class="g-h">{t('account.title')}</div>
      <div class="group">
        <label class="g-row"><div class="gb"><div class="gm">{t('account.name')}</div>
          <input class="fld bare" defaultValue={me.name} maxLength={100} onBlur={event => { const name = event.currentTarget.value.trim(); if (name && name !== me.name) save({ name }); }} />
        </div></label>
        <div class="g-row"><div class="gb"><div class="gm">{t('account.email')}</div><span class="tr sel">{me.email}</span></div></div>
        <label class="g-row"><div class="gb"><div class="gm">{t('account.timezone')}</div>
          <select class="fld bare" aria-label={t('account.timezone')} value={me.timezone_auto ? 'auto' : me.timezone}
            onChange={event => { const v = event.currentTarget.value; if (v !== 'auto') keepZone(); save(v === 'auto' ? { timezone_auto: true, timezone: deviceZone() } : { timezone_auto: false, timezone: v }); }}>
            <option value="auto">{t('account.timezoneAuto', { zone: deviceZone() })}</option>
            {timeZones(me.timezone).map(zone => <option value={zone}>{zone}</option>)}
          </select>
        </div></label>
        <div class="g-row"><div class="gb"><div class="gm">{t('account.hours')}</div>
          <input class="fld bare" type="time" aria-label={t('account.workStart')} defaultValue={me.work_start} onBlur={event => { const v = event.currentTarget.value; if (v && v !== me.work_start) save({ work_start: v }); }} />
          <span>–</span>
          <input class="fld bare" type="time" aria-label={t('account.workEnd')} defaultValue={me.work_end} onBlur={event => { const v = event.currentTarget.value; if (v && v !== me.work_end) save({ work_end: v }); }} />
        </div></div>
        {changing ? (
          <form class="g-form" onSubmit={event => { event.preventDefault(); void changePassword(); }}>
            <input class="fld" type="password" required autocomplete="current-password" placeholder={t('account.currentPassword')} aria-label={t('account.currentPassword')} value={current} onInput={event => setCurrent(event.currentTarget.value)} />
            <input class="fld" type="password" required minLength={8} autocomplete="new-password" placeholder={t('account.newPassword')} aria-label={t('account.newPassword')} value={next} onInput={event => setNext(event.currentTarget.value)} />
            <div class="frow end">
              <button type="button" class="sb" onClick={() => setChanging(false)}>{t('common.cancel')}</button>
              <button type="submit" class="sb go">{t('common.save')}</button>
            </div>
          </form>
        ) : (
          <button class="g-row act" onClick={() => setChanging(true)}><div class="gb">{t('account.changePassword')}</div></button>
        )}
        <button class="g-row act" onClick={() => { void signOut(); }}><div class="gb">{t('account.signOut')}</div></button>
      </div>
    </div>
  );
}

function TokenRow({ token, icon }: { token: Token; icon: 'term' | 'phone' }): JSX.Element {
  const [confirming, setConfirming] = useState(false);
  const scope = token.kind === 'caldav' ? t('token.caldav')
    : token.scope === 'read' ? t('token.read')
    : token.confirm_delete ? t('token.writeConfirm') : t('token.write');
  return (
    <div class="g-row">
      <span class="ico" style={{ '--c': icon === 'term' ? 'var(--indigo)' : 'var(--gray)' }}><Icon name={icon} /></span>
      <div class="gb">
        <div class="gm">
          <div class="nl">{token.name}</div>
          <div class="sub">{scope} · {token.last_used_at ? t('token.lastUsed', { when: stamp(token.last_used_at, today.value) }) : t('token.neverUsed')}</div>
        </div>
        {confirming
          ? <button class="sb danger" onClick={() => { void write(api.deleteToken(token.id)); }}>{t('token.confirmRevoke')}</button>
          : <button class="tb" onClick={() => setConfirming(true)}>{t('token.revoke')}</button>}
      </div>
    </div>
  );
}

/** The secret of a token that was just created. It cannot be shown again. */
function Secret({ token, onDone }: { token: NewToken; onDone: () => void }): JSX.Element {
  return (
    <div class="g-form">
      <div class="sub">{t('token.created', { name: token.name })}</div>
      <code class="secret">{token.token}</code>
      <div class="warn">{t('token.once')}</div>
      <div class="frow end">
        <CopyButton text={token.token} />
        <button type="button" class="sb go" onClick={onDone}>{t('common.done')}</button>
      </div>
    </div>
  );
}

function NewTokenForm({ kind, onClose }: { kind: Token['kind']; onClose: () => void }): JSX.Element {
  const [name, setName] = useState('');
  const [scope, setScope] = useState<Token['scope']>('write');
  const [confirmDelete, setConfirmDelete] = useState(true);
  const [created, setCreated] = useState<NewToken | null>(null);
  if (created) return <Secret token={created} onDone={onClose} />;
  const create = async () => {
    if (!name.trim()) return;
    const body = kind === 'agent' ? { name: name.trim(), kind, scope, confirm_delete: confirmDelete } : { name: name.trim(), kind };
    const token = await write(api.createToken(body));
    if (token) setCreated(token);
  };
  return (
    <form class="g-form" onKeyDown={keepComposing} onSubmit={event => { event.preventDefault(); void create(); }}>
      <input class="fld" required autoFocus maxLength={100} placeholder={kind === 'agent' ? t('token.namePlaceholder') : t('token.devicePlaceholder')} aria-label={t('token.name')} value={name} onInput={event => setName(event.currentTarget.value)} />
      {kind === 'agent' && (
        <>
          <div class="frow">
            <span class="grow">{t('token.scope')}</span>
            <div class="seg small" role="group" aria-label={t('token.scope')}>
              <button type="button" class={scope === 'write' ? 'on' : ''} aria-pressed={scope === 'write'} onClick={() => setScope('write')}>{t('token.write')}</button>
              <button type="button" class={scope === 'read' ? 'on' : ''} aria-pressed={scope === 'read'} onClick={() => setScope('read')}>{t('token.read')}</button>
            </div>
          </div>
          {scope === 'write' && (
            <label class="switch"><span>{t('token.confirmDelete')}</span><input type="checkbox" checked={confirmDelete} onChange={event => setConfirmDelete(event.currentTarget.checked)} /></label>
          )}
        </>
      )}
      <div class="frow end">
        <button type="button" class="sb" onClick={onClose}>{t('common.cancel')}</button>
        <button type="submit" class="sb go">{t('common.create')}</button>
      </div>
    </form>
  );
}

function Tokens({ tokens }: { tokens: Token[] }): JSX.Element {
  const [adding, setAdding] = useState(false);
  return (
    <div>
      <div class="g-h">{t('token.title')}</div>
      <div class="group">
        {tokens.map(token => <TokenRow key={token.id} token={token} icon="term" />)}
        {adding
          ? <NewTokenForm kind="agent" onClose={() => setAdding(false)} />
          : <button class="g-row act" onClick={() => setAdding(true)}><div class="gb">{t('token.new')}</div></button>}
      </div>
      <div class="g-f">{t('token.footer')}</div>
    </div>
  );
}

function SystemCalendar({ tokens }: { tokens: Token[] }): JSX.Element {
  const [adding, setAdding] = useState(false);
  const server = `${location.origin}/dav/`;
  const email = user.value?.email ?? '';
  return (
    <div>
      <div class="g-h">{t('caldav.title')}</div>
      <div class="group">
        <div class="g-row"><div class="gb"><div class="gm">{t('caldav.server')}<div class="sub sel"><code>{server}</code></div></div><CopyButton text={server} /></div></div>
        <div class="g-row"><div class="gb"><div class="gm">{t('caldav.user')}<div class="sub sel">{email}</div></div><CopyButton text={email} /></div></div>
        {tokens.map(token => <TokenRow key={token.id} token={token} icon="phone" />)}
        {adding
          ? <NewTokenForm kind="caldav" onClose={() => setAdding(false)} />
          : <button class="g-row act" onClick={() => setAdding(true)}><div class="gb">{t('caldav.new')}</div></button>}
      </div>
      <div class="g-f">{t('caldav.footer')}</div>
      <ul class="g-f howto">
        <li>{t('caldav.iphone', { host: location.host })}</li>
        <li>{t('caldav.mac', { host: location.host })}</li>
        <li>{t('caldav.android')}</li>
      </ul>
    </div>
  );
}

function Cli(): JSX.Element {
  const lines: Array<[comment: string, ...commands: string[]]> = [
    [t('cli.login'), `keduly login --server ${location.origin}`],
    [t('cli.look'), 'keduly agenda --today --json', `keduly free --date ${addDays(today.value, 1)} --duration 60m`],
    [t('cli.add'), t('cli.addCommand')],
    [t('cli.plan'), 'keduly plan --today', 'keduly undo'],
  ];
  return (
    <div>
      <div class="g-h">{t('cli.title')}</div>
      <div class="group">
        <pre class="cli">
          {lines.map(([comment, ...commands]) => <><span class="c"># {comment}</span>{`\n${commands.join('\n')}\n`}</>)}
        </pre>
      </div>
    </div>
  );
}

const THEMES: ReadonlyArray<[Theme, 'theme.auto' | 'theme.light' | 'theme.dark']> = [['auto', 'theme.auto'], ['light', 'theme.light'], ['dark', 'theme.dark']];

function Appearance(): JSX.Element {
  return (
    <div>
      <div class="g-h">{t('theme.title')}</div>
      <div class="group">
        <div class="g-row"><div class="gb">
          <div class="gm">{t('theme.mode')}</div>
          <div class="seg" role="group" aria-label={t('theme.mode')}>
            {THEMES.map(([value, label]) => (
              <button class={theme.value === value ? 'on' : ''} aria-pressed={theme.value === value} onClick={() => setTheme(value)}>{t(label)}</button>
            ))}
          </div>
        </div></div>
      </div>
    </div>
  );
}

export function Settings({ onClose }: { onClose: () => void }): JSX.Element {
  const tokens = useResource('tokens', api.listTokens).data ?? [];
  return (
    <Dialog onClose={onClose} label={t('settings.title')}>
      <div class="sheet-h"><b>{t('settings.title')}</b><button onClick={onClose}>{t('common.done')}</button></div>
      <div class="groups">
        <Account />
        <Tokens tokens={tokens.filter(x => x.kind === 'agent')} />
        <SystemCalendar tokens={tokens.filter(x => x.kind === 'caldav')} />
        <Cli />
        <Appearance />
      </div>
      {config.value && <p class="end">Keduly {config.value.version}</p>}
    </Dialog>
  );
}
