// Sign-in and sign-up, shown when there is no session.
import type { JSX } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import * as api from '../api/client';
import { ApiError } from '../api/client';
import { keepComposing } from '../lib/keys';
import { t } from '../i18n';
import { approveUrl } from '../state/link';
import { config, enter, errorText } from '../state/store';
import { Qr } from './Qr';

const POLL_MS = 2000;
/** A little past the two minutes a request lives, in case the server cannot be reached to say so. */
const GIVE_UP_MS = 150_000;

type Started = Awaited<ReturnType<typeof api.startLoginRequest>>;

/** Shows a QR code for a signed-in device to scan, and signs in once that device allows it. */
function QrLogin({ onBack }: { onBack: () => void }): JSX.Element {
  const [started, setStarted] = useState<Started | null>(null);
  const [state, setState] = useState<'loading' | 'waiting' | 'expired' | 'failed'>('loading');
  const [round, setRound] = useState(0);

  useEffect(() => {
    let live = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    setState('loading');
    void (async () => {
      let request: Started;
      try {
        request = await api.startLoginRequest();
      } catch {
        if (live) setState('failed');
        return;
      }
      if (!live) return;
      setStarted(request);
      setState('waiting');
      const since = Date.now();
      const poll = async () => {
        try {
          const answer = await api.claimLoginRequest(request.request.id, request.secret);
          // The session cookie is set, so enter even if this screen was left meanwhile.
          if (answer.status === 'approved') return enter();
        } catch (err) {
          if (err instanceof ApiError && err.status === 404) {
            if (live) setState('expired');
            return;
          }
        }
        if (!live) return;
        if (Date.now() - since > GIVE_UP_MS) setState('expired');
        else timer = setTimeout(() => { void poll(); }, POLL_MS);
      };
      timer = setTimeout(() => { void poll(); }, POLL_MS);
    })();
    return () => { live = false; clearTimeout(timer); };
  }, [round]);

  return (
    <div class="auth-card qr-card">
      <h1>Keduly</h1>
      <p>{t('qr.lead')}</p>
      {state === 'waiting' && started ? (
        <>
          <Qr text={approveUrl(started.request.id)} label={t('qr.label')} />
          <p class="tip">{t('qr.pinLead')}</p>
          <output class="pin-show" aria-label={t('qr.pinLabel')}>{started.pin}</output>
        </>
      ) : (
        <div class="qr blank">
          {state === 'loading' ? <span>{t('common.loading')}</span> : (
            <>
              <span>{t(state === 'expired' ? 'qr.expired' : 'qr.failed')}</span>
              <button type="button" class="sb go" onClick={() => setRound(round + 1)}>{t('qr.refresh')}</button>
            </>
          )}
        </div>
      )}
      <p class="tip">{t('qr.note')}</p>
      <button type="button" class="tb" onClick={onBack}>{t('qr.usePassword')}</button>
    </div>
  );
}

export function Auth({ approving = false }: { approving?: boolean }): JSX.Element {
  const canRegister = config.value?.registration === 'open';
  const [mode, setMode] = useState<'login' | 'register' | 'qr'>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const registering = mode === 'register' && canRegister;

  const submit = async () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      if (registering) {
        await api.register({ email: email.trim(), password, name: name.trim(), timezone: Intl.DateTimeFormat().resolvedOptions().timeZone });
      } else {
        await api.login({ email: email.trim(), password });
      }
      await enter();
    } catch (err) {
      const wrongLogin = err instanceof ApiError && err.status === 401;
      const taken = err instanceof ApiError && err.status === 409;
      setError(wrongLogin ? t('auth.wrong') : taken ? t('auth.taken') : errorText(err));
    } finally {
      setBusy(false);
    }
  };

  if (mode === 'qr' && !approving) return <div class="auth"><QrLogin onBack={() => setMode('login')} /></div>;
  return (
    <div class="auth">
      <form class="auth-card" onKeyDown={keepComposing} onSubmit={event => { event.preventDefault(); void submit(); }}>
        <h1>Keduly</h1>
        <p>{approving ? t('auth.approveLead') : registering ? t('auth.registerLead') : t('auth.loginLead')}</p>
        {registering && (
          <label class="lbl">{t('auth.name')}
            <input class="fld" required autocomplete="name" maxLength={100} value={name} onInput={event => setName(event.currentTarget.value)} />
          </label>
        )}
        <label class="lbl">{t('auth.email')}
          <input class="fld" type="email" required autocomplete="email" autoFocus value={email} onInput={event => setEmail(event.currentTarget.value)} />
        </label>
        <label class="lbl">{t('auth.password')}
          <input class="fld" type="password" required minLength={registering ? 8 : undefined} autocomplete={registering ? 'new-password' : 'current-password'} value={password} onInput={event => setPassword(event.currentTarget.value)} />
        </label>
        {registering && <p class="tip">{t('auth.passwordTip')}</p>}
        {error && <p class="err" role="alert">{error}</p>}
        <button type="submit" class="pbtn go big" disabled={busy}>{registering ? t('auth.register') : t('auth.login')}</button>
        {!registering && !approving && <button type="button" class="tb" onClick={() => setMode('qr')}>{t('auth.toQr')}</button>}
        {canRegister && (
          <button type="button" class="tb" onClick={() => { setMode(registering ? 'login' : 'register'); setError(null); }}>
            {registering ? t('auth.toLogin') : t('auth.toRegister')}
          </button>
        )}
      </form>
    </div>
  );
}
