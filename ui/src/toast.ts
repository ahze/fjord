import { writable } from 'svelte/store';

// HIG-style toasts: transient overlaid feedback, bottom-center, auto-dismiss,
// optional single action. Errors linger longer.
export type Toast = {
  id: number;
  message: string;
  kind: 'info' | 'success' | 'error';
  actionLabel?: string;
  onAction?: () => void;
};

let nextId = 1;
export const toasts = writable<Toast[]>([]);

export function dismissToast(id: number) {
  toasts.update((all) => all.filter((t) => t.id !== id));
}

export function toast(
  message: string,
  opts: { kind?: Toast['kind']; actionLabel?: string; onAction?: () => void; timeout?: number } = {},
) {
  const t: Toast = { id: nextId++, message, kind: opts.kind ?? 'info', actionLabel: opts.actionLabel, onAction: opts.onAction };
  toasts.update((all) => [...all, t]);
  const ms = opts.timeout ?? (t.kind === 'error' ? 7000 : 4000);
  setTimeout(() => dismissToast(t.id), ms);
  return t.id;
}
