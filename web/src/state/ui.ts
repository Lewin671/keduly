// Transient feedback: the error toast and the bottom HUD that offers an undo or one other action.
import { signal } from '@preact/signals';

export interface Hud {
  text: string;
  undo?: () => void;
  /** What the button says when it is not an undo. */
  label?: string;
}

export const toast = signal<string | null>(null);
export const hud = signal<Hud | null>(null);

let toastTimer: ReturnType<typeof setTimeout> | undefined;
let hudTimer: ReturnType<typeof setTimeout> | undefined;

export function showToast(text: string): void {
  toast.value = text;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => { toast.value = null; }, 5000);
}

export function showHud(text: string, undo?: () => void, label?: string): void {
  hud.value = { text, undo, label };
  clearTimeout(hudTimer);
  hudTimer = setTimeout(() => { hud.value = null; }, 5000);
}

export function hideHud(): void {
  clearTimeout(hudTimer);
  hud.value = null;
}
