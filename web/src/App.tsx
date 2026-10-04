import type { JSX } from 'preact';
import { Auth } from './components/Auth';
import { Sprite } from './components/Icons';
import { Skeleton } from './components/items/parts';
import { Shell, Toast } from './components/Shell';
import { ApproveLogin, CodeSignIn } from './components/SignInLink';
import { link } from './state/link';
import { session } from './state/store';

export function App(): JSX.Element {
  const state = session.value;
  const scanned = link.value;
  return (
    <>
      <Sprite />
      {state === 'ready' ? <Shell /> : state === 'anon' ? <Auth approving={scanned?.kind === 'approve'} /> : <div class="splash"><Skeleton /></div>}
      {scanned?.kind === 'signin' ? <CodeSignIn code={scanned.code} />
        : scanned && state === 'ready' ? <ApproveLogin id={scanned.id} /> : null}
      <Toast />
    </>
  );
}
