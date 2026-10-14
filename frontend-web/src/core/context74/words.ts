import pt from '../../i18n/context74/pt-BR.json';import en from '../../i18n/context74/en.json';import es from '../../i18n/context74/es.json';import fr from '../../i18n/context74/fr.json';import zh from '../../i18n/context74/zh-CN.json';
const copy:Record<string,typeof pt>={'pt-BR':pt,pt,en,es,fr,'zh-CN':zh,zh};export const words74=(locale:string)=>copy[locale]||pt;
