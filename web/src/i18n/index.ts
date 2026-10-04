// All user-visible text goes through t(). Adding a language means adding a file like zh.ts and
// choosing between them here.
import { zh } from './zh';

export type MessageKey = keyof typeof zh;

const messages: Record<MessageKey, string> = zh;

export function hasMessage(key: string): key is MessageKey {
  return Object.hasOwn(messages, key);
}

/** Looks a message up and fills in its `{name}` placeholders. */
export function t(key: MessageKey, vars?: Record<string, string | number>): string {
  const text = messages[key];
  return vars ? text.replace(/\{(\w+)\}/g, (whole, name: string) => (name in vars ? String(vars[name]) : whole)) : text;
}
