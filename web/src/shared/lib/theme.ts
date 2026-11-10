import { useCallback, useEffect, useState } from 'react';

export type ThemePref = 'light' | 'dark' | 'system';
const KEY = 'mv-theme';

export function readTheme(): ThemePref {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'light' || v === 'dark' ? v : 'system';
  } catch {
    return 'system';
  }
}

export function applyTheme(pref: ThemePref) {
  const root = document.documentElement;
  if (pref === 'system') root.removeAttribute('data-theme');
  else root.setAttribute('data-theme', pref);
}

/** Theme preference persisted in localStorage; "system" follows prefers-color-scheme. */
export function useTheme() {
  const [pref, setPref] = useState<ThemePref>(readTheme);
  useEffect(() => applyTheme(pref), [pref]);
  const set = useCallback((p: ThemePref) => {
    try {
      if (p === 'system') localStorage.removeItem(KEY);
      else localStorage.setItem(KEY, p);
    } catch {
      // storage may be unavailable; the choice still applies for this session
    }
    setPref(p);
  }, []);
  return { pref, setPref: set };
}
