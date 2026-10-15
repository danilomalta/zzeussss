import pt from '../../i18n/operations84/pt-BR.json';
import en from '../../i18n/operations84/en.json';
import es from '../../i18n/operations84/es.json';
import fr from '../../i18n/operations84/fr.json';
import zh from '../../i18n/operations84/zh-CN.json';
export function dictionary84(language:string):Record<string,unknown>{return language.startsWith('en')?en:language.startsWith('es')?es:language.startsWith('fr')?fr:language.startsWith('zh')?zh:pt;}
