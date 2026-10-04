// The two screens a scanned QR code opens: letting another device in, and being let in.
import type { JSX } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import * as api from '../api/client';
import { ApiError } from '../api/client';
import type { LoginRequest } from '../api/types';
import { t } from '../i18n';
import { link } from '../state/link';
import { enterFresh, errorText, user } from '../state/store';

const gone = (err: unknown) => err instanceof ApiError && err.status === 404;
const close = () => { link.value = null; };

function Gone({ lead }: { lead: string }): JSX.Element {
  return (
    <>
      <h2>{t('link.gone')}</h2>
      <p>{lead}</p>
      <button type="button" class="pbtn go big" onClick={close}>{t('common.ok')}</button>
    </>
  );
}

/** Shown on a signed-in device that scanned the QR code of one asking to sign in. */
export function ApproveLogin({ id }: { id: string }): JSX.Element {
  const [request, setRequest] = useState<LoginRequest | null>(null);
  const [state, setState] = useState<'loading' | 'asking' | 'approved' | 'gone'>('loading');
  const [pin, setPin] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    api.getLoginRequest(id).then(
      found => { if (live) { setRequest(found); setState('asking'); } },
      err => { if (!live) return; if (gone(err)) setState('gone'); else { setError(errorText(err)); setState('asking'); } },
    );
    return () => { live = false; };
  }, [id]);

  const approve = async () => {
    if (busy || pin.length !== 4) return;
    setBusy(true);
    setError(null);
    try {
      await api.approveLoginRequest(id, pin);
      setState('approved');
    } catch (err) {
      if (gone(err)) setState('gone');
      else {
        setPin('');
        setError(err instanceof ApiError && err.status === 400 ? t('approve.wrongPin') : errorText(err));
      }
    } finally {
      setBusy(false);
    }
  };
  const refuse = () => {
    // Whether or not the server still has the request, this device is done with it.
    api.refuseLoginRequest(id).catch(() => {});
    close();
  };

  return (
    <div class="auth over" role="dialog" aria-modal="true" aria-label={t('approve.title')}>
      <form class="auth-card link-card" onSubmit={event => { event.preventDefault(); void approve(); }}>
        {state === 'gone' ? <Gone lead={t('approve.goneLead')} />
          : state === 'approved' ? (
            <>
              <h2>{t('approve.done')}</h2>
              <p>{t('approve.doneLead')}</p>
              <button type="button" class="pbtn go big" onClick={close}>{t('common.done')}</button>
            </>
          ) : (
            <>
              <h2>{t('approve.title')}</h2>
              <p>{t(request?.device ? 'approve.lead' : 'approve.leadUnknown', { device: request?.device ?? '', email: user.value?.email ?? '' })}</p>
              <label class="lbl">{t('approve.pin')}
                <input class="fld pin" required autoFocus inputMode="numeric" autocomplete="off" pattern="[0-9]{4}" maxLength={4} value={pin}
                  onInput={event => setPin(event.currentTarget.value.replace(/\D/g, ''))} />
              </label>
              <p class="warn">{t('approve.warning')}</p>
              {error && <p class="err" role="alert">{error}</p>}
              <button type="submit" class="pbtn go big" disabled={busy || state === 'loading' || pin.length !== 4}>{t('approve.allow')}</button>
              <button type="button" class="tb" onClick={refuse}>{t('approve.refuse')}</button>
            </>
          )}
      </form>
    </div>
  );
}

/** Shown on a device that scanned a sign-in code. Nothing happens until the user confirms. */
export function CodeSignIn({ code }: { code: string }): JSX.Element {
  const [account, setAccount] = useState<{ name: string; email: string } | null>(null);
  const [state, setState] = useState<'loading' | 'asking' | 'gone'>('loading');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const here = user.value;

  useEffect(() => {
    let live = true;
    api.checkLoginCode(code).then(
      found => { if (live) { setAccount(found); setState('asking'); } },
      err => { if (!live) return; if (gone(err)) setState('gone'); else { setError(errorText(err)); setState('asking'); } },
    );
    return () => { live = false; };
  }, [code]);

  const signIn = async () => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      await api.redeemLoginCode(code);
      await enterFresh();
      close();
    } catch (err) {
      if (gone(err)) setState('gone');
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div class="auth over" role="dialog" aria-modal="true" aria-label={t('signin.title')}>
      <form class="auth-card link-card" onSubmit={event => { event.preventDefault(); void signIn(); }}>
        {state === 'gone' ? <Gone lead={t('signin.goneLead')} />
          : account && here?.email === account.email ? (
            <>
              <h2>{t('signin.already')}</h2>
              <p>{account.email}</p>
              <button type="button" class="pbtn go big" onClick={close}>{t('common.ok')}</button>
            </>
          ) : (
            <>
              <h2>{t('signin.title')}</h2>
              <p>{account ? t('signin.lead', account) : t('common.loading')}</p>
              {account && here && <p class="warn">{t('signin.replaces', { email: here.email })}</p>}
              {error && <p class="err" role="alert">{error}</p>}
              <button type="submit" class="pbtn go big" disabled={busy || !account}>{t('auth.login')}</button>
              <button type="button" class="tb" onClick={close}>{t('common.cancel')}</button>
            </>
          )}
      </form>
    </div>
  );
}
