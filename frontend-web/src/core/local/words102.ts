import pt from '../../i18n/report102/pt-BR.json';import en from '../../i18n/report102/en.json';import es from '../../i18n/report102/es.json';import fr from '../../i18n/report102/fr.json';import zh from '../../i18n/report102/zh-CN.json';
export function words102(l:string):Record<string,string>{return l.startsWith('en')?en:l.startsWith('es')?es:l.startsWith('fr')?fr:l.startsWith('zh')?zh:pt;}
