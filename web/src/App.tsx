import type { JSX } from 'preact';
import { Auth } from './components/Auth';
import { Sprite } from './components/Icons';
import { Skeleton } from './components/items/parts';
import { Shell, Toast } from './components/Shell';
import { session } from './state/store';

export function App(): JSX.Element {
  const state = session.value;
  return (
    <>
      <Sprite />
      {state === 'ready' ? <Shell /> : state === 'anon' ? <Auth /> : <div class="splash"><Skeleton /></div>}
      <Toast />
    </>
  );
}
