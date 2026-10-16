import pt from '../../i18n/business85/pt-BR.json';
import en from '../../i18n/business85/en.json';
import es from '../../i18n/business85/es.json';
import fr from '../../i18n/business85/fr.json';
import zh from '../../i18n/business85/zh-CN.json';
export function dictionary85(language:string):Record<string,unknown>{return language.startsWith('en')?en:language.startsWith('es')?es:language.startsWith('fr')?fr:language.startsWith('zh')?zh:pt;}
