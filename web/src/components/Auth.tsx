// Sign-in and sign-up, shown when there is no session.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../api/client';
import { ApiError } from '../api/client';
import { keepComposing } from '../lib/keys';
import { t } from '../i18n';
import { config, enter, errorText } from '../state/store';

export function Auth(): JSX.Element {
  const canRegister = config.value?.registration === 'open';
  const [mode, setMode] = useState<'login' | 'register'>('login');
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

  return (
    <div class="auth">
      <form class="auth-card" onKeyDown={keepComposing} onSubmit={event => { event.preventDefault(); void submit(); }}>
        <h1>Keduly</h1>
        <p>{registering ? t('auth.registerLead') : t('auth.loginLead')}</p>
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
        {canRegister && (
          <button type="button" class="tb" onClick={() => { setMode(registering ? 'login' : 'register'); setError(null); }}>
            {registering ? t('auth.toLogin') : t('auth.toRegister')}
          </button>
        )}
      </form>
    </div>
  );
}
