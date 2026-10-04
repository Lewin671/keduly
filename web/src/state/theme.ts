// The theme follows the system unless the user forces one in Settings.
import { signal } from '@preact/signals';
import { readStored, writeStored } from '../lib/storage';

export type Theme = 'auto' | 'light' | 'dark';

const KEY = 'keduly.theme';
const stored = readStored(KEY);

export const theme = signal<Theme>(stored === 'light' || stored === 'dark' ? stored : 'auto');

function apply(value: Theme): void {
  if (value === 'auto') delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = value;
}

export function setTheme(value: Theme): void {
  theme.value = value;
  writeStored(KEY, value === 'auto' ? null : value);
  apply(value);
}

export function initTheme(): void {
  apply(theme.value);
}
