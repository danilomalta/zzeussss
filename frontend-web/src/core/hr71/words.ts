import pt from '../../i18n/hr71/pt-BR.json';
import en from '../../i18n/hr71/en.json';
import es from '../../i18n/hr71/es.json';
import fr from '../../i18n/hr71/fr.json';
import zh from '../../i18n/hr71/zh-CN.json';
const dictionaries: Record<string, typeof pt> = { 'pt-BR': pt, pt, en, es, fr, 'zh-CN': zh, zh };
export function wordsFor(locale: string) {
  return dictionaries[locale] || pt;
}
