import {useEffect, useState} from 'react';
type Theme = 'light'|'dark';
function readTheme():Theme {
  try {const saved = localStorage.getItem('titansystem-theme'); if (saved === 'light' || saved === 'dark') return saved;} catch { /* Keep preference in memory. */ }
  const current = document.documentElement.dataset.theme;
  if (current === 'light' || current === 'dark') return current;
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}
function applyTheme(theme:Theme) {
  document.documentElement.dataset.theme = theme;
  document.documentElement.style.colorScheme = theme;
  try {localStorage.setItem('titansystem-theme',theme);} catch { /* Keep preference in memory. */ }
  window.dispatchEvent(new CustomEvent('titan-theme',{detail:theme}));
}
export function useTheme() {
  const [theme,setValue] = useState<Theme>(readTheme);
  useEffect(() => {
    const handler = (event:Event) => setValue((event as CustomEvent<Theme>).detail);
    window.addEventListener('titan-theme',handler);
    applyTheme(readTheme());
    return () => window.removeEventListener('titan-theme',handler);
  },[]);
  return {theme,setTheme:applyTheme};
}
