import i18n from 'i18next';
import {initReactI18next} from 'react-i18next';
import pt from './locales/pt-BR.json';
import en from './locales/en.json';
import es from './locales/es.json';
import fr from './locales/fr.json';
import zh from './locales/zh-CN.json';

export const languages = [
  {id:'pt-BR', code:'BR', name:'Português (Brasil)'},
  {id:'en', code:'EN', name:'English'},
  {id:'es', code:'ES', name:'Español'},
  {id:'fr', code:'FR', name:'Français'},
  {id:'zh-CN', code:'ZH', name:'中文 (简体)'},
];
function normalize(value:string):string|undefined {
  const prefix = value.toLowerCase().split('-')[0];
  return ({pt:'pt-BR',en:'en',es:'es',fr:'fr',zh:'zh-CN'} as Record<string,string>)[prefix];
}
function initialLanguage() {
  try {const saved = localStorage.getItem('titan-language'); if (saved && languages.some(language => language.id === saved)) return saved;} catch { /* Keep preference in memory. */ }
  for (const candidate of navigator.languages ?? [navigator.language]) {const locale = normalize(candidate); if (locale) return locale;}
  return 'pt-BR';
}
function applyLanguage(language:string) {
  const locale = normalize(language) ?? 'pt-BR';
  document.documentElement.lang = locale;
  try {localStorage.setItem('titan-language',locale);} catch { /* Storage may be unavailable. */ }
}
void i18n.use(initReactI18next).init({
  resources:{'pt-BR':{translation:pt},en:{translation:en},es:{translation:es},fr:{translation:fr},'zh-CN':{translation:zh}},
  lng:initialLanguage(), fallbackLng:'pt-BR', supportedLngs:languages.map(language => language.id),
  keySeparator:false, interpolation:{escapeValue:false}, react:{useSuspense:false},
});
applyLanguage(i18n.language);
i18n.on('languageChanged',applyLanguage);
export default i18n;
